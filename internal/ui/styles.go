package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vyoogam/cloudmanager/internal/config"
)

// Package-level style vars, initialized by InitStyles.
var (
	Subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	Highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	Special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	Info      = lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}
	Amber     = lipgloss.AdaptiveColor{Light: "#B7791F", Dark: "#FFB000"}
	Alert     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}

	StatusRunning     = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#22C55E"}
	StatusAvailable   = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#06B6D4"}
	StatusReady       = lipgloss.AdaptiveColor{Light: "#65A30D", Dark: "#A3E635"}
	StatusInUse       = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"}
	StatusStarting    = lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}
	StatusStopping    = lipgloss.AdaptiveColor{Light: "#EA580C", Dark: "#FB923C"}
	StatusStopped     = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#F59E0B"}
	StatusTerminated  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"}
	StatusDeallocated = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	StatusUnknown     = lipgloss.AdaptiveColor{Light: "#737373", Dark: "#737373"}
	StatusReachable   = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}
	StatusUnreachable = lipgloss.AdaptiveColor{Light: "#EA580C", Dark: "#F97316"}
	ColumnName        = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A855F7"}
	ColumnID          = lipgloss.AdaptiveColor{Light: "#4338CA", Dark: "#818CF8"}
	ColumnIP          = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#06B6D4"}
	ColumnProvider    = lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"}
	ColumnRegion      = lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"}
	ColumnType        = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	ColumnMeta        = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}

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
	Subtle = themeColor(subtle, "#D9DCCF", "#6B7280")
	Highlight = themeColor(highlight, "#874BFD", "#6D28D9")
	Special = themeColor(special, "#43BF6D", "#047857")
	Alert = themeColor(alert, "#FF5F87", "#BE123C")
	Info = themeColor("", "#38BDF8", "#0369A1")
	Amber = themeColor("", "#F59E0B", "#B45309")

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

func InitTheme(theme config.ThemeConfig) {
	InitStyles(theme.Subtle, theme.Highlight, theme.Special, theme.Alert)
	Info = themeColor(theme.Info, "#38BDF8", "#0369A1")
	Amber = themeColor(theme.Amber, "#F59E0B", "#B45309")
	StatusRunning = themeColor(theme.StatusRunning, "#22C55E", "#15803D")
	StatusAvailable = themeColor(theme.StatusAvailable, "#06B6D4", "#0891B2")
	StatusReady = themeColor(theme.StatusReady, "#A3E635", "#65A30D")
	StatusInUse = themeColor(theme.StatusInUse, "#60A5FA", "#2563EB")
	StatusStarting = themeColor(theme.StatusStarting, "#38BDF8", "#0284C7")
	StatusStopping = themeColor(theme.StatusStopping, "#FB923C", "#EA580C")
	StatusStopped = themeColor(theme.StatusStopped, "#F59E0B", "#B45309")
	StatusTerminated = themeColor(theme.StatusTerminated, "#EF4444", "#DC2626")
	StatusDeallocated = themeColor(theme.StatusDeallocated, "#A78BFA", "#7C3AED")
	StatusUnknown = themeColor(theme.StatusUnknown, "#737373", "#737373")
	StatusReachable = themeColor(theme.StatusReachable, "#10B981", "#059669")
	StatusUnreachable = themeColor(theme.StatusUnreachable, "#F97316", "#EA580C")
	ColumnName = themeColor(theme.ColumnName, "#A855F7", "#6D28D9")
	ColumnID = themeColor(theme.ColumnID, "#818CF8", "#4338CA")
	ColumnIP = themeColor(theme.ColumnIP, "#06B6D4", "#0891B2")
	ColumnProvider = themeColor(theme.ColumnProvider, "#F472B6", "#BE185D")
	ColumnRegion = themeColor(theme.ColumnRegion, "#2DD4BF", "#0F766E")
	ColumnType = themeColor(theme.ColumnType, "#FBBF24", "#B45309")
	ColumnMeta = themeColor(theme.ColumnMeta, "#94A3B8", "#64748B")
}

func themeColor(value, darkDefault, lightDefault string) lipgloss.AdaptiveColor {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, darkDefault) {
		return lipgloss.AdaptiveColor{Light: lightDefault, Dark: darkDefault}
	}
	return lipgloss.AdaptiveColor{Light: value, Dark: value}
}
