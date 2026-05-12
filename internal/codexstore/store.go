package codexstore

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Conversation struct {
	SessionID    string
	FullPath     string
	FirstPrompt  string
	Cwd          string
	MessageCount int
	Created      time.Time
	Modified     time.Time
	Model        string
	CliVersion   string
	FileSize     int64

	CustomName string
	Tags       []string
}

func (c Conversation) DisplayName() string {
	if c.CustomName != "" {
		return c.CustomName
	}
	return c.FirstPrompt
}

type sessionMetaPayload struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Cwd        string `json:"cwd"`
	CliVersion string `json:"cli_version"`
	Git        struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

type turnContextPayload struct {
	Model string `json:"model"`
}

type responseItemPayload struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content json.RawMessage `json:"content"`
}

type jsonlLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

func CodexDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func LoadConversations() ([]Conversation, error) {
	sessionsDir := filepath.Join(CodexDir(), "sessions")
	if _, err := os.Stat(sessionsDir); err != nil {
		return nil, err
	}

	meta, _ := LoadMeta()
	if meta.Sessions == nil {
		meta.Sessions = map[string]SessionMeta{}
	}

	var files []string
	filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})

	var conversations []Conversation
	for _, f := range files {
		conv, err := parseCodexJSONL(f)
		if err != nil {
			continue
		}
		if sm, ok := meta.Sessions[conv.SessionID]; ok {
			conv.CustomName = sm.CustomName
			conv.Tags = sm.Tags
		}
		conversations = append(conversations, conv)
	}

	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].Modified.After(conversations[j].Modified)
	})

	return conversations, nil
}

func parseCodexJSONL(path string) (Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return Conversation{}, err
	}
	defer f.Close()

	var sessionID, cwd, model, cliVersion, firstPrompt string
	var created, modified time.Time
	var messageCount int
	var gotFirstPrompt bool

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		var line jsonlLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}

		// Parse timestamp for modified time
		if line.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339, line.Timestamp)
			if err == nil {
				if created.IsZero() {
					created = ts
				}
				modified = ts
			}
		}

		switch line.Type {
		case "session_meta":
			var p sessionMetaPayload
			if json.Unmarshal(line.Payload, &p) == nil {
				sessionID = p.ID
				cwd = p.Cwd
				cliVersion = p.CliVersion
				if p.Timestamp != "" {
					ts, err := time.Parse(time.RFC3339, p.Timestamp)
					if err == nil {
						created = ts
					}
				}
			}

		case "turn_context":
			var p turnContextPayload
			if json.Unmarshal(line.Payload, &p) == nil && p.Model != "" {
				model = p.Model
			}

		case "response_item":
			var p responseItemPayload
			if json.Unmarshal(line.Payload, &p) != nil {
				continue
			}
			if p.Role == "user" && p.Type == "message" && !gotFirstPrompt {
				text := extractUserText(p.Content)
				if text != "" && !strings.HasPrefix(text, "<environment") {
					firstPrompt = text
					gotFirstPrompt = true
				}
			}
			if p.Role == "user" || p.Role == "assistant" {
				messageCount++
			}
		}
	}

	if modified.IsZero() {
		if info, err := os.Stat(path); err == nil {
			modified = info.ModTime()
		}
	}

	var fileSize int64
	if info, err := os.Stat(path); err == nil {
		fileSize = info.Size()
	}

	// Extract session ID from filename as fallback
	if sessionID == "" {
		base := filepath.Base(path)
		parts := strings.SplitN(base, "-", 3)
		if len(parts) >= 2 {
			sessionID = strings.TrimSuffix(base, ".jsonl")
		}
	}

	firstPrompt = strings.ReplaceAll(firstPrompt, "\n", " ")
	firstPrompt = strings.ReplaceAll(firstPrompt, "\r", " ")

	return Conversation{
		SessionID:    sessionID,
		FullPath:     path,
		FirstPrompt:  truncateStr(firstPrompt, 200),
		Cwd:          cwd,
		MessageCount: messageCount,
		Created:      created,
		Modified:     modified,
		Model:        model,
		CliVersion:   cliVersion,
		FileSize:     fileSize,
	}, nil
}

func extractUserText(raw json.RawMessage) string {
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	for _, b := range blocks {
		if b.Type == "input_text" && len(strings.TrimSpace(b.Text)) > 0 {
			return b.Text
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

func Delete(conversations []Conversation) error {
	for _, c := range conversations {
		os.Remove(c.FullPath)
	}

	meta, err := LoadMeta()
	if err != nil {
		return nil
	}
	for _, c := range conversations {
		delete(meta.Sessions, c.SessionID)
	}
	return SaveMeta(meta)
}
