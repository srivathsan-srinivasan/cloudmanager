package vms

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/vyoogam/cloudmanager/internal/access"
	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	"github.com/vyoogam/cloudmanager/internal/iac"
	"github.com/vyoogam/cloudmanager/internal/localdb"
	applog "github.com/vyoogam/cloudmanager/internal/logging"
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/ui"
	"github.com/vyoogam/cloudmanager/internal/views/firewalls"
)

const (
	paneTable = iota
	paneActions
	paneAccess
	paneDescribe
	paneColumnConfig
	paneSortConfig
	paneConfirm
	paneTag
)

// --- Bubble Tea messages ---

type vmFetchMsg struct {
	requestKey string
	vms        []core.VM
	fromCache  bool
	err        error
}
type vmCostEnrichedMsg struct {
	requestKey   string
	vms          []core.VM
	cacheUpdates map[string]costCacheEntry
	err          error
}
type commandCompleteMsg struct {
	output string
	err    error
}
type describeCompleteMsg struct {
	output     string
	consoleURL string
	err        error
}
type accessResolvedMsg struct {
	vm      core.VM
	methods []core.AccessMethod
}
type sshCompleteMsg struct{ err error }
type clipboardCompleteMsg struct {
	text string
	err  error
}
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

type accessItem struct {
	method core.AccessMethod
}

func (i accessItem) Title() string {
	if !i.method.Available {
		return "[unavailable] " + i.method.Label
	}
	return i.method.Label
}

func (i accessItem) Description() string {
	if !i.method.Available {
		return i.method.Reason
	}
	if strings.TrimSpace(i.method.CopyText) != "" {
		return i.method.CopyText
	}
	return "Enter to open, c to copy"
}

func (i accessItem) FilterValue() string { return i.method.Label + " " + i.method.CopyText }

type keyFileItem struct {
	path string
}

func (i keyFileItem) Title() string       { return filepath.Base(i.path) }
func (i keyFileItem) Description() string { return i.path }
func (i keyFileItem) FilterValue() string { return i.path }

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
	accessMethods    list.Model
	keyFileList      list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	tagInput         textinput.Model
	accessUserInput  textinput.Model
	activePane       int

	vmData       []core.VM
	visibleVMs   []core.VM
	vmCache      map[string]cacheEntry
	costCache    map[string]costCacheEntry
	metricsCache map[string]metricsCacheEntry
	tableCols    []table.Column
	cfg          *config.AppConfig

	activeCtx      core.CloudContext
	sortColumn     string
	sortAsc        bool
	sortHeader     ui.HeaderSortState
	columnOffset   int
	canScrollLeft  bool
	canScrollRight bool
	isSearching    bool
	loading        bool
	breadcrumbs    string
	statusMsg      string
	copyableText   string
	detailURL      string

	pendingAction     actionItem
	pendingVM         core.VM
	pendingAccess     []core.AccessMethod
	accessParent      []core.AccessMethod
	accessMode        string
	editingAccessUser bool
	keyDropdownOpen   bool
	selectedSSHKey    string
	selectedSSHIPKind string

	width, height int
	showSidebar   bool
	requestKey    string

	filterTerms         []string
	filterLabel         string
	showKubernetesNodes bool
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

	accessDelegate := list.NewDefaultDelegate()
	accessDelegate.ShowDescription = true
	accessList := list.New(nil, accessDelegate, 50, 15)
	accessList.Title = "Access Methods"
	accessList.SetShowStatusBar(false)
	accessList.SetFilteringEnabled(false)
	keyDelegate := list.NewDefaultDelegate()
	keyDelegate.ShowDescription = true
	keyList := list.New(nil, keyDelegate, 56, 10)
	keyList.Title = "Select SSH key"
	keyList.SetShowStatusBar(false)
	keyList.SetFilteringEnabled(true)

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
	vp.Style = lipgloss.NewStyle()

	searchInput := textinput.New()
	searchInput.Placeholder = "Search instances..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30
	tagInput := textinput.New()
	tagInput.Placeholder = "VFWEB,prod-web"
	tagInput.Prompt = "tags> "
	tagInput.CharLimit = 160
	tagInput.Width = 48
	accessUserInput := textinput.New()
	accessUserInput.Placeholder = "ubuntu"
	accessUserInput.Prompt = "user> "
	accessUserInput.CharLimit = 64
	accessUserInput.Width = 32

	vmTable, tableCols, _, canScrollLeft, canScrollRight := createVMTable(*cfg, 80, 0)

	return &VMsView{
		vms: vmTable, actions: actionList, accessMethods: accessList, keyFileList: keyList,
		columnConfigList: colList, sortList: sortList,
		descView: vp, searchInput: searchInput, tagInput: tagInput, accessUserInput: accessUserInput,
		activePane: paneTable, tableCols: tableCols,
		cfg: cfg, vmCache: make(map[string]cacheEntry),
		costCache:    make(map[string]costCacheEntry),
		metricsCache: make(map[string]metricsCacheEntry),
		sortColumn:   "Name", sortAsc: true,
		canScrollLeft: canScrollLeft, canScrollRight: canScrollRight,
		breadcrumbs:         "Select a context to view instances",
		filterTerms:         filterTerms,
		filterLabel:         filterLabel,
		showKubernetesNodes: !cfg.HideKubernetesNodes,
	}
}

func (v *VMsView) Title() string { return "VMs" }

func (v *VMsView) ShortHelp() string {
	return "\u2191\u2193: Nav \u2022 ↑ at top: Columns \u2022 \u2190\u2192: Pan \u2022 Enter: Sort/Menu \u2022 K:K8s nodes \u2022 t: Tag \u2022 s: SSH \u2022 d: Describe \u2022 /: Search"
}

func (v *VMsView) SetSearchQuery(query string) {
	v.searchInput.SetValue(query)
	v.isSearching = false
	v.searchInput.Blur()
	v.syncVisibleRows()
}

func (v *VMsView) SetKubernetesNodesVisible(show bool) {
	v.showKubernetesNodes = show
	v.syncVisibleRows()
}

