package vms

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
	"cloudmanager/internal/views/firewalls"
)

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneColumnConfig
	paneSortConfig
	paneConfirm
)

// --- Bubble Tea messages ---

type vmFetchMsg struct {
	requestKey string
	vms        []core.VM
	err        error
}
type vmCostEnrichedMsg struct {
	requestKey string
	vms        []core.VM
	err        error
}
type commandCompleteMsg struct {
	output string
	err    error
}
type describeCompleteMsg struct {
	output string
	err    error
}
type sshCompleteMsg struct{ err error }
type finopsRecommendMsg struct {
	recommendation string
	err            error
}
type vmMetricsMsg struct {
	requestKey string
	vmID       string
	metrics    *core.VMMetrics
	err        error
}

type cacheEntry struct {
	vms       []core.VM
	timestamp time.Time
}

type costCacheEntry struct {
	monthlyCost string
	costTrend   string
	timestamp   time.Time
}

type metricsCacheEntry struct {
	metrics   *core.VMMetrics
	timestamp time.Time
}

// --- list item adapters ---

type actionItem struct {
	title string
	desc  string
}

func (i actionItem) Title() string       { return i.title }
func (i actionItem) Description() string { return i.desc }
func (i actionItem) FilterValue() string { return i.title }

type columnItem struct {
	name     string
	selected bool
}

func (i columnItem) Title() string {
	if i.selected {
		return "[x] " + i.name
	}
	return "[ ] " + i.name
}
func (i columnItem) Description() string { return "Column" }
func (i columnItem) FilterValue() string { return i.name }

type sortItem struct{ name string }

func (i sortItem) Title() string       { return i.name }
func (i sortItem) Description() string { return "Sort by this column" }
func (i sortItem) FilterValue() string { return i.name }

// --- VMsView ---

// VMsView implements ui.View for the VM list table.
type VMsView struct {
	vms              table.Model
	actions          list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	activePane       int

	vmData     []core.VM
	visibleVMs []core.VM
	vmCache    map[string]cacheEntry
	costCache  map[string]costCacheEntry
	metricsCache map[string]metricsCacheEntry
	tableCols  []table.Column
	cfg        *config.AppConfig

	activeCtx      core.CloudContext
	sortColumn     string
	sortAsc        bool
	columnOffset   int
	canScrollLeft  bool
	canScrollRight bool
	isSearching    bool
	loading        bool
	breadcrumbs    string
	statusMsg      string

	pendingAction actionItem
	pendingVM     core.VM

	width, height int
	showSidebar   bool
	requestKey    string

	filterTerms []string
	filterLabel string
}

var getProvider = providers.GetProvider

// New creates a new VMsView.
func New(cfg *config.AppConfig) *VMsView {
	return NewFiltered(cfg, nil, "")
}

