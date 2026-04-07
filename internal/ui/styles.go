package ui

import "github.com/charmbracelet/lipgloss"

// Package-level style vars, initialized by InitStyles.
var (
	Subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	Highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	Special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	Alert     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}

	BaseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Subtle)

	ActiveStyle = BaseStyle.
			BorderForeground(Highlight)

	TitleStyle = lipgloss.NewStyle().
			Foreground(Highlight).
			Bold(true).
			MarginLeft(2).
			MarginBottom(1)

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(Special).
			Bold(true).
			Padding(0, 1)

	StatusLineStyle = lipgloss.NewStyle().
			Foreground(Subtle).
			Padding(0, 1)

	OverlayStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Highlight).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(1, 2)

	ShellStyle = BaseStyle.Copy()
)

// InitStyles updates the package-level styles from a theme config.
func InitStyles(subtle, highlight, special, alert string) {
	Subtle = lipgloss.AdaptiveColor{Light: subtle, Dark: subtle}
	Highlight = lipgloss.AdaptiveColor{Light: highlight, Dark: highlight}
	Special = lipgloss.AdaptiveColor{Light: special, Dark: special}
	Alert = lipgloss.AdaptiveColor{Light: alert, Dark: alert}

	BaseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Subtle)
	ActiveStyle = BaseStyle.BorderForeground(Highlight)
	TitleStyle = lipgloss.NewStyle().Foreground(Highlight).Bold(true).MarginLeft(2).MarginBottom(1)
	BreadcrumbStyle = lipgloss.NewStyle().Foreground(Special).Bold(true).Padding(0, 1)
	StatusLineStyle = lipgloss.NewStyle().Foreground(Subtle).Padding(0, 1)
	OverlayStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(Highlight).Background(lipgloss.Color("#1a1a1a")).Padding(1, 2)
	ShellStyle = BaseStyle.Copy()
}