func (v *VMsView) IsInputActive() bool {
	return v.sortHeader.Active || v.isSearching || v.activePane == paneColumnConfig || v.activePane == paneSortConfig || v.activePane == paneActions || v.activePane == paneAccess || v.activePane == paneConfirm || v.activePane == paneDescribe || v.activePane == paneTag
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
	v.copyableText = ""
	v.detailURL = ""
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
	v.descView.Width = width
	v.descView.Height = height
	v.columnConfigList.SetSize(width-4, height-4)
	v.sortList.SetSize(width-4, height-4)
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
	v.accessMethods.SetSize(64, ui.ActionListHeight(len(v.accessMethods.Items()), height))
	v.keyFileList.SetSize(56, ui.ActionListHeight(len(v.keyFileList.Items()), height))
}

func (v *VMsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.sortHeader.Active {
			return v.handleHeaderSortKeys(msg)
		}
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		// Intercept global keys
		if msg.String() == "esc" {
			switch v.activePane {
			case paneAccess:
				if v.accessMode == "keys" {
					if v.keyDropdownOpen {
						v.keyDropdownOpen = false
					} else {
						v.restoreAccessParent()
					}
					return v, nil
				}
				v.activePane = paneTable
				v.refreshTable()
				return v, nil
			case paneActions, paneDescribe, paneColumnConfig, paneSortConfig, paneTag:
				v.activePane = paneTable
				v.tagInput.Blur()
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
		case paneAccess:
			_, cmd = v.handleAccessKeys(msg)
		case paneConfirm:
			_, cmd = v.handleConfirmKeys(msg)
		case paneDescribe:
			_, cmd = v.handleDescribeKeys(msg)
		case paneColumnConfig:
			_, cmd = v.handleColumnConfigKeys(msg)
		case paneSortConfig:
			_, cmd = v.handleSortConfigKeys(msg)
		case paneTag:
			_, cmd = v.handleTagKeys(msg)
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
		if msg.err != nil {
			applog.Errorf("component=vms event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.vmData = nil
			v.visibleVMs = nil
			v.vms.SetRows([]table.Row{})
		} else {
			taggedVMs := iac.ApplyTerraformToVMs(v.cfg.TerraformStatePaths, v.activeCtx, config.ApplyResourceTagsToVMs(*v.cfg, v.activeCtx, msg.vms))
			if !msg.fromCache {
				v.vmCache[v.activeCtx.CacheKey()] = cacheEntry{vms: taggedVMs, timestamp: time.Now()}
			}
			v.vmData = prepareVMs(taggedVMs)
			applog.Infof("component=vms event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(taggedVMs))
			var metricsCmd tea.Cmd
			if v.anyMetricsColumnEnabled() {
				metricsCmd = v.enrichMetricsCmd(v.vmData)
			}
			v.sortCurrentVMs()
			v.syncVisibleRows()
			v.statusMsg = fmt.Sprintf("Loaded %d instances.", len(taggedVMs))
			if !v.showKubernetesNodes {
				if hidden := countKubernetesNodes(v.vmData, v.searchInput.Value(), v.filterTerms); hidden > 0 {
					v.statusMsg = fmt.Sprintf("Loaded %d instances. Hiding %d Kubernetes worker nodes.", len(taggedVMs), hidden)
				}
			}
			indexCtx := v.activeCtx
			indexVMs := taggedVMs
			cmds = append(cmds, func() tea.Msg {
				return ui.VMIndexUpdateMsg{Ctx: indexCtx, VMs: indexVMs}
			})
			if metricsCmd != nil {
				cmds = append(cmds, metricsCmd)
			}
		}

	case vmCostEnrichedMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Cost enrichment failed: %v", msg.err)
		} else {
			for key, entry := range msg.cacheUpdates {
				v.costCache[key] = entry
			}
			v.vmData = prepareVMs(iac.ApplyTerraformToVMs(v.cfg.TerraformStatePaths, v.activeCtx, config.ApplyResourceTagsToVMs(*v.cfg, v.activeCtx, msg.vms)))
			if v.anyMetricsColumnEnabled() {
				v.enrichMetricsCmd(v.vmData)
			}
			v.sortCurrentVMs()
			v.syncVisibleRows()
			v.statusMsg = "Loaded instance costs."
		}

	case vmMetricsMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		if msg.err == nil && msg.metrics != nil {
			v.metricsCache[msg.vmID] = metricsCacheEntry{metrics: msg.metrics, timestamp: time.Now()}
		}
		v.updateVMMetrics(msg.vmID, msg.metrics, msg.err)
		v.syncVisibleRows()

	case commandCompleteMsg:
		v.loading = false
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.showCopyableDetail(fmt.Sprintf("ACTION FAILED\n\nError: %v\n\nOutput:\n%s", msg.err, msg.output))
			cmds = append(cmds, statusCmd("Action failed. Press c to copy, Esc to close."))
		} else {
			v.statusMsg = msg.output
			v.loading = true
			return v, v.fetchVMsCmd(true)
		}
	case describeCompleteMsg:
		v.loading = false
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.showCopyableDetail(fmt.Sprintf("COMMAND FAILED\n\nError: %v\n\nOutput:\n%s", msg.err, msg.output))
			cmds = append(cmds, statusCmd("Command failed. Press c to copy, Esc to close."))
		} else {
			v.showCopyableDetailWithURL(msg.output, msg.consoleURL)
			v.statusMsg = "Viewing details (c copy, o open, Esc close, Up/Down scroll)."
			cmds = append(cmds, statusCmd(v.statusMsg))
		}

	case accessResolvedMsg:
		v.pendingVM = msg.vm
		v.pendingAccess = msg.methods
		v.accessParent = nil
		v.accessMode = "methods"
		v.editingAccessUser = false
		v.accessUserInput.Blur()
		v.accessMethods.Title = fmt.Sprintf("Access: %s", msg.vm.Name)
		v.setAccessItems(msg.methods)
		v.accessMethods.SetSize(64, ui.ActionListHeight(len(v.accessMethods.Items()), v.height))
		v.loading = false
		v.activePane = paneAccess
		v.statusMsg = "Choose access method (Enter run, c copy, Esc close)."
		cmds = append(cmds, statusCmd(v.statusMsg))

	case sshCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("SSH failed: %v", msg.err)
			if len(v.pendingAccess) > 0 {
				v.activePane = paneAccess
				cmds = append(cmds, statusCmd("Access failed. Pick another method, or c to copy the selected command."))
			} else {
				v.showCopyableDetail(sshRemediationGuide(v.pendingVM, v.activeCtx))
				cmds = append(cmds, statusCmd("SSH failed. Press c to copy the remediation guide, Esc to close."))
			}
		} else {
			v.statusMsg = "SSH session closed."
		}

	case clipboardCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Copy failed: %v", msg.err)
		} else {
			v.statusMsg = "Copied to clipboard."
		}
		cmds = append(cmds, statusCmd(v.statusMsg))
	case ui.BrowserOpenMsg:
		if msg.Err != nil {
			v.statusMsg = fmt.Sprintf("Open failed: %v", msg.Err)
		} else {
			v.statusMsg = "Opened provider console."
		}
		cmds = append(cmds, statusCmd(v.statusMsg))

	case finopsRecommendMsg:
		v.loading = false
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Gemini Error: %v", msg.err)

			errorMsg := fmt.Sprintf("GEMINI FINOPS ERROR\n\nFailed to generate recommendation: %v\n\n", msg.err)
			if strings.Contains(msg.err.Error(), "GEMINI_API_KEY") {
				errorMsg += "To use CloudManager's AI FinOps features, you must provide a Google Gemini API Key.\n\n"
				errorMsg += "1. Get a free key at: https://aistudio.google.com/app/apikey\n"
				errorMsg += "2. Set the environment variable in your terminal:\n\n"
				errorMsg += "   export GEMINI_API_KEY=\"your_api_key_here\"\n\n"
				errorMsg += "3. Restart CloudManager and try again."
			}
			v.showCopyableDetail(errorMsg)
			cmds = append(cmds, statusCmd("FinOps error. Press c to copy, Esc to close."))
		} else {
			v.showCopyableDetail(fmt.Sprintf("GEMINI FINOPS RECOMMENDATION\n\n%s", msg.recommendation))
			v.statusMsg = "Viewing FinOps recommendation (c copy, Esc close)."
			cmds = append(cmds, statusCmd(v.statusMsg))
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
	case paneAccess:
		if v.keyDropdownOpen {
			v.keyFileList, cmd = v.keyFileList.Update(msg)
		} else if v.editingAccessUser {
			v.accessUserInput, cmd = v.accessUserInput.Update(msg)
		} else {
			v.accessMethods, cmd = v.accessMethods.Update(msg)
		}
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
	if !v.showKubernetesNodes {
		hidden := countKubernetesNodes(v.vmData, v.searchInput.Value(), v.filterTerms)
		if hidden > 0 {
			header = ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(fmt.Sprintf("%s | K8s nodes hidden: %d (K to show)", v.breadcrumbs, hidden), v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)
		}
	}
	tableContent := ui.ColorizeOperationalStates(v.vms.View())

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
	case paneAccess:
		overlayContent := v.accessMethods.View()
		if v.accessMode == "keys" {
			panelWidth := privateKeyAccessPanelWidth(v.width)
			overlay := ui.OverlayStyle.Copy().Width(panelWidth).Render(v.renderPrivateKeyAccess(panelWidth - 6))
			body := lipgloss.Place(v.width, maxInt(6, v.height-4), lipgloss.Center, lipgloss.Top, overlay, lipgloss.WithWhitespaceChars(" "))
			return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
		}
		overlay := ui.OverlayStyle.Render(overlayContent)
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-70).Render(tableContent), overlay)
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
	case paneTag:
		form := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(ui.Highlight).Bold(true).Render("CloudManager Tags"),
			"",
			v.tagInput.View(),
			"",
			lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Save \u2022 Esc: Cancel"),
		)
		overlay := ui.OverlayStyle.Render(form)
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(tableContent), overlay)
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", tableContent), v.width, v.height)
}

