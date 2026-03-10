package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.activePane == paneSplash {
		splash := `
   ____ _                 _ __  __                                   
  / ___| | ___  _   _  __| |  \/  | __ _ _ __   __ _  __ _  ___ _ __ 
 | |   | |/ _ \| | | |/ _` + "`" + ` | |\/| |/ _` + "`" + ` | '_ \ / _` + "`" + ` |/ _` + "`" + ` |/ _ \ '__|
 | |___| | (_) | |_| | (_| | |  | | (_| | | | | (_| | (_| |  __/ |   
  \____|_|\___/ \__,_|\__,_|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|   
                                                     |___/           
`
		splashStyle := lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			Align(lipgloss.Center)

		statusStyle := lipgloss.NewStyle().
			Foreground(subtle).
			Align(lipgloss.Center).
			MarginTop(2)

		content := lipgloss.JoinVertical(lipgloss.Center,
			splashStyle.Render(splash),
			statusStyle.Render(m.statusMsg),
		)

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	if m.activePane == paneConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.configList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space: Toggle \u2022 Enter: Save \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneColumnConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.columnConfigList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space: Toggle \u2022 Enter: Save \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneSortConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.sortList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Enter: Sort (Toggles Asc/Desc) \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneDescribe {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.descView.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Scroll \u2022 Esc: Close | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	sidebarWidth := m.width / 3
	if !m.showSidebar {
		sidebarWidth = 0
	}

	// Contexts Pane
	sidebarView := ""
	if m.showSidebar {
		m.contexts.SetSize(sidebarWidth-2, m.height-5)
		cStyle := baseStyle.Copy().Width(sidebarWidth).Height(m.height - 3).MaxWidth(sidebarWidth)
		if m.activePane == paneContexts {
			cStyle = activeStyle.Copy().Width(sidebarWidth).Height(m.height - 3).MaxWidth(sidebarWidth)
		}
		sidebarView = cStyle.Render(m.contexts.View())
	}

	// VMs Pane
	mainWidth := m.width - sidebarWidth - 2
	if !m.showSidebar {
		mainWidth = m.width - 2
	}

	vStyle := baseStyle.Copy().Width(mainWidth).Height(m.height - 3).MaxWidth(mainWidth)
	if m.activePane == paneVMs || m.activePane == paneActions {
		vStyle = activeStyle.Copy().Width(mainWidth).Height(m.height - 3).MaxWidth(mainWidth)
	}

	header := breadcrumbStyle.Render(m.breadcrumbs)

	tableContent := m.vms.View()
	if m.loading {
		tableContent = lipgloss.NewStyle().Padding(2).Foreground(subtle).Render("Loading instances from CLI...")
	} else if m.isSearching || m.searchInput.Value() != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(m.searchInput.View()),
			tableContent,
		)
	}

	mainContentView := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"\n",
		tableContent,
	)

	// Check if we need to render the action overlay
	if m.activePane == paneActions {
		overlay := overlayStyle.Render(m.actions.View())
		mainContentView = lipgloss.Place(
			vStyle.GetWidth(), vStyle.GetHeight(),
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if m.activePane == paneConfirm {
		confirmMsg := fmt.Sprintf("Are you sure you want to %s instance %s?", m.pendingAction.title, m.pendingVM.Name)
		confirmStyle := overlayStyle.Copy().BorderForeground(alert).Padding(1, 2).Width(50)
		confirmView := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(alert).Bold(true).Render("⚠️  CONFIRM ACTION"),
			"\n",
			lipgloss.NewStyle().Align(lipgloss.Center).Render(confirmMsg),
			"\n",
			lipgloss.NewStyle().Foreground(subtle).Render("Enter: Confirm \u2022 Esc: Cancel"),
		)
		overlay := confirmStyle.Render(confirmView)
		mainContentView = lipgloss.Place(
			vStyle.GetWidth(), vStyle.GetHeight(),
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	mainView := vStyle.Render(mainContentView)

	// Combine horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainView)

	// Footer
	footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Enter: Select \u2022 Tab: Switch Pane \u2022 Esc/q: Back/Quit | Status: %s", m.statusMsg))

	return lipgloss.JoinVertical(lipgloss.Left, panes, footer)
}

// Background Task Stubs