func NewFiltered(cfg *config.AppConfig, filterTerms []string, filterLabel string) *VMsView {
	// Actions list
	var actionItems []list.Item
	for _, a := range core.VMActions() {
		actionItems = append(actionItems, actionItem{title: a.Title, desc: a.Description})
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actionList := list.New(actionItems, actionDelegate, 30, 15)
	actionList.Title = "Instance Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	// Column config
	var colItems []list.Item
	for _, c := range core.DefaultVMColumns {
		selected := false
		for _, cfgCol := range cfg.VMColumns {
			if cfgCol == c {
				selected = true
				break
			}
		}
		colItems = append(colItems, columnItem{name: c, selected: selected})
	}
	colList := list.New(colItems, list.NewDefaultDelegate(), 0, 0)
	colList.Title = "Configure VM Columns (Space to toggle, Enter to save, Esc to cancel)"
	colList.SetShowStatusBar(false)
	colList.SetFilteringEnabled(false)

	// Sort config
	var sItems []list.Item
	for _, c := range core.DefaultVMColumns {
		sItems = append(sItems, sortItem{name: c})
	}
	sortList := list.New(sItems, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = "Sort VMs by (Enter to select, Esc to cancel)"
	sortList.SetShowStatusBar(false)
	sortList.SetFilteringEnabled(false)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search instances..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	vmTable, tableCols, _, canScrollLeft, canScrollRight := createVMTable(*cfg, 80, 0)

	return &VMsView{
		vms: vmTable, actions: actionList,
		columnConfigList: colList, sortList: sortList,
		descView: vp, searchInput: searchInput,
		activePane: paneTable, tableCols: tableCols,
		cfg: cfg, vmCache: make(map[string]cacheEntry), 
		costCache: make(map[string]costCacheEntry),
		metricsCache: make(map[string]metricsCacheEntry),
		sortColumn: "Name", sortAsc: true,
		canScrollLeft: canScrollLeft, canScrollRight: canScrollRight,
		breadcrumbs: "Select a context to view instances",
		filterTerms: filterTerms,
		filterLabel: filterLabel,
	}
}

func (v *VMsView) Title() string { return "VMs" }

func (v *VMsView) ShortHelp() string {
	return "\u2191\u2193: Nav \u2022 \u2190\u2192: Pan \u2022 c: Cost \u2022 s: SSH \u2022 d: Describe \u2022 ctrl+d: Terminate \u2022 Enter: Menu \u2022 /: Search"
}

func (v *VMsView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneColumnConfig || v.activePane == paneSortConfig || v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneDescribe
}

func (v *VMsView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.vmData = nil
	v.visibleVMs = nil
	v.columnOffset = 0
	v.requestKey = ctx.CacheKey()
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s", ctx.Provider, ctx.DisplayName(), ctx.Region)
	if v.filterLabel != "" {
		v.breadcrumbs = fmt.Sprintf("%s \u203A %s", v.breadcrumbs, v.filterLabel)
	}
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching instances for %s...", ctx.DisplayName())
	v.refreshTable()
	v.vms.Focus()
	return v.fetchVMsCmd(false)
}

func (v *VMsView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.refreshTable()
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.columnConfigList.SetSize(width-4, height-4)
	v.sortList.SetSize(width-4, height-4)
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
}

func (v *VMsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		// Intercept global keys
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneColumnConfig, paneSortConfig:
				v.activePane = paneTable
				if msg.String() == "esc" {
					v.refreshTable()
				}
				return v, nil
			case paneConfirm:
				v.activePane = paneActions
				return v, nil
			}
		}

		// Pane-specific handle logic, but don't return early so components get navigation keys
		switch v.activePane {
		case paneActions:
			_, cmd = v.handleActionKeys(msg)
		case paneConfirm:
			_, cmd = v.handleConfirmKeys(msg)
		case paneDescribe:
			_, cmd = v.handleDescribeKeys(msg)
		case paneColumnConfig:
			_, cmd = v.handleColumnConfigKeys(msg)
		case paneSortConfig:
			_, cmd = v.handleSortConfigKeys(msg)
		case paneTable:
			_, cmd = v.handleTableKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case vmFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.vmData = prepareVMs(msg.vms)
		if msg.err != nil {
			applog.Errorf("component=vms event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.visibleVMs = nil
			v.vms.SetRows([]table.Row{})
		} else {
			applog.Infof("component=vms event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(msg.vms))
			v.sortCurrentVMs()
			v.syncVisibleRows()
			v.statusMsg = fmt.Sprintf("Loaded %d instances.", len(msg.vms))
			if v.anyMetricsColumnEnabled() {
				cmds = append(cmds, v.enrichMetricsCmd(v.vmData))
			}
			if v.cfg.BillingEnabled {
				cmds = append(cmds, v.enrichCostCmd(v.vmData))
			}
		}

	case vmCostEnrichedMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Cost enrichment failed: %v", msg.err)
		} else {
			v.vmData = prepareVMs(msg.vms)
			v.sortCurrentVMs()
			v.syncVisibleRows()
			v.statusMsg = "Loaded instance costs."
		}

	case vmMetricsMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.updateVMMetrics(msg.vmID, msg.metrics, msg.err)
		v.syncVisibleRows()

	case commandCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = msg.output
			v.loading = true
			return v, v.fetchVMsCmd(true)
		}

	case describeCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.descView.SetContent(msg.output)
			v.activePane = paneDescribe
			v.statusMsg = "Viewing details (Esc to close, Up/Down to scroll)."
		}

	case sshCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("SSH failed: %v", msg.err)
			v.descView.SetContent(sshRemediationGuide(v.pendingVM, v.activeCtx))
			v.activePane = paneDescribe
		} else {
			v.statusMsg = "SSH session closed."
		}
		return v, nil

	case finopsRecommendMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Gemini Error: %v", msg.err)
		} else {
			v.descView.SetContent(fmt.Sprintf("GEMINI FINOPS RECOMMENDATION\n\n%s", msg.recommendation))
			v.activePane = paneDescribe
			v.statusMsg = "Viewing FinOps recommendation."
		}

	case tea.WindowSizeMsg:
		v.Resize(msg.Width, msg.Height, v.showSidebar)
	}

	// Route to active component
	switch v.activePane {
	case paneTable:
		v.vms, cmd = v.vms.Update(msg)
		cmds = append(cmds, cmd)
	case paneActions:
		v.actions, cmd = v.actions.Update(msg)
		cmds = append(cmds, cmd)
	case paneDescribe:
		v.descView, cmd = v.descView.Update(msg)
		cmds = append(cmds, cmd)
	case paneColumnConfig:
		v.columnConfigList, cmd = v.columnConfigList.Update(msg)
		cmds = append(cmds, cmd)
	case paneSortConfig:
		v.sortList, cmd = v.sortList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *VMsView) Render() string {
	header := ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)
	tableContent := v.vms.View()

	if v.loading {
		tableContent = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading instances...")
	} else if v.isSearching || v.searchInput.Value() != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(v.searchInput.View()),
			tableContent,
		)
	}

	switch v.activePane {
	case paneColumnConfig:
		return ui.ClampToWindow(v.columnConfigList.View(), v.width, v.height)
	case paneSortConfig:
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	case paneDescribe:
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	case paneActions:
		overlay := ui.OverlayStyle.Render(v.actions.View())
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(tableContent), overlay)
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	case paneConfirm:
		confirmMsg := fmt.Sprintf("Are you sure you want to %s instance %s?", v.pendingAction.title, v.pendingVM.Name)
		confirmStyle := ui.OverlayStyle.Copy().BorderForeground(ui.Alert).Padding(1, 2).Width(50)
		confirmView := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ui.Alert).Bold(true).Render("⚠️  CONFIRM ACTION"),
			"\n", lipgloss.NewStyle().Align(lipgloss.Center).Render(confirmMsg),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Confirm \u2022 Esc: Cancel"),
		)
		overlay := confirmStyle.Render(confirmView)
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(tableContent), overlay)
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", tableContent), v.width, v.height)
}

// --- Key handlers ---