func (v *VMsView) renderPrivateKeyAccess(width int) string {
	if width < 32 {
		width = 32
	}
	keyLabel := "Select key"
	if strings.TrimSpace(v.selectedSSHKey) != "" {
		keyLabel = filepath.Base(v.selectedSSHKey)
	}
	ipLabel := v.selectedSSHIPKind
	if ipLabel == "" {
		ipLabel = defaultSSHIPKind(v.pendingVM)
	}
	command := "Select a key to build command."
	if method, ok := v.currentPrivateKeyMethod(); ok {
		command = method.CopyText
	}
	v.accessUserInput.Width = width - 8
	v.keyFileList.SetSize(width, ui.ActionListHeight(len(v.keyFileList.Items()), v.height-12))
	lines := []string{
		lipgloss.NewStyle().Foreground(ui.Highlight).Bold(true).Render("Private key SSH"),
		v.accessUserInput.View(),
		fmt.Sprintf("key> %s", ui.TruncateText(keyLabel, width-5)),
		fmt.Sprintf("ip> %s", ui.TruncateText(ipLabel, width-4)),
		"",
		lipgloss.NewStyle().Foreground(ui.Subtle).Render(ui.TruncateText(command, width)),
		"",
		lipgloss.NewStyle().Foreground(ui.Subtle).Render(ui.TruncateText("k key • u user • p IP • b bootstrap • Enter run • c copy • Esc back", width)),
	}
	if v.keyDropdownOpen {
		lines = append(lines, "", v.keyFileList.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func privateKeyAccessPanelWidth(windowWidth int) int {
	width := windowWidth - 8
	if width > 76 {
		width = 76
	}
	if width < 42 {
		width = 42
	}
	return width
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- Key handlers ---

func (v *VMsView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	vm, ok := v.selectedVM()

	switch msg.String() {
	case "enter":
		if !ok {
			return v, nil
		}
		v.actions.Title = fmt.Sprintf("Actions: %s", vm.Name)
		v.activePane = paneActions
		v.refreshTable()
	case "d":
		if !ok {
			return v, nil
		}
		v.activePane = paneTable
		v.loading = true
		v.statusMsg = fmt.Sprintf("Describing %s...", vm.Name)
		return v, executeActionCmd("Describe", vm, v.activeCtx, v.cfg)
	case "f":
		if !ok {
			return v, nil
		}
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Querying Gemini AI for %s...", vm.Name)
		v.loading = true
		return v, v.finopsRecommendCmd(vm, v.cachedMetrics(vm.ID))
	case "c":
		if !ok {
			return v, nil
		}
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Fetching cost report for %s...", vm.Name)
		v.loading = true
		return v, executeCostCommandCmd(vm, v.activeCtx, *v.cfg)
	case "s":
		if !ok {
			return v, nil
		}
		v.pendingVM = vm
		v.activePane = paneTable
		v.loading = true
		v.statusMsg = fmt.Sprintf("Resolving access methods for %s...", vm.Name)
		return v, v.resolveAccessCmd(vm)
	case "t":
		if !ok {
			return v, nil
		}
		v.pendingVM = vm
		v.tagInput.SetValue("")
		v.tagInput.Focus()
		v.activePane = paneTag
		v.statusMsg = fmt.Sprintf("Tag %s with CloudManager-only tags.", vm.Name)
		return v, textinput.Blink
	case "ctrl+d":
		if !ok {
			return v, nil
		}
		v.pendingAction = actionItem{title: "Terminate", desc: "Permanently delete the virtual machine"}
		v.pendingVM = vm
		v.activePane = paneConfirm
		v.refreshTable()
	case "/":
		v.isSearching = true
		v.searchInput.Focus()
		v.statusMsg = "Search (Enter/Esc to apply)"
	case "K":
		v.showKubernetesNodes = !v.showKubernetesNodes
		v.syncVisibleRows()
		if v.showKubernetesNodes {
			v.statusMsg = "Showing Kubernetes worker nodes."
		} else {
			v.statusMsg = "Hiding Kubernetes worker nodes."
		}
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
	case "up":
		if v.vms.Cursor() == 0 {
			v.sortHeader.Activate(v.tableCols)
			v.refreshTable()
		}
	case "r":
		v.loading = true
		v.statusMsg = fmt.Sprintf("Refreshing instances for %s...", v.activeCtx.DisplayName())
		return v, v.fetchVMsCmd(true)
	case "S":
		v.sortHeader.Activate(v.tableCols)
		v.refreshTable()
	case "C":
		v.activePane = paneColumnConfig
	}
	return v, nil
}

func (v *VMsView) handleTagKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.tagInput.Blur()
		v.activePane = paneTable
		v.statusMsg = "Tag canceled."
		return v, nil
	case "enter":
		tags := splitTagInput(v.tagInput.Value())
		if len(tags) == 0 {
			v.statusMsg = "No tags entered."
			return v, statusCmd(v.statusMsg)
		}
		if _, err := config.BackupConfig(); err != nil {
			v.statusMsg = fmt.Sprintf("Tag save canceled; backup failed: %v", err)
			return v, statusCmd(v.statusMsg)
		}
		*v.cfg = config.UpsertVMResourceTags(*v.cfg, v.activeCtx, v.pendingVM, tags)
		if err := config.Save(*v.cfg); err != nil {
			v.statusMsg = fmt.Sprintf("Tag save failed: %v", err)
			return v, statusCmd(v.statusMsg)
		}
		v.vmData = iac.ApplyTerraformToVMs(v.cfg.TerraformStatePaths, v.activeCtx, config.ApplyResourceTagsToVMs(*v.cfg, v.activeCtx, v.vmData))
		v.sortCurrentVMs()
		v.syncVisibleRows()
		v.tagInput.Blur()
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Tagged %s with %s.", v.pendingVM.Name, strings.Join(tags, ","))
		indexCtx := v.activeCtx
		indexVMs := v.vmData
		return v, tea.Batch(statusCmd(v.statusMsg), func() tea.Msg {
			return ui.VMIndexUpdateMsg{Ctx: indexCtx, VMs: indexVMs}
		})
	}
	var cmd tea.Cmd
	v.tagInput, cmd = v.tagInput.Update(msg)
	return v, cmd
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
				return v, v.finopsRecommendCmd(vm, v.cachedMetrics(vm.ID))
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
				v.pendingVM = vm
				v.activePane = paneTable
				v.loading = true
				v.statusMsg = fmt.Sprintf("Resolving access methods for %s...", vm.Name)
				return v, v.resolveAccessCmd(vm)
			}
			if action.title == "Open Console" {
				return v.openConsole(core.VMConsoleURL(v.activeCtx, vm))
			}
			// Other actions (Start, Stop, Restart, Describe)
			v.activePane = paneTable
			v.statusMsg = fmt.Sprintf("Executing %s on %s...", action.title, vm.Name)
			return v, executeActionCmd(action.title, vm, v.activeCtx, v.cfg)
		}
	}
	return v, nil
}

