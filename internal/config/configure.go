package config

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

func (m configureModel) Init() tea.Cmd { return nil }

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
func (i backendItem) FilterValue() string { return string(i) }

// RunConfigureTUI opens an interactive TUI for selecting the backend.
func RunConfigureTUI() {
	items := []list.Item{backendItem("cli"), backendItem("sdk")}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select CloudManager Backend"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	p := tea.NewProgram(configureModel{list: l})
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running configure TUI: %v\n", err)
		return
	}
	if fm, ok := finalModel.(configureModel); ok && fm.choice != "" {
		cfg := Load()
		cfg.Backend = fm.choice
		if err := Save(cfg); err != nil {
			fmt.Printf("Error saving configuration: %v\n", err)
		} else {
			fmt.Printf("Successfully set backend to '%s'.\n", fm.choice)
		}
	}
}
