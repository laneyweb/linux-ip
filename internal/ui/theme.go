// Package ui renders Snapshots with lipgloss dashboard panels.
//
// Design: basic view fits in one screen (primary iface + DNS + gateway).
// Advanced view appends sysadmin sections (routes, ports, firewall, all
// interfaces). Piped output auto-falls back to plain tables (no ANSI).
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the lipgloss styles; Disabled when --no-color / not a TTY.
type Theme struct {
	Title   lipgloss.Style
	Header  lipgloss.Style
	Label   lipgloss.Style
	Value   lipgloss.Style
	Dim     lipgloss.Style
	OK      lipgloss.Style
	Warn    lipgloss.Style
	Sel     lipgloss.Style
	Box     lipgloss.Style
	Enabled bool
	// ShowIPv6 mirrors the --ip6 flag so the UI can hint hidden addresses.
	ShowIPv6 bool
}

// NewTheme builds the default dark-terminal-friendly palette.
func NewTheme(enabled bool) Theme {
	if !enabled {
		// Zero styles = plain text, keeps --json/piped output clean.
		return Theme{}
	}
	return Theme{
		Enabled: true,
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DD3FC")).
			Padding(0, 1),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F0ABFC")),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")),
		Dim: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")),
		OK: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ADE80")).
			Bold(true),
		Warn: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FACC15")).
			Bold(true),
		// Sel is the cursor/hover row background (click/arrow target).
		Sel: lipgloss.NewStyle().
			Background(lipgloss.Color("#264F78")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1).
			MarginBottom(1),
	}
}

// dot returns a colored status glyph (or ASCII fallback when disabled).
func (t Theme) dot(up bool) string {
	if !t.Enabled {
		if up {
			return "[up]"
		}
		return "[down]"
	}
	if up {
		return t.OK.Render("●")
	}
	return t.Warn.Render("○")
}