func (v *VMsView) handleAccessKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	method, ok := v.selectedAccessMethod()
	if v.keyDropdownOpen {
		switch msg.String() {
		case "enter":
			if item, ok := v.keyFileList.SelectedItem().(keyFileItem); ok {
				v.selectedSSHKey = item.path
				v.keyDropdownOpen = false
				v.statusMsg = fmt.Sprintf("Selected SSH key %s.", filepath.Base(item.path))
				return v, statusCmd(v.statusMsg)
			}
			return v, nil
		case "esc":
			v.keyDropdownOpen = false
			return v, nil
		}
		return v, nil
	}
	if v.editingAccessUser {
		switch msg.String() {
		case "enter":
			v.editingAccessUser = false
			v.accessUserInput.Blur()
			v.statusMsg = "Username applied. Choose key/IP and press Enter."
			return v, statusCmd(v.statusMsg)
		case "esc":
			v.editingAccessUser = false
			v.accessUserInput.Blur()
			return v, nil
		}
		return v, nil
	}
	if v.accessMode == "keys" {
		switch msg.String() {
		case "esc":
			v.restoreAccessParent()
			return v, nil
		case "k":
			v.openKeyDropdown()
			return v, nil
		case "u":
			v.editingAccessUser = true
			v.accessUserInput.Focus()
			v.statusMsg = "Enter SSH username, then press Enter."
			return v, textinput.Blink
		case "p":
			v.toggleSelectedSSHIPKind()
			return v, nil
		case "b":
			command, ok := v.defaultKeyBootstrapCommand()
			if !ok {
				v.statusMsg = "Select key/IP first and ensure ~/.ssh/id_rsa.pub exists."
				return v, statusCmd(v.statusMsg)
			}
			v.showCopyableDetail(command)
			v.statusMsg = "Bootstrap command ready. Press c to copy, Esc to close."
			return v, statusCmd(v.statusMsg)
		case "c", "C":
			method, ok := v.currentPrivateKeyMethod()
			if !ok {
				v.statusMsg = "Select a key first."
				return v, statusCmd(v.statusMsg)
			}
			v.showCopyableDetail(method.CopyText)
			v.statusMsg = "Copying access command..."
			return v, copyToClipboardCmd(method.CopyText)
		case "enter":
			method, ok := v.currentPrivateKeyMethod()
			if !ok {
				v.openKeyDropdown()
				v.statusMsg = "Select a key first."
				return v, statusCmd(v.statusMsg)
			}
			cmd, err := access.ExecCommand(context.Background(), method)
			if err != nil {
				v.statusMsg = fmt.Sprintf("Access method failed: %v", err)
				return v, statusCmd(v.statusMsg)
			}
			v.rememberPrivateKeyAccess()
			v.activePane = paneTable
			v.statusMsg = fmt.Sprintf("Starting %s...", method.Label)
			return v, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return sshCompleteMsg{err: err}
			})
		}
		return v, nil
	}
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "u":
	case "c", "C":
		if !ok || strings.TrimSpace(method.CopyText) == "" {
			v.statusMsg = "Nothing to copy."
			return v, statusCmd(v.statusMsg)
		}
		v.showCopyableDetail(method.CopyText)
		v.statusMsg = "Copying access command..."
		return v, copyToClipboardCmd(method.CopyText)
	case "enter":
		if !ok {
			return v, nil
		}
		if method.Kind == "private_key_picker" {
			v.openPrivateKeyPicker()
			return v, statusCmd(v.statusMsg)
		}
		if method.Kind == "remediation" || len(method.Command) == 0 {
			v.showCopyableDetail(sshRemediationGuide(v.pendingVM, v.activeCtx))
			v.statusMsg = "Viewing remediation guide (c copy, Esc close)."
			return v, statusCmd(v.statusMsg)
		}
		if !method.Available {
			v.statusMsg = strings.TrimSpace(method.Reason)
			if v.statusMsg == "" {
				v.statusMsg = "Access method unavailable."
			}
			return v, statusCmd(v.statusMsg)
		}
		cmd, err := access.ExecCommand(context.Background(), method)
		if err != nil {
			v.statusMsg = fmt.Sprintf("Access method failed: %v", err)
			return v, statusCmd(v.statusMsg)
		}
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Starting %s...", method.Label)
		return v, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return sshCompleteMsg{err: err}
		})
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
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
	case "c", "C":
		if strings.TrimSpace(v.copyableText) == "" {
			v.statusMsg = "Nothing to copy."
			return v, statusCmd(v.statusMsg)
		}
		v.statusMsg = "Copying to clipboard..."
		return v, copyToClipboardCmd(v.copyableText)
	case "o", "O":
		return v.openConsole(v.detailURL)
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
			cmd := v.enrichMetricsCmd(v.vmData)
			v.syncVisibleRows()
			return v, cmd
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

