package ui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/logging"
	"cloudmanager/internal/providers"
)

const (
	focusSidebar = iota
	focusMain
)

// --- Bubble Tea messages ---

type contextLoadMsg struct {
	tree     []*TreeNode
	warnings []string
}

type gcpProjectFetchMsg struct {
	items []list.Item
}

type logRefreshMsg struct {
	content string
	err     error
}

// --- list item types for GCP config ---

type gcpProjectItem struct {
	projectId string
	selected  bool
}

func (i gcpProjectItem) Title() string {
	if i.selected {
		return "[x] " + i.projectId
	}
	return "[ ] " + i.projectId
}
func (i gcpProjectItem) Description() string { return "GCP Project" }
func (i gcpProjectItem) FilterValue() string { return i.projectId }

// --- App model (owns sidebar, status, view stack) ---

// App is the top-level Bubble Tea model. It owns the context sidebar,
// status bar, and a stack of Views for drill-down navigation.
type App struct {
	contexts      list.Model
	configList    list.Model
	rootNodes     []*TreeNode
	viewStack     []View
	resourceViews map[string]registeredView
	activeTab     string
	activeCtx     core.CloudContext
	cfg           config.AppConfig
	focus         int // focusSidebar or focusMain

	statusMsg      string
	parserWarnings []string
	showSidebar    bool
	showConfig     bool
	showLogs       bool
	showSplash     bool
	width, height  int
	Version        string
	BuildTime      string
	logView        viewport.Model
	logPath        string
}

type registeredView struct {
	TabIndex   string
	Capability providers.Capability
	View       View
}

const footerHeight = 1

// NewApp creates the initial App model.
func NewApp(cfg config.AppConfig, version, buildTime string) App {
	ctxList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	ctxList.Title = "Cloud Contexts"
	ctxList.SetShowStatusBar(false)
	ctxList.SetFilteringEnabled(true)

	configDelegate := list.NewDefaultDelegate()
	configList := list.New([]list.Item{}, configDelegate, 0, 0)
	configList.Title = "Configure GCP Projects (Space to toggle, Enter to save, Esc to cancel)"
	configList.SetShowStatusBar(false)

	logView := viewport.New(0, 0)
	logView.Style = lipgloss.NewStyle().Padding(0, 1)

	return App{
		contexts:      ctxList,
		configList:    configList,
		resourceViews: make(map[string]registeredView),
		activeTab:     "1",
		cfg:           cfg,
		Version:       version,
		BuildTime:     buildTime,
		focus:         focusSidebar,
		statusMsg:     "Ready.",
		showSidebar:   true,
		showSplash:    true,
		logView:       logView,
		logPath:       logging.Path(),
	}
}

// RegisterView adds a main view mapped to a tab index (1-5).
func (a *App) RegisterView(tabIndex string, capability providers.Capability, v View) {
	a.resourceViews[tabIndex] = registeredView{TabIndex: tabIndex, Capability: capability, View: v}
	if a.activeTab == tabIndex && len(a.viewStack) == 0 {
		a.viewStack = []View{v}
	}
}

func (a App) Init() tea.Cmd {
	return fetchContextsCmd()
}

func (a App) availableTabs() []registeredView {
	tabs := make([]registeredView, 0, len(a.resourceViews))
	for _, view := range a.resourceViews {
		if a.activeCtx.Provider != "" && !providers.Supports(a.activeCtx.Provider, view.Capability) {
			continue
		}
		tabs = append(tabs, view)
	}
	sort.Slice(tabs, func(i, j int) bool {
		return tabs[i].TabIndex < tabs[j].TabIndex
	})
	return tabs
}

