# claude-manager

> Because your Claude Code sessions deserve better than `ls -la ~/.claude`

If you use [Claude Code](https://claude.ai/code) regularly, you know the feeling — dozens of sessions piling up across projects, eating disk space, with no good way to figure out what's what. You could poke around in JSON files... or you could use this.

**claude-manager** is a terminal UI that gives you a bird's-eye view of every Claude Code session you've ever had. Browse by project, search, sort, rename, tag, and delete — all without leaving your terminal.

## What it looks like

**Session list** (what you see when you open it):

```
 Claude Manager         sort:date          47 conversations
 / search…

 ● Refactor the auth middleware   [work]          2.4M   2d ago
   ~/projects/webapp · main · 28 msgs

   Dashboard redesign                             128K   1w ago
   ~/projects/admin-panel · dev · 12 msgs

   Database migration script                      4.1M   3h ago
   ~/projects/api-service · main · 156 msgs

 ↑↓/jk · space select · a all · / search · s sort · p projects · r rename · t tag · d delete · q quit
```

**Project view** (press `p`):

```
 Projects                                     12 projects
 / search project…

 ~/projects/webapp                                (3 sessions)      5.2M   1d ago
 ~/projects/admin-panel                           (1 session)       128K   1w ago
 ~/projects/api-service                           (5 sessions)     12.1M   3h ago

 ↑↓/jk · enter select · / search project · esc back · q quit
```

## What you can do

- **Browse by project** — press `p` to see all your projects at a glance, then dive into one
- **See everything** — all sessions from every project in one list, with file size and age
- **Find fast** — search by name, project, branch, or tag
- **Sort** — by date, size, or name (press `s` to cycle)
- **Rename** — give sessions meaningful names so you remember what they were about
- **Tag** — label sessions with tags like `work`, `bugfix`, `important`
- **Bulk delete** — select multiple sessions and delete them all at once
- **See disk usage** — know exactly which sessions are taking up space

## Installation

### Go install (quickest)

```bash
go install github.com/peravit/claude-manager@latest
```

### Download binary

Grab the latest release for your platform from [GitHub Releases](https://github.com/peravit/claude-manager/releases).

### Build from source

```bash
git clone https://github.com/peravit/claude-manager.git
cd claude-manager
make build
```

### Want a shorter command?

```bash
# Option 1: alias (add to ~/.bashrc or ~/.zshrc)
alias claude-m=claude-manager

# Option 2: symlink
ln -s $(which claude-manager) ~/.local/bin/claude-m
```

## Usage

Just run:

```bash
claude-manager
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `space` | Select session |
| `a` | Select / deselect all |
| `/` | Search |
| `s` | Change sort (date → size → name) |
| `p` | Open project view |
| `r` | Rename session |
| `t` | Edit tags |
| `d` | Delete selected (asks for confirmation) |
| `esc` | Clear search / deselect / cancel / go back |
| `q` | Quit |

### Common workflows

**Clean up old sessions to save space:**

Press `s` to sort by size → `space` to select the big ones → `d` → `y`

**Find a specific session:**

Press `/` → type a keyword → `enter`

**Browse by project:**

Press `p` → navigate to a project → `enter` to see its sessions → `esc` to go back

**Organize sessions:**

Press `r` to give a session a memorable name → `t` to add tags → now you can search by name or tag later

## How it works

- Reads session metadata from `~/.claude/projects/*/sessions-index.json` (maintained by Claude Code)
- Falls back to scanning `.jsonl` session files directly when no index exists
- Your custom names and tags are stored separately in `~/.claude/claude-manager-meta.json` — never touches Claude Code's own files
- All file writes are atomic (temp file + rename) so nothing gets corrupted if the process is interrupted

## Requirements

- [Claude Code](https://claude.ai/code) with conversation history in `~/.claude`

## License

[MIT](LICENSE)
