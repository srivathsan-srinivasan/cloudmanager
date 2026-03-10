package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type configureModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m configureModel) Init() tea.Cmd {
	return nil
}

func (m configureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if i, ok := m.list.SelectedItem().(backendItem); ok {
				m.choice = string(i)
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m configureModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

type backendItem string

func (i backendItem) Title() string {
	if i == "cli" {
		return "CLI (Default)"
	}
	return "Native SDK"
}

func (i backendItem) Description() string {
	if i == "cli" {
		return "Uses os/exec wrapping 'aws', 'gcloud', 'az'. Easy auth, but slower."
	}
	return "Uses native Go SDKs. Faster performance. Uses existing CLI login sessions."
}

func (i backendItem) FilterValue() string {
	return string(i)
}

func runConfigureTUI() {
	items := []list.Item{
		backendItem("cli"),
		backendItem("sdk"),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select CloudManager Backend"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := configureModel{list: l}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running configure TUI: %v\n", err)
		return
	}

	if finalModel, ok := finalModel.(configureModel); ok && finalModel.choice != "" {
		cfg := loadAppConfig()
		cfg.Backend = finalModel.choice
		err := saveAppConfig(cfg)
		if err != nil {
			fmt.Printf("Error saving configuration: %v\n", err)
		} else {
			fmt.Printf("Successfully set backend to '%s'.\n", finalModel.choice)
		}
	}
}