func (a *App) ensureActiveTab() {
	tabs := a.availableTabs()
	if len(tabs) == 0 {
		a.viewStack = nil
		a.activeTab = ""
		return
	}
	for _, tab := range tabs {
		if tab.TabIndex == a.activeTab {
			if len(a.viewStack) == 0 || a.viewStack[0].Title() != tab.View.Title() {
				a.viewStack = []View{tab.View}
			}
			return
		}
	}
	a.activeTab = tabs[0].TabIndex
	a.viewStack = []View{tabs[0].View}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case PushViewMsg:
		a.viewStack = append(a.viewStack, msg.View)
		a.focus = focusMain
		logging.Infof("component=ui event=push_view title=%s provider=%s account=%s region=%s", msg.View.Title(), msg.Ctx.Provider, msg.Ctx.AccountID, msg.Ctx.Region)
		initCmd := msg.View.Init(msg.Ctx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
		return a, initCmd

	case PopViewMsg:
		if len(a.viewStack) > 1 {
			a.viewStack = a.viewStack[:len(a.viewStack)-1]
		}
		logging.Infof("component=ui event=pop_view depth=%d", len(a.viewStack))
		return a, nil

	case tea.KeyMsg:
		if a.showConfig {
			return a.handleConfigKeys(msg)
		}
		if a.showLogs {
			return a.handleLogKeys(msg)
		}

		isInputActive := false
		if a.focus == focusMain && len(a.viewStack) > 0 {
			isInputActive = a.viewStack[len(a.viewStack)-1].IsInputActive()
		} else if a.focus == focusSidebar && a.contexts.FilterState() == list.Filtering {
			isInputActive = true
		}

		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7":
			if !isInputActive {
				if view, ok := a.resourceViews[msg.String()]; ok {
					if a.activeCtx.Provider != "" && !providers.Supports(a.activeCtx.Provider, view.Capability) {
						a.statusMsg = fmt.Sprintf("%s not supported for %s.", view.View.Title(), a.activeCtx.Provider)
						return a, nil
					}
					a.activeTab = msg.String()
					a.viewStack = []View{view.View}
					a.focus = focusMain
					logging.Infof("component=ui event=tab_switch tab=%s title=%s provider=%s account=%s region=%s mode=%s", msg.String(), view.View.Title(), a.activeCtx.Provider, a.activeCtx.AccountID, a.activeCtx.Region, a.backendMode())
					if a.activeCtx.AccountID != "" || a.activeCtx.AccountName != "" {
						initCmd := view.View.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
						a.statusMsg = fmt.Sprintf("Switching to %s...", view.View.Title())
						return a, initCmd
					}
					a.statusMsg = fmt.Sprintf("Switched to %s. Select a context.", view.View.Title())
				}
				return a, nil
			}
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if !isInputActive {
				if a.focus == focusSidebar || (a.focus == focusMain && len(a.viewStack) <= 1) {
					return a, tea.Quit
				}
			}
		case "b":
			if !isInputActive {
				a.showSidebar = !a.showSidebar
				if !a.showSidebar && a.focus == focusSidebar {
					a.focus = focusMain
				}
				a.resizeViews()
				return a, nil
			}
		case "c":
			if !isInputActive && a.focus == focusSidebar {
				a.showConfig = true
				a.statusMsg = "Loading GCP projects..."
				cmds = append(cmds, fetchAllGCPProjectsCmd())
				return a, tea.Batch(cmds...)
			}
		case "L":
			if !isInputActive {
				a.showLogs = true
				a.resizeLogView()
				a.statusMsg = fmt.Sprintf("Viewing application logs: %s", a.logPath)
				logging.Infof("component=ui event=logs_open path=%s", a.logPath)
				return a, loadLogsCmd()
			}
		case "tab":
			if !isInputActive {
				if a.focus == focusSidebar {
					a.focus = focusMain
				} else {
					a.focus = focusSidebar
				}
				return a, nil
			}
		case "esc":
			if !isInputActive {
				if a.focus == focusMain && len(a.viewStack) > 1 {
					a.viewStack = a.viewStack[:len(a.viewStack)-1]
					return a, nil
				}
				if a.focus == focusMain {
					a.focus = focusSidebar
					return a, nil
				}
			}
		case "enter":
			if !isInputActive && a.focus == focusSidebar {
				selected := a.contexts.SelectedItem()
				if selected != nil {
					node := selected.(*TreeNode)
					if node.IsLeaf {
						a.activeCtx = node.Context
						a.ensureActiveTab()
						a.focus = focusMain
						logging.Infof("component=ui event=context_select provider=%s account=%s region=%s", a.activeCtx.Provider, a.activeCtx.AccountID, a.activeCtx.Region)
						if len(a.viewStack) > 0 {
							topView := a.viewStack[len(a.viewStack)-1]
							initCmd := topView.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
							a.statusMsg = fmt.Sprintf("Fetching instances for %s...", a.activeCtx.DisplayName())
							return a, initCmd
						}
					} else {
						node.Expanded = !node.Expanded
						a.contexts.SetItems(BuildFlatList(a.rootNodes))
					}
				}
				return a, tea.Batch(cmds...)
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.contexts.SetSize(a.sidebarContentWidth(), a.sidebarContentHeight())
		a.configList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.resizeViews()
		a.resizeLogView()
		return a, nil

	case contextLoadMsg:
		a.rootNodes = msg.tree
		items := BuildFlatList(a.rootNodes)
		a.contexts.SetItems(items)
		a.parserWarnings = msg.warnings
		if len(items) == 0 {
			if len(msg.warnings) > 0 {
				a.statusMsg = fmt.Sprintf("No contexts. %s", strings.Join(msg.warnings, " | "))
			} else {
				a.statusMsg = "No cloud contexts found. Configure a supported provider CLI or add managed contexts."
			}
		} else if len(msg.warnings) > 0 {
			a.statusMsg = fmt.Sprintf("Loaded %d contexts. Warnings: %s", len(items), strings.Join(msg.warnings, " | "))
		} else {
			a.statusMsg = fmt.Sprintf("Loaded %d contexts.", len(items))
		}
		logging.Infof("component=ui event=contexts_loaded count=%d warnings=%d", len(items), len(msg.warnings))
		a.showSplash = false

	case gcpProjectFetchMsg:
		a.configList.SetItems(msg.items)
		a.statusMsg = "Select GCP projects (Space to toggle, Enter to save)."
		logging.Infof("component=ui event=gcp_projects_loaded count=%d", len(msg.items))

	case logRefreshMsg:
		if msg.err != nil {
			a.logView.SetContent(fmt.Sprintf("Failed to load log file:\n\n%v", msg.err))
			a.statusMsg = fmt.Sprintf("Log read failed: %v", msg.err)
			logging.Errorf("component=ui event=logs_read_failed err=%v", msg.err)
		} else {
			a.logView.SetContent(msg.content)
			a.logView.GotoBottom()
			a.statusMsg = fmt.Sprintf("Loaded logs from %s", a.logPath)
		}
	}

	// Route message to the active view if focus is on main
	if a.focus == focusMain && len(a.viewStack) > 0 {
		topView := a.viewStack[len(a.viewStack)-1]
		newView, viewCmd := topView.Update(msg)
		a.viewStack[len(a.viewStack)-1] = newView
		cmds = append(cmds, viewCmd)

		// Check if the view returned a status update
		if su, ok := msg.(statusUpdateMsg); ok {
			a.statusMsg = su.msg
		}
	}

	// Always route to sidebar
	if a.focus == focusSidebar {
		a.contexts, cmd = a.contexts.Update(msg)
		cmds = append(cmds, cmd)
	}

	if a.showConfig {
		a.configList, cmd = a.configList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// statusUpdateMsg lets views update the app-level status bar.
type statusUpdateMsg struct{ msg string }

func (a App) View() string {
	if a.width == 0 {
		return "Initializing..."
	}

	if a.showSplash {
		return fitToWindow(a.renderSplash(), a.width, a.height)
	}

	if a.showConfig {
		mainView := a.renderShellPane(a.configList.View(), true)
		footer := renderFooter(a.width, fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space: Toggle \u2022 Enter: Save \u2022 Esc: Cancel | %s", a.statusMsg))
		return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
	}
	if a.showLogs {
		return a.renderLogsView()
	}

	shellInnerWidth := a.shellContentWidth()
	shellInnerHeight := a.shellContentHeight()
	sidebarWidth := a.sidebarPanelWidth()
	mainWidth := shellInnerWidth - sidebarWidth
	if mainWidth < 0 {
		mainWidth = 0
	}
	mainContent := ""

	tabBar := renderTabBar(mainWidth, a.mainContentWidth(), a.activeTab, a.availableTabs())

	if len(a.viewStack) > 0 {
		mainContent = a.viewStack[len(a.viewStack)-1].Render()
	} else {
		mainContent = lipgloss.NewStyle().Padding(2).Foreground(Subtle).Render("Select a context to view resources")
	}

	mainView := lipgloss.NewStyle().
		Width(mainWidth).
		MaxWidth(mainWidth).
		Height(shellInnerHeight).
		MaxHeight(shellInnerHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, tabBar, mainContent))

	sidebarView := ""
	if a.showSidebar {
		a.contexts.SetSize(a.sidebarContentWidth(), a.sidebarContentHeight())
		sidebarStyle := sidebarPaneStyle(a.focus == focusSidebar, sidebarWidth, shellInnerHeight)
		sidebarView = sidebarStyle.Render(a.contexts.View())
	}

	panes := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainView)
	mainShell := a.renderShellPane(panes, a.focus == focusMain)
	footerText := fmt.Sprintf("\u2191\u2193 \u2022 Enter \u2022 Tab \u2022 Esc \u2022 L:Logs | Mode:%s | %s | v%s", a.backendMode(), a.statusMsg, a.Version)
	footer := renderFooter(a.width, footerText)
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainShell, footer), a.width, a.height)
}