func (v *VMsView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	vm, ok := v.selectedVM()

	switch msg.String() {
	case "enter":
		if !ok { return v, nil }
		v.actions.Title = fmt.Sprintf("Actions: %s", vm.Name)
		v.activePane = paneActions
		v.refreshTable()
	case "d":
		if !ok { return v, nil }
		v.descView.SetContent(core.DescribeVM(vm))
		v.activePane = paneDescribe
	case "c":
		if !ok { return v, nil }
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Fetching cost report for %s...", vm.Name)
		v.loading = true
		return v, executeCostCommandCmd(vm, v.activeCtx, *v.cfg)
	case "s":
		if !ok { return v, nil }
		provider := getProvider(*v.cfg)
		sshCmd, err := provider.GetSSHCmd(context.Background(), vm, v.activeCtx)
		if err != nil || sshCmd == nil {
			v.statusMsg = fmt.Sprintf("SSH not supported for %s", v.activeCtx.Provider)
			v.activePane = paneTable
			return v, nil
		}
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Starting SSH session with %s...", vm.Name)
		return v, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
			return sshCompleteMsg{err: err}
		})
	case "ctrl+d":
		if !ok { return v, nil }
		v.pendingAction = actionItem{title: "Terminate", desc: "Permanently delete the virtual machine"}
		v.pendingVM = vm
		v.activePane = paneConfirm
		v.refreshTable()
	case "/":
		v.isSearching = true
		v.searchInput.Focus()
		v.statusMsg = "Search (Enter/Esc to apply)"
	case "left", "h":
		if v.columnOffset > 0 {
			v.columnOffset--
			v.refreshTable()
			v.statusMsg = "Scrolled columns left."
		}
	case "right", "l":
		if v.canScrollRight {
			v.columnOffset++
			v.refreshTable()
			v.statusMsg = "Scrolled columns right."
		}
	case "r":
		v.loading = true
		v.statusMsg = fmt.Sprintf("Refreshing instances for %s...", v.activeCtx.DisplayName())
		return v, v.fetchVMsCmd(true)
	case "S":
		v.activePane = paneSortConfig
	case "C":
		v.activePane = paneColumnConfig
	}
	return v, nil
}

func (v *VMsView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		v.isSearching = false
		v.searchInput.Blur()
		v.syncVisibleRows()
		v.statusMsg = "Search applied."
		return v, nil
	}
	var cmd tea.Cmd
	v.searchInput, cmd = v.searchInput.Update(msg)
	v.syncVisibleRows()
	return v, cmd
}

func (v *VMsView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		vm, ok := v.selectedVM()
		if ok {
			action := v.actions.SelectedItem().(actionItem)

			if action.title == "Terminate" || action.title == "Delete" {
				v.pendingAction = action
				v.pendingVM = vm
				v.activePane = paneConfirm
				return v, nil
			}
			if action.title == "FinOps" {
				v.activePane = paneTable
				v.statusMsg = fmt.Sprintf("Querying Gemini AI for %s...", vm.Name)
				return v, v.finopsRecommendCmd(vm)
			}
			if action.title == "View Firewalls" {
				filters := vmFirewallFilters(vm, v.activeCtx.Provider)
				if len(filters) == 0 {
					v.activePane = paneTable
					v.statusMsg = fmt.Sprintf("No related firewall metadata available for %s", vm.Name)
					return v, nil
				}
				v.activePane = paneTable
				return v, func() tea.Msg {
					return ui.PushViewMsg{
						View: firewalls.NewFiltered(v.cfg, filters, fmt.Sprintf("VM: %s", vm.Name)),
						Ctx:  v.activeCtx,
					}
				}
			}
			if action.title == "Cost" {
				v.activePane = paneTable
				v.statusMsg = fmt.Sprintf("Fetching cost report for %s...", vm.Name)
				v.loading = true
				return v, executeCostCommandCmd(vm, v.activeCtx, *v.cfg)
			}
			if action.title == "SSH" {
				provider := getProvider(*v.cfg)
				sshCmd, err := provider.GetSSHCmd(context.Background(), vm, v.activeCtx)
				if err != nil || sshCmd == nil {
					v.statusMsg = fmt.Sprintf("SSH not supported for %s", v.activeCtx.Provider)
					v.activePane = paneTable
					return v, nil
				}
				v.activePane = paneTable
				v.statusMsg = fmt.Sprintf("Starting SSH session with %s...", vm.Name)
				return v, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
					return sshCompleteMsg{err: err}
				})
			}
			// Other actions (Start, Stop, Restart, Describe)
			v.activePane = paneTable
			v.statusMsg = fmt.Sprintf("Executing %s on %s...", action.title, vm.Name)
			return v, executeActionCmd(action.title, vm, v.activeCtx, v.cfg)
		}
	}
	return v, nil
}

func (v *VMsView) handleConfirmKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Executing %s on %s...", v.pendingAction.title, v.pendingVM.Name)
		return v, executeActionCmd(v.pendingAction.title, v.pendingVM, v.activeCtx, v.cfg)
	}
	return v, nil
}

func (v *VMsView) handleDescribeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() == "esc" {
		v.activePane = paneTable
	}
	return v, nil
}

func (v *VMsView) handleColumnConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case " ":
		selected := v.columnConfigList.SelectedItem()
		if selected != nil {
			idx := v.columnConfigList.Index()
			item := selected.(columnItem)
			item.selected = !item.selected
			v.columnConfigList.SetItem(idx, item)
		}
		return v, nil
	case "enter":
		var selectedColumns []string
		for _, item := range v.columnConfigList.Items() {
			c := item.(columnItem)
			if c.selected {
				selectedColumns = append(selectedColumns, c.name)
			}
		}
		if len(selectedColumns) == 0 {
			selectedColumns = config.DefaultVMColumns
		}
		v.cfg.VMColumns = selectedColumns
		config.Save(*v.cfg)
		v.refreshTable()
		v.activePane = paneTable
		v.statusMsg = "Columns saved."
		if v.anyMetricsColumnEnabled() {
			return v, v.enrichMetricsCmd(v.vmData)
		}
		return v, nil
	}
	return v, nil
}

func (v *VMsView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		selected := v.sortList.SelectedItem()
		if selected != nil {
			sItem := selected.(sortItem)
			if v.sortColumn == sItem.name {
				v.sortAsc = !v.sortAsc
			} else {
				v.sortColumn = sItem.name
				v.sortAsc = true
			}
			v.sortCurrentVMs()
			v.syncVisibleRows()
			v.activePane = paneTable
			dir := "asc"
			if !v.sortAsc {
				dir = "desc"
			}
			v.statusMsg = fmt.Sprintf("Sorted by %s (%s)", v.sortColumn, dir)
		}
		return v, nil
	}
	return v, nil
}

