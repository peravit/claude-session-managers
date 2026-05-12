package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/peravit/claude-session-managers/internal/codexstore"
	"github.com/peravit/claude-session-managers/internal/store"
	"github.com/peravit/claude-session-managers/internal/ui"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("codex-manager", version)
		os.Exit(0)
	}

	convs, err := codexstore.LoadConversations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading Codex sessions: %v\n", err)
		os.Exit(1)
	}

	// Convert codex conversations to store format for the shared UI
	storeConvs := make([]store.Conversation, len(convs))
	for i, c := range convs {
		storeConvs[i] = store.Conversation{
			SessionID:    c.SessionID,
			FullPath:     c.FullPath,
			FirstPrompt:  c.FirstPrompt,
			MessageCount: c.MessageCount,
			Created:      c.Created,
			Modified:     c.Modified,
			GitBranch:    c.Model,
			ProjectPath:  c.Cwd,
			FolderName:   filepath.Base(c.Cwd),
			FileSize:     c.FileSize,
			CustomName:   c.CustomName,
			Tags:         c.Tags,
		}
	}

	model, err := ui.NewWithConversations(storeConvs, store.CodexDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