func (a App) renderSplash() string {
	splash := `
   ____ _                 _ __  __                                   
  / ___| | ___  _   _  __| |  \/  | __ _ _ __   __ _  __ _  ___ _ __ 
 | |   | |/ _ \| | | |/ _` + "`" + ` | |\/| |/ _` + "`" + ` | '_ \ / _` + "`" + ` |/ _` + "`" + ` |/ _ \ '__|
 | |___| | (_) | |_| | (_| | |  | | (_| | | | | (_| | (_| |  __/ |   
  \____|_|\___/ \__,_|\__,_|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|   
                                                      |___/           
`
	splashStyle := lipgloss.NewStyle().Foreground(Highlight).Bold(true).Align(lipgloss.Center)
	statusStyle := lipgloss.NewStyle().Foreground(Subtle).Align(lipgloss.Center).MarginTop(2)
	status := statusStyle.Render(fmt.Sprintf("Loading cloud contexts...\nVersion: %s (%s)", a.Version, a.BuildTime))
	return lipgloss.Place(a.width, a.height-1, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, splashStyle.Render(splash), status))
}

func (a App) panesHeight() int {
	height := a.height - footerHeight
	if height < 0 {
		return 0
	}
	return height
}

func (a App) sidebarOuterWidth() int {
	if !a.showSidebar {
		return 0
	}
	width := a.width / 3
	if width < 24 && a.width >= 24 {
		width = 24
	}
	if width > a.width {
		width = a.width
	}
	return width
}

