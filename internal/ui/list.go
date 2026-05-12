package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/peravit/claude-session-managers/internal/store"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleCursor   = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleTag      = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stylePrompt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
)

const (
	rowHeight    = 2
	rightColWidth = 18 // "  2.4M  2d ago"
)

// ── Session view rendering ────────────────────────────────────────────────

func renderHeader(selectedCount, totalVisible, totalAll int, width int, sortLabel string) string {
	title := styleTitle.Render("Claude Manager")
	sortInfo := " " + styleDim.Render("sort:"+sortLabel)

	var right string
	if selectedCount > 0 {
		right = styleSelected.Render(fmt.Sprintf("(%d selected)", selectedCount))
	} else {
		right = styleDim.Render(fmt.Sprintf("%d conversations", totalAll))
	}
	if totalVisible < totalAll {
		right = styleDim.Render(fmt.Sprintf("%d/%d", totalVisible, totalAll)) + "  " + right
	}

	left := title + sortInfo
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderRow(conv store.Conversation, isCursor, isSelected bool, width int) string {
	marker := "  "
	if isSelected {
		marker = styleSelected.Render("● ")
	}

	name := conv.DisplayName()
	tags := styleTag.Render(renderTags(conv.Tags))

	// Right column: fixed width for alignment
	sizeStr := humanSize(conv.FileSize)
	dateStr := relativeTime(conv.Modified)
	right := styleDim.Render(fmt.Sprintf("%8s  %7s", sizeStr, dateStr))

	// Left column: marker + name + tags, padded to fill remaining space
	leftWidth := width - rightColWidth
	if leftWidth < 10 {
		leftWidth = 10
	}
	nameMax := leftWidth - lipgloss.Width(marker) - lipgloss.Width(tags) - 2
	if nameMax < 4 {
		nameMax = 4
	}
	name = truncate(name, nameMax)

	line1 := lipgloss.NewStyle().Width(leftWidth).MaxWidth(leftWidth).Render(marker + name + "  " + tags) + right

	// Line 2: project path + branch + message count
	project := conv.ProjectPath
	branch := ""
	if conv.GitBranch != "" {
		branch = " · " + conv.GitBranch
	}
	msgs := fmt.Sprintf(" · %d msgs", conv.MessageCount)
	line2 := styleDim.Render("  " + truncate(project+branch+msgs, width-4))

	row := line1 + "\n" + line2
	if isCursor {
		row = styleCursor.Render(line1) + "\n" + styleCursor.Render(line2)
	}
	return row
}

func renderFooter(width int) string {
	hint := "↑↓/jk nav · space select · a all · / search · s sort · p projects · r rename · t tag · d delete · q quit"
	return styleHint.Render(truncate(hint, width))
}

// ── Project view rendering ────────────────────────────────────────────────

func renderProjectHeader(filtered, total int, width int) string {
	title := styleTitle.Render("Projects")
	right := styleDim.Render(fmt.Sprintf("%d projects", total))
	if filtered < total {
		right = styleDim.Render(fmt.Sprintf("%d/%d", filtered, total))
	}
	gap := width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return title + strings.Repeat(" ", gap) + right
}

func renderProjectRow(pe projectEntry, isCursor bool, width int) string {
	sizeStr := humanSize(pe.TotalSize)
	dateStr := relativeTime(pe.LastModified)
	right := styleDim.Render(fmt.Sprintf("%8s  %7s", sizeStr, dateStr))

	leftWidth := width - rightColWidth
	if leftWidth < 10 {
		leftWidth = 10
	}

	name := truncate(pe.ProjectPath, leftWidth-4)
	count := styleDim.Render(fmt.Sprintf("(%d sessions)", pe.SessionCount))

	line1 := lipgloss.NewStyle().Width(leftWidth).MaxWidth(leftWidth).Render(name + "  " + count) + right

	if isCursor {
		line1 = styleCursor.Render(line1)
	}
	return line1
}

func renderProjectFooter(width int) string {
	hint := "↑↓/jk nav · enter select · / search project · esc back · q quit"
	return styleHint.Render(truncate(hint, width))
}

// ── Helpers ───────────────────────────────────────────────────────────────

func renderTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	var b strings.Builder
	for _, tag := range tags {
		b.WriteString("[")
		b.WriteString(tag)
		b.WriteString("]")
	}
	return b.String()
}

func humanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return t.Format("Jan 2")
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