func (v *VMsView) showCopyableDetail(content string) {
	v.copyableText = content
	v.detailURL = ""
	v.descView.SetContent(content)
	v.descView.GotoTop()
	v.activePane = paneDescribe
}

func (v *VMsView) showCopyableDetailWithURL(content, consoleURL string) {
	v.detailURL = strings.TrimSpace(consoleURL)
	content = core.DetailWithConsoleURL(content, v.detailURL)
	v.copyableText = content
	v.descView.SetContent(content)
	v.descView.GotoTop()
	v.activePane = paneDescribe
}

func (v *VMsView) openConsole(consoleURL string) (ui.View, tea.Cmd) {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		v.statusMsg = "No provider console URL available."
		return v, statusCmd(v.statusMsg)
	}
	v.statusMsg = "Opening provider console..."
	return v, tea.Batch(statusCmd(v.statusMsg), ui.OpenURLCmd(consoleURL))
}

func statusCmd(msg string) tea.Cmd {
	return func() tea.Msg {
		return ui.StatusUpdateMsg{Msg: msg}
	}
}

func (v *VMsView) refreshTable() {
	if v.width == 0 {
		return
	}

	availWidth := v.width
	if v.activePane == paneActions || v.activePane == paneAccess || v.activePane == paneConfirm || v.activePane == paneTag {
		availWidth = v.width - 55
		if v.activePane == paneAccess {
			availWidth = v.width - 70
		}
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
	newVMs.SetColumns(ui.DecorateSortColumns(newCols, v.sortHeader, v.sortColumn, v.sortAsc))
	if cursor >= 0 && cursor < len(rows) {
		newVMs.SetCursor(cursor)
	}
	if focused {
		newVMs.Focus()
	}
	v.vms = newVMs
	v.tableCols = newCols
}

func (v *VMsView) handleHeaderSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "down":
		v.sortHeader.Deactivate()
	case "left", "h":
		if !v.sortHeader.Move(-1, v.tableCols) && v.columnOffset > 0 {
			v.columnOffset--
			v.refreshTable()
			v.sortHeader.Index = len(v.tableCols) - 1
		}
	case "right", "l":
		if !v.sortHeader.Move(1, v.tableCols) && v.canScrollRight {
			v.columnOffset++
			v.sortHeader.Index = 0
			v.refreshTable()
		}
	case "enter":
		column := v.sortHeader.SelectedColumn(v.tableCols)
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, column)
		sortVMs(v.vmData, v.sortColumn, v.sortAsc)
		v.syncVisibleRows()
		v.statusMsg = fmt.Sprintf("Sorted by %s (%s).", v.sortColumn, ui.SortDirectionLabel(v.sortAsc))
	}
	v.refreshTable()
	return v, nil
}

func (v *VMsView) syncVisibleRows() {
	v.visibleVMs = filterVMs(v.vmData, v.searchInput.Value(), v.filterTerms, v.showKubernetesNodes)
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
	return filterVMs(v.vmData, v.searchInput.Value(), v.filterTerms, v.showKubernetesNodes)
}

func (v *VMsView) selectedVM() (core.VM, bool) {
	cursor := v.vms.Cursor()
	if cursor < 0 || cursor >= len(v.visibleVMs) {
		return core.VM{}, false
	}
	return v.visibleVMs[cursor], true
}

func (v *VMsView) selectedAccessMethod() (core.AccessMethod, bool) {
	item := v.accessMethods.SelectedItem()
	if item == nil {
		return core.AccessMethod{}, false
	}
	accessItem, ok := item.(accessItem)
	if !ok {
		return core.AccessMethod{}, false
	}
	return accessItem.method, true
}

func (v *VMsView) setAccessItems(methods []core.AccessMethod) {
	v.pendingAccess = methods
	items := make([]list.Item, 0, len(methods))
	for _, method := range methods {
		items = append(items, accessItem{method: method})
	}
	v.accessMethods.SetItems(items)
	if len(items) > 0 {
		v.accessMethods.Select(0)
	}
}