// --- Table helpers ---

func (v *VMsView) refreshTable() {
	if v.width == 0 {
		return
	}

	availWidth := v.width
	if v.activePane == paneActions || v.activePane == paneConfirm {
		availWidth = v.width - 55
		if availWidth < 40 {
			availWidth = 40
		}
	}

	cursor := v.vms.Cursor()
	focused := v.vms.Focused()
	newVMs, newCols, nextOffset, canScrollLeft, canScrollRight := createVMTable(*v.cfg, availWidth, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	newVMs.SetHeight(ui.TableHeight(v.height))
	newVMs.SetWidth(ui.TableViewportWidth(availWidth))
	rows := mapVMsToRows(v.visibleRowsSource(), newCols)
	newVMs.SetRows(rows)
	if cursor >= 0 && cursor < len(rows) {
		newVMs.SetCursor(cursor)
	}
	if focused {
		newVMs.Focus()
	}
	v.vms = newVMs
	v.tableCols = newCols
}

func (v *VMsView) syncVisibleRows() {
	v.visibleVMs = filterVMs(v.vmData, v.searchInput.Value(), v.filterTerms)
	rows := mapVMsToRows(v.visibleVMs, v.tableCols)
	v.vms.SetRows(rows)
	if len(rows) == 0 {
		v.vms.SetCursor(0)
		return
	}
	if cursor := v.vms.Cursor(); cursor >= len(rows) {
		v.vms.SetCursor(len(rows) - 1)
	}
}

func (v *VMsView) visibleRowsSource() []core.VM {
	if v.visibleVMs != nil {
		return v.visibleVMs
	}
	return filterVMs(v.vmData, v.searchInput.Value(), v.filterTerms)
}

func (v *VMsView) selectedVM() (core.VM, bool) {
	cursor := v.vms.Cursor()
	if cursor < 0 || cursor >= len(v.visibleVMs) {
		return core.VM{}, false
	}
	return v.visibleVMs[cursor], true
}

func filterVMs(vms []core.VM, query string, filters []string) []core.VM {
	var filtered []core.VM
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))

	for _, vm := range vms {
		// Apply hard filters first (from drill-down)
		if len(filters) > 0 {
			matched := false
			for _, f := range filters {
				f = strings.ToLower(f)
				if strings.Contains(strings.ToLower(vm.ID), f) ||
					strings.Contains(strings.ToLower(vm.Name), f) ||
					strings.Contains(strings.ToLower(vm.Network), f) ||
					strings.Contains(strings.ToLower(vm.Subnet), f) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Apply search query
		if normalizedQuery != "" {
			if !strings.Contains(strings.ToLower(vm.Name), normalizedQuery) &&
				!strings.Contains(strings.ToLower(vm.ID), normalizedQuery) &&
				!strings.Contains(strings.ToLower(vm.PrivateIP), normalizedQuery) &&
				!strings.Contains(strings.ToLower(vm.PublicIP), normalizedQuery) &&
				!strings.Contains(strings.ToLower(vm.Labels), normalizedQuery) {
				continue
			}
		}
		filtered = append(filtered, vm)
	}
	return filtered
}

func createVMTable(cfg config.AppConfig, availableWidth int, offset int) (table.Model, []table.Column, int, bool, bool) {
	columns := buildVMColumns(cfg)
	return ui.NewResourceTable(columns, availableWidth, offset, "Name")
}

func buildVMColumns(cfg config.AppConfig) []table.Column {
	var columns []table.Column
	preferredWidths := map[string]int{
		"Name": 22, "Instance ID": 18, "Type": 12, "State": 10,
		"Private IP": 15, "Public IP": 15, "Zone": 14,
		"Resource Group": 18, "Network": 14, "Subnet": 14, "Labels": 24,
		"Security Groups": 20,
		"CPU %": 8, "Memory %": 10, "Disk Read": 12, "Disk Write": 12, "Net In": 12, "Net Out": 12,
		"Cost": 10, "Cost Trend": 10, "Recommendation": 25, "Est. Savings": 12,
	}
	for _, col := range config.SanitizeVMColumns(cfg.VMColumns) {
		width := preferredWidths[col]
		if width == 0 {
			width = 12
		}
		columns = append(columns, table.Column{Title: col, Width: width})
	}
	return columns
}

func mapVMsToRows(vms []core.VM, columns []table.Column) []table.Row {
	var rows []table.Row
	for _, vm := range vms {
		var row []string
		for _, col := range columns {
			val := vm.GetField(col.Title)
			if col.Title == "Name" {
				icon := getStatusIcon(vm)
				if icon != "" {
					val = icon + " " + val
				}
			}
			row = append(row, ui.TruncateText(val, col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	return rows
}

func getStatusIcon(vm core.VM) string {
	if vm.CPUPercent == "-" && vm.MemoryPercent == "-" {
		return ""
	}
	cpu := parsePercent(vm.CPUPercent)
	mem := parsePercent(vm.MemoryPercent)

	if cpu > 90 || mem > 90 {
		return "\U0001f534" // Red circle
	}
	if cpu > 70 || mem > 70 {
		return "\U0001f7e1" // Yellow circle
	}
	if cpu > 0 || mem > 0 {
		return "\U0001f7e2" // Green circle
	}
	return ""
}

func parsePercent(s string) float64 {
	if s == "-" || s == "" {
		return 0
	}
	var val float64
	fmt.Sscanf(s, "%f%%", &val)
	return val
}

func sortVMs(vms []core.VM, column string, asc bool) {
	sortSlice(vms, func(i, j int) bool {
		return compareVMs(vms[i], vms[j], column, asc)
	})
}

func sortSlice(vms []core.VM, less func(i, j int) bool) {
	// Simple insertion sort (fine for typical VM counts)
	for i := 1; i < len(vms); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			vms[j], vms[j-1] = vms[j-1], vms[j]
		}
	}
}

func (v *VMsView) sortCurrentVMs() {
	sortVMs(v.vmData, v.sortColumn, v.sortAsc)
}

func prepareVMs(vms []core.VM) []core.VM {
	prepared := make([]core.VM, len(vms))
	copy(prepared, vms)
	for i := range prepared {
		if strings.TrimSpace(prepared[i].MonthlyCost) == "" {
			prepared[i].MonthlyCost = "-"
		}
		if strings.TrimSpace(prepared[i].CostTrend) == "" {
			prepared[i].CostTrend = "-"
		}
		prepared[i].CPUPercent = "-"
		prepared[i].MemoryPercent = "-"
		prepared[i].DiskIORead = "-"
		prepared[i].DiskIOWrite = "-"
		prepared[i].NetworkIn = "-"
		prepared[i].NetworkOut = "-"
		prepared[i].Recommendation = "-"
		prepared[i].EstSavings = "-"
	}
	return prepared
}

func compareVMs(left, right core.VM, column string, asc bool) bool {
	leftRank := vmStateRank(left.State)
	rightRank := vmStateRank(right.State)
	if leftRank != rightRank {
		return leftRank < rightRank
	}

	valueCmp := compareVMField(left, right, column)
	if valueCmp != 0 {
		if asc {
			return valueCmp < 0
		}
		return valueCmp > 0
	}

	nameCmp := strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	if nameCmp != 0 {
		return nameCmp < 0
	}
	return strings.Compare(strings.ToLower(left.ID), strings.ToLower(right.ID)) < 0
}

func vmFirewallFilters(vm core.VM, provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gcp":
		if strings.TrimSpace(vm.Network) == "" || strings.TrimSpace(vm.Network) == "-" {
			return nil
		}
		return []string{vm.Network}
	default:
		return splitNonEmpty(vm.SecurityGroups)
	}
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func vmStateRank(state string) int {
	if isRunningState(state) {
		return 0
	}
	return 1
}

func isRunningState(state string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(state)), "running")
}

func compareVMField(left, right core.VM, column string) int {
	return strings.Compare(strings.ToLower(left.GetField(column)), strings.ToLower(right.GetField(column)))
}

// --- Commands ---

func (v *VMsView) anyMetricsColumnEnabled() bool {
	for _, col := range v.cfg.VMColumns {
		switch col {
		case "CPU %", "Memory %", "Disk Read", "Disk Write", "Net In", "Net Out":
			return true
		}
	}
	return false
}

func (v *VMsView) updateVMMetrics(vmID string, metrics *core.VMMetrics, err error) {
	if err != nil {
		applog.Errorf("component=vms event=metrics_failed vm_id=%s err=%v", vmID, err)
		return
	}
	if metrics == nil {
		return
	}

	for i := range v.vmData {
		if v.vmData[i].ID == vmID {
			if metrics.CPUUtilization != nil {
				v.vmData[i].CPUPercent = core.FormatMetricValue(metrics.CPUUtilization.Current, metrics.CPUUtilization.Unit)
			}
			if metrics.MemoryUtilization != nil {
				v.vmData[i].MemoryPercent = core.FormatMetricValue(metrics.MemoryUtilization.Current, metrics.MemoryUtilization.Unit)
			}
			if metrics.DiskReadBytes != nil {
				v.vmData[i].DiskIORead = core.FormatMetricValue(metrics.DiskReadBytes.Current, metrics.DiskReadBytes.Unit)
			}
			if metrics.DiskWriteBytes != nil {
				v.vmData[i].DiskIOWrite = core.FormatMetricValue(metrics.DiskWriteBytes.Current, metrics.DiskWriteBytes.Unit)
			}
			if metrics.NetworkInBytes != nil {
				v.vmData[i].NetworkIn = core.FormatMetricValue(metrics.NetworkInBytes.Current, metrics.NetworkInBytes.Unit)
			}
			if metrics.NetworkOutBytes != nil {
				v.vmData[i].NetworkOut = core.FormatMetricValue(metrics.NetworkOutBytes.Current, metrics.NetworkOutBytes.Unit)
			}
			break
		}
	}
}

func (v *VMsView) enrichMetricsCmd(vms []core.VM) tea.Cmd {
	var cmds []tea.Cmd
	for _, vm := range vms {
		cmds = append(cmds, v.fetchSingleVMMetricsCmd(vm))
	}
	return tea.Batch(cmds...)
}

func (v *VMsView) fetchSingleVMMetricsCmd(vm core.VM) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	metricsTTL := 15 * time.Minute // TODO: from config

	return func() tea.Msg {
		provider := getProvider(*v.cfg)
		metricsProvider, ok := provider.(providers.MetricsProvider)
		if !ok {
			return nil
		}

		cacheKey := vm.ID
		if entry, ok := v.metricsCache[cacheKey]; ok && time.Since(entry.timestamp) < metricsTTL {
			return vmMetricsMsg{requestKey: requestKey, vmID: vm.ID, metrics: entry.metrics}
		}

		m, err := metricsProvider.FetchVMMetrics(context.Background(), vm, activeCtx, 24*time.Hour)
		if err == nil && m != nil {
			v.metricsCache[cacheKey] = metricsCacheEntry{metrics: m, timestamp: time.Now()}
		}
		return vmMetricsMsg{requestKey: requestKey, vmID: vm.ID, metrics: m, err: err}
	}
}