func (a App) mainOuterWidth() int {
	width := a.width
	if a.showSidebar {
		width -= a.sidebarOuterWidth()
	}
	if width < 0 {
		return 0
	}
	return width
}

func (a App) shellContentWidth() int {
	return styleInnerWidth(ShellStyle, a.width)
}

func (a App) shellContentHeight() int {
	return styleInnerHeight(ShellStyle, a.panesHeight())
}

func (a App) sidebarPanelWidth() int {
	if !a.showSidebar {
		return 0
	}
	width := a.sidebarContentWidth() + sidebarPaneStyle(false, 0, 0).GetHorizontalFrameSize()
	if width > a.shellContentWidth() {
		return a.shellContentWidth()
	}
	return width
}

func (a App) mainContentWidth() int {
	width := a.shellContentWidth() - a.sidebarPanelWidth()
	if width < 0 {
		return 0
	}
	return width
}

func (a App) mainPaneInnerHeight() int {
	return a.shellContentHeight()
}

func (a App) mainContentHeight() int {
	height := a.mainPaneInnerHeight() - lipgloss.Height(renderTabBar(a.mainContentWidth(), a.mainContentWidth(), a.activeTab, a.availableTabs()))
	if height < 0 {
		return 0
	}
	return height
}

func (a App) sidebarContentWidth() int {
	width := a.sidebarOuterWidth() - sidebarPaneStyle(false, 0, 0).GetHorizontalFrameSize()
	if width < 0 {
		return 0
	}
	return width
}

