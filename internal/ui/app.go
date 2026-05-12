package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/peravit/claude-session-managers/internal/store"
)

type sortMode int

const (
	sortDate sortMode = iota
	sortSize
	sortName
)

func (s sortMode) String() string {
	switch s {
	case sortSize:
		return "size"
	case sortName:
		return "name"
	default:
		return "date"
	}
}

type appState int

const (
	stateList appState = iota
	stateSearching
	stateRenaming
	stateTagging
	stateConfirming
	stateProjectList
	stateSearchingProjects
)

// projectEntry holds aggregated data for a single project folder.
type projectEntry struct {
	ProjectPath  string
	FolderName   string
	SessionCount int
	TotalSize    int64
	LastModified time.Time
}

// Model is the root Bubble Tea model.
type Model struct {
	conversations []store.Conversation
	filtered      []int
	cursor        int
	offset        int
	selected      map[string]bool
	state         appState
	sort          sortMode

	searchInput textinput.Model
	editInput   textinput.Model

	width  int
	height int
	err    error

	// Project view
	projects        []projectEntry
	projectFiltered []int
	projectCursor   int
	projectOffset   int
	selectedProject string // folder name filter, "" = all

	// Pluggable backends (default to store package)
	deleteFn func([]store.Conversation) error
	metaDir  string // directory for meta file
}

func New() (Model, error) {
	convs, err := store.LoadConversations()
	if err != nil {
		return Model{}, err
	}
	m := newModel(convs)
	m.deleteFn = store.Delete
	m.metaDir = store.ClaudeDir()
	m = m.rebuildFiltered()
	return m, nil
}

func NewWithConversations(convs []store.Conversation, metaDir string) (Model, error) {
	m := newModel(convs)
	m.deleteFn = store.DeleteFiles
	m.metaDir = metaDir
	m = m.rebuildFiltered()
	return m, nil
}

func newModel(convs []store.Conversation) Model {
	search := textinput.New()
	search.Placeholder = "search…"
	search.CharLimit = 120

	edit := textinput.New()
	edit.CharLimit = 200

	return Model{
		conversations: convs,
		selected:      map[string]bool{},
		searchInput:   search,
		editInput:     edit,
	}
}

func (m Model) WithSize(w, h int) Model {
	m.width = w
	m.height = h
	m = m.rebuildFiltered()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateSearching:
			return m.updateSearching(msg)
		case stateRenaming:
			return m.updateRenaming(msg)
		case stateTagging:
			return m.updateTagging(msg)
		case stateConfirming:
			return m.updateConfirming(msg)
		case stateProjectList:
			return m.updateProjectList(msg)
		case stateSearchingProjects:
			return m.updateSearchingProjects(msg)
		}
	}
	return m, nil
}

// ── Session list handler ──────────────────────────────────────────────────

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
			m = m.clampOffset()
		}

	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m = m.clampOffset()
		}

	case key.Matches(msg, keys.Select):
		if len(m.filtered) > 0 {
			id := m.conversations[m.filtered[m.cursor]].SessionID
			m.selected[id] = !m.selected[id]
			if !m.selected[id] {
				delete(m.selected, id)
			}
		}

	case key.Matches(msg, keys.SelectAll):
		if len(m.selected) == len(m.filtered) {
			m.selected = map[string]bool{}
		} else {
			for _, idx := range m.filtered {
				m.selected[m.conversations[idx].SessionID] = true
			}
		}

	case key.Matches(msg, keys.Search):
		m.state = stateSearching
		m.searchInput.Focus()
		return m, textinput.Blink

	case key.Matches(msg, keys.Sort):
		m.sort = (m.sort + 1) % 3
		m = m.sortConversations()

	case key.Matches(msg, keys.Projects):
		m = m.buildProjects()
		m.state = stateProjectList

	case key.Matches(msg, keys.Rename):
		if len(m.filtered) == 0 {
			break
		}
		conv := m.conversations[m.filtered[m.cursor]]
		m.editInput.Placeholder = "custom name…"
		m.editInput.SetValue(conv.CustomName)
		m.editInput.Focus()
		m.state = stateRenaming
		return m, textinput.Blink

	case key.Matches(msg, keys.Tag):
		if len(m.filtered) == 0 {
			break
		}
		conv := m.conversations[m.filtered[m.cursor]]
		m.editInput.Placeholder = "tags, comma separated (e.g. glm, work)"
		m.editInput.SetValue(strings.Join(conv.Tags, ", "))
		m.editInput.Focus()
		m.state = stateTagging
		return m, textinput.Blink

	case key.Matches(msg, keys.Delete):
		if len(m.selected) > 0 {
			m.state = stateConfirming
		}

	case key.Matches(msg, keys.Escape):
		if m.selectedProject != "" {
			m.selectedProject = ""
			m.searchInput.SetValue("")
			m = m.buildProjects()
			m.state = stateProjectList
		} else {
			m.selected = map[string]bool{}
			m.searchInput.SetValue("")
			m = m.rebuildFiltered()
		}
	}

	return m, nil
}

