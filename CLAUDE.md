# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`claude-manager` is a Go TUI tool for managing Claude Code conversations stored in `~/.claude`. It lets users browse, search, delete, and annotate conversation sessions without manual file operations.

## Tech Stack

- **Language**: Go 1.22+
- **TUI**: Bubble Tea + Lipgloss (charmbracelet stack)
- **List rendering**: Custom (not `bubbles/list`) — needed for multi-select + tag display
- **Text input**: `bubbles/textinput` for search and rename fields
- **Data source**: `~/.claude/projects/*/sessions-index.json`
- **Metadata store**: `~/.claude/claude-manager-meta.json` (owned by this tool only)

## Project Structure

```
claude-manager/
├── cmd/claude-manager/
│   └── main.go                  # entry point — wires store + UI
├── internal/
│   ├── store/
│   │   ├── store.go             # load/delete conversations, atomic writes
│   │   ├── meta.go              # read/write claude-manager-meta.json
│   │   └── decode.go            # DecodePath logic + tests
│   └── ui/
│       ├── app.go               # root Bubble Tea model, state machine
│       ├── list.go              # custom list renderer
│       └── keys.go              # keybinding definitions
├── internal/store/decode_test.go
├── go.mod
├── Makefile
└── README.md
```

## Build & Run

```bash
make build          # go build ./cmd/claude-manager
make install        # go install ./cmd/claude-manager
./claude-manager    # run directly
```

For releases (requires goreleaser):
```bash
make release        # builds Mac + Linux binaries
```

## Architecture

### Data Layer

#### `internal/store/store.go`

```go
type Conversation struct {
    SessionID    string
    FullPath     string    // path to .jsonl file
    FirstPrompt  string
    Summary      string
    MessageCount int
    Created      time.Time
    Modified     time.Time
    GitBranch    string
    ProjectPath  string    // from sessions-index.json "projectPath" field directly — no decoding needed
    FolderName   string    // raw encoded folder name (kept for grouping only)
    IndexPath    string    // path to parent sessions-index.json

    // populated by merging with meta store after load
    CustomName   string
    Tags         []string
}

func ClaudeDir() string                              // ~/.claude on Mac/Linux/WSL
func LoadConversations() ([]Conversation, error)     // reads all sessions-index.json, merges meta
func Delete(conversations []Conversation) error      // atomic delete: .jsonl + index entry + meta entry
```

**Atomic write rule**: never write directly to `sessions-index.json`. Always write to a `.tmp` sibling file then `os.Rename` into place. This is atomic on all supported platforms and prevents corruption if the process is killed mid-write.

**sessions-index.json is owned by Claude Code** — only remove entries, never add or modify existing fields.

#### `internal/store/meta.go`

Owns `~/.claude/claude-manager-meta.json`. This file is separate from Claude Code's files so it is never overwritten by Claude Code.

```go
type Meta struct {
    Sessions map[string]SessionMeta `json:"sessions"` // key = sessionId
}

type SessionMeta struct {
    CustomName string   `json:"customName,omitempty"`
    Tags       []string `json:"tags,omitempty"`
}

func LoadMeta() (Meta, error)
func SaveMeta(m Meta) error          // atomic write via .tmp + rename
func (m *Meta) Set(sessionID string, sm SessionMeta)
func (m *Meta) Delete(sessionID string)
```

Tags are free-form strings. Recommended convention: `"claude"`, `"glm"`, or any project label. No enforcement — user decides.

#### `internal/store/decode.go`

`DecodePath` is **only used as a fallback** when `projectPath` is empty in the index (older Claude Code versions). Use `projectPath` directly whenever available.

When fallback decoding is needed: the folder name encodes a Unix path where `/` was replaced with `-`. The leading `-` represents the root `/`. To resolve ambiguity (e.g. `-mnt-d-2025-Airflow3`), validate each candidate segment against the real filesystem — try treating each `-` as a separator, walk the longest valid path first.

```go
func DecodePath(folderName string) string   // filesystem-validated decode
```

Edge case covered by tests: directory names containing hyphens (e.g. `2025-Airflow3`, `my-project`).

### UI Layer

#### States

```
List → Searching (/) → List (esc)
List → Renaming (r) → List (enter/esc)
List → Tagging (t) → List (enter/esc)
List → Confirming delete (d) → List (y/n)
```

#### `internal/ui/app.go`

Root Bubble Tea model. Holds:
- `conversations []store.Conversation` — full list
- `filtered []int` — indices into conversations matching current search
- `selected map[string]bool` — selected sessionIDs
- `state` — enum: list / searching / renaming / tagging / confirming

#### `internal/ui/list.go`

Custom renderer (not `bubbles/list`). Each row:
```
[x] CustomName or Summary          tags: glm work    2d ago   42 msgs
    ProjectPath                    branch: main
```
- Selected items highlighted with lipgloss
- Tags shown as dim pills
- Modified date shown as relative ("2d ago", "1w ago")
- Sorted by `Modified` descending by default

#### `internal/ui/keys.go`

```
↑↓ / jk   navigate
space      toggle select
a          select all visible
/          focus search
esc        clear search / deselect all / cancel
r          rename selected (single) or focused item
t          edit tags of focused item (comma-separated input)
d          delete selected (confirm prompt)
q          quit
```

### sessions-index.json Schema (Claude Code owned, read-only for us)

```json
{
  "entries": [{
    "sessionId": "uuid",
    "fullPath": "/home/.../.claude/projects/.../uuid.jsonl",
    "firstPrompt": "first user message",
    "summary": "session summary",
    "messageCount": 78,
    "created": "ISO8601",
    "modified": "ISO8601",
    "gitBranch": "main",
    "projectPath": "/original/fs/path"
  }]
}
```

### claude-manager-meta.json Schema (owned by this tool)

```json
{
  "sessions": {
    "uuid-1": {
      "customName": "Airflow migration planning",
      "tags": ["glm", "work"]
    },
    "uuid-2": {
      "tags": ["claude"]
    }
  }
}
```

## Dependencies

```
github.com/charmbracelet/bubbletea v1.x
github.com/charmbracelet/bubbles v0.x    # textinput only
github.com/charmbracelet/lipgloss v1.x
```

## Platform Notes

- `ClaudeDir()` returns `~/.claude` on Linux and WSL. On Mac, check `~/Library/Application Support/Claude` first, fall back to `~/.claude`.
- Atomic rename (`os.Rename`) works correctly on both Linux/WSL and Mac for same-filesystem moves.
- Test on both WSL and Mac before releasing.

## Testing

- `decode_test.go` must cover: simple path, path with hyphenated dir name, path with multiple hyphenated segments, root-level path.
- `meta_test.go` must cover: load missing file (returns empty Meta, no error), save+load roundtrip, delete entry.
- No mocking of the filesystem — use `t.TempDir()` for real file I/O in tests.
