package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	SelectAll key.Binding
	Search    key.Binding
	Sort      key.Binding
	Projects  key.Binding
	Rename    key.Binding
	Tag       key.Binding
	Delete    key.Binding
	Escape    key.Binding
	Enter     key.Binding
	Quit      key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Select:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
	SelectAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Sort:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
	Projects:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "projects")),
	Rename:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	Tag:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tag")),
	Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Escape:    key.NewBinding(key.WithKeys("esc")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