func (a App) sidebarContentHeight() int {
	return a.shellContentHeight()
}

func (a App) fullScreenContentWidth() int {
	return a.shellContentWidth()
}

func (a App) fullScreenContentHeight() int {
	return a.shellContentHeight()
}

func (a *App) resizeViews() {
	for _, v := range a.viewStack {
		v.Resize(a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
	}
}

func (a *App) resizeLogView() {
	width := a.fullScreenContentWidth() - 2
	height := a.fullScreenContentHeight() - 4
	if width < 20 {
		width = 20
	}
	if height < 5 {
		height = 5
	}
	a.logView.Width = width
	a.logView.Height = height
}

func renderTabBar(width int, contentWidth int, activeTab string, tabs []registeredView) string {
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(Highlight).Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().Foreground(Subtle).Padding(0, 1)

	if len(tabs) == 0 {
		return lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Width(contentWidth).
			MaxWidth(contentWidth).
			Render("")
	}

	labelSets := make([][]string, 0, 4)
	full := make([]string, 0, len(tabs))
	short := make([]string, 0, len(tabs))
	tiny := make([]string, 0, len(tabs))
	indexOnly := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		title := tab.View.Title()
		full = append(full, fmt.Sprintf("%s:%s", tab.TabIndex, title))
		shortTitle := title
		if len(shortTitle) > 5 {
			shortTitle = shortTitle[:5]
		}
		short = append(short, fmt.Sprintf("%s:%s", tab.TabIndex, shortTitle))
		tiny = append(tiny, fmt.Sprintf("%s:%s", tab.TabIndex, strings.ToUpper(title[:1])))
		indexOnly = append(indexOnly, tab.TabIndex)
	}
	labelSets = append(labelSets, full, short, tiny, indexOnly)

	selected := labelSets[len(labelSets)-1]
	for _, labels := range labelSets {
		var renderedTabs []string
		for i, lbl := range labels {
			tabIdx := tabs[i].TabIndex
			if tabIdx == activeTab {
				renderedTabs = append(renderedTabs, activeStyle.Render(lbl))
			} else {
				renderedTabs = append(renderedTabs, inactiveStyle.Render(lbl))
			}
		}
		candidate := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
		if lipgloss.Width(candidate) <= width {
			selected = labels
			break
		}
	}

	var renderedTabs []string
	for i, lbl := range selected {
		tabIdx := tabs[i].TabIndex
		if tabIdx == activeTab {
			renderedTabs = append(renderedTabs, activeStyle.Render(lbl))
		} else {
			renderedTabs = append(renderedTabs, inactiveStyle.Render(lbl))
		}
	}
	content := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	return lipgloss.NewStyle().
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(contentWidth).
		MaxWidth(contentWidth).
		Render(content)
}