func (v *VMsView) openPrivateKeyPicker() {
	v.accessParent = append([]core.AccessMethod(nil), v.pendingAccess...)
	v.accessMode = "keys"
	v.editingAccessUser = false
	v.keyDropdownOpen = false
	v.accessUserInput.SetValue("ubuntu")
	v.accessUserInput.Blur()
	v.selectedSSHIPKind = defaultSSHIPKind(v.pendingVM)
	v.selectedSSHKey = ""
	v.refreshKeyFileItems()
	v.statusMsg = "Private key SSH. Press k to select key, u for username, p for IP."
}

func (v *VMsView) refreshKeyFileItems() {
	keys := access.SSHKeyFiles()
	items := make([]list.Item, 0, len(keys))
	for _, keyPath := range keys {
		items = append(items, keyFileItem{path: keyPath})
	}
	v.keyFileList.SetItems(items)
	if len(items) > 0 {
		v.keyFileList.Select(0)
	}
	v.keyFileList.SetSize(56, ui.ActionListHeight(len(items), v.height))
}

func (v *VMsView) openKeyDropdown() {
	v.refreshKeyFileItems()
	v.keyDropdownOpen = true
	if strings.TrimSpace(v.selectedSSHKey) != "" {
		for i, item := range v.keyFileList.Items() {
			if key, ok := item.(keyFileItem); ok && key.path == v.selectedSSHKey {
				v.keyFileList.Select(i)
				break
			}
		}
	}
	v.statusMsg = "Select SSH key."
}

func (v *VMsView) currentPrivateKeyMethod() (core.AccessMethod, bool) {
	return access.PrivateKeySSHMethod(v.pendingVM, v.accessUserInput.Value(), v.selectedSSHKey, v.selectedSSHIPKind)
}

func (v *VMsView) rememberPrivateKeyAccess() {
	ip := selectedIPForKind(v.pendingVM, v.selectedSSHIPKind)
	if ip == "" {
		return
	}
	db, err := localdb.Open(context.Background())
	if err != nil {
		applog.Warnf("component=vms event=access_profile_open_failed err=%v", err)
		return
	}
	defer db.Close()
	if err := localdb.UpsertAccessProfile(context.Background(), db, v.activeCtx, v.pendingVM, localdb.AccessProfile{
		Username:        v.accessUserInput.Value(),
		IPKind:          v.selectedSSHIPKind,
		IP:              ip,
		KeyPath:         v.selectedSSHKey,
		DefaultKeyReady: false,
	}); err != nil {
		applog.Warnf("component=vms event=access_profile_save_failed vm=%s err=%v", v.pendingVM.ID, err)
	}
}

func (v *VMsView) defaultKeyBootstrapCommand() (string, bool) {
	method, ok := v.currentPrivateKeyMethod()
	if !ok || len(method.Command) == 0 {
		return "", false
	}
	pubPath := expandLocalPath("~/.ssh/id_rsa.pub")
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return "", false
	}
	pubKey := strings.TrimSpace(string(pub))
	if pubKey == "" {
		return "", false
	}
	remote := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && grep -qxF %s ~/.ssh/authorized_keys 2>/dev/null || echo %s >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys", shellQuote(pubKey), shellQuote(pubKey))
	args := append([]string{}, method.Command...)
	args = append(args, remote)
	return access.FormatCommand(args), true
}

func (v *VMsView) toggleSelectedSSHIPKind() {
	switch v.selectedSSHIPKind {
	case "public":
		if hasUsableIP(v.pendingVM.PrivateIP) {
			v.selectedSSHIPKind = "private"
		}
	case "private":
		if hasUsableIP(v.pendingVM.PublicIP) {
			v.selectedSSHIPKind = "public"
		}
	default:
		v.selectedSSHIPKind = defaultSSHIPKind(v.pendingVM)
	}
}

func defaultSSHIPKind(vm core.VM) string {
	if hasUsableIP(vm.PublicIP) {
		return "public"
	}
	return "private"
}

func hasUsableIP(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "-"
}

func selectedIPForKind(vm core.VM, kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "private":
		if hasUsableIP(vm.PrivateIP) {
			return strings.TrimSpace(vm.PrivateIP)
		}
	default:
		if hasUsableIP(vm.PublicIP) {
			return strings.TrimSpace(vm.PublicIP)
		}
	}
	if hasUsableIP(vm.PrivateIP) {
		return strings.TrimSpace(vm.PrivateIP)
	}
	return ""
}

func expandLocalPath(path string) string {
	path = os.ExpandEnv(strings.TrimSpace(path))
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func (v *VMsView) restoreAccessParent() {
	v.accessMode = "methods"
	v.editingAccessUser = false
	v.keyDropdownOpen = false
	v.accessUserInput.Blur()
	v.selectedSSHKey = ""
	v.selectedSSHIPKind = ""
	v.accessMethods.Title = fmt.Sprintf("Access: %s", v.pendingVM.Name)
	v.setAccessItems(v.accessParent)
	v.accessMethods.SetSize(64, ui.ActionListHeight(len(v.accessMethods.Items()), v.height))
	v.statusMsg = "Choose access method (Enter run, c copy, Esc close)."
}

func filterVMs(vms []core.VM, query string, filters []string, showKubernetesNodes bool) []core.VM {
	var filtered []core.VM
	for _, vm := range vms {
		if !showKubernetesNodes && vm.IsKubernetesNode() {
			continue
		}
		if vmMatchesFilters(vm, query, filters) {
			filtered = append(filtered, vm)
		}
	}
	return filtered
}

func countKubernetesNodes(vms []core.VM, query string, filters []string) int {
	count := 0
	for _, vm := range vms {
		if vm.IsKubernetesNode() && vmMatchesFilters(vm, query, filters) {
			count++
		}
	}
	return count
}

func vmMatchesFilters(vm core.VM, query string, filters []string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
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
			return false
		}
	}

	if normalizedQuery == "" {
		return true
	}
	return strings.Contains(strings.ToLower(vm.Name), normalizedQuery) ||
		strings.Contains(strings.ToLower(vm.ID), normalizedQuery) ||
		strings.Contains(strings.ToLower(vm.PrivateIP), normalizedQuery) ||
		strings.Contains(strings.ToLower(vm.PublicIP), normalizedQuery) ||
		strings.Contains(strings.ToLower(vm.Labels), normalizedQuery)
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
		"CPU %":           8, "Memory %": 10, "Disk Read": 12, "Disk Write": 12, "Net In": 12, "Net Out": 12,
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

