package codexstore

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Meta struct {
	Sessions map[string]SessionMeta `json:"sessions"`
}

type SessionMeta struct {
	CustomName string   `json:"customName,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func metaPath() string {
	return filepath.Join(CodexDir(), "codex-manager-meta.json")
}

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

func SaveMeta(m Meta) error {
	return atomicWriteJSON(metaPath(), m)
}

func (m *Meta) Set(sessionID string, sm SessionMeta) {
	m.Sessions[sessionID] = sm
}

func (m *Meta) Delete(sessionID string) {
	delete(m.Sessions, sessionID)
}

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