func (m Model) updateSearching(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m = m.rebuildFiltered()
		m.state = stateList
		return m, nil

	case key.Matches(msg, keys.Enter):
		m.searchInput.Blur()
		m.state = stateList
		return m, nil

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m = m.rebuildFiltered()
		return m, cmd
	}
}

func (m Model) updateRenaming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.editInput.Blur()
		m.state = stateList
		return m, nil

	case key.Matches(msg, keys.Enter):
		if len(m.filtered) > 0 {
			idx := m.filtered[m.cursor]
			m.conversations[idx].CustomName = strings.TrimSpace(m.editInput.Value())
			m.saveMeta()
		}
		m.editInput.Blur()
		m.state = stateList
		return m, nil

	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

func (m Model) updateTagging(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.editInput.Blur()
		m.state = stateList
		return m, nil

	case key.Matches(msg, keys.Enter):
		if len(m.filtered) > 0 {
			idx := m.filtered[m.cursor]
			m.conversations[idx].Tags = parseTags(m.editInput.Value())
			m.saveMeta()
		}
		m.editInput.Blur()
		m.state = stateList
		return m, nil

	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

func (m Model) updateConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		toDelete := m.selectedConversations()
		if err := m.deleteFn(toDelete); err != nil {
			m.err = err
			m.state = stateList
			return m, nil
		}
		deleteSet := map[string]bool{}
		for _, c := range toDelete {
			deleteSet[c.SessionID] = true
		}
		kept := m.conversations[:0]
		for _, c := range m.conversations {
			if !deleteSet[c.SessionID] {
				kept = append(kept, c)
			}
		}
		m.conversations = kept
		m.selected = map[string]bool{}
		m = m.rebuildFiltered()
		m.state = stateList

	default:
		m.state = stateList
	}
	return m, nil
}

// ── Project list handler ──────────────────────────────────────────────────

func (m Model) updateProjectList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		if m.projectCursor > 0 {
			m.projectCursor--
		}

	case key.Matches(msg, keys.Down):
		if m.projectCursor < len(m.projectFiltered)-1 {
			m.projectCursor++
		}

	case key.Matches(msg, keys.Enter):
		if len(m.projectFiltered) > 0 {
			m.selectedProject = m.projects[m.projectFiltered[m.projectCursor]].FolderName
			m.searchInput.SetValue("")
			m.cursor = 0
			m.offset = 0
			m = m.rebuildFiltered()
			m.state = stateList
		}

	case key.Matches(msg, keys.Search):
		m.state = stateSearchingProjects
		m.searchInput.Focus()
		return m, textinput.Blink

	case key.Matches(msg, keys.Escape):
		m.selectedProject = ""
		m.searchInput.SetValue("")
		m = m.rebuildFiltered()
		m.state = stateList
	}

	return m, nil
}

func (m Model) updateSearchingProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m = m.rebuildProjectFiltered()
		m.state = stateProjectList
		return m, nil

	case key.Matches(msg, keys.Enter):
		m.searchInput.Blur()
		m.state = stateProjectList
		return m, nil

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m = m.rebuildProjectFiltered()
		return m, cmd
	}
}

