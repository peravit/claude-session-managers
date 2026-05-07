package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodePath_Empty(t *testing.T) {
	if got := DecodePath(""); got != "" {
		t.Errorf("DecodePath(\"\") = %q, want \"\"", got)
	}
}

func TestDecodePath_Fallback_Simple(t *testing.T) {
	// No matching dirs on disk → fallback: treat every '-' as separator.
	got := DecodePath("-home-user-projects")
	want := "/home/user/projects"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecodePath_Fallback_SingleSegment(t *testing.T) {
	got := DecodePath("-tmp")
	// /tmp exists on most systems, so it may return "/tmp" via filesystem hit.
	// Either way, must start with "/"
	if !strings.HasPrefix(got, "/") {
		t.Errorf("got %q, want path starting with /", got)
	}
}

func TestDecodePath_FilesystemValidation_Simple(t *testing.T) {
	tmp := t.TempDir()

	// Create: <tmp>/mine/proj
	dir := filepath.Join(tmp, "mine", "proj")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Encode the path manually: replace "/" with "-", strip leading "-"
	// tmp looks like /tmp/TestXxx123/001, so encoded starts with "-tmp-..."
	encoded := strings.ReplaceAll(tmp, "/", "-") + "-mine-proj"

	got := DecodePath(encoded)
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestDecodePath_FilesystemValidation_HyphenatedDirName(t *testing.T) {
	tmp := t.TempDir()

	// Create: <tmp>/projects/2025-Airflow3  (hyphen in dir name)
	dir := filepath.Join(tmp, "projects", "2025-Airflow3")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	encoded := strings.ReplaceAll(tmp, "/", "-") + "-projects-2025-Airflow3"

	got := DecodePath(encoded)
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestDecodePath_FilesystemValidation_PrefersDeepest(t *testing.T) {
	tmp := t.TempDir()

	// Create both a shallow and a deeper path that share a prefix:
	//   <tmp>/a/b
	//   <tmp>/a-b          (hyphenated dir name at the same level)
	shallow := filepath.Join(tmp, "a-b")
	deep := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(shallow, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	encoded := strings.ReplaceAll(tmp, "/", "-") + "-a-b"

	got := DecodePath(encoded)
	// Both exist; we want the deepest (most specific) path.
	if got != deep {
		t.Errorf("got %q, want deepest path %q", got, deep)
	}
}
