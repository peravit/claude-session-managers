package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Conversation represents a single Claude Code session.
type Conversation struct {
	SessionID    string
	FullPath     string    // path to .jsonl file
	FirstPrompt  string
	Summary      string
	MessageCount int
	Created      time.Time
	Modified     time.Time
	GitBranch    string
	ProjectPath  string    // from sessions-index.json; decoded only as fallback
	FolderName   string    // raw encoded folder name
	IndexPath    string    // path to parent sessions-index.json
	FileSize     int64     // .jsonl file size in bytes

	// Populated by merging with meta store after load.
	CustomName string
	Tags       []string
}

// DisplayName returns CustomName if set, otherwise falls back to Summary,
// then FirstPrompt.
func (c Conversation) DisplayName() string {
	if c.CustomName != "" {
		return c.CustomName
	}
	if c.Summary != "" {
		return c.Summary
	}
	return c.FirstPrompt
}

// indexEntry mirrors the JSON structure of sessions-index.json entries.
type indexEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	FirstPrompt  string `json:"firstPrompt"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
	GitBranch    string `json:"gitBranch"`
	ProjectPath  string `json:"projectPath"`
}

type sessionsIndex struct {
	Entries []indexEntry `json:"entries"`
}

// jsonlLine is a partial decode of a single line in a .jsonl session file.
// We only extract the fields we need for display.
type jsonlLine struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId"`
	Timestamp  string `json:"timestamp"`
	GitBranch  string `json:"gitBranch"`
	IsMeta     *bool  `json:"isMeta"`
	LastPrompt string `json:"lastPrompt"`
	Message    struct {
		Role    string `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// ClaudeDir returns the Claude configuration directory.
// On macOS it prefers ~/Library/Application Support/Claude when it exists,
// otherwise falls back to ~/.claude (Linux / WSL).
func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		mac := filepath.Join(home, "Library", "Application Support", "Claude")
		if info, err := os.Stat(mac); err == nil && info.IsDir() {
			return mac
		}
	}
	return filepath.Join(home, ".claude")
}

// LoadConversations reads all sessions-index.json files and falls back to
// scanning .jsonl files directly when no index exists. Merges metadata and
// returns conversations sorted by Modified descending.
func LoadConversations() ([]Conversation, error) {
	projectsDir := filepath.Join(ClaudeDir(), "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	meta, err := LoadMeta()
	if err != nil {
		meta = Meta{Sessions: map[string]SessionMeta{}}
	}

	var conversations []Conversation
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDir := filepath.Join(projectsDir, entry.Name())
		indexPath := filepath.Join(projectDir, "sessions-index.json")

		var convs []Conversation
		if _, err := os.Stat(indexPath); err == nil {
			convs, _ = loadFromIndex(indexPath, entry.Name())
		}
		if len(convs) == 0 {
			convs, _ = loadFromJSONLDir(projectDir, entry.Name())
		}
		conversations = append(conversations, convs...)
	}

	// Merge user metadata
	for i := range conversations {
		if sm, ok := meta.Sessions[conversations[i].SessionID]; ok {
			conversations[i].CustomName = sm.CustomName
			conversations[i].Tags = sm.Tags
		}
	}

	// Sort by Modified descending (most recent first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].Modified.After(conversations[j].Modified)
	})

	return conversations, nil
}

func loadFromIndex(indexPath, folderName string) ([]Conversation, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var idx sessionsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	var convs []Conversation
	for _, e := range idx.Entries {
		projectPath := e.ProjectPath
		if projectPath == "" {
			projectPath = DecodePath(folderName)
		}

		created, _ := time.Parse(time.RFC3339, e.Created)
		modified, _ := time.Parse(time.RFC3339, e.Modified)

		var fileSize int64
		if info, err := os.Stat(e.FullPath); err == nil {
			fileSize = info.Size()
		}

		convs = append(convs, Conversation{
			SessionID:    e.SessionID,
			FullPath:     e.FullPath,
			FirstPrompt:  e.FirstPrompt,
			Summary:      e.Summary,
			MessageCount: e.MessageCount,
			Created:      created,
			Modified:     modified,
			GitBranch:    e.GitBranch,
			ProjectPath:  projectPath,
			FolderName:   folderName,
			IndexPath:    indexPath,
			FileSize:     fileSize,
		})
	}

	return convs, nil
}

// loadFromJSONLDir scans a project directory for .jsonl files and extracts
// metadata from each file directly. Used as fallback when sessions-index.json
// does not exist.
func loadFromJSONLDir(projectDir, folderName string) ([]Conversation, error) {
	files, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	projectPath := DecodePath(folderName)

	var convs []Conversation
	for _, f := range files {
		conv, err := parseJSONLFile(f, folderName, projectPath)
		if err != nil {
			continue
		}
		convs = append(convs, conv)
	}
	return convs, nil
}

// parseJSONLFile reads a .jsonl session file and extracts conversation metadata.
func parseJSONLFile(path, folderName, projectPath string) (Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return Conversation{}, err
	}
	defer f.Close()

	var firstTimestamp, lastTimestamp time.Time
	var firstPrompt, gitBranch, sessionID string
	var messageCount int

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1MB (some messages can be large)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		var line jsonlLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		// Parse timestamp
		if line.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339, line.Timestamp)
			if err == nil {
				if firstTimestamp.IsZero() {
					firstTimestamp = ts
				}
				lastTimestamp = ts
			}
		}

		// Extract session-level metadata from first line that has it
		if sessionID == "" && line.SessionID != "" {
			sessionID = line.SessionID
		}
		if gitBranch == "" && line.GitBranch != "" {
			gitBranch = line.GitBranch
		}

		// Use last-prompt if available (most accurate)
		if line.Type == "last-prompt" && line.LastPrompt != "" {
			firstPrompt = line.LastPrompt
		}

		// Count real user messages (skip meta/system messages)
		if line.Type == "user" {
			isMeta := line.IsMeta != nil && *line.IsMeta
			if !isMeta {
				messageCount++
				// Fallback: extract first prompt from first real user message
				if firstPrompt == "" {
					firstPrompt = extractTextContent(line.Message.Content)
				}
			}
		}
		if line.Type == "assistant" {
			messageCount++
		}
	}

	// Override modified time with file mtime if no timestamp found
	modTime := lastTimestamp
	if modTime.IsZero() {
		if info, err := os.Stat(path); err == nil {
			modTime = info.ModTime()
		}
	}

	var fileSize int64
	if info, err := os.Stat(path); err == nil {
		fileSize = info.Size()
	}

	// IndexPath points to the .jsonl itself when loaded from file scan
	// (no sessions-index.json exists for this project)
	return Conversation{
		SessionID:    sessionID,
		FullPath:     path,
		FirstPrompt:  truncateStr(firstPrompt, 200),
		MessageCount: messageCount,
		Created:      firstTimestamp,
		Modified:     modTime,
		GitBranch:    gitBranch,
		ProjectPath:  projectPath,
		FolderName:   folderName,
		IndexPath:    path, // delete will just remove the .jsonl
		FileSize:     fileSize,
	}, nil
}

// extractTextContent tries to extract plain text from a message content field.
// Content can be a string or an array of content blocks.
func extractTextContent(raw json.RawMessage) string {
	// Try as plain string first
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		// Skip command invocations
		if strings.Contains(s, "command-name") {
			return ""
		}
		return s
	}

	// Try as array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && len(strings.TrimSpace(b.Text)) > 0 {
				return b.Text
			}
		}
	}

	return ""
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Delete removes the given conversations from their index files and deletes
// their .jsonl files. Also removes any stored metadata for deleted sessions.
func Delete(conversations []Conversation) error {
	// Group by index file so we do one read-modify-write per file.
	byIndex := map[string][]Conversation{}
	for _, c := range conversations {
		byIndex[c.IndexPath] = append(byIndex[c.IndexPath], c)
	}

	for indexPath, toDelete := range byIndex {
		// Check if indexPath points to a sessions-index.json or a .jsonl file
		if filepath.Ext(indexPath) == ".jsonl" {
			// No index — just delete the files directly
			continue
		}
		if err := deleteFromIndex(indexPath, toDelete); err != nil {
			return err
		}
	}

	// Delete .jsonl files (best-effort — don't fail if already gone).
	for _, c := range conversations {
		os.Remove(c.FullPath)
	}

	// Clean up metadata.
	meta, err := LoadMeta()
	if err != nil {
		return nil
	}
	for _, c := range conversations {
		meta.Delete(c.SessionID)
	}
	return SaveMeta(meta)
}

func deleteFromIndex(indexPath string, toDelete []Conversation) error {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var idx sessionsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}

	deleteSet := make(map[string]bool, len(toDelete))
	for _, c := range toDelete {
		deleteSet[c.SessionID] = true
	}

	kept := idx.Entries[:0]
	for _, e := range idx.Entries {
		if !deleteSet[e.SessionID] {
			kept = append(kept, e)
		}
	}
	idx.Entries = kept

	return atomicWriteJSON(indexPath, idx)
}

// atomicWriteJSON marshals v to JSON and writes it to path via a temp file +
// rename, so the file is never partially written.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