func renderFooter(width int, text string) string {
	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}
	return StatusLineStyle.Render(truncateText(text, contentWidth))
}

func (a App) renderShellPane(content string, active bool) string {
	style := sizedPaneStyle(ShellStyle.Copy(), a.width, a.panesHeight())
	if active {
		style = style.BorderForeground(Highlight)
	}
	return style.Render(content)
}

func sidebarPaneStyle(active bool, width, height int) lipgloss.Style {
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(Subtle).
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height)
	if active {
		style = style.BorderForeground(Highlight)
	}
	return style
}

func sizedPaneStyle(style lipgloss.Style, outerWidth, outerHeight int) lipgloss.Style {
	return style.
		Width(styleInnerWidth(style, outerWidth)).
		MaxWidth(styleInnerWidth(style, outerWidth)).
		Height(styleInnerHeight(style, outerHeight)).
		MaxHeight(styleInnerHeight(style, outerHeight))
}

func styleInnerWidth(style lipgloss.Style, outerWidth int) int {
	width := outerWidth - style.GetHorizontalFrameSize()
	if width < 0 {
		return 0
	}
	return width
}

func styleInnerHeight(style lipgloss.Style, outerHeight int) int {
	height := outerHeight - style.GetVerticalFrameSize()
	if height < 0 {
		return 0
	}
	return height
}

func truncateText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	runes := []rune(s)
	return string(runes[:max-1]) + "…"
}

func fitToWindow(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

func (a App) renderLogsView() string {
	title := TitleStyle.Render("Application Logs")
	meta := StatusLineStyle.Render(truncateText(fmt.Sprintf("Path: %s", a.logPath), a.fullScreenContentWidth()-2))
	body := a.logView.View()
	if strings.TrimSpace(body) == "" {
		body = lipgloss.NewStyle().Foreground(Subtle).Padding(1, 0).Render("No log content loaded.")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, meta, "", body)
	mainView := a.renderShellPane(content, true)
	footer := renderFooter(a.width, fmt.Sprintf("↑↓ Scroll • PgUp/PgDn • r Reload • Esc Close | Mode:%s | %s", a.backendMode(), a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) backendMode() string {
	mode := strings.ToUpper(strings.TrimSpace(a.cfg.Backend))
	if mode == "" {
		return "CLI"
	}
	return mode
}

func (a App) handleLogKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "L":
		a.showLogs = false
		a.statusMsg = "Closed application logs."
		logging.Infof("component=ui event=logs_close")
		return a, nil
	case "r":
		a.statusMsg = fmt.Sprintf("Reloading application logs from %s", a.logPath)
		return a, loadLogsCmd()
	}
	var cmd tea.Cmd
	a.logView, cmd = a.logView.Update(msg)
	return a, cmd
}

func (a App) handleConfigKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showConfig = false
		a.statusMsg = "Canceled configuration."
		return a, nil
	case " ":
		selected := a.configList.SelectedItem()
		if selected != nil {
			idx := a.configList.Index()
			item := selected.(gcpProjectItem)
			item.selected = !item.selected
			cmd := a.configList.SetItem(idx, item)
			return a, cmd
		}
		return a, nil
	case "enter":
		var selectedProjects []string
		for _, item := range a.configList.Items() {
			p := item.(gcpProjectItem)
			if p.selected {
				selectedProjects = append(selectedProjects, p.projectId)
			}
		}
		var managedGCP []config.ManagedCloudContext
		for _, projectID := range selectedProjects {
			managedGCP = append(managedGCP, config.ManagedCloudContext{
				Provider:    "GCP",
				AccountID:   projectID,
				AccountName: projectID,
				Regions:     []string{"global"},
			})
		}
		a.cfg = config.ReplaceManagedContextsForProvider(a.cfg, "GCP", managedGCP)
		a.cfg.GCPConfigured = true
		a.cfg.GCPProjects = selectedProjects
		config.Save(a.cfg)
		a.showConfig = false
		a.statusMsg = "GCP configuration saved. Reloading contexts..."
		return a, fetchContextsCmd()
	}
	var cmd tea.Cmd
	a.configList, cmd = a.configList.Update(msg)
	return a, cmd
}

