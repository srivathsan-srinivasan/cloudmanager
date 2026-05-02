package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"cloudmanager/internal/core"
)

// View is the pluggable interface for TUI modules (VMs list, VM details, networks, etc.).
// The App shell maintains a stack of Views for k9s-style drill-down navigation.
type View interface {
	// Init returns the initial command to run when this view becomes active.
	Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd

	// Update handles messages and returns the (possibly new) view and a command.
	Update(msg tea.Msg) (View, tea.Cmd)

	// Render returns the string to display in the main content area.
	Render() string

	// Title returns the view's display name for breadcrumbs.
	Title() string

	// ShortHelp returns a concise help string for the footer.
	ShortHelp() string

	// Resize is called when the terminal dimensions change.
	Resize(width, height int, showSidebar bool)

	// IsInputActive returns true if the view is currently capturing text input.
	IsInputActive() bool
}

// PushViewMsg is returned by a View when it wants to drill down into a child view.
type PushViewMsg struct {
	View View
	Ctx  core.CloudContext
}

// PopViewMsg is returned by a View when it wants to go back to the parent view.
type PopViewMsg struct{}

// StatusUpdateMsg lets child views update the app-level status bar.
type StatusUpdateMsg struct {
	Msg string
}

// ManualHostsChangedMsg tells the shell to reload config-backed manual hosts,
// context tree entries, and the local global-search index.
type ManualHostsChangedMsg struct {
	Msg string
}