// ── View ──────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.state {
	case stateProjectList, stateSearchingProjects:
		return m.viewProjectList()
	default:
		return m.viewSessionList()
	}
}

func (m Model) viewSessionList() string {
	var b strings.Builder

	b.WriteString(renderHeader(len(m.selected), len(m.filtered), len(m.conversations), m.width, m.sort.String()))
	b.WriteString("\n")

	// Search bar
	if m.state == stateSearching {
		b.WriteString("/ " + m.searchInput.View())
	} else if m.searchInput.Value() != "" {
		b.WriteString(styleDim.Render("/ " + m.searchInput.Value()))
	} else {
		b.WriteString(styleDim.Render("/ search…"))
	}
	b.WriteString("\n\n")

	// If filtered by project, show which project
	if m.selectedProject != "" {
		var projName string
		for _, pe := range m.projects {
			if pe.FolderName == m.selectedProject {
				projName = pe.ProjectPath
				break
			}
		}
		if projName == "" {
			projName = m.selectedProject
		}
		b.WriteString(styleDim.Render("  project: " + projName + "  (esc to go back)"))
		b.WriteString("\n")
	}

	// List rows
	visibleCount := m.visibleRows()
	for i := m.offset; i < m.offset+visibleCount && i < len(m.filtered); i++ {
		conv := m.conversations[m.filtered[i]]
		isCursor := i == m.cursor
		isSelected := m.selected[conv.SessionID]
		b.WriteString(renderRow(conv, isCursor, isSelected, m.width))
		b.WriteString("\n")
	}

	rendered := m.offset + min(visibleCount, len(m.filtered)-m.offset)
	for i := rendered; i < m.offset+visibleCount; i++ {
		b.WriteString("\n\n")
	}

	// Overlay
	switch m.state {
	case stateRenaming:
		b.WriteString(stylePrompt.Render("Rename: ") + m.editInput.View() + "\n")
	case stateTagging:
		b.WriteString(stylePrompt.Render("Tags: ") + m.editInput.View() + "\n")
	case stateConfirming:
		b.WriteString(stylePrompt.Render(
			fmt.Sprintf("Delete %d conversation(s)? [y/N] ", len(m.selected)),
		))
		b.WriteString("\n")
	default:
		if m.err != nil {
			b.WriteString(styleError.Render("Error: "+m.err.Error()) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(renderFooter(m.width))
	return b.String()
}

func (m Model) viewProjectList() string {
	var b strings.Builder

	b.WriteString(renderProjectHeader(len(m.projectFiltered), len(m.projects), m.width))
	b.WriteString("\n")

	// Search bar
	if m.state == stateSearchingProjects {
		b.WriteString("/ " + m.searchInput.View())
	} else if m.searchInput.Value() != "" {
		b.WriteString(styleDim.Render("/ " + m.searchInput.Value()))
	} else {
		b.WriteString(styleDim.Render("/ search project…"))
	}
	b.WriteString("\n\n")

	// Project rows
	visibleCount := m.visibleProjectRows()
	for i := m.projectOffset; i < m.projectOffset+visibleCount && i < len(m.projectFiltered); i++ {
		pe := m.projects[m.projectFiltered[i]]
		isCursor := i == m.projectCursor
		b.WriteString(renderProjectRow(pe, isCursor, m.width))
		b.WriteString("\n")
	}

	rendered := m.projectOffset + min(visibleCount, len(m.projectFiltered)-m.projectOffset)
	for i := rendered; i < m.projectOffset+visibleCount; i++ {
		b.WriteString("\n\n")
	}

	b.WriteString("\n")
	b.WriteString(renderProjectFooter(m.width))
	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────

func (m Model) visibleRows() int {
	extra := 0
	if m.selectedProject != "" {
		extra = 1
	}
	available := m.height - 6 - extra
	if available < 1 {
		available = 1
	}
	return available / rowHeight
}

func (m Model) visibleProjectRows() int {
	available := m.height - 6
	if available < 1 {
		available = 1
	}
	return available / rowHeight
}

func (m Model) clampOffset() Model {
	visible := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	return m
}

func (m Model) sortConversations() Model {
	sort.SliceStable(m.conversations, func(i, j int) bool {
		switch m.sort {
		case sortSize:
			return m.conversations[i].FileSize > m.conversations[j].FileSize
		case sortName:
			return m.conversations[i].DisplayName() < m.conversations[j].DisplayName()
		default:
			return m.conversations[i].Modified.After(m.conversations[j].Modified)
		}
	})
	return m.rebuildFiltered()
}

func (m Model) rebuildFiltered() Model {
	query := strings.ToLower(m.searchInput.Value())
	m.filtered = nil
	for i, conv := range m.conversations {
		if m.selectedProject != "" && conv.FolderName != m.selectedProject {
			continue
		}
		if query == "" || matchConv(conv, query) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) == 0 {
			m.cursor = 0
		} else {
			m.cursor = len(m.filtered) - 1
		}
	}
	return m.clampOffset()
}

func (m Model) buildProjects() Model {
	byFolder := map[string]*projectEntry{}
	for i := range m.conversations {
		conv := &m.conversations[i]
		key := conv.FolderName
		if _, ok := byFolder[key]; !ok {
			byFolder[key] = &projectEntry{
				ProjectPath: conv.ProjectPath,
				FolderName:  conv.FolderName,
			}
		}
		pe := byFolder[key]
		pe.SessionCount++
		pe.TotalSize += conv.FileSize
		if conv.Modified.After(pe.LastModified) {
			pe.LastModified = conv.Modified
		}
	}

	m.projects = make([]projectEntry, 0, len(byFolder))
	for _, pe := range byFolder {
		m.projects = append(m.projects, *pe)
	}

	sort.Slice(m.projects, func(i, j int) bool {
		return m.projects[i].LastModified.After(m.projects[j].LastModified)
	})

	m.projectCursor = 0
	m.projectOffset = 0
	return m.rebuildProjectFiltered()
}

func (m Model) rebuildProjectFiltered() Model {
	query := strings.ToLower(m.searchInput.Value())
	m.projectFiltered = nil
	for i, p := range m.projects {
		if query == "" || strings.Contains(strings.ToLower(p.ProjectPath), query) {
			m.projectFiltered = append(m.projectFiltered, i)
		}
	}
	if m.projectCursor >= len(m.projectFiltered) {
		if len(m.projectFiltered) == 0 {
			m.projectCursor = 0
		} else {
			m.projectCursor = len(m.projectFiltered) - 1
		}
	}
	return m
}

func matchConv(c store.Conversation, query string) bool {
	fields := []string{
		c.CustomName, c.Summary, c.FirstPrompt,
		c.ProjectPath, c.GitBranch,
		strings.Join(c.Tags, " "),
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), query) {
			return true
		}
	}
	return false
}

func (m Model) selectedConversations() []store.Conversation {
	var out []store.Conversation
	for _, c := range m.conversations {
		if m.selected[c.SessionID] {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) saveMeta() {
	metaPath := m.metaDir + "/claude-manager-meta.json"
	data, _ := os.ReadFile(metaPath)
	var meta store.Meta
	if data != nil {
		json.Unmarshal(data, &meta)
	}
	if meta.Sessions == nil {
		meta.Sessions = map[string]store.SessionMeta{}
	}
	for _, conv := range m.conversations {
		if conv.CustomName != "" || len(conv.Tags) > 0 {
			meta.Set(conv.SessionID, store.SessionMeta{
				CustomName: conv.CustomName,
				Tags:       conv.Tags,
			})
		} else {
			meta.Delete(conv.SessionID)
		}
	}
	out, _ := json.MarshalIndent(meta, "", "  ")
	tmp := metaPath + ".tmp"
	os.WriteFile(tmp, out, 0644)
	os.Rename(tmp, metaPath)
}

func parseTags(s string) []string {
	var tags []string
	for _, part := range strings.Split(s, ",") {
		t := strings.TrimSpace(part)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