func (v *VMsView) fetchVMsCmd(force bool) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	mode := strings.ToUpper(v.cfg.Backend)
	return func() tea.Msg {
		applog.Infof("component=vms event=fetch_start provider=%s account=%s region=%s mode=%s force=%t", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, force)
		cacheKey := activeCtx.CacheKey()
		ttl := time.Duration(v.cfg.CacheTTL) * time.Minute
		if !force {
			if entry, ok := v.vmCache[cacheKey]; ok {
				if time.Since(entry.timestamp) < ttl {
					return vmFetchMsg{requestKey: requestKey, vms: entry.vms}
				}
			}
		}
		provider := getProvider(*v.cfg)
		rows, err := provider.FetchVMs(context.Background(), activeCtx)
		if err != nil {
			return vmFetchMsg{requestKey: requestKey, err: err}
		}
		v.vmCache[cacheKey] = cacheEntry{vms: rows, timestamp: time.Now()}
		return vmFetchMsg{requestKey: requestKey, vms: rows}
	}
}

func (v *VMsView) enrichCostCmd(vms []core.VM) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	billingTTL := time.Duration(v.cfg.BillingCacheTTL) * time.Minute
	return func() tea.Msg {
		provider := getProvider(*v.cfg)
		billingProvider, ok := provider.(providers.BillingProvider)
		if !ok {
			return vmCostEnrichedMsg{requestKey: requestKey, vms: prepareVMs(vms)}
		}

		enrichedVMs := prepareVMs(vms)

		// 1. Fetch recommendations (once per context)
		recs, _ := billingProvider.FetchRecommendations(context.Background(), activeCtx)
		recMap := make(map[string]core.Recommendation)
		for _, r := range recs {
			// Extract ID from full resource ID if needed (for Azure)
			id := r.ResourceID
			if strings.Contains(id, "/") {
				parts := strings.Split(id, "/")
				id = parts[len(parts)-1]
			}
			recMap[id] = r
		}

		sem := make(chan struct{}, 8)
		var costCacheMu sync.Mutex

		var wg sync.WaitGroup
		for i := range enrichedVMs {
			// Match recommendation
			vmID := enrichedVMs[i].ID
			if r, ok := recMap[vmID]; ok {
				enrichedVMs[i].Recommendation = r.Summary
				enrichedVMs[i].EstSavings = core.FormatCost(r.EstimatedSavings)
			}

			cacheKey := vmCostCacheKey(activeCtx, enrichedVMs[i].ID)
			if entry, ok := v.costCache[cacheKey]; ok && time.Since(entry.timestamp) < billingTTL {
				enrichedVMs[i].MonthlyCost = entry.monthlyCost
				enrichedVMs[i].CostTrend = entry.costTrend
				continue
			}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				vm := enrichedVMs[idx]
				cost, err := billingProvider.FetchResourceCost(context.Background(), vm.ID, activeCtx)
				if err == nil && cost != nil {
					monthlyCost := core.FormatCost(cost.CurrentMonthCost)
					costTrend := core.CostChangePercent(cost.CurrentMonthCost, cost.PreviousMonthCost)
					enrichedVMs[idx].MonthlyCost = monthlyCost
					enrichedVMs[idx].CostTrend = costTrend
					costCacheMu.Lock()
					v.costCache[cacheKey] = costCacheEntry{
						monthlyCost: monthlyCost,
						costTrend:   costTrend,
						timestamp:   time.Now(),
					}
					costCacheMu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		return vmCostEnrichedMsg{requestKey: requestKey, vms: enrichedVMs}
	}
}

func executeActionCmd(action string, vm core.VM, cloudCtx core.CloudContext, cfg *config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		provider := getProvider(*cfg)
		output, err := provider.ExecuteAction(context.Background(), action, vm, cloudCtx)
		if action == "Describe" {
			return describeCompleteMsg{output: output, err: err}
		}
		return commandCompleteMsg{output: output, err: err}
	}
}