func splitTagInput(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	tags := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key := strings.ToLower(field)
		if seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, field)
	}
	return tags
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
	ui.SortByColumn(vms, column, asc, func(vm core.VM, column string) string {
		return vm.GetField(column)
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
		if metrics := v.cachedMetrics(vm.ID); metrics != nil {
			v.updateVMMetrics(vm.ID, metrics, nil)
			continue
		}
		cmds = append(cmds, v.fetchSingleVMMetricsCmd(vm))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (v *VMsView) fetchSingleVMMetricsCmd(vm core.VM) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	period := v.metricsPeriod()
	cfg := *v.cfg

	return func() tea.Msg {
		provider := getProvider(cfg)
		metricsProvider, ok := provider.(providers.MetricsProvider)
		if !ok {
			return nil
		}
		m, err := metricsProvider.FetchVMMetrics(context.Background(), vm, activeCtx, period)
		return vmMetricsMsg{requestKey: requestKey, vmID: vm.ID, metrics: m, err: err}
	}
}

func (v *VMsView) fetchVMsCmd(force bool) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	mode := strings.ToUpper(v.cfg.Backend)
	if !force {
		cacheKey := activeCtx.CacheKey()
		ttl := time.Duration(v.cfg.CacheTTL) * time.Minute
		if entry, ok := v.vmCache[cacheKey]; ok && time.Since(entry.timestamp) < ttl {
			cachedRows := entry.vms
			return func() tea.Msg {
				applog.Infof("component=vms event=fetch_start provider=%s account=%s region=%s mode=%s force=%t cache=hit", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, force)
				return vmFetchMsg{requestKey: requestKey, vms: cachedRows, fromCache: true}
			}
		}
	}
	cfg := *v.cfg
	return func() tea.Msg {
		applog.Infof("component=vms event=fetch_start provider=%s account=%s region=%s mode=%s force=%t", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, force)
		provider := getProvider(cfg)
		rows, err := provider.FetchVMs(context.Background(), activeCtx)
		if err != nil {
			return vmFetchMsg{requestKey: requestKey, err: err}
		}
		return vmFetchMsg{requestKey: requestKey, vms: rows}
	}
}

func (v *VMsView) enrichCostCmd(vms []core.VM) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	billingTTL := v.billingCacheTTL()
	cachedCosts := v.cachedCosts(activeCtx, vms, billingTTL)
	cfg := *v.cfg
	return func() tea.Msg {
		provider := getProvider(cfg)
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

		cacheUpdates := make(map[string]costCacheEntry)
		for i := range enrichedVMs {
			vmID := enrichedVMs[i].ID
			if r, ok := recMap[vmID]; ok {
				enrichedVMs[i].Recommendation = r.Summary
				enrichedVMs[i].EstSavings = core.FormatCost(r.EstimatedSavings)
			}

			cacheKey := vmCostCacheKey(activeCtx, enrichedVMs[i].ID)
			if entry, ok := cachedCosts[cacheKey]; ok {
				enrichedVMs[i].MonthlyCost = entry.monthlyCost
				enrichedVMs[i].CostTrend = entry.costTrend
				continue
			}
			cost, err := billingProvider.FetchResourceCost(context.Background(), enrichedVMs[i].ID, activeCtx)
			if err != nil || cost == nil {
				continue
			}
			monthlyCost := core.FormatCost(cost.CurrentMonthCost)
			costTrend := core.CostChangePercent(cost.CurrentMonthCost, cost.PreviousMonthCost)
			enrichedVMs[i].MonthlyCost = monthlyCost
			enrichedVMs[i].CostTrend = costTrend
			cacheUpdates[cacheKey] = costCacheEntry{
				monthlyCost: monthlyCost,
				costTrend:   costTrend,
				timestamp:   time.Now(),
			}
		}
		return vmCostEnrichedMsg{requestKey: requestKey, vms: enrichedVMs, cacheUpdates: cacheUpdates}
	}
}

func executeActionCmd(action string, vm core.VM, cloudCtx core.CloudContext, cfg *config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		provider := getProvider(*cfg)
		output, err := provider.ExecuteAction(context.Background(), action, vm, cloudCtx)
		if action == "Describe" {
			return describeCompleteMsg{output: output, consoleURL: core.VMConsoleURL(cloudCtx, vm), err: err}
		}
		return commandCompleteMsg{output: output, err: err}
	}
}

func (v *VMsView) resolveAccessCmd(vm core.VM) tea.Cmd {
	req := access.Request{
		Context:      v.activeCtx,
		VM:           vm,
		NativeMethod: v.providerNativeAccessMethod(vm),
	}
	return func() tea.Msg {
		methods := access.Resolve(context.Background(), req)
		if learned, ok := learnedAccessMethod(context.Background(), v.activeCtx, vm); ok {
			methods = append([]core.AccessMethod{learned}, methods...)
			methods = access.PruneFallbacks(methods)
		}
		return accessResolvedMsg{
			vm:      vm,
			methods: methods,
		}
	}
}

func learnedAccessMethod(ctx context.Context, cloudCtx core.CloudContext, vm core.VM) (core.AccessMethod, bool) {
	db, err := localdb.Open(ctx)
	if err != nil {
		applog.Warnf("component=vms event=access_profile_open_failed err=%v", err)
		return core.AccessMethod{}, false
	}
	defer db.Close()
	profile, ok, err := localdb.LookupAccessProfile(ctx, db, cloudCtx, vm)
	if err != nil {
		applog.Warnf("component=vms event=access_profile_lookup_failed vm=%s err=%v", vm.ID, err)
		return core.AccessMethod{}, false
	}
	if !ok {
		return core.AccessMethod{}, false
	}
	return localdb.LearnedAccessMethod(profile)
}

func (v *VMsView) providerNativeAccessMethod(vm core.VM) *core.AccessMethod {
	method := core.AccessMethod{
		ID:       "provider-native",
		Kind:     "native",
		Label:    fmt.Sprintf("%s native access", v.activeCtx.Provider),
		Priority: 10,
	}
	provider := getProvider(*v.cfg)
	cmd, err := provider.GetSSHCmd(context.Background(), vm, v.activeCtx)
	if err != nil || cmd == nil || len(cmd.Args) == 0 {
		method.Reason = fmt.Sprintf("%s native access unavailable", v.activeCtx.Provider)
		return &method
	}
	method.Label = providerNativeAccessLabel(v.activeCtx.Provider, cmd.Args)
	method.Command = cmd.Args
	method.CopyText = access.FormatCommand(cmd.Args)
	method.Available = true
	return &method
}

