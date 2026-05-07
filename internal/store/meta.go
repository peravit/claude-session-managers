package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Meta holds all user-managed metadata for conversations.
// Stored in ~/.claude/claude-manager-meta.json, never touched by Claude Code.
type Meta struct {
	Sessions map[string]SessionMeta `json:"sessions"`
}

// SessionMeta is the user-editable annotation for a single conversation.
type SessionMeta struct {
	CustomName string   `json:"customName,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func metaPath() string {
	return filepath.Join(ClaudeDir(), "claude-manager-meta.json")
}

// LoadMeta reads the meta file. Returns an empty Meta (not an error) when the
// file does not exist yet.
func LoadMeta() (Meta, error) {
	data, err := os.ReadFile(metaPath())
	if os.IsNotExist(err) {
		return Meta{Sessions: map[string]SessionMeta{}}, nil
	}
	if err != nil {
		return Meta{}, err
	}

	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, err
	}
	if m.Sessions == nil {
		m.Sessions = map[string]SessionMeta{}
	}
	return m, nil
}

// SaveMeta writes the meta file atomically.
func SaveMeta(m Meta) error {
	return atomicWriteJSON(metaPath(), m)
}

// Set stores or replaces metadata for a session.
func (m *Meta) Set(sessionID string, sm SessionMeta) {
	m.Sessions[sessionID] = sm
}

// Delete removes metadata for a session.
func (m *Meta) Delete(sessionID string) {
	delete(m.Sessions, sessionID)
}