// --- Commands ---

func fetchContextsCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Infof("component=ui event=contexts_load_start")
		cfg := config.Load()
		var allCtx []core.CloudContext
		var discovered []core.CloudContext
		var warnings []string

		allCtx = append(allCtx, config.ManagedContexts(cfg)...)

		for _, registered := range providers.RegisteredProviders() {
			providerName := registered.Metadata.DisplayName
			if config.HasManagedContextsForProvider(cfg, providerName) {
				continue
			}
			ctxs, providerWarnings := providers.DiscoverContexts(providerName)
			discovered = append(discovered, ctxs...)
			warnings = append(warnings, providerWarnings...)
		}

		if updatedCfg, changed := config.MergeDiscoveredContexts(cfg, discovered); changed {
			if err := config.Save(updatedCfg); err != nil {
				warnings = append(warnings, fmt.Sprintf("Cloud contexts discovered but failed to persist: %v", err))
				allCtx = config.ManagedContexts(updatedCfg)
			} else {
				allCtx = config.ManagedContexts(updatedCfg)
				warnings = append(warnings, fmt.Sprintf("Imported %d cloud context groups into %s", len(updatedCfg.CloudContexts), config.GetConfigPath()))
			}
		} else if len(allCtx) == 0 {
			allCtx = append(allCtx, discovered...)
		}

		// Restore mock fallback if no contexts found
		if len(allCtx) == 0 {
			allCtx = []core.CloudContext{
				{Provider: "AWS", AccountID: "123456789012", AccountName: "production", Region: "us-east-1"},
				{Provider: "AWS", AccountID: "123456789012", AccountName: "production", Region: "us-west-2"},
				{Provider: "AWS", AccountID: "987654321098", AccountName: "staging", Region: "eu-central-1"},
				{Provider: "GCP", AccountID: "my-gcp-project-1", AccountName: "backend-services", Region: "us-central1"},
				{Provider: "GCP", AccountID: "my-gcp-project-2", AccountName: "data-pipeline", Region: "europe-west1"},
				{Provider: "Azure", AccountID: "sub-abc-123", AccountName: "core-infra", Region: "eastus"},
				{Provider: "DigitalOcean", AccountID: "do-demo-account", AccountName: "sandbox", Region: "global"},
			}
		}

		tree := BuildContextTree(allCtx)
		return contextLoadMsg{tree: tree, warnings: warnings}
	}
}

func fetchAllGCPProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Infof("component=ui event=gcp_projects_load_start")
		cmd := exec.Command("gcloud", "projects", "list", "--format=json(projectId)")
		output, err := cmd.Output()
		if err != nil {
			return gcpProjectFetchMsg{items: nil}
		}
		var projects []struct {
			ProjectId string `json:"projectId"`
		}
		if err := json.Unmarshal(output, &projects); err != nil {
			return gcpProjectFetchMsg{items: nil}
		}
		cfg := config.Load()
		var items []list.Item
		for _, p := range projects {
			selected := config.IsManagedAccountSelected(cfg, "GCP", p.ProjectId)
			if !config.HasManagedContextsForProvider(cfg, "GCP") {
				selected = !cfg.GCPConfigured || config.IsGCPProjectSelected(p.ProjectId, cfg)
			}
			items = append(items, gcpProjectItem{projectId: p.ProjectId, selected: selected})
		}
		return gcpProjectFetchMsg{items: items}
	}
}

func loadLogsCmd() tea.Cmd {
	return func() tea.Msg {
		content, err := logging.ReadTail(64 * 1024)
		return logRefreshMsg{content: content, err: err}
	}
}