func vmCostCacheKey(cloudCtx core.CloudContext, resourceID string) string {
	return fmt.Sprintf("%s|%s", cloudCtx.CacheKey(), resourceID)
}

func buildCostCommand(vm core.VM, cloudCtx core.CloudContext, cfg config.AppConfig) string {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)

	switch cloudCtx.Provider {
	case "AWS":
		return fmt.Sprintf("aws --no-cli-pager ce get-cost-and-usage --time-period Start=%s,End=%s --granularity MONTHLY --metrics UnblendedCost --filter '{\"Dimensions\":{\"Key\":\"RESOURCE_ID\",\"Values\":[\"%s\"]}}' --profile %s --region us-east-1 --output table", start.Format("2006-01-02"), end.Format("2006-01-02"), vm.ID, cloudCtx.AuthRef())
	case "GCP":
		billingTable := gcpBillingTablePath(cloudCtx, cfg)
		globalName := gcpVMGlobalName(vm, cloudCtx)
		return fmt.Sprintf("bq query --use_legacy_sql=false --format=prettyjson 'SELECT service.description AS service, sku.description AS sku, SUM(cost) AS cost FROM `%s` WHERE usage_start_time >= TIMESTAMP(\"%s\") AND usage_start_time < TIMESTAMP(\"%s\") AND (resource.name = \"%s\" OR resource.global_name = \"%s\") GROUP BY service, sku ORDER BY cost DESC'", billingTable, start.Format("2006-01-02"), end.Format("2006-01-02"), vm.Name, globalName)
	case "Azure":
		scope := fmt.Sprintf("/subscriptions/%s", cloudCtx.AccountID)
		return fmt.Sprintf("az rest --method post --url \"https://management.azure.com%s/providers/Microsoft.CostManagement/query?api-version=2025-03-01\" --body '{\"type\":\"ActualCost\",\"timeframe\":\"MonthToDate\",\"dataset\":{\"granularity\":\"None\",\"filter\":{\"dimensions\":{\"name\":\"ResourceId\",\"operator\":\"In\",\"values\":[\"%s\"]}},\"aggregation\":{\"totalCost\":{\"name\":\"PreTaxCost\",\"function\":\"Sum\"}},\"grouping\":[{\"type\":\"Dimension\",\"name\":\"ServiceName\"}]}}' --output table", scope, vm.ID)
	default:
		return "echo 'Cost lookup not supported for this provider.'"
	}
}

