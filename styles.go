package main

import "github.com/charmbracelet/lipgloss"

// Styling
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	alert     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle)

	activeStyle = baseStyle.
			BorderForeground(highlight)

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			MarginLeft(2).
			MarginBottom(1)

	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(special).
			Bold(true).
			Padding(0, 1)

	statusLineStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(1, 2)
)

func initStyles(cfg AppConfig) {
	subtleColor := lipgloss.Color(cfg.Theme.Subtle)
	highlightColor := lipgloss.Color(cfg.Theme.Highlight)
	specialColor := lipgloss.Color(cfg.Theme.Special)
	alertColor := lipgloss.Color(cfg.Theme.Alert)

	subtle = lipgloss.AdaptiveColor{Light: string(subtleColor), Dark: string(subtleColor)}
	highlight = lipgloss.AdaptiveColor{Light: string(highlightColor), Dark: string(highlightColor)}
	special = lipgloss.AdaptiveColor{Light: string(specialColor), Dark: string(specialColor)}
	alert = lipgloss.AdaptiveColor{Light: string(alertColor), Dark: string(alertColor)}

	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtle)

	activeStyle = baseStyle.
		BorderForeground(highlight)

	titleStyle = lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginLeft(2).
		MarginBottom(1)

	breadcrumbStyle = lipgloss.NewStyle().
		Foreground(special).
		Bold(true).
		Padding(0, 1)

	statusLineStyle = lipgloss.NewStyle().
		Foreground(subtle).
		Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(highlight).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(1, 2)
}
