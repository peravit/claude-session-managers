package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// overrideClaudeDir temporarily redirects ClaudeDir() to a temp directory
// by pointing the home to a controlled location for the duration of the test.
// We do this by writing the meta file directly to a known temp path and
// swapping the metaPath function pointer is not possible, so instead we test
// LoadMeta / SaveMeta via a helper that accepts an explicit path.

func TestLoadMeta_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "claude-manager-meta.json")

	m, err := loadMetaFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Sessions == nil {
		t.Fatal("Sessions map should be initialized, got nil")
	}
	if len(m.Sessions) != 0 {
		t.Fatalf("expected empty Sessions, got %v", m.Sessions)
	}
}

func TestSaveAndLoadMeta_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "claude-manager-meta.json")

	m := Meta{Sessions: map[string]SessionMeta{
		"session-1": {CustomName: "My Project", Tags: []string{"glm", "work"}},
		"session-2": {Tags: []string{"claude"}},
	}}

	if err := saveMetaTo(path, m); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := loadMetaFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !reflect.DeepEqual(m, got) {
		t.Errorf("roundtrip mismatch:\n  want %+v\n  got  %+v", m, got)
	}
}

func TestMeta_DeleteEntry(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "claude-manager-meta.json")

	m := Meta{Sessions: map[string]SessionMeta{
		"session-1": {CustomName: "keep"},
		"session-2": {CustomName: "delete me"},
	}}
	if err := saveMetaTo(path, m); err != nil {
		t.Fatal(err)
	}

	loaded, _ := loadMetaFrom(path)
	loaded.Delete("session-2")
	if err := saveMetaTo(path, loaded); err != nil {
		t.Fatal(err)
	}

	final, _ := loadMetaFrom(path)
	if _, ok := final.Sessions["session-2"]; ok {
		t.Error("session-2 should have been deleted")
	}
	if final.Sessions["session-1"].CustomName != "keep" {
		t.Error("session-1 should still exist")
	}
}

func TestMeta_SetEntry(t *testing.T) {
	m := Meta{Sessions: map[string]SessionMeta{}}
	m.Set("s1", SessionMeta{CustomName: "hello", Tags: []string{"glm"}})

	if m.Sessions["s1"].CustomName != "hello" {
		t.Error("Set did not store CustomName")
	}
}

// loadMetaFrom and saveMetaTo are path-explicit helpers used only in tests.
// They duplicate the logic of LoadMeta/SaveMeta with a controlled path.

func loadMetaFrom(path string) (Meta, error) {
	data, err := os.ReadFile(path)
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

func saveMetaTo(path string, m Meta) error {
	return atomicWriteJSON(path, m)
}