func executeCostCommandCmd(vm core.VM, cloudCtx core.CloudContext, cfg config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		cmdStr := buildCostCommand(vm, cloudCtx, cfg)
		cmd := exec.Command("bash", "-c", cmdStr)
		output, err := cmd.CombinedOutput()
		
		formattedOutput := fmt.Sprintf("COST REPORT FOR %s (%s)\n\n$ %s\n\n%s", vm.Name, cloudCtx.Provider, cmdStr, string(output))
		
		return describeCompleteMsg{output: formattedOutput, err: err}
	}
}

func sshRemediationGuide(vm core.VM, cloudCtx core.CloudContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "SSH CONNECTION FAILED\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", cloudCtx.Provider)
	fmt.Fprintf(&b, "VM: %s\n", vm.Name)
	fmt.Fprintf(&b, "ID: %s\n\n", vm.ID)

	switch cloudCtx.Provider {
	case "GCP":
		network := vm.Network
		if network == "-" || network == "" {
			network = "default"
		}
		fmt.Fprintf(&b, "GCP uses Identity-Aware Proxy (IAP) for SSH when public IPs are missing or blocked.\n")
		fmt.Fprintf(&b, "If your connection timed out or failed due to permissions, run these commands:\n\n")
		fmt.Fprintf(&b, "1. Add IAP Firewall Rule (allows Google's IAP IPs to reach port 22):\n")
		fmt.Fprintf(&b, "   gcloud compute firewall-rules create allow-ssh-ingress-from-iap \\\n")
		fmt.Fprintf(&b, "     --direction=INGRESS --action=allow --rules=tcp:22 \\\n")
		fmt.Fprintf(&b, "     --source-ranges=35.235.240.0/20 \\\n")
		fmt.Fprintf(&b, "     --network=%s --project=%s\n\n", network, cloudCtx.AccountID)
		fmt.Fprintf(&b, "2. Grant yourself the IAP Tunnel Accessor role:\n")
		fmt.Fprintf(&b, "   gcloud projects add-iam-policy-binding %s \\\n", cloudCtx.AccountID)
		fmt.Fprintf(&b, "     --member=\"user:YOUR_EMAIL@domain.com\" \\\n")
		fmt.Fprintf(&b, "     --role=\"roles/iap.tunnelResourceAccessor\"\n")
	case "AWS":
		profileFlag := ""
		if cloudCtx.CredentialProfile != "" {
			profileFlag = fmt.Sprintf(" --profile %s", cloudCtx.CredentialProfile)
		}
		fmt.Fprintf(&b, "AWS relies on SSM Session Manager for private instances or EC2 Instance Connect.\n")
		fmt.Fprintf(&b, "If SSM is missing, you must attach the 'AmazonSSMManagedInstanceCore' IAM policy.\n\n")
		fmt.Fprintf(&b, "1. Create an IAM role for EC2 with SSM permissions:\n")
		fmt.Fprintf(&b, "   aws iam create-role --role-name EC2-SSM-Role \\\n")
		fmt.Fprintf(&b, "     --assume-role-policy-document '{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":{\"Service\":\"ec2.amazonaws.com\"},\"Action\":\"sts:AssumeRole\"}]}'%s\n\n", profileFlag)
		fmt.Fprintf(&b, "   aws iam attach-role-policy --role-name EC2-SSM-Role \\\n")
		fmt.Fprintf(&b, "     --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore%s\n\n", profileFlag)
		fmt.Fprintf(&b, "2. Attach to the instance:\n")
		fmt.Fprintf(&b, "   aws iam create-instance-profile --instance-profile-name EC2-SSM-Role-Profile%s\n", profileFlag)
		fmt.Fprintf(&b, "   aws iam add-role-to-instance-profile --instance-profile-name EC2-SSM-Role-Profile --role-name EC2-SSM-Role%s\n", profileFlag)
		fmt.Fprintf(&b, "   aws ec2 associate-iam-instance-profile --instance-id %s --iam-instance-profile Name=EC2-SSM-Role-Profile --region %s%s\n", vm.ID, cloudCtx.Region, profileFlag)
	case "Azure":
		fmt.Fprintf(&b, "Azure SSH typically requires Azure Bastion for private VMs, or an open NSG port for public VMs.\n\n")
		fmt.Fprintf(&b, "To temporarily allow your current IP address via NSG:\n")
		fmt.Fprintf(&b, "   az network nsg rule create --resource-group %s \\\n", vm.ResourceGroup)
		fmt.Fprintf(&b, "     --nsg-name YOUR_NSG_NAME --name Allow-SSH-Temp \\\n")
		fmt.Fprintf(&b, "     --protocol Tcp --direction Inbound --priority 100 \\\n")
		fmt.Fprintf(&b, "     --source-address-prefix $(curl -s ifconfig.me) \\\n")
		fmt.Fprintf(&b, "     --source-port-range '*' --destination-address-prefix '*' \\\n")
		fmt.Fprintf(&b, "     --destination-port-range 22 --access Allow \\\n")
		fmt.Fprintf(&b, "     --subscription %s\n", cloudCtx.AccountID)
	case "DigitalOcean":
		fmt.Fprintf(&b, "DigitalOcean instances require port 22 to be open on your Cloud Firewall.\n\n")
		fmt.Fprintf(&b, "1. Verify your local SSH key is added to the Droplet.\n")
		fmt.Fprintf(&b, "2. Check your Cloud Firewalls:\n")
		fmt.Fprintf(&b, "   doctl compute firewall list\n")
		fmt.Fprintf(&b, "   # Ensure port 22 is permitted for your current IP.\n")
	default:
		fmt.Fprintf(&b, "No provider-specific troubleshooting is available.\n")
	}

	return b.String()
}