func providerNativeAccessLabel(provider string, args []string) string {
	joined := strings.ToLower(strings.Join(args, " "))
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws":
		if strings.Contains(joined, " ssm ") || strings.Contains(joined, "start-session") {
			return "AWS SSM Session Manager"
		}
		if strings.Contains(joined, "ec2-instance-connect") {
			return "AWS EC2 Instance Connect"
		}
	case "gcp":
		return "GCP OS Login / gcloud SSH"
	case "azure":
		return "Azure native SSH"
	}
	return fmt.Sprintf("%s native access", provider)
}

func vmCostCacheKey(cloudCtx core.CloudContext, resourceID string) string {
	return fmt.Sprintf("%s|%s", cloudCtx.CacheKey(), resourceID)
}

func buildCostCommand(vm core.VM, cloudCtx core.CloudContext, cfg config.AppConfig) []string {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)

	switch cloudCtx.Provider {
	case "AWS":
		args := []string{
			"aws",
			"--no-cli-pager",
			"ce",
			"get-cost-and-usage",
			"--time-period",
			fmt.Sprintf("Start=%s,End=%s", start.Format("2006-01-02"), end.Format("2006-01-02")),
			"--granularity",
			"MONTHLY",
			"--metrics",
			"UnblendedCost",
			"--filter",
			fmt.Sprintf("{\"Dimensions\":{\"Key\":\"RESOURCE_ID\",\"Values\":[\"%s\"]}}", vm.ID),
			"--region",
			"us-east-1",
			"--output",
			"table",
		}
		if profile := strings.TrimSpace(cloudCtx.AuthRef()); profile != "" {
			args = append(args, "--profile", profile)
		}
		return args
	case "GCP":
		billingTable := gcpBillingTablePath(cloudCtx, cfg)
		globalName := gcpVMGlobalName(vm, cloudCtx)
		query := fmt.Sprintf(
			"SELECT service.description AS service, sku.description AS sku, SUM(cost) AS cost FROM `%s` WHERE usage_start_time >= TIMESTAMP(\"%s\") AND usage_start_time < TIMESTAMP(\"%s\") AND (resource.name = \"%s\" OR resource.global_name = \"%s\") GROUP BY service, sku ORDER BY cost DESC",
			billingTable,
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
			vm.Name,
			globalName,
		)
		return []string{"bq", "query", "--use_legacy_sql=false", "--format=prettyjson", query}
	case "Azure":
		scope := fmt.Sprintf("/subscriptions/%s", cloudCtx.AccountID)
		body := fmt.Sprintf("{\"type\":\"ActualCost\",\"timeframe\":\"MonthToDate\",\"dataset\":{\"granularity\":\"None\",\"filter\":{\"dimensions\":{\"name\":\"ResourceId\",\"operator\":\"In\",\"values\":[\"%s\"]}},\"aggregation\":{\"totalCost\":{\"name\":\"PreTaxCost\",\"function\":\"Sum\"}},\"grouping\":[{\"type\":\"Dimension\",\"name\":\"ServiceName\"}]}}", vm.ID)
		return []string{
			"az",
			"rest",
			"--method",
			"post",
			"--url",
			fmt.Sprintf("https://management.azure.com%s/providers/Microsoft.CostManagement/query?api-version=2025-03-01", scope),
			"--body",
			body,
			"--output",
			"table",
		}
	default:
		return []string{"echo", "Cost lookup not supported for this provider."}
	}
}

func executeCostCommandCmd(vm core.VM, cloudCtx core.CloudContext, cfg config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		cmdArgs := buildCostCommand(vm, cloudCtx, cfg)
		if len(cmdArgs) == 0 {
			return describeCompleteMsg{output: "", err: fmt.Errorf("cost command is empty")}
		}
		cmd := exec.CommandContext(context.Background(), cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.CombinedOutput()

		formattedOutput := fmt.Sprintf("COST REPORT FOR %s (%s)\n\n$ %s\n\n%s", vm.Name, cloudCtx.Provider, formatShellCommand(cmdArgs), string(output))

		return describeCompleteMsg{output: formattedOutput, err: err}
	}
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardCompleteMsg{text: text, err: copyToClipboard(text)}
	}
}

var copyToClipboard = writeClipboard

func writeClipboard(text string) error {
	args, ok := clipboardCommand()
	if !ok {
		return fmt.Errorf("clipboard command not found")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func clipboardCommand() ([]string, bool) {
	candidates := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate, true
		}
	}
	return nil, false
}

func (v *VMsView) cachedMetrics(vmID string) *core.VMMetrics {
	entry, ok := v.metricsCache[vmID]
	if !ok || time.Since(entry.timestamp) >= v.metricsCacheTTL() {
		return nil
	}
	return entry.metrics
}

func (v *VMsView) cachedCosts(cloudCtx core.CloudContext, vms []core.VM, ttl time.Duration) map[string]costCacheEntry {
	cached := make(map[string]costCacheEntry)
	for _, vm := range vms {
		cacheKey := vmCostCacheKey(cloudCtx, vm.ID)
		entry, ok := v.costCache[cacheKey]
		if !ok || time.Since(entry.timestamp) >= ttl {
			continue
		}
		cached[cacheKey] = entry
	}
	return cached
}

func (v *VMsView) metricsCacheTTL() time.Duration {
	if v.cfg != nil && v.cfg.MetricsCacheTTL > 0 {
		return time.Duration(v.cfg.MetricsCacheTTL) * time.Minute
	}
	return 15 * time.Minute
}

func (v *VMsView) billingCacheTTL() time.Duration {
	if v.cfg != nil && v.cfg.BillingCacheTTL > 0 {
		return time.Duration(v.cfg.BillingCacheTTL) * time.Minute
	}
	return 30 * time.Minute
}

func (v *VMsView) metricsPeriod() time.Duration {
	if v.cfg != nil && v.cfg.MetricsPeriodHours > 0 {
		return time.Duration(v.cfg.MetricsPeriodHours) * time.Hour
	}
	return 24 * time.Hour
}

func formatShellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$&;|<>`(){}[]*?!") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
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

func (v *VMsView) finopsRecommendCmd(vm core.VM, metrics *core.VMMetrics) tea.Cmd {
	activeCtx := v.activeCtx
	cfg := *v.cfg
	return func() tea.Msg {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return finopsRecommendMsg{err: fmt.Errorf("GEMINI_API_KEY not set")}
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
