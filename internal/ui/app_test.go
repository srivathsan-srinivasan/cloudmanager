package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
)

type mockView struct {
	rendered         string
	lastResizeWidth  int
	lastResizeHeight int
	lastShowSidebar  bool
	sawWindowMsg     bool
}

func (m *mockView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	m.Resize(width, height, showSidebar)
	return nil
}

func (m *mockView) Update(msg tea.Msg) (View, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		m.sawWindowMsg = true
	}
	return m, nil
}

func (m *mockView) Render() string    { return m.rendered }
func (m *mockView) Title() string     { return "Mock" }
func (m *mockView) ShortHelp() string { return "" }
func (m *mockView) Resize(width, height int, showSidebar bool) {
	m.lastResizeWidth = width
	m.lastResizeHeight = height
	m.lastShowSidebar = showSidebar
}
func (m *mockView) IsInputActive() bool { return false }

func TestAppViewFitsWindowWidth(t *testing.T) {
	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.width = 80
	app.height = 24
	app.showSplash = false
	app.showSidebar = true
	app.statusMsg = strings.Repeat("status ", 20)
	app.viewStack = []View{&mockView{rendered: "content"}}

	rendered := strings.TrimRight(app.View(), "\n")
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > app.width {
			t.Fatalf("rendered line exceeds window width: got %d want <= %d\n%s", lipgloss.Width(line), app.width, line)
		}
	}
	if got := len(strings.Split(rendered, "\n")); got > app.height {
		t.Fatalf("rendered view exceeds window height: got %d want <= %d\n%s", got, app.height, rendered)
	}
}

func TestWindowResizeOnlyUsesResizeHook(t *testing.T) {
	mv := &mockView{rendered: "content"}
	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.showSplash = false
	app.showSidebar = true
	app.focus = focusMain
	app.viewStack = []View{mv}

	model, cmd := app.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	if cmd != nil {
		t.Fatal("expected no command from window resize")
	}

	updated := model.(App)
	if mv.sawWindowMsg {
		t.Fatal("expected window size message to be handled by app without forwarding to the view")
	}
	if mv.lastResizeWidth != updated.mainContentWidth() {
		t.Fatalf("expected resize width %d, got %d", updated.mainContentWidth(), mv.lastResizeWidth)
	}
	if mv.lastResizeHeight != updated.mainContentHeight() {
		t.Fatalf("expected resize height %d, got %d", updated.mainContentHeight(), mv.lastResizeHeight)
	}
	if !mv.lastShowSidebar {
		t.Fatal("expected resize to preserve sidebar visibility")
	}
}

func TestAppViewClipsOversizedViewContent(t *testing.T) {
	longLine := strings.Repeat("0123456789", 20)
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, longLine)
	}

	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.width = 100
	app.height = 25
	app.showSplash = false
	app.showSidebar = true
	app.statusMsg = "testing clipping"
	app.viewStack = []View{&mockView{rendered: strings.Join(lines, "\n")}}

	rendered := strings.TrimRight(app.View(), "\n")
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > app.width {
			t.Fatalf("rendered line exceeds window width: got %d want <= %d", lipgloss.Width(line), app.width)
		}
	}
	if got := len(strings.Split(rendered, "\n")); got > app.height {
		t.Fatalf("rendered view exceeds window height: got %d want <= %d", got, app.height)
	}
}

func TestAppFooterShowsBackendMode(t *testing.T) {
	app := NewApp(config.AppConfig{Backend: "sdk"}, "1.0.0", "today")
	app.width = 120
	app.height = 24
	app.showSplash = false
	app.showSidebar = false
	app.statusMsg = "ready"
	app.viewStack = []View{&mockView{rendered: "content"}}

	rendered := app.View()
	if !strings.Contains(rendered, "Mode:SDK") {
		t.Fatalf("expected backend mode indicator in footer, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "L:Logs") {
		t.Fatalf("expected logs hint in footer, got:\n%s", rendered)
	}
}

func TestLogsViewFitsWindow(t *testing.T) {
	app := NewApp(config.AppConfig{Backend: "cli"}, "1.0.0", "today")
	app.width = 90
	app.height = 24
	app.showSplash = false
	app.showLogs = true
	app.logPath = "/tmp/cloudmanager.log"
	app.resizeLogView()
	app.logView.SetContent(strings.Repeat("very long log line 0123456789\n", 40))
	app.statusMsg = "Loaded logs"

	rendered := strings.TrimRight(app.View(), "\n")
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > app.width {
			t.Fatalf("rendered line exceeds window width: got %d want <= %d\n%s", lipgloss.Width(line), app.width, line)
		}
	}
	if got := len(strings.Split(rendered, "\n")); got > app.height {
		t.Fatalf("rendered view exceeds window height: got %d want <= %d", got, app.height)
	}
	if !strings.Contains(rendered, "Application Logs") {
		t.Fatalf("expected logs title in render, got:\n%s", rendered)
	}
}