func gcpBillingTablePath(cloudCtx core.CloudContext, cfg config.AppConfig) string {
	datasetProject := cloudCtx.AccountID
	datasetName := "YOUR_BILLING_EXPORT_DATASET"
	if raw := strings.TrimSpace(cfg.GCPBillingDataset); raw != "" {
		parts := strings.Split(raw, ".")
		if len(parts) == 2 {
			datasetProject = parts[0]
			datasetName = parts[1]
		} else {
			datasetName = raw
		}
	} else {
		datasetProject = "YOUR_BILLING_EXPORT_PROJECT"
	}
	return fmt.Sprintf("%s.%s.gcp_billing_export_resource_v1_*", datasetProject, datasetName)
}

func gcpVMGlobalName(vm core.VM, cloudCtx core.CloudContext) string {
	if vm.Zone == "" || vm.Zone == "-" {
		return fmt.Sprintf("//compute.googleapis.com/projects/%s/instances/%s", cloudCtx.AccountID, vm.Name)
	}
	return fmt.Sprintf("//compute.googleapis.com/projects/%s/zones/%s/instances/%s", cloudCtx.AccountID, vm.Zone, vm.Name)
}

func (v *VMsView) finopsRecommendCmd(vm core.VM) tea.Cmd {
	activeCtx := v.activeCtx
	cfg := *v.cfg
	return func() tea.Msg {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return finopsRecommendMsg{err: fmt.Errorf("GEMINI_API_KEY not set")}
		}

		// 1. Gather all data (cached or fresh)
		var metrics *core.VMMetrics
		if entry, ok := v.metricsCache[vm.ID]; ok {
			metrics = entry.metrics
		}

		// Use a dedicated context for the Gemini call
		ctxAI, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client, err := genai.NewClient(ctxAI, option.WithAPIKey(apiKey))
		if err != nil {
			return finopsRecommendMsg{err: fmt.Errorf("failed to create Gemini client: %w", err)}
		}
		defer client.Close()

		model := client.GenerativeModel(cfg.GeminiModel)
		model.SetTemperature(0.2)

		// Build Metrics section
		metricsSection := "UTILIZATION METRICS: Not available."
		if metrics != nil {
			cpuAvg, cpuMax, cpuCur := 0.0, 0.0, 0.0
			if metrics.CPUUtilization != nil {
				cpuAvg, cpuMax, cpuCur = metrics.CPUUtilization.Average, metrics.CPUUtilization.Maximum, metrics.CPUUtilization.Current
			}
			memAvg, memMax, memCur := 0.0, 0.0, 0.0
			if metrics.MemoryUtilization != nil {
				memAvg, memMax, memCur = metrics.MemoryUtilization.Average, metrics.MemoryUtilization.Maximum, metrics.MemoryUtilization.Current
			}
			metricsSection = fmt.Sprintf(`=== UTILIZATION METRICS (Last 24h) ===
CPU:    Avg=%.1f%% | Max=%.1f%% | Current=%.1f%%
Memory: Avg=%.1f%% | Max=%.1f%% | Current=%.1f%%
Disk Read:  %s | Disk Write: %s
Net In:     %s | Net Out:    %s`,
				cpuAvg, cpuMax, cpuCur,
				memAvg, memMax, memCur,
				vm.DiskIORead, vm.DiskIOWrite,
				vm.NetworkIn, vm.NetworkOut)
		}

		// Build Cost section
		costSection := fmt.Sprintf(`=== COST DATA ===
Current Month (MTD): %s
Cost Trend:          %s`, vm.MonthlyCost, vm.CostTrend)

		// Build Recommendation section
		recSection := fmt.Sprintf(`=== PROVIDER RECOMMENDATION ===
Finding:           %s
Estimated Savings: %s`, vm.Recommendation, vm.EstSavings)

		prompt := fmt.Sprintf(`You are an expert Cloud FinOps Architect advising a DevOps Engineer using a CLI tool.

=== VM METADATA ===
Provider: %s | Name: %s | ID: %s
Instance Type: %s | State: %s | Zone: %s
Network: %s / Subnet: %s | Labels: %s

%s

%s

%s

=== YOUR TASK ===
Based on the metrics, cost, and provider recommendations above, provide a comprehensive FinOps analysis.

1. **Rightsizing Verdict:** Is this VM over-provisioned, under-provisioned, or optimized? Cite the specific CPU/Memory averages and Maximums.
2. **Specific Action & Savings:** Recommend a concrete new instance type/size if applicable, or state if it should be terminated/stopped. Estimate the savings.
3. **Commitment Strategy:** Should this workload be covered by a Reserved Instance, Savings Plan, or Committed Use Discount based on its uptime?
4. **Execution Plan (CLI):** Provide the exact CLI command (e.g., 'aws ec2 modify-instance-attribute', 'gcloud compute instances set-machine-type', 'az vm resize') the user should run to apply your primary recommendation.

Be concise but highly technical. Use markdown formatting (bolding, lists, code blocks for CLI commands).`,
			activeCtx.Provider, vm.Name, vm.ID, vm.Type, vm.State, vm.Zone,
			vm.Network, vm.Subnet, vm.Labels,
			metricsSection, costSection, recSection)

		resp, err := model.GenerateContent(ctxAI, genai.Text(prompt))
		if err != nil {
			return finopsRecommendMsg{err: fmt.Errorf("Gemini API error: %w", err)}
		}
		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return finopsRecommendMsg{err: fmt.Errorf("empty response from Gemini")}
		}
		if text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return finopsRecommendMsg{recommendation: string(text)}
		}
		return finopsRecommendMsg{err: fmt.Errorf("unexpected response format")}
	}
}
