package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	hostinventory "github.com/vyoogam/cloudmanager/internal/hosts"
	"github.com/vyoogam/cloudmanager/internal/iac"
	"github.com/vyoogam/cloudmanager/internal/logging"
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/sysusage"
)

const (
	focusSidebar = iota
	focusMain
)

// --- Bubble Tea messages ---

type contextLoadMsg struct {
	tree     []*TreeNode
	contexts []core.CloudContext
	warnings []string
}

type gcpProjectFetchMsg struct {
	items []list.Item
}

type logRefreshMsg struct {
	content string
	err     error
}

type vmIndexCacheLoadMsg struct {
	index map[string]vmSearchRecord
	total int
	err   error
}

type resourceIndexCacheLoadMsg struct {
	cache resourceIndexCacheBundle
	err   error
}

type vmPrefetchMsg struct {
	ctx core.CloudContext
	vms []core.VM
	err error
}

type VMIndexUpdateMsg struct {
	Ctx core.CloudContext
	VMs []core.VM
}

type ClusterIndexUpdateMsg struct {
	Ctx      core.CloudContext
	Clusters []core.Cluster
	Err      error
}

type DatabaseIndexUpdateMsg struct {
	Ctx       core.CloudContext
	Databases []core.Database
	Err       error
}

type StorageIndexUpdateMsg struct {
	Ctx     core.CloudContext
	Buckets []core.StorageBucket
	Err     error
}

type ResourceSummaryUpdateMsg struct {
	Ctx            core.CloudContext
	Resource       string
	Count          int
	Extra          int
	Disks          []core.Disk
	Snapshots      []core.Snapshot
	Networks       []core.Network
	Subnets        []core.Subnet
	SecurityGroups []core.SecurityGroup
	StorageBuckets []core.StorageBucket
	Err            error
}

type providerLoginCompleteMsg struct {
	provider    string
	contextName string
	err         error
}

type azureSubscriptionLoadMsg struct {
	contexts []core.CloudContext
	warnings []string
}

type contextDiscoveryLoadMsg struct {
	contexts []core.CloudContext
	warnings []string
}

type vmSearchRecord struct {
	VM      core.VM
	Context core.CloudContext
	SeenAt  time.Time
}

type findScope string

const (
	findScopeVMs       findScope = "vms"
	findScopeDisks     findScope = "disks"
	findScopeSnapshots findScope = "snapshots"
	findScopeDatabases findScope = "databases"
	findScopeK8s       findScope = "kubernetes"
	findScopeNetworks  findScope = "networks"
	findScopeSubnets   findScope = "subnets"
	findScopeFirewalls findScope = "firewalls"
	findScopeStorage   findScope = "storage"
	findScopeHosts     findScope = "hosts"
	findScopeAll       findScope = "all"
)

type findScopeItem struct {
	scope       findScope
	title       string
	description string
}

func (i findScopeItem) Title() string       { return i.title }
func (i findScopeItem) Description() string { return i.description }
func (i findScopeItem) FilterValue() string { return i.title + " " + i.description }

type findRecord struct {
	Kind       string
	Name       string
	ID         string
	PublicIP   string
	Location   string
	Status     string
	Match      string
	Context    core.CloudContext
	Capability providers.Capability
	Filter     string
}

type clusterIndexRecord struct {
	Cluster core.Cluster
	Context core.CloudContext
	SeenAt  time.Time
}

type databaseIndexRecord struct {
	Database core.Database
	Context  core.CloudContext
	SeenAt   time.Time
}

type diskIndexRecord struct {
	Disk    core.Disk
	Context core.CloudContext
	SeenAt  time.Time
}

type snapshotIndexRecord struct {
	Snapshot core.Snapshot
	Context  core.CloudContext
	SeenAt   time.Time
}

type networkIndexRecord struct {
	Network core.Network
	Context core.CloudContext
	SeenAt  time.Time
}

type subnetIndexRecord struct {
	Subnet  core.Subnet
	Context core.CloudContext
	SeenAt  time.Time
}

type firewallIndexRecord struct {
	Group   core.SecurityGroup
	Context core.CloudContext
	SeenAt  time.Time
}

type storageIndexRecord struct {
	Bucket  core.StorageBucket
	Context core.CloudContext
	SeenAt  time.Time
}

type resourceSummaryRecord struct {
	Count int
	Extra int
}

type vmDashboardStats struct {
	Total           int
	Running         int
	Stopped         int
	Other           int
	WithPublicIP    int
	IndexedContexts int
	Stale           int
	LastSeen        time.Time
	ByProvider      map[string]int
}

type databaseDashboardStats struct {
	Total           int
	Running         int
	Stopped         int
	Other           int
	IndexedContexts int
}

type kubernetesDashboardStats struct {
	Clusters int
	Pools    int
	Nodes    int
}

type infrastructureDashboardStats struct {
	Disks            int
	Snapshots        int
	Networks         int
	Subnets          int
	SecurityGroups   int
	Storage          int
	TerraformManaged int
	ManualHosts      int
}

type dashboardWidgetData struct {
	contextCount          int
	vmStats               vmDashboardStats
	dbStats               databaseDashboardStats
	k8sStats              kubernetesDashboardStats
	infraStats            infrastructureDashboardStats
	hiddenKubernetesNodes int
	lastSeen              string
}

type dashboardWidgetEntry struct {
	key    string
	metric string
	label  string
}

type dashboardThemeSpec struct {
	name       string
	rule       string
	accent     string
	cardBg     lipgloss.Color
	text       lipgloss.Color
	muted      lipgloss.Color
	metric     lipgloss.Color
	metricAlt  lipgloss.Color
	highlight  lipgloss.Color
	showBlocks bool
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

type settingsItem struct {
	key         string
	title       string
	description string
	value       string
	toggle      bool
	enabled     bool
}

func (i settingsItem) Title() string {
	if i.toggle {
		if i.enabled {
			return "[x] " + i.title
		}
		return "[ ] " + i.title
	}
	if i.value != "" {
		return i.title + ": " + i.value
	}
	return i.title
}
func (i settingsItem) Description() string { return i.description }
func (i settingsItem) FilterValue() string { return i.title }

type credentialItem struct {
	index   int
	ctx     config.ManagedCloudContext
	current bool
}

func (i credentialItem) Title() string {
	name := strings.TrimSpace(i.ctx.ContextName)
	if name == "" {
		name = managedContextLabel(i.ctx)
	}
	current := " "
	if i.current {
		current = "*"
	}
	return fmt.Sprintf("%s %s  %s", current, strings.TrimSpace(i.ctx.Provider), name)
}
func (i credentialItem) Description() string {
	auth := strings.TrimSpace(i.ctx.CredentialProfile)
	if auth == "" {
		auth = "-"
	}
	mode := config.SanitizeAuthMode(i.ctx.AuthMode)
	persistence := config.SanitizeCredentialPersistence(i.ctx.CredentialPersistence, mode)
	regions := strings.Join(i.ctx.Regions, ",")
	if regions == "" {
		regions = "-"
	}
	return fmt.Sprintf("target=%s mode=%s persist=%s auth=%s tenant=%s regions=%s", managedContextLabel(i.ctx), mode, persistence, auth, orFallback(i.ctx.Tenant, "-"), regions)
}
func (i credentialItem) FilterValue() string { return i.Title() + " " + i.Description() }

type azureSubscriptionItem struct {
	ctx      core.CloudContext
	selected bool
	existing bool
}

func (i azureSubscriptionItem) Title() string {
	marker := "[ ]"
	if i.selected {
		marker = "[x]"
	}
	existing := ""
	if i.existing {
		existing = "  saved"
	}
	return fmt.Sprintf("%s %s%s", marker, i.ctx.DisplayName(), existing)
}
func (i azureSubscriptionItem) Description() string {
	return fmt.Sprintf("subscription=%s tenant=%s", orFallback(i.ctx.AccountID, "-"), orFallback(i.ctx.Tenant, "-"))
}
func (i azureSubscriptionItem) FilterValue() string {
	return strings.Join([]string{i.ctx.ContextName, i.ctx.AccountName, i.ctx.AccountID, i.ctx.Tenant}, " ")
}

type discoveryContextItem struct {
	ctx      core.CloudContext
	selected bool
	existing bool
}

func (i discoveryContextItem) Title() string {
	marker := "[ ]"
	if i.selected {
		marker = "[x]"
	}
	existing := ""
	if i.existing {
		existing = "  saved"
	}
	return fmt.Sprintf("%s %s  %s%s", marker, i.ctx.Provider, i.ctx.DisplayName(), existing)
}
func (i discoveryContextItem) Description() string {
	return fmt.Sprintf("context=%s account=%s tenant=%s region=%s auth=%s", orFallback(i.ctx.ContextName, "-"), orFallback(i.ctx.AccountID, "-"), orFallback(i.ctx.Tenant, "-"), orFallback(i.ctx.Region, "-"), orFallback(i.ctx.AuthRef(), "-"))
}
func (i discoveryContextItem) FilterValue() string {
	return strings.Join([]string{i.ctx.Provider, i.ctx.ContextName, i.ctx.AccountName, i.ctx.AccountID, i.ctx.Tenant, i.ctx.Region, i.ctx.AuthRef()}, " ")
}

type providerLoginItem struct {
	provider    string
	title       string
	description string
	command     []string
	available   bool
	reason      string
}

func (i providerLoginItem) Title() string {
	if !i.available {
		return "[missing] " + i.title
	}
	return i.title
}
func (i providerLoginItem) Description() string {
	if !i.available {
		return i.reason
	}
	return strings.Join(i.command, " ")
}
func (i providerLoginItem) FilterValue() string { return i.title + " " + i.description }

// --- App model (owns sidebar, status, view stack) ---

// App is the top-level Bubble Tea model. It owns the context sidebar,
// status bar, and a stack of Views for drill-down navigation.
type App struct {
	contexts       list.Model
	configList     list.Model
	settingsList   list.Model
	credentialList list.Model
	loginList      list.Model
	azureSubList   list.Model
	discoveryList  list.Model
	findScopeList  list.Model
	findSortList   list.Model
	rootNodes      []*TreeNode
	allContexts    []core.CloudContext
	viewStack      []View
	resourceViews  map[string]registeredView
	activeTab      string
	activeCtx      core.CloudContext
	cfg            config.AppConfig
	focus          int // focusSidebar or focusMain

	statusMsg               string
	parserWarnings          []string
	showSidebar             bool
	showConfig              bool
	showSettings            bool
	showCredentials         bool
	showProviderLogin       bool
	showAzureSubscriptions  bool
	showContextDiscovery    bool
	showFindPicker          bool
	showHostForm            bool
	editCredential          bool
	showLogs                bool
	showHelp                bool
	showGlobalSearch        bool
	showFindSort            bool
	showSplash              bool
	showCmdBar              bool
	cmdBar                  textinput.Model
	helpSearchInput         textinput.Model
	globalSearchInput       textinput.Model
	globalSearchTable       table.Model
	credentialInputs        []textinput.Model
	credentialEditIndex     int
	credentialInputFocus    int
	confirmCredentialDelete bool
	credentialDeleteIndex   int
	hostInputs              []textinput.Model
	hostInputFocus          int
	vmIndex                 map[string]vmSearchRecord
	clusterIndex            map[string]clusterIndexRecord
	databaseIndex           map[string]databaseIndexRecord
	diskIndex               map[string]diskIndexRecord
	snapshotIndex           map[string]snapshotIndexRecord
	networkIndex            map[string]networkIndexRecord
	subnetIndex             map[string]subnetIndexRecord
	firewallIndex           map[string]firewallIndexRecord
	storageIndex            map[string]storageIndexRecord
	diskSummaryIndex        map[string]resourceSummaryRecord
	snapshotSummaryIndex    map[string]resourceSummaryRecord
	networkSummaryIndex     map[string]resourceSummaryRecord
	firewallSummaryIndex    map[string]resourceSummaryRecord
	storageSummaryIndex     map[string]resourceSummaryRecord
	vmSearchRows            []vmSearchRecord
	findScope               findScope
	findRows                []findRecord
	findSortColumn          string
	findSortAsc             bool
	findSortHeader          HeaderSortState
	dashboardCursor         int
	vmPrefetchQueue         []core.CloudContext
	vmPrefetchTotal         int
	vmPrefetchDone          int
	vmPrefetchRunning       int
	vmIndexCacheChecked     bool
	showKubernetesNodes     bool
	width, height           int
	Version                 string
	BuildTime               string
	logView                 viewport.Model
	helpView                viewport.Model
	logPath                 string
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

	settingsDelegate := list.NewDefaultDelegate()
	settingsList := list.New(buildSettingsItems(cfg), settingsDelegate, 0, 0)
	settingsList.Title = "Settings (Space/Enter to toggle, Esc to close)"
	settingsList.SetShowStatusBar(false)
	settingsList.SetFilteringEnabled(false)

	credentialList := list.New(buildCredentialItems(cfg), list.NewDefaultDelegate(), 0, 0)
	credentialList.Title = "Profiles / Contexts (a:Add e:Edit u:Use l:Login z:Azure d:Remove r:Discover)"
	credentialList.SetShowStatusBar(false)
	credentialList.SetFilteringEnabled(false)

	loginList := list.New(buildProviderLoginItems(), list.NewDefaultDelegate(), 0, 0)
	loginList.Title = "Provider CLI Login"
	loginList.SetShowStatusBar(false)
	loginList.SetFilteringEnabled(false)

	azureSubList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	azureSubList.Title = "Azure Subscriptions (Space:Select Enter:Import a:All Esc:Back)"
	azureSubList.SetShowStatusBar(false)
	azureSubList.SetFilteringEnabled(true)

	discoveryList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	discoveryList.Title = "Discovered Contexts (Space:Select Enter:Import a:All /:Filter Esc:Back)"
	discoveryList.SetShowStatusBar(false)
	discoveryList.SetFilteringEnabled(true)

	findScopeList := list.New(buildFindScopeItems(), list.NewDefaultDelegate(), 0, 0)
	findScopeList.Title = "Find"
	findScopeList.SetShowStatusBar(false)
	findScopeList.SetFilteringEnabled(false)

	findSortList := NewSortList("Sort Find by (Enter to select, Esc to cancel)", globalSearchColumns())

	logView := viewport.New(0, 0)
	logView.Style = lipgloss.NewStyle().Padding(0, 1)
	helpView := viewport.New(0, 0)
	helpView.Style = lipgloss.NewStyle().Padding(0, 1)

	cmdInput := textinput.New()
	cmdInput.Prompt = ":"
	cmdInput.Placeholder = "command (e.g., vms, disks, ctx dev)"

	helpSearchInput := textinput.New()
	helpSearchInput.Prompt = "/ "
	helpSearchInput.Placeholder = "filter shortcuts"
	helpSearchInput.CharLimit = 80

	searchInput := textinput.New()
	searchInput.Prompt = "/ "
	searchInput.Placeholder = "find resources"
	searchInput.CharLimit = 100

	searchTable := table.New(
		table.WithColumns(globalSearchColumns()),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	searchTable.SetStyles(DefaultTableStyles())

	return App{
		contexts:              ctxList,
		configList:            configList,
		settingsList:          settingsList,
		credentialList:        credentialList,
		loginList:             loginList,
		azureSubList:          azureSubList,
		discoveryList:         discoveryList,
		findScopeList:         findScopeList,
		findSortList:          findSortList,
		resourceViews:         make(map[string]registeredView),
		activeTab:             "1",
		cfg:                   cfg,
		Version:               version,
		BuildTime:             buildTime,
		focus:                 focusMain,
		statusMsg:             "Ready.",
		showSidebar:           true,
		showSplash:            true,
		showCmdBar:            false,
		cmdBar:                cmdInput,
		helpSearchInput:       helpSearchInput,
		globalSearchInput:     searchInput,
		globalSearchTable:     searchTable,
		credentialDeleteIndex: -1,
		findScope:             findScopeVMs,
		findSortAsc:           true,
		vmIndex:               make(map[string]vmSearchRecord),
		clusterIndex:          make(map[string]clusterIndexRecord),
		databaseIndex:         make(map[string]databaseIndexRecord),
		diskIndex:             make(map[string]diskIndexRecord),
		snapshotIndex:         make(map[string]snapshotIndexRecord),
		networkIndex:          make(map[string]networkIndexRecord),
		subnetIndex:           make(map[string]subnetIndexRecord),
		firewallIndex:         make(map[string]firewallIndexRecord),
		storageIndex:          make(map[string]storageIndexRecord),
		diskSummaryIndex:      make(map[string]resourceSummaryRecord),
		snapshotSummaryIndex:  make(map[string]resourceSummaryRecord),
		networkSummaryIndex:   make(map[string]resourceSummaryRecord),
		firewallSummaryIndex:  make(map[string]resourceSummaryRecord),
		storageSummaryIndex:   make(map[string]resourceSummaryRecord),
		showKubernetesNodes:   !cfg.HideKubernetesNodes,
		logView:               logView,
		helpView:              helpView,
		logPath:               logging.Path(),
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
	return tea.Batch(fetchContextsCmd(a.cfg.DiscoverOnStart), loadVMIndexCacheCmd(a.cfg), loadResourceIndexCacheCmd(a.cfg))
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
		if a.showGlobalSearch {
			return a.handleGlobalSearchKeys(msg)
		}
		if a.showConfig {
			return a.handleConfigKeys(msg)
		}
		if a.showSettings {
			return a.handleSettingsKeys(msg)
		}
		if a.showFindPicker {
			return a.handleFindScopeKeys(msg)
		}
		if a.showAzureSubscriptions {
			return a.handleAzureSubscriptionKeys(msg)
		}
		if a.showContextDiscovery {
			return a.handleContextDiscoveryKeys(msg)
		}
		if a.showCredentials {
			return a.handleCredentialKeys(msg)
		}
		if a.showHostForm {
			return a.handleHostFormKeys(msg)
		}
		if a.showProviderLogin {
			return a.handleProviderLoginKeys(msg)
		}
		if a.showLogs {
			return a.handleLogKeys(msg)
		}
		if a.showHelp {
			return a.handleHelpKeys(msg)
		}

		if a.showCmdBar {
			switch msg.String() {
			case "esc":
				a.showCmdBar = false
				a.cmdBar.Blur()
				return a, nil
			case "enter":
				a.showCmdBar = false
				a.cmdBar.Blur()
				return a.handleCommand(a.cmdBar.Value())
			}
			var cmd tea.Cmd
			a.cmdBar, cmd = a.cmdBar.Update(msg)
			return a, cmd
		}

		isInputActive := false
		if a.focus == focusMain && len(a.viewStack) > 0 && !a.isHomeActive() {
			isInputActive = a.viewStack[len(a.viewStack)-1].IsInputActive()
		} else if a.contexts.FilterState() == list.Filtering {
			isInputActive = true
		}

		if msg.String() == "enter" && a.contexts.FilterState() != list.Unfiltered {
			a.contexts, cmd = a.contexts.Update(msg)
			cmds = append(cmds, cmd)
			return a.activateFilteredContext()
		}

		if !isInputActive && a.focus == focusMain && a.isHomeActive() {
			switch msg.String() {
			case "left", "h":
				a.moveDashboardCursor(-1)
				return a, nil
			case "right", "l":
				a.moveDashboardCursor(1)
				return a, nil
			case "up", "k":
				a.moveDashboardCursor(-a.dashboardColumns())
				return a, nil
			case "down", "j":
				a.moveDashboardCursor(a.dashboardColumns())
				return a, nil
			case "enter":
				return a.activateDashboardWidget()
			}
		}

		switch msg.String() {
		case "g":
			if !isInputActive && a.cfg.GlobalSearch {
				a.openFindScopePicker()
				return a, nil
			}
		case "?", "f1":
			if !isInputActive {
				a.openHelp()
				return a, nil
			}
		case ",":
			if !isInputActive {
				a.openSettings()
				return a, nil
			}
		case "H":
			if !isInputActive {
				a.openHome()
				return a, nil
			}
		case "K":
			if !isInputActive {
				a.toggleKubernetesNodes()
				return a, nil
			}
		case ":":
			if !isInputActive {
				a.showCmdBar = true
				a.cmdBar.SetValue("")
				a.cmdBar.Focus()
				return a, textinput.Blink
			}
		case "1", "2", "3", "4", "5", "6", "7", "8":
			if !isInputActive {
				if view, ok := a.resourceViews[msg.String()]; ok {
					if a.activeCtx.Provider != "" && !providers.Supports(a.activeCtx.Provider, view.Capability) {
						a.statusMsg = fmt.Sprintf("%s not supported for %s.", view.View.Title(), a.activeCtx.Provider)
						return a, nil
					}
					a.activeTab = msg.String()
					a.viewStack = []View{view.View}
					a.applyKubernetesNodeVisibility()
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
		case "B":
			if !isInputActive {
				if strings.ToLower(a.cfg.Backend) == "sdk" {
					a.cfg.Backend = "cli"
				} else {
					a.cfg.Backend = "sdk"
				}
				config.Save(a.cfg)
				a.statusMsg = fmt.Sprintf("Switched backend to %s", strings.ToUpper(a.cfg.Backend))
				logging.Infof("component=ui event=backend_toggle new_backend=%s", a.cfg.Backend)

				// Automatically refresh the active view with the new backend
				if len(a.viewStack) > 0 {
					var cmd tea.Cmd
					a.viewStack[len(a.viewStack)-1], cmd = a.viewStack[len(a.viewStack)-1].Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
					return a, cmd
				}
				return a, nil
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
				a.showContextDiscovery = false
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
				return a.activateSelectedContext()
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.contexts.SetSize(a.sidebarContentWidth(), a.sidebarContentHeight())
		a.configList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.settingsList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.credentialList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.loginList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.azureSubList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.discoveryList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.findScopeList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.resizeGlobalSearch()
		a.resizeViews()
		a.resizeLogView()
		a.resizeHelpView()
		if a.showHelp {
			a.refreshHelpContent()
		}
		return a, nil

	case contextLoadMsg:
		a.rootNodes = msg.tree
		a.allContexts = msg.contexts
		items := BuildFlatList(a.rootNodes)
		a.contexts.SetItems(items)
		if idx, ctx, ok := currentContextListSelection(items, a.cfg.CurrentContext); ok {
			a.contexts.Select(idx)
			a.activeCtx = ctx
			a.ensureActiveTab()
			if len(a.viewStack) > 0 {
				cmds = append(cmds, a.viewStack[len(a.viewStack)-1].Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar))
			}
		}
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
		a.indexManualHosts()
		if len(a.viewStack) == 0 {
			a.focus = focusMain
		}
		a.showSplash = false
		cmds = append(cmds, a.startStartupVMPrefetch()...)

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

	case vmIndexCacheLoadMsg:
		a.vmIndexCacheChecked = true
		if msg.err != nil {
			logging.Warnf("component=ui event=vm_index_cache_load_failed err=%v", msg.err)
		} else if len(msg.index) > 0 {
			for key, rec := range msg.index {
				rec.VM = config.ApplyResourceTagsToVM(a.cfg, rec.Context, rec.VM)
				msg.index[key] = rec
			}
			a.vmIndex = msg.index
			a.refreshGlobalSearchResults()
			a.statusMsg = fmt.Sprintf("Loaded %d cached VM index records.", len(msg.index))
			logging.Infof("component=ui event=vm_index_cache_loaded count=%d total=%d", len(msg.index), msg.total)
		}
		if len(msg.index) == 0 && len(a.allContexts) > 0 {
			cmds = append(cmds, a.startStartupVMPrefetch()...)
		}

	case resourceIndexCacheLoadMsg:
		if msg.err != nil {
			logging.Warnf("component=ui event=resource_index_cache_load_failed err=%v", msg.err)
			break
		}
		if msg.cache.Total > 0 {
			a.clusterIndex = msg.cache.Clusters
			a.databaseIndex = msg.cache.Databases
			a.diskIndex = msg.cache.Disks
			a.snapshotIndex = msg.cache.Snapshots
			a.networkIndex = msg.cache.Networks
			a.subnetIndex = msg.cache.Subnets
			a.firewallIndex = msg.cache.Firewalls
			a.storageIndex = msg.cache.Storage
			a.applyCloudManagerTagsToResourceIndexes()
			a.diskSummaryIndex = msg.cache.DiskSummaries
			a.snapshotSummaryIndex = msg.cache.SnapshotSummaries
			a.networkSummaryIndex = msg.cache.NetworkSummaries
			a.firewallSummaryIndex = msg.cache.FirewallSummaries
			a.storageSummaryIndex = msg.cache.StorageSummaries
			a.refreshGlobalSearchResults()
			a.statusMsg = fmt.Sprintf("Loaded %d cached resource index records.", msg.cache.Total)
			logging.Infof("component=ui event=resource_index_cache_loaded count=%d", msg.cache.Total)
		}

	case vmPrefetchMsg:
		a.vmPrefetchRunning--
		if a.vmPrefetchRunning < 0 {
			a.vmPrefetchRunning = 0
		}
		a.vmPrefetchDone++
		if msg.err != nil {
			logging.Warnf("component=ui event=vm_prefetch_failed provider=%s account=%s region=%s err=%v", msg.ctx.Provider, msg.ctx.AccountID, msg.ctx.Region, msg.err)
		} else {
			a.indexVMs(msg.ctx, msg.vms)
			a.refreshGlobalSearchResults()
			logging.Infof("component=ui event=vm_prefetch_completed provider=%s account=%s region=%s count=%d", msg.ctx.Provider, msg.ctx.AccountID, msg.ctx.Region, len(msg.vms))
		}
		cmds = append(cmds, a.continueVMPrefetch()...)
		if a.vmPrefetchTotal > 0 {
			a.statusMsg = fmt.Sprintf("Indexing VMs: %d/%d contexts", a.vmPrefetchDone, a.vmPrefetchTotal)
			if a.vmPrefetchDone >= a.vmPrefetchTotal {
				a.statusMsg = fmt.Sprintf("Indexed %d VMs across %d contexts.", len(a.vmIndex), a.vmPrefetchTotal)
			}
		}

	case VMIndexUpdateMsg:
		a.indexVMs(msg.Ctx, msg.VMs)
		a.refreshGlobalSearchResults()

	case ClusterIndexUpdateMsg:
		if msg.Err != nil {
			logging.Warnf("component=ui event=cluster_summary_failed provider=%s account=%s region=%s err=%v", msg.Ctx.Provider, msg.Ctx.AccountID, msg.Ctx.Region, msg.Err)
			break
		}
		a.indexClusters(msg.Ctx, msg.Clusters)
		a.persistResourceIndex()
		a.refreshGlobalSearchResults()

	case DatabaseIndexUpdateMsg:
		if msg.Err != nil {
			logging.Warnf("component=ui event=database_summary_failed provider=%s account=%s region=%s err=%v", msg.Ctx.Provider, msg.Ctx.AccountID, msg.Ctx.Region, msg.Err)
			break
		}
		a.indexDatabases(msg.Ctx, msg.Databases)
		a.persistResourceIndex()
		a.refreshGlobalSearchResults()

	case StorageIndexUpdateMsg:
		if msg.Err != nil {
			logging.Warnf("component=ui event=storage_summary_failed provider=%s account=%s region=%s err=%v", msg.Ctx.Provider, msg.Ctx.AccountID, msg.Ctx.Region, msg.Err)
			break
		}
		a.indexStorage(msg.Ctx, msg.Buckets)
		a.indexResourceSummary(msg.Ctx, "storage", len(msg.Buckets), 0)
		a.persistResourceIndex()
		a.refreshGlobalSearchResults()

	case ResourceSummaryUpdateMsg:
		if msg.Err != nil {
			logging.Warnf("component=ui event=resource_summary_failed resource=%s provider=%s account=%s region=%s err=%v", msg.Resource, msg.Ctx.Provider, msg.Ctx.AccountID, msg.Ctx.Region, msg.Err)
			break
		}
		a.indexResourceSummaryMsg(msg)
		a.persistResourceIndex()
		a.refreshGlobalSearchResults()
		a.statusMsg = fmt.Sprintf("Indexed %s summary for %s.", msg.Resource, msg.Ctx.DisplayName())

	case providerLoginCompleteMsg:
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf("%s login failed: %v", msg.provider, msg.err)
			break
		}
		if msg.contextName != "" {
			a.cfg.CurrentContext = msg.contextName
			_ = config.Save(a.cfg)
			a.statusMsg = fmt.Sprintf("%s login complete for %s. Discovering contexts...", msg.provider, msg.contextName)
		} else {
			a.statusMsg = fmt.Sprintf("%s login complete. Discovering contexts...", msg.provider)
		}
		cmds = append(cmds, fetchDiscoveredContextsCmd())

	case azureSubscriptionLoadMsg:
		if len(msg.warnings) > 0 && len(msg.contexts) == 0 {
			a.statusMsg = strings.Join(msg.warnings, " | ")
			break
		}
		a.showAzureSubscriptions = true
		a.showContextDiscovery = false
		a.showCredentials = false
		a.showSettings = false
		a.azureSubList.SetItems(buildAzureSubscriptionItems(a.cfg, msg.contexts))
		a.azureSubList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		if len(msg.contexts) == 0 {
			a.statusMsg = "No Azure subscriptions found. Run Azure login first."
		} else {
			a.statusMsg = fmt.Sprintf("Found %d Azure subscriptions. Space selects, Enter imports.", len(msg.contexts))
		}

	case contextDiscoveryLoadMsg:
		a.showContextDiscovery = true
		a.showCredentials = false
		a.showSettings = false
		a.showAzureSubscriptions = false
		a.discoveryList.SetItems(buildDiscoveryContextItems(a.cfg, msg.contexts))
		a.discoveryList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
		a.parserWarnings = msg.warnings
		if len(msg.contexts) == 0 {
			if len(msg.warnings) > 0 {
				a.statusMsg = fmt.Sprintf("No discoverable contexts. %s", strings.Join(msg.warnings, " | "))
			} else {
				a.statusMsg = "No discoverable contexts found."
			}
		} else if len(msg.warnings) > 0 {
			a.statusMsg = fmt.Sprintf("Found %d contexts. Filter, Space-select, Enter imports. Warnings: %s", len(msg.contexts), strings.Join(msg.warnings, " | "))
		} else {
			a.statusMsg = fmt.Sprintf("Found %d contexts. Filter, Space-select, Enter imports.", len(msg.contexts))
		}

	case StatusUpdateMsg:
		a.statusMsg = msg.Msg

	case ManualHostsChangedMsg:
		a.cfg = config.Load()
		a.allContexts = config.ManagedContexts(a.cfg)
		a.rootNodes = BuildContextTree(a.allContexts)
		a.contexts.SetItems(BuildFlatList(a.rootNodes))
		a.pruneManualHostsFromIndex()
		a.indexManualHosts()
		a.statusMsg = msg.Msg
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
	if a.showSettings {
		a.settingsList, cmd = a.settingsList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if a.showFindPicker {
		a.findScopeList, cmd = a.findScopeList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if a.showAzureSubscriptions {
		a.azureSubList, cmd = a.azureSubList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if a.showCredentials {
		a.credentialList, cmd = a.credentialList.Update(msg)
		cmds = append(cmds, cmd)
	}
	if a.showProviderLogin {
		a.loginList, cmd = a.loginList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func currentContextListSelection(items []list.Item, name string) (int, core.CloudContext, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return 0, core.CloudContext{}, false
	}
	for i, item := range items {
		node, ok := item.(*TreeNode)
		if !ok || !node.IsLeaf {
			continue
		}
		ctx := node.Context
		for _, candidate := range []string{ctx.ContextName, ctx.DisplayName(), ctx.AccountID, ctx.AccountName, ctx.CredentialProfile} {
			if strings.ToLower(strings.TrimSpace(candidate)) == name {
				return i, ctx, true
			}
		}
	}
	return 0, core.CloudContext{}, false
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
	if a.showSettings {
		mainView := a.renderShellPane(a.settingsList.View(), true)
		footer := renderFooter(a.width, fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space/Enter: Toggle \u2022 Esc: Close | %s", a.statusMsg))
		return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
	}
	if a.showFindPicker {
		return a.renderFindScopeView()
	}
	if a.showAzureSubscriptions {
		return a.renderAzureSubscriptionView()
	}
	if a.showContextDiscovery {
		return a.renderContextDiscoveryView()
	}
	if a.showCredentials {
		return a.renderCredentialsView()
	}
	if a.showHostForm {
		return a.renderHostFormView()
	}
	if a.showProviderLogin {
		return a.renderProviderLoginView()
	}
	if a.showLogs {
		return a.renderLogsView()
	}
	if a.showHelp {
		return a.renderHelpView()
	}
	if a.showGlobalSearch {
		return a.renderGlobalSearchView()
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

	if a.activeCtx.Provider == "" {
		mainContent = a.renderHomePanel(mainWidth, shellInnerHeight-lipgloss.Height(tabBar))
		if a.showCmdBar {
			cmdStyle := lipgloss.NewStyle().Width(mainWidth).Padding(0, 1).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("255")).Bold(true)
			tabBar = cmdStyle.Render(a.cmdBar.View())
		}
	} else if len(a.viewStack) > 0 {
		mainContent = a.viewStack[len(a.viewStack)-1].Render()
		if a.showCmdBar {
			cmdStyle := lipgloss.NewStyle().Width(mainWidth).Padding(0, 1).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("255")).Bold(true)
			tabBar = cmdStyle.Render(a.cmdBar.View())
		}
	} else {
		mainContent = a.renderHomePanel(mainWidth, shellInnerHeight-lipgloss.Height(tabBar))
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
	footerText := fmt.Sprintf("\u2191\u2193 \u2022 Enter \u2022 ?:Help \u2022 H:Home \u2022 g:Find \u2022 ,:Set \u2022 Tab \u2022 Esc \u2022 L:Logs \u2022 B:Mode | Mode:%s | %s | %s | v%s", a.backendMode(), sysusage.FooterText(), a.statusMsg, a.Version)
	footer := renderFooter(a.width, footerText)
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainShell, footer), a.width, a.height)
}

func (a App) isHomeActive() bool {
	return strings.TrimSpace(a.activeCtx.Provider) == ""
}

func (a App) renderHomePanel(width, height int) string {
	stats := a.vmDashboardStats()
	dbStats := a.databaseDashboardStats()
	k8sStats := a.kubernetesDashboardStats()
	infraStats := a.infrastructureDashboardStats()
	visibleRecords := a.visibleVMIndexRecords()
	hiddenKubernetesNodes := len(a.vmIndex) - len(visibleRecords)
	indexStatus := fmt.Sprintf("%d VMs indexed", len(visibleRecords))
	if a.vmPrefetchTotal > 0 && a.vmPrefetchDone < a.vmPrefetchTotal {
		indexStatus = fmt.Sprintf("Indexing VMs: %d/%d contexts", a.vmPrefetchDone, a.vmPrefetchTotal)
	}
	contextCount := len(a.allContexts)
	if contextCount == 0 {
		contextCount = countContextLeaves(a.rootNodes)
	}
	lastSeen := "No VM index yet"
	if !stats.LastSeen.IsZero() {
		age := compactDuration(time.Since(stats.LastSeen))
		if age == "just now" {
			lastSeen = "Last indexed just now"
		} else {
			lastSeen = fmt.Sprintf("Last indexed %s ago", age)
		}
	}
	data := dashboardWidgetData{
		contextCount:          contextCount,
		vmStats:               stats,
		dbStats:               dbStats,
		k8sStats:              k8sStats,
		infraStats:            infraStats,
		hiddenKubernetesNodes: hiddenKubernetesNodes,
		lastSeen:              lastSeen,
	}
	theme := dashboardTheme(a.cfg.DashboardTheme)
	cardRows := a.renderDashboardWidgetRows(data, width, theme)
	providerLine := "VMs by provider: " + formatProviderCounts(stats.ByProvider)
	if len(stats.ByProvider) == 0 {
		providerLine = "VMs by provider: none indexed yet"
	}
	cacheLine := fmt.Sprintf("Indexed contexts: %d", stats.IndexedContexts)
	if stats.Stale > 0 {
		cacheLine = fmt.Sprintf("%s | stale records: %d", cacheLine, stats.Stale)
	}
	kubernetesLine := "K8s worker nodes: shown"
	if !a.showKubernetesNodes {
		kubernetesLine = fmt.Sprintf("K8s worker nodes: hidden %d | K to show", hiddenKubernetesNodes)
	}
	runningLine := fmt.Sprintf("VMs running: %d of %d %s", stats.Running, stats.Total, ratioBar(stats.Running, stats.Total, 24))
	contentWidth := width - 4
	if contentWidth > 132 {
		contentWidth = 132
	}
	if contentWidth < 42 {
		contentWidth = width
	}
	title := lipgloss.NewStyle().Foreground(theme.highlight).Bold(true).Render("CloudManager")
	headerMeta := lipgloss.NewStyle().Foreground(theme.muted).Render(indexStatus + " | " + lastSeen)
	headerGap := contentWidth - lipgloss.Width(title) - lipgloss.Width(headerMeta)
	if headerGap < 2 {
		headerGap = 2
	}
	header := title + strings.Repeat(" ", headerGap) + headerMeta
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Foreground(theme.highlight).Render(strings.Repeat(theme.rule, contentWidth)),
		"",
		cardRows,
		"",
		renderDashboardSummary(contentWidth, theme, []string{
			runningLine,
			providerLine,
			cacheLine,
			kubernetesLine,
			fmt.Sprintf("Databases: %d total | %d ready | %d down | %d other | %d contexts", dbStats.Total, dbStats.Running, dbStats.Stopped, dbStats.Other, dbStats.IndexedContexts),
			fmt.Sprintf("K8s clusters:pools:nodes: %d:%d:%d", k8sStats.Clusters, k8sStats.Pools, k8sStats.Nodes),
			fmt.Sprintf("Disks indexed: %d | snapshots indexed: %d", infraStats.Disks, infraStats.Snapshots),
			fmt.Sprintf("Networks seen: %d | subnets seen: %d | security groups seen: %d", infraStats.Networks, infraStats.Subnets, infraStats.SecurityGroups),
			fmt.Sprintf("Storage indexed: %d", infraStats.Storage),
			fmt.Sprintf("Terraform-managed: %d | manual hosts: %d", infraStats.TerraformManaged, infraStats.ManualHosts),
		}),
		"",
		renderDashboardCommands(contentWidth, theme),
	)
	content := lipgloss.NewStyle().Width(contentWidth).MaxWidth(contentWidth).Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content)
}

func (a App) vmDashboardStats() vmDashboardStats {
	stats := vmDashboardStats{ByProvider: make(map[string]int)}
	contexts := make(map[string]struct{})
	now := time.Now()
	ttl := time.Duration(a.cfg.VMIndexCacheTTL) * time.Hour
	for _, rec := range a.visibleVMIndexRecords() {
		stats.Total++
		switch vmStateClass(rec.VM.State) {
		case "running":
			stats.Running++
		case "stopped":
			stats.Stopped++
		default:
			stats.Other++
		}
		if hasUsablePublicIP(rec.VM.PublicIP) {
			stats.WithPublicIP++
		}
		provider := strings.TrimSpace(rec.Context.Provider)
		if provider == "" {
			provider = "Unknown"
		}
		stats.ByProvider[provider]++
		if key := rec.Context.CacheKey(); key != "" {
			contexts[key] = struct{}{}
		}
		if !rec.SeenAt.IsZero() {
			if rec.SeenAt.After(stats.LastSeen) {
				stats.LastSeen = rec.SeenAt
			}
			if ttl > 0 && now.Sub(rec.SeenAt) > ttl {
				stats.Stale++
			}
		}
	}
	stats.IndexedContexts = len(contexts)
	return stats
}

func (a App) infrastructureDashboardStats() infrastructureDashboardStats {
	var stats infrastructureDashboardStats
	networks := map[string]struct{}{}
	subnets := map[string]struct{}{}
	securityGroups := map[string]struct{}{}
	for _, rec := range a.diskSummaryIndex {
		stats.Disks += rec.Count
	}
	for _, rec := range a.snapshotSummaryIndex {
		stats.Snapshots += rec.Count
	}
	for _, rec := range a.networkSummaryIndex {
		stats.Networks += rec.Count
		stats.Subnets += rec.Extra
	}
	for _, rec := range a.firewallSummaryIndex {
		stats.SecurityGroups += rec.Count
	}
	for _, rec := range a.storageSummaryIndex {
		stats.Storage += rec.Count
	}
	for _, rec := range a.visibleVMIndexRecords() {
		if strings.EqualFold(rec.Context.Provider, "Manual") {
			stats.ManualHosts++
		}
		if strings.Contains(strings.ToLower(rec.VM.Labels), "iac:terraform") {
			stats.TerraformManaged++
		}
		addDashboardSetValues(networks, rec.VM.Network)
		addDashboardSetValues(subnets, rec.VM.Subnet)
		addDashboardSetValues(securityGroups, rec.VM.SecurityGroups)
	}
	for _, rec := range a.databaseIndex {
		if strings.Contains(strings.ToLower(rec.Database.Labels), "iac:terraform") {
			stats.TerraformManaged++
		}
	}
	for _, rec := range a.clusterIndex {
		if strings.Contains(strings.ToLower(rec.Cluster.Labels), "iac:terraform") {
			stats.TerraformManaged++
		}
	}
	if stats.ManualHosts == 0 {
		stats.ManualHosts = len(a.cfg.ManualHosts)
	}
	if stats.Networks == 0 {
		stats.Networks = len(networks)
	}
	if stats.Subnets == 0 {
		stats.Subnets = len(subnets)
	}
	if stats.SecurityGroups == 0 {
		stats.SecurityGroups = len(securityGroups)
	}
	if stats.Storage == 0 {
		stats.Storage = len(a.storageIndex)
	}
	return stats
}

func addDashboardSetValues(set map[string]struct{}, value string) {
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			continue
		}
		set[strings.ToLower(part)] = struct{}{}
	}
}

func (a App) renderDashboardWidgetRows(data dashboardWidgetData, width int, theme dashboardThemeSpec) string {
	entries := a.dashboardWidgetEntries(data)
	cards := make([]string, 0, len(entries))
	cardWidth := dashboardCardWidth(width)
	selected := a.dashboardSelectedIndex()
	for i, entry := range entries {
		cards = append(cards, renderDashboardCard(entry.metric, entry.label, cardWidth, theme, i, i == selected))
	}
	if len(cards) == 0 {
		return ""
	}
	perRow := dashboardCardsPerRow(width, cardWidth)
	rows := make([]string, 0, (len(cards)+perRow-1)/perRow)
	for i := 0; i < len(cards); i += perRow {
		end := i + perRow
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (a App) dashboardWidgetEntries(data dashboardWidgetData) []dashboardWidgetEntry {
	widgets := config.SanitizeDashboardWidgets(a.cfg.DashboardWidgets)
	entries := make([]dashboardWidgetEntry, 0, len(widgets))
	for _, widget := range widgets {
		metric, label, ok := a.dashboardWidget(widget, data)
		if !ok {
			continue
		}
		entries = append(entries, dashboardWidgetEntry{key: widget, metric: metric, label: label})
	}
	return entries
}

func (a App) dashboardWidget(widget string, data dashboardWidgetData) (string, string, bool) {
	switch widget {
	case "contexts":
		return fmt.Sprintf("%d", data.contextCount), "Contexts", true
	case "indexed_vms":
		return fmt.Sprintf("%d", data.vmStats.Total), "Indexed VMs", true
	case "running_vms":
		return fmt.Sprintf("%d", data.vmStats.Running), "Running VMs", true
	case "stopped_vms":
		return fmt.Sprintf("%d", data.vmStats.Stopped), "Stopped VMs", true
	case "public_ips":
		return fmt.Sprintf("%d", data.vmStats.WithPublicIP), "Public IPs", true
	case "disks":
		return fmt.Sprintf("%d", data.infraStats.Disks), "Disks indexed", true
	case "snapshots":
		return fmt.Sprintf("%d", data.infraStats.Snapshots), "Snapshots indexed", true
	case "networks":
		return fmt.Sprintf("%d", data.infraStats.Networks), "Networks seen", true
	case "subnets":
		return fmt.Sprintf("%d", data.infraStats.Subnets), "Subnets seen", true
	case "firewalls":
		return fmt.Sprintf("%d", data.infraStats.SecurityGroups), "Security groups seen", true
	case "storage":
		return fmt.Sprintf("%d", data.infraStats.Storage), "Storage indexed", true
	case "backend":
		return strings.ToUpper(a.backendMode()), "Backend", true
	case "databases":
		return fmt.Sprintf("%d", data.dbStats.Total), formatDatabaseDashboardLabel(data.dbStats), true
	case "kubernetes":
		return fmt.Sprintf("%d:%d:%d", data.k8sStats.Clusters, data.k8sStats.Pools, data.k8sStats.Nodes), "K8s clusters:pools:nodes", true
	case "terraform":
		return fmt.Sprintf("%d", data.infraStats.TerraformManaged), "Terraform managed", true
	case "manual_hosts":
		return fmt.Sprintf("%d", data.infraStats.ManualHosts), "Manual hosts", true
	case "db_contexts":
		return fmt.Sprintf("%d", data.dbStats.IndexedContexts), "DB contexts indexed", true
	case "vm_providers":
		return fmt.Sprintf("%d", len(data.vmStats.ByProvider)), "VM providers indexed", true
	case "vm_index_age":
		return strings.TrimPrefix(data.lastSeen, "Last indexed "), "VM index age", true
	case "k8s_hidden":
		return fmt.Sprintf("%d", data.hiddenKubernetesNodes), "Hidden K8s nodes", true
	default:
		return "", "", false
	}
}

func (a App) dashboardData() dashboardWidgetData {
	contextCount := len(a.allContexts)
	if contextCount == 0 {
		contextCount = countContextLeaves(a.rootNodes)
	}
	stats := a.vmDashboardStats()
	lastSeen := "No VM index yet"
	if !stats.LastSeen.IsZero() {
		age := compactDuration(time.Since(stats.LastSeen))
		if age == "just now" {
			lastSeen = "Last indexed just now"
		} else {
			lastSeen = fmt.Sprintf("Last indexed %s ago", age)
		}
	}
	return dashboardWidgetData{
		contextCount:          contextCount,
		vmStats:               stats,
		dbStats:               a.databaseDashboardStats(),
		k8sStats:              a.kubernetesDashboardStats(),
		infraStats:            a.infrastructureDashboardStats(),
		hiddenKubernetesNodes: len(a.vmIndex) - len(a.visibleVMIndexRecords()),
		lastSeen:              lastSeen,
	}
}

func (a App) dashboardSelectedIndex() int {
	count := len(a.dashboardWidgetEntries(a.dashboardData()))
	if count == 0 {
		return 0
	}
	if a.dashboardCursor < 0 {
		return 0
	}
	if a.dashboardCursor >= count {
		return count - 1
	}
	return a.dashboardCursor
}

func (a App) dashboardColumns() int {
	return dashboardCardsPerRow(a.width, dashboardCardWidth(a.width))
}

func (a *App) moveDashboardCursor(delta int) {
	count := len(a.dashboardWidgetEntries(a.dashboardData()))
	if count == 0 {
		a.dashboardCursor = 0
		return
	}
	a.dashboardCursor += delta
	if a.dashboardCursor < 0 {
		a.dashboardCursor = 0
	}
	if a.dashboardCursor >= count {
		a.dashboardCursor = count - 1
	}
}

func (a App) activateDashboardWidget() (App, tea.Cmd) {
	entries := a.dashboardWidgetEntries(a.dashboardData())
	if len(entries) == 0 {
		a.statusMsg = "No dashboard widgets configured."
		return a, nil
	}
	selected := a.dashboardSelectedIndex()
	key := entries[selected].key
	switch key {
	case "contexts":
		a.openCredentials()
		return a, nil
	case "databases", "db_contexts":
		return a.openFind(findScopeDatabases, "")
	case "kubernetes":
		return a.openFind(findScopeK8s, "")
	case "manual_hosts":
		return a.openFind(findScopeHosts, "")
	case "terraform":
		return a.openFind(findScopeAll, "iac:terraform")
	case "running_vms":
		return a.openFind(findScopeVMs, "running")
	case "stopped_vms":
		return a.openFind(findScopeVMs, "stopped")
	case "public_ips":
		return a.openFind(findScopeVMs, "has:public-ip")
	case "disks":
		return a.openFind(findScopeDisks, "")
	case "snapshots":
		return a.openFind(findScopeSnapshots, "")
	case "networks":
		return a.openFind(findScopeNetworks, "")
	case "subnets":
		return a.openFind(findScopeSubnets, "")
	case "firewalls":
		return a.openFind(findScopeFirewalls, "")
	case "storage":
		return a.openFind(findScopeStorage, "")
	case "backend":
		a.openSettings()
		return a, nil
	case "k8s_hidden":
		a.toggleKubernetesNodes()
		return a, nil
	default:
		return a.openFind(findScopeVMs, "")
	}
}

func dashboardCardWidth(width int) int {
	contentWidth := width - 4
	if contentWidth < 12 {
		contentWidth = 12
	}
	base := 21
	switch {
	case width >= 132:
		base = 25
	case width >= 96:
		base = 23
	}
	if base > contentWidth {
		return contentWidth
	}
	return base
}

func dashboardCardsPerRow(width, cardWidth int) int {
	contentWidth := width - 4
	if contentWidth > 132 {
		contentWidth = 132
	}
	cardOuterWidth := cardWidth + 1
	if cardOuterWidth <= 0 {
		cardOuterWidth = 1
	}
	perRow := contentWidth / cardOuterWidth
	if perRow < 1 {
		return 1
	}
	if perRow > 5 {
		return 5
	}
	return perRow
}

func renderDashboardCard(metric, label string, width int, theme dashboardThemeSpec, index int, selected bool) string {
	metricColor := theme.metric
	if index%2 == 1 {
		metricColor = theme.metricAlt
	}
	cardStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(width).
		MarginRight(1).
		MarginBottom(1)
	if theme.showBlocks {
		cardStyle = cardStyle.Background(theme.cardBg)
	}
	if selected {
		cardStyle = cardStyle.Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.highlight).
			Padding(0, 1)
	}
	metricStyle := lipgloss.NewStyle().Foreground(metricColor).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(theme.muted)
	return cardStyle.Render(metricStyle.Render(uiSafeMetric(metric, width-4)) + "\n" + labelStyle.Render(uiSafeMetric(label, width-4)))
}

func uiSafeMetric(value string, width int) string {
	if width < 4 {
		width = 4
	}
	return truncateText(value, width)
}

func dashboardTheme(name string) dashboardThemeSpec {
	switch config.SanitizeDashboardTheme(name) {
	case "classic":
		return dashboardThemeSpec{
			name:       "classic",
			rule:       "━",
			accent:     "▌",
			cardBg:     lipgloss.Color("236"),
			text:       lipgloss.Color("252"),
			muted:      lipgloss.Color("245"),
			metric:     lipgloss.Color(string(Special.Dark)),
			metricAlt:  lipgloss.Color(string(Highlight.Dark)),
			highlight:  lipgloss.Color(string(Highlight.Dark)),
			showBlocks: true,
		}
	case "neon":
		return dashboardThemeSpec{
			name:       "neon",
			rule:       "━",
			accent:     "▌",
			cardBg:     lipgloss.Color("#10101A"),
			text:       lipgloss.Color("#F2F7FF"),
			muted:      lipgloss.Color("#8B9BB4"),
			metric:     lipgloss.Color("#00E5FF"),
			metricAlt:  lipgloss.Color("#FF5FD7"),
			highlight:  lipgloss.Color("#C084FC"),
			showBlocks: true,
		}
	case "mono":
		return dashboardThemeSpec{
			name:       "mono",
			rule:       "━",
			accent:     "│",
			cardBg:     lipgloss.Color(""),
			text:       lipgloss.Color("252"),
			muted:      lipgloss.Color("245"),
			metric:     lipgloss.Color("252"),
			metricAlt:  lipgloss.Color("250"),
			highlight:  lipgloss.Color("252"),
			showBlocks: false,
		}
	default:
		return dashboardThemeSpec{
			name:       "btop",
			rule:       "━",
			accent:     "▌",
			cardBg:     lipgloss.Color("#0E1720"),
			text:       lipgloss.Color("#EAF2FF"),
			muted:      lipgloss.Color("#8CA0B8"),
			metric:     lipgloss.Color("#5EEAD4"),
			metricAlt:  lipgloss.Color("#FDE047"),
			highlight:  lipgloss.Color("#38BDF8"),
			showBlocks: true,
		}
	}
}

func renderDashboardSummary(width int, theme dashboardThemeSpec, lines []string) string {
	headingStyle := lipgloss.NewStyle().Foreground(theme.highlight).Bold(true)
	lineStyle := lipgloss.NewStyle().Foreground(theme.text)
	accent := lipgloss.NewStyle().Foreground(theme.metric).Render(theme.accent)
	rendered := []string{headingStyle.Render("Inventory")}
	for _, line := range lines {
		rendered = append(rendered, accent+" "+lineStyle.Render(truncateText(line, width-3)))
	}
	return strings.Join(rendered, "\n")
}

func renderDashboardCommands(width int, theme dashboardThemeSpec) string {
	commandStyle := lipgloss.NewStyle().Foreground(theme.muted)
	keyStyle := lipgloss.NewStyle().Foreground(theme.highlight).Bold(true)
	commands := []string{
		keyStyle.Render("arrows") + commandStyle.Render(" select"),
		keyStyle.Render("Enter") + commandStyle.Render(" open"),
		keyStyle.Render("g") + commandStyle.Render(" find"),
		keyStyle.Render(":index-all") + commandStyle.Render(" index all"),
		keyStyle.Render(":summary") + commandStyle.Render(" refresh summary"),
		keyStyle.Render(":index") + commandStyle.Render(" refresh VMs"),
		keyStyle.Render(":index-db") + commandStyle.Render(" refresh DBs"),
		keyStyle.Render(":index-storage") + commandStyle.Render(" refresh storage"),
		keyStyle.Render(":login") + commandStyle.Render(" provider login"),
		keyStyle.Render(",") + commandStyle.Render(" settings"),
		keyStyle.Render("K") + commandStyle.Render(" k8s nodes"),
	}
	var rows []string
	var row []string
	rowWidth := 0
	sep := commandStyle.Render("  •  ")
	for _, command := range commands {
		part := command
		if len(row) > 0 {
			part = sep + command
		}
		partWidth := lipgloss.Width(part)
		if len(row) > 0 && rowWidth+partWidth > width {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, row...))
			row = nil
			rowWidth = 0
			part = command
			partWidth = lipgloss.Width(part)
		}
		row = append(row, part)
		rowWidth += partWidth
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, row...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func buildFindScopeItems() []list.Item {
	return []list.Item{
		findScopeItem{scope: findScopeVMs, title: "VMs", description: "Find across indexed virtual machines"},
		findScopeItem{scope: findScopeDisks, title: "Disks", description: "Find across indexed disks"},
		findScopeItem{scope: findScopeSnapshots, title: "Snapshots", description: "Find across indexed snapshots"},
		findScopeItem{scope: findScopeDatabases, title: "Databases", description: "Find across indexed databases"},
		findScopeItem{scope: findScopeK8s, title: "Kubernetes", description: "Find clusters from the local cluster index"},
		findScopeItem{scope: findScopeNetworks, title: "Networks", description: "Find indexed VPCs and VNets"},
		findScopeItem{scope: findScopeSubnets, title: "Subnets", description: "Find indexed subnets"},
		findScopeItem{scope: findScopeFirewalls, title: "Security groups", description: "Find indexed firewalls and security groups"},
		findScopeItem{scope: findScopeStorage, title: "Storage", description: "Find indexed object storage buckets and accounts"},
		findScopeItem{scope: findScopeHosts, title: "Manual hosts", description: "Find manually managed SSH/RDP hosts"},
		findScopeItem{scope: findScopeAll, title: "All indexed resources", description: "Broad fallback across indexed resources"},
	}
}

func findScopeTitle(scope findScope) string {
	switch scope {
	case findScopeDisks:
		return "Disks"
	case findScopeSnapshots:
		return "Snapshots"
	case findScopeDatabases:
		return "Databases"
	case findScopeK8s:
		return "Kubernetes"
	case findScopeNetworks:
		return "Networks"
	case findScopeSubnets:
		return "Subnets"
	case findScopeFirewalls:
		return "Security Groups"
	case findScopeStorage:
		return "Storage"
	case findScopeHosts:
		return "Manual Hosts"
	case findScopeAll:
		return "All Resources"
	default:
		return "VMs"
	}
}

func (a App) databaseDashboardStats() databaseDashboardStats {
	stats := databaseDashboardStats{}
	contexts := make(map[string]struct{})
	for _, rec := range a.databaseIndex {
		stats.Total++
		switch serviceStateClass(rec.Database.Status) {
		case "running":
			stats.Running++
		case "stopped":
			stats.Stopped++
		default:
			stats.Other++
		}
		if key := rec.Context.CacheKey(); key != "" {
			contexts[key] = struct{}{}
		}
	}
	stats.IndexedContexts = len(contexts)
	return stats
}

func (a App) kubernetesDashboardStats() kubernetesDashboardStats {
	clusters := make(map[string]struct{})
	pools := make(map[string]struct{})
	nodes := 0
	for _, rec := range a.clusterIndex {
		name := strings.TrimSpace(rec.Cluster.Name)
		if name == "" {
			name = rec.Cluster.ID
		}
		if name != "" {
			clusters[rec.Context.CacheKey()+"|"+strings.ToLower(name)] = struct{}{}
		}
	}
	for _, rec := range a.vmIndex {
		if !rec.VM.IsKubernetesNode() {
			continue
		}
		nodes++
		clusterName := rec.VM.KubernetesClusterName()
		if clusterName != "" {
			clusters[rec.Context.CacheKey()+"|"+strings.ToLower(clusterName)] = struct{}{}
		}
		poolName := rec.VM.KubernetesNodePoolName()
		if poolName != "" {
			pools[rec.Context.CacheKey()+"|"+strings.ToLower(clusterName)+"|"+strings.ToLower(poolName)] = struct{}{}
		}
	}
	return kubernetesDashboardStats{Clusters: len(clusters), Pools: len(pools), Nodes: nodes}
}

func (a App) visibleVMIndexRecords() []vmSearchRecord {
	records := make([]vmSearchRecord, 0, len(a.vmIndex))
	for _, rec := range a.vmIndex {
		if !a.showKubernetesNodes && rec.VM.IsKubernetesNode() {
			continue
		}
		records = append(records, rec)
	}
	return records
}

func serviceStateClass(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	for _, token := range []string{"available", "running", "ready", "online", "active", "succeeded"} {
		if strings.Contains(normalized, token) {
			return "running"
		}
	}
	for _, token := range []string{"stopped", "terminated", "deleted", "suspended", "failed", "unavailable"} {
		if strings.Contains(normalized, token) {
			return "stopped"
		}
	}
	return "other"
}

func formatDatabaseDashboardLabel(stats databaseDashboardStats) string {
	label := fmt.Sprintf("%d ready %d down", stats.Running, stats.Stopped)
	if stats.Other > 0 {
		label = fmt.Sprintf("%s +%d", label, stats.Other)
	}
	return label
}

func vmStateClass(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	if strings.Contains(normalized, "running") {
		return "running"
	}
	for _, token := range []string{"stopped", "terminated", "deallocated", "halted", "shut", "off"} {
		if strings.Contains(normalized, token) {
			return "stopped"
		}
	}
	return "other"
}

func hasUsablePublicIP(ip string) bool {
	ip = strings.TrimSpace(strings.ToLower(ip))
	return ip != "" && ip != "-" && ip != "none" && ip != "n/a" && ip != "<nil>"
}

func formatProviderCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", key, counts[key]))
	}
	return strings.Join(parts, " | ")
}

func ratioBar(part, total, width int) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	filled := part * width / total
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func countContextLeaves(nodes []*TreeNode) int {
	count := 0
	for _, node := range nodes {
		if node.IsLeaf {
			count++
		}
		count += countContextLeaves(node.Children)
	}
	return count
}

func (a App) renderCredentialsView() string {
	body := a.credentialList.View()
	if a.editCredential {
		body = a.renderCredentialForm()
	}
	mainView := a.renderShellPane(body, true)
	keys := "a:Add e:Edit u:Use l:Login z:Azure d:Remove r:Discover b:Backup Esc:Back"
	if a.confirmCredentialDelete {
		keys = "y:Confirm remove n/Esc:Cancel"
	}
	footer := renderFooter(a.width, fmt.Sprintf("%s | %s", keys, a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderFindScopeView() string {
	mainView := a.renderShellPane(a.findScopeList.View(), true)
	footer := renderFooter(a.width, fmt.Sprintf("Find Resources • Enter: Choose scope • Esc: Close | %s", a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderAzureSubscriptionView() string {
	mainView := a.renderShellPane(a.azureSubList.View(), true)
	footer := renderFooter(a.width, fmt.Sprintf("Space:Select • a:All • Enter:Import selected • /:Filter • Esc:Back | %s", a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderContextDiscoveryView() string {
	mainView := a.renderShellPane(a.discoveryList.View(), true)
	footer := renderFooter(a.width, fmt.Sprintf("Space:Select • a:All • Enter:Import selected • /:Filter • Esc:Back | %s", a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderProviderLoginView() string {
	mainView := a.renderShellPane(a.loginList.View(), true)
	footer := renderFooter(a.width, fmt.Sprintf("Enter: Run login • Esc: Close • :discover after manual auth | %s", a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderHostFormView() string {
	labels := []string{"Name", "Host/IP", "Username", "Connection", "SSH Config Host", "Key Path", "Tags"}
	lines := []string{TitleStyle.Render("Manual Host")}
	for i, input := range a.hostInputs {
		label := labels[i]
		if i == a.hostInputFocus {
			label = lipgloss.NewStyle().Foreground(Highlight).Bold(true).Render(label)
		}
		lines = append(lines, fmt.Sprintf("%-16s %s", label, input.View()))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(Subtle).Render("Enter/F2: Save • Tab: Next • Esc: Cancel"))
	body := lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
	mainView := a.renderShellPane(body, true)
	footer := renderFooter(a.width, a.statusMsg)
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) renderCredentialForm() string {
	labels := []string{"Context Name", "Provider", "Account ID", "Account Name", "Tenant", "Auth Mode", "Persistence", "Credential Ref", "Regions"}
	lines := []string{TitleStyle.Render("Managed Context")}
	for i, input := range a.credentialInputs {
		label := labels[i]
		if i == a.credentialInputFocus {
			label = lipgloss.NewStyle().Foreground(Special).Bold(true).Render(label)
		}
		lines = append(lines, fmt.Sprintf("%s\n%s", label, input.View()))
	}
	lines = append(lines, StatusLineStyle.Render("Tab/Shift+Tab: Field • Enter: Save • Esc: Cancel"))
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n\n"))
}

func (a App) renderGlobalSearchView() string {
	title := fmt.Sprintf("Find %s", findScopeTitle(a.findScope))
	body := lipgloss.JoinVertical(lipgloss.Left,
		TitleStyle.Render(title),
		lipgloss.NewStyle().Padding(0, 1).Render(a.globalSearchInput.View()),
		ColorizeOperationalStates(a.globalSearchTable.View()),
	)
	meta := a.findMeta()
	if a.findScope == findScopeVMs && !a.showKubernetesNodes {
		visibleIndexed := len(a.visibleVMIndexRecords())
		meta = fmt.Sprintf("%s | K8s nodes hidden: %d", meta, len(a.vmIndex)-visibleIndexed)
	}
	if a.vmPrefetchTotal > 0 && a.vmPrefetchDone < a.vmPrefetchTotal {
		meta = fmt.Sprintf("%s | indexing %d/%d contexts", meta, a.vmPrefetchDone, a.vmPrefetchTotal)
	}
	mainView := a.renderShellPane(body, true)
	footer := renderFooter(a.width, fmt.Sprintf("\u2191\u2193: Navigate \u2022 ↑ at top: Columns \u2022 /:Filter \u2022 Enter: Sort/Open \u2022 K:K8s nodes \u2022 Esc: Close | %s | %s", meta, a.statusMsg))
	return fitToWindow(lipgloss.JoinVertical(lipgloss.Left, mainView, footer), a.width, a.height)
}

func (a App) findMeta() string {
	switch a.findScope {
	case findScopeDisks:
		return fmt.Sprintf("%d matches from %d indexed disks", len(a.findRows), len(a.diskIndex))
	case findScopeSnapshots:
		return fmt.Sprintf("%d matches from %d indexed snapshots", len(a.findRows), len(a.snapshotIndex))
	case findScopeDatabases:
		return fmt.Sprintf("%d matches from %d indexed DBs", len(a.findRows), len(a.databaseIndex))
	case findScopeK8s:
		return fmt.Sprintf("%d matches from %d indexed clusters", len(a.findRows), len(a.clusterIndex))
	case findScopeNetworks:
		return fmt.Sprintf("%d matches from %d indexed networks", len(a.findRows), len(a.networkIndex))
	case findScopeSubnets:
		return fmt.Sprintf("%d matches from %d indexed subnets", len(a.findRows), len(a.subnetIndex))
	case findScopeFirewalls:
		return fmt.Sprintf("%d matches from %d indexed security groups", len(a.findRows), len(a.firewallIndex))
	case findScopeStorage:
		return fmt.Sprintf("%d matches from %d indexed storage resources", len(a.findRows), len(a.storageIndex))
	case findScopeHosts:
		total := len(filterVMRecordsByProvider(a.visibleVMIndexRecords(), "Manual"))
		return fmt.Sprintf("%d matches from %d manual hosts", len(a.findRows), total)
	case findScopeAll:
		total := len(a.visibleVMIndexRecords()) + a.resourceIndexSize()
		return fmt.Sprintf("%d matches from %d indexed resources", len(a.findRows), total)
	default:
		return fmt.Sprintf("%d matches from %d indexed VMs", len(a.findRows), len(a.visibleVMIndexRecords()))
	}
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

func (a *App) resizeHelpView() {
	width := a.fullScreenContentWidth() - 2
	height := a.fullScreenContentHeight() - 4
	if width < 20 {
		width = 20
	}
	if height < 5 {
		height = 5
	}
	a.helpView.Width = width
	a.helpView.Height = height
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
		style = style.BorderForeground(Subtle)
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

func (a App) handleCommand(query string) (App, tea.Cmd) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return a, nil
	}

	logging.Infof("component=ui event=execute_command query=%s", query)

	// Command syntax parsing
	parts := strings.Fields(query)
	cmd := parts[0]

	switch cmd {
	case "home", "dashboard", "dash":
		a.openHome()
		return a, nil
	case "help", "shortcuts", "keys", "?":
		a.openHelp()
		return a, nil
	case "summary", "refresh-dashboard", "dashboard-refresh":
		cmds := a.startDashboardSummaryRefresh()
		if len(cmds) == 0 {
			a.statusMsg = "No database or cluster capable contexts available."
			return a, nil
		}
		a.statusMsg = "Refreshing dashboard summary..."
		return a, tea.Batch(cmds...)
	case "index-all", "refresh-all", "reindex-all":
		cmds := a.startAllIndexRefresh()
		if len(cmds) == 0 {
			a.statusMsg = "No supported contexts available to index."
			return a, nil
		}
		a.statusMsg = "Indexing all supported resources..."
		return a, tea.Batch(cmds...)
	case "k8s-nodes", "kubernetes-nodes":
		a.toggleKubernetesNodes()
		return a, nil
	case "vms", "ec2", "instances":
		return a.switchTabByCapability(providers.CapabilityVMs)
	case "disks", "volumes", "ebs":
		return a.switchTabByCapability(providers.CapabilityDisks)
	case "snaps", "snapshots":
		return a.switchTabByCapability(providers.CapabilitySnapshots)
	case "fw", "firewalls", "sg":
		return a.switchTabByCapability(providers.CapabilityFirewalls)
	case "nets", "networks", "vpc":
		return a.switchTabByCapability(providers.CapabilityNetworks)
	case "clusters", "eks", "gke", "aks":
		return a.switchTabByCapability(providers.CapabilityClusters)
	case "db", "dbs", "databases", "rds", "sql":
		return a.switchTabByCapability(providers.CapabilityDatabases)
	case "storage", "buckets", "bucket", "s3", "blob":
		return a.switchTabByCapability(providers.CapabilityStorage)
	case "hosts", "host", "manual-hosts":
		return a.switchTabByCapability(providers.CapabilityHosts)
	case "find", "search":
		if !a.cfg.GlobalSearch {
			a.statusMsg = "Find is disabled."
			return a, nil
		}
		query := ""
		if len(parts) > 1 {
			query = strings.Join(parts[1:], " ")
		}
		return a.openFind(findScopeVMs, query)
	case "find-vms", "find-vm", "find-hosts", "find-host":
		query := strings.Join(parts[1:], " ")
		scope := findScopeVMs
		if cmd == "find-hosts" || cmd == "find-host" {
			scope = findScopeHosts
		}
		return a.openFind(scope, query)
	case "find-disks", "find-disk":
		return a.openFind(findScopeDisks, strings.Join(parts[1:], " "))
	case "find-snaps", "find-snap", "find-snapshots", "find-snapshot":
		return a.openFind(findScopeSnapshots, strings.Join(parts[1:], " "))
	case "find-dbs", "find-db", "find-databases", "find-database":
		return a.openFind(findScopeDatabases, strings.Join(parts[1:], " "))
	case "find-k8s", "find-clusters", "find-cluster", "find-kubernetes":
		return a.openFind(findScopeK8s, strings.Join(parts[1:], " "))
	case "find-networks", "find-network", "find-vpcs", "find-vpc", "find-vnets", "find-vnet":
		return a.openFind(findScopeNetworks, strings.Join(parts[1:], " "))
	case "find-subnets", "find-subnet":
		return a.openFind(findScopeSubnets, strings.Join(parts[1:], " "))
	case "find-firewalls", "find-firewall", "find-sgs", "find-sg", "find-security-groups", "find-security-group":
		return a.openFind(findScopeFirewalls, strings.Join(parts[1:], " "))
	case "find-storage", "find-buckets", "find-bucket":
		return a.openFind(findScopeStorage, strings.Join(parts[1:], " "))
	case "find-all", "find-resources":
		return a.openFind(findScopeAll, strings.Join(parts[1:], " "))
	case "settings", "set", "prefs", "preferences":
		a.openSettings()
		return a, nil
	case "logs", "log", "tail-log", "tail-logs":
		a.showLogs = true
		a.resizeLogView()
		a.statusMsg = fmt.Sprintf("Viewing application logs: %s", a.logPath)
		logging.Infof("component=ui event=logs_open path=%s source=command", a.logPath)
		return a, loadLogsCmd()
	case "login", "auth", "provider-login", "cli-login":
		if a.showCredentials {
			return a.loginSelectedCredential()
		}
		a.openProviderLogin()
		return a, nil
	case "profiles", "profile", "contexts", "creds", "credentials":
		a.openCredentials()
		return a, nil
	case "azure-profiles", "azure-contexts", "azure-subscriptions", "import-azure":
		a.statusMsg = "Loading Azure subscriptions..."
		return a, fetchAzureSubscriptionsCmd()
	case "add-provider", "provider", "providers", "add-profile", "add-context":
		a.openCredentials()
		a.startCredentialEdit(-1)
		a.statusMsg = "Add profile/context: choose auth mode and persistence explicitly."
		return a, textinput.Blink
	case "add-host", "host-add", "manual-host":
		a.openHostForm()
		return a, textinput.Blink
	case "discover", "discover-contexts", "scan-contexts":
		a.statusMsg = "Discovering cloud contexts..."
		return a, fetchDiscoveredContextsCmd()
	case "refresh-index", "reindex", "index":
		cmds := a.startVMIndexRefresh()
		if len(cmds) == 0 {
			a.statusMsg = "No VM-capable contexts available to index."
			return a, nil
		}
		a.statusMsg = "Refreshing VM index..."
		return a, tea.Batch(cmds...)
	case "index-db", "index-dbs", "index-databases":
		cmds := a.startDatabaseIndexRefresh()
		if len(cmds) == 0 {
			a.statusMsg = "No database-capable contexts available to index."
			return a, nil
		}
		a.statusMsg = "Refreshing database index..."
		return a, tea.Batch(cmds...)
	case "index-storage", "refresh-storage", "index-buckets", "refresh-buckets":
		cmds := a.startStorageIndexRefresh()
		if len(cmds) == 0 {
			a.statusMsg = "No storage-capable contexts available to index."
			return a, nil
		}
		a.statusMsg = "Refreshing storage index..."
		return a, tea.Batch(cmds...)
	case "export-public-endpoints", "export-endpoints", "export-ips", "export-domains":
		path := ""
		if len(parts) > 1 {
			path = strings.Join(parts[1:], " ")
		}
		count, writtenPath, err := exportPublicEndpointsCSV(a, path)
		if err != nil {
			a.statusMsg = fmt.Sprintf("Endpoint export failed: %v", err)
			return a, nil
		}
		a.statusMsg = fmt.Sprintf("Exported %d public endpoints to %s", count, writtenPath)
		return a, nil
	case "ctx", "context", "project", "account":
		if len(parts) > 1 {
			target := strings.Join(parts[1:], " ")
			return a.switchContext(target)
		}
		a.statusMsg = "Usage: :ctx <name or id>"
		return a, nil
	case "q", "quit", "exit":
		return a, tea.Quit
	default:
		a.statusMsg = fmt.Sprintf("Unknown command: %s", cmd)
		return a, nil
	}
}

func (a App) switchTabByCapability(cap providers.Capability) (App, tea.Cmd) {
	for tabIndex, view := range a.resourceViews {
		if view.Capability == cap {
			if a.activeCtx.Provider != "" && !providers.Supports(a.activeCtx.Provider, view.Capability) {
				a.statusMsg = fmt.Sprintf("%s not supported for %s.", view.View.Title(), a.activeCtx.Provider)
				return a, nil
			}
			a.activeTab = tabIndex
			a.viewStack = []View{view.View}
			a.applyKubernetesNodeVisibility()
			a.focus = focusMain
			if a.activeCtx.AccountID != "" || a.activeCtx.AccountName != "" {
				initCmd := view.View.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
				a.statusMsg = fmt.Sprintf("Switched to %s.", view.View.Title())
				return a, initCmd
			}
			a.statusMsg = fmt.Sprintf("Switched to %s. Select a context.", view.View.Title())
			return a, nil
		}
	}
	return a, nil
}

func (a App) switchContext(query string) (App, tea.Cmd) {
	for i, item := range a.contexts.Items() {
		node, ok := item.(*TreeNode)
		if !ok || !node.IsLeaf {
			continue
		}
		if strings.Contains(strings.ToLower(node.Context.DisplayName()), query) ||
			strings.Contains(strings.ToLower(node.Context.AccountID), query) ||
			strings.Contains(strings.ToLower(node.Context.Region), query) {
			a.contexts.Select(i)
			a.activeCtx = node.Context
			a.ensureActiveTab()
			a.focus = focusMain
			logging.Infof("component=ui event=context_select provider=%s account=%s region=%s", a.activeCtx.Provider, a.activeCtx.AccountID, a.activeCtx.Region)
			if len(a.viewStack) > 0 {
				topView := a.viewStack[len(a.viewStack)-1]
				initCmd := topView.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
				a.statusMsg = fmt.Sprintf("Fetching resources for %s...", a.activeCtx.DisplayName())
				return a, initCmd
			}
			return a, nil
		}
	}
	a.statusMsg = fmt.Sprintf("Context not found: %s", query)
	return a, nil
}

func (a App) activateFilteredContext() (App, tea.Cmd) {
	query := a.contexts.FilterValue()
	if ctx, ok := findContextMatch(a.rootNodes, query); ok {
		a.contexts.ResetFilter()
		return a.activateContext(ctx)
	}
	return a.activateSelectedContext()
}

func (a App) activateSelectedContext() (App, tea.Cmd) {
	selected := a.contexts.SelectedItem()
	if selected == nil {
		return a, nil
	}
	node, ok := selected.(*TreeNode)
	if !ok {
		return a, nil
	}
	if node.IsLeaf {
		return a.activateContext(node.Context)
	}
	if ctx, ok := firstLeafContext(node); ok && a.contexts.FilterState() != list.Unfiltered {
		a.contexts.ResetFilter()
		return a.activateContext(ctx)
	}
	node.Expanded = !node.Expanded
	a.contexts.SetItems(BuildFlatList(a.rootNodes))
	return a, nil
}

func (a App) activateContext(ctx core.CloudContext) (App, tea.Cmd) {
	a.activeCtx = ctx
	a.ensureActiveTab()
	a.focus = focusMain
	logging.Infof("component=ui event=context_select provider=%s account=%s region=%s", a.activeCtx.Provider, a.activeCtx.AccountID, a.activeCtx.Region)
	if len(a.viewStack) > 0 {
		topView := a.viewStack[len(a.viewStack)-1]
		initCmd := topView.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
		a.statusMsg = fmt.Sprintf("Fetching instances for %s...", a.activeCtx.DisplayName())
		return a, initCmd
	}
	return a, nil
}

func findContextMatch(nodes []*TreeNode, query string) (core.CloudContext, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return core.CloudContext{}, false
	}
	for _, node := range nodes {
		if node.IsLeaf && strings.Contains(strings.ToLower(node.FilterValue()), query) {
			return node.Context, true
		}
		if ctx, ok := findContextMatch(node.Children, query); ok {
			return ctx, true
		}
	}
	return core.CloudContext{}, false
}

func firstLeafContext(node *TreeNode) (core.CloudContext, bool) {
	if node == nil {
		return core.CloudContext{}, false
	}
	if node.IsLeaf {
		return node.Context, true
	}
	for _, child := range node.Children {
		if ctx, ok := firstLeafContext(child); ok {
			return ctx, true
		}
	}
	return core.CloudContext{}, false
}

func (a App) openGlobalSearch(query string) (App, tea.Cmd) {
	return a.openFind(findScopeVMs, query)
}

func (a App) openFind(scope findScope, query string) (App, tea.Cmd) {
	if !a.cfg.GlobalSearch {
		a.statusMsg = "Find is disabled."
		return a, nil
	}
	a.showGlobalSearch = true
	a.showFindSort = false
	a.showFindPicker = false
	a.showContextDiscovery = false
	a.findScope = scope
	if a.findScope == "" {
		a.findScope = findScopeVMs
	}
	a.globalSearchInput.Placeholder = fmt.Sprintf("find %s", strings.ToLower(findScopeTitle(a.findScope)))
	a.globalSearchInput.SetValue(query)
	a.globalSearchInput.Blur()
	a.globalSearchTable.Focus()
	a.refreshGlobalSearchResults()
	a.resizeGlobalSearch()
	a.statusMsg = fmt.Sprintf("Find %s uses local indexed/cache data.", strings.ToLower(findScopeTitle(a.findScope)))
	return a, nil
}

func (a *App) openHome() {
	a.activeCtx = core.CloudContext{}
	a.viewStack = nil
	a.focus = focusMain
	a.showGlobalSearch = false
	a.showSettings = false
	a.showCredentials = false
	a.showProviderLogin = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.showHostForm = false
	a.showLogs = false
	a.showHelp = false
	a.showConfig = false
	a.statusMsg = "Dashboard."
}

func (a *App) toggleKubernetesNodes() {
	a.showKubernetesNodes = !a.showKubernetesNodes
	a.cfg.HideKubernetesNodes = !a.showKubernetesNodes
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Kubernetes node filter save failed: %v", err)
		return
	}
	a.applyKubernetesNodeVisibility()
	a.refreshGlobalSearchResults()
	if a.showKubernetesNodes {
		a.statusMsg = "Showing Kubernetes worker nodes."
	} else {
		a.statusMsg = "Hiding Kubernetes worker nodes."
	}
}

func (a *App) applyKubernetesNodeVisibility() {
	for i, view := range a.viewStack {
		if configurable, ok := view.(interface{ SetKubernetesNodesVisible(bool) }); ok {
			configurable.SetKubernetesNodesVisible(a.showKubernetesNodes)
			a.viewStack[i] = view
		}
	}
	for key, registered := range a.resourceViews {
		if configurable, ok := registered.View.(interface{ SetKubernetesNodesVisible(bool) }); ok {
			configurable.SetKubernetesNodesVisible(a.showKubernetesNodes)
			a.resourceViews[key] = registered
		}
	}
}

func (a *App) openSettings() {
	a.showSettings = true
	a.showHelp = false
	a.showProviderLogin = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.showHostForm = false
	a.settingsList.SetItems(buildSettingsItems(a.cfg))
	a.settingsList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
	a.statusMsg = "Settings opened."
}

func (a *App) openFindScopePicker() {
	a.showFindPicker = true
	a.showGlobalSearch = false
	a.showSettings = false
	a.showHelp = false
	a.showCredentials = false
	a.showProviderLogin = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.showHostForm = false
	a.findScopeList.SetItems(buildFindScopeItems())
	a.findScopeList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
	a.findSortList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
	a.statusMsg = "Choose what to find."
}

func (a *App) openCredentials() {
	a.showCredentials = true
	a.showHelp = false
	a.showProviderLogin = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.showHostForm = false
	a.editCredential = false
	a.confirmCredentialDelete = false
	a.credentialDeleteIndex = -1
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	a.credentialList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
	a.statusMsg = "Profiles / contexts opened. Select one, press u to use or l to login."
}

func (a *App) openHostForm() {
	a.showHostForm = true
	a.showSettings = false
	a.showHelp = false
	a.showCredentials = false
	a.showProviderLogin = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.hostInputFocus = 0
	values := []string{"", "", "root", "ssh", "", "", ""}
	placeholders := []string{
		"name",
		"ip or dns",
		"username",
		"ssh",
		"ssh config alias",
		"private key path",
		"tags comma-separated",
	}
	a.hostInputs = make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.SetValue(value)
		input.Placeholder = placeholders[i]
		input.Width = 48
		if i == 0 {
			input.Focus()
		}
		a.hostInputs[i] = input
	}
	a.statusMsg = "Add manual host. Config stores metadata only; keep secrets in ssh-agent/keychain."
}

func (a *App) openProviderLogin() {
	a.showProviderLogin = true
	a.showSettings = false
	a.showHelp = false
	a.showCredentials = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.loginList.SetItems(buildProviderLoginItems())
	a.loginList.SetSize(a.fullScreenContentWidth(), a.fullScreenContentHeight())
	a.statusMsg = "Choose provider login. The native CLI will take over until it exits."
}

func (a *App) openHelp() {
	a.showHelp = true
	a.showConfig = false
	a.showSettings = false
	a.showFindPicker = false
	a.showAzureSubscriptions = false
	a.showContextDiscovery = false
	a.showCredentials = false
	a.showHostForm = false
	a.showProviderLogin = false
	a.showLogs = false
	a.showGlobalSearch = false
	a.showCmdBar = false
	a.cmdBar.Blur()
	a.helpSearchInput.SetValue("")
	a.helpSearchInput.Blur()
	a.resizeHelpView()
	a.refreshHelpContent()
	a.helpView.GotoTop()
	a.statusMsg = "Help opened."
}

func (a App) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showSettings = false
		a.statusMsg = "Settings closed."
		return a, nil
	case " ", "enter":
		selected := a.settingsList.SelectedItem()
		if selected == nil {
			return a, nil
		}
		item := selected.(settingsItem)
		var cmds []tea.Cmd
		switch item.key {
		case "backend":
			if strings.EqualFold(a.cfg.Backend, "sdk") {
				a.cfg.Backend = "cli"
			} else {
				a.cfg.Backend = "sdk"
			}
			a.statusMsg = fmt.Sprintf("Backend set to %s.", strings.ToUpper(a.cfg.Backend))
		case "global_search":
			a.cfg.GlobalSearch = !a.cfg.GlobalSearch
			if !a.cfg.GlobalSearch {
				a.showGlobalSearch = false
			}
			a.statusMsg = settingStatus("Find resources", a.cfg.GlobalSearch)
		case "discover_on_start":
			a.cfg.DiscoverOnStart = !a.cfg.DiscoverOnStart
			a.statusMsg = settingStatus("Discover contexts on startup", a.cfg.DiscoverOnStart)
		case "vm_indexing":
			a.cfg.PrefetchOnStart = !a.cfg.PrefetchOnStart
			a.cfg.PrefetchResources = ensureResourceSetting(a.cfg.PrefetchResources, "vms", a.cfg.PrefetchOnStart)
			a.statusMsg = settingStatus("VM startup indexing", a.cfg.PrefetchOnStart)
			if a.cfg.PrefetchOnStart {
				cmds = append(cmds, a.startVMPrefetch()...)
			}
		case "db_indexing":
			enabled := !config.PrefetchesResource(a.cfg, "databases")
			a.cfg.PrefetchResources = ensureResourceSetting(a.cfg.PrefetchResources, "databases", enabled)
			a.statusMsg = settingStatus("Database indexing", enabled)
			if enabled {
				cmds = append(cmds, a.startDatabaseIndexRefresh()...)
			}
		case "storage_indexing":
			enabled := !config.PrefetchesResource(a.cfg, "storage")
			a.cfg.PrefetchResources = ensureResourceSetting(a.cfg.PrefetchResources, "storage", enabled)
			a.statusMsg = settingStatus("Storage indexing", enabled)
			if enabled {
				cmds = append(cmds, a.startStorageIndexRefresh()...)
			}
		case "hide_kubernetes_nodes":
			a.cfg.HideKubernetesNodes = !a.cfg.HideKubernetesNodes
			a.showKubernetesNodes = !a.cfg.HideKubernetesNodes
			a.applyKubernetesNodeVisibility()
			a.refreshGlobalSearchResults()
			a.statusMsg = settingStatus("Hide Kubernetes nodes", a.cfg.HideKubernetesNodes)
		case "vm_index_persistence":
			a.cfg.VMIndexPersistence = !a.cfg.VMIndexPersistence
			a.statusMsg = settingStatus("Persist VM index", a.cfg.VMIndexPersistence)
			if a.cfg.VMIndexPersistence {
				a.persistVMIndex()
			}
		case "vm_index_ttl":
			a.cfg.VMIndexCacheTTL = cycleInt(a.cfg.VMIndexCacheTTL, []int{1, 6, 12, 24, 72, 168})
			a.statusMsg = fmt.Sprintf("VM index cache TTL set to %d hours.", a.cfg.VMIndexCacheTTL)
		case "resource_index_persistence":
			a.cfg.ResourceIndexPersistence = !a.cfg.ResourceIndexPersistence
			a.statusMsg = settingStatus("Persist resource index", a.cfg.ResourceIndexPersistence)
			if a.cfg.ResourceIndexPersistence {
				a.persistResourceIndex()
			}
		case "resource_index_ttl":
			a.cfg.ResourceIndexCacheTTL = cycleInt(a.cfg.ResourceIndexCacheTTL, []int{1, 6, 12, 24, 72, 168})
			a.statusMsg = fmt.Sprintf("Resource index cache TTL set to %d hours.", a.cfg.ResourceIndexCacheTTL)
		case "dashboard_theme":
			a.cfg.DashboardTheme = cycleString(config.SanitizeDashboardTheme(a.cfg.DashboardTheme), []string{"btop", "neon", "classic", "mono"})
			a.statusMsg = fmt.Sprintf("Dashboard theme set to %s.", a.cfg.DashboardTheme)
		case "refresh_vm_index":
			cmds = append(cmds, a.startVMIndexRefresh()...)
			a.statusMsg = "Refreshing VM index..."
		case "refresh_all_index":
			cmds = append(cmds, a.startAllIndexRefresh()...)
			a.statusMsg = "Indexing all supported resources..."
		case "provider_login":
			a.openProviderLogin()
			return a, nil
		case "azure_subscriptions":
			a.showSettings = false
			a.statusMsg = "Loading Azure subscriptions..."
			return a, fetchAzureSubscriptionsCmd()
		case "add_provider":
			a.showSettings = false
			a.openCredentials()
			a.startCredentialEdit(-1)
			a.statusMsg = "Add provider: choose auth mode and persistence explicitly."
			return a, textinput.Blink
		case "add_host":
			a.openHostForm()
			return a, textinput.Blink
		case "credentials":
			a.showSettings = false
			a.openCredentials()
			return a, nil
		case "backup":
			path, err := config.BackupConfig()
			if err != nil {
				a.statusMsg = fmt.Sprintf("Backup failed: %v", err)
			} else if path == "" {
				a.statusMsg = "No config file exists yet."
			} else {
				a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
			}
			return a, nil
		case "billing":
			a.cfg.BillingEnabled = !a.cfg.BillingEnabled
			a.statusMsg = settingStatus("Billing", a.cfg.BillingEnabled)
		case "metrics":
			a.cfg.MetricsEnabled = !a.cfg.MetricsEnabled
			a.statusMsg = settingStatus("Metrics", a.cfg.MetricsEnabled)
		case "prefetch_concurrency":
			a.cfg.PrefetchConcurrency = cycleInt(a.cfg.PrefetchConcurrency, []int{1, 2, 4, 8, 16})
			a.statusMsg = fmt.Sprintf("Prefetch concurrency set to %d.", a.cfg.PrefetchConcurrency)
		case "cache_ttl":
			a.cfg.CacheTTL = cycleInt(a.cfg.CacheTTL, []int{1, 5, 15, 30, 60})
			a.statusMsg = fmt.Sprintf("Resource cache TTL set to %d minutes.", a.cfg.CacheTTL)
		case "billing_ttl":
			a.cfg.BillingCacheTTL = cycleInt(a.cfg.BillingCacheTTL, []int{5, 15, 30, 60, 120})
			a.statusMsg = fmt.Sprintf("Billing cache TTL set to %d minutes.", a.cfg.BillingCacheTTL)
		case "metrics_period":
			a.cfg.MetricsPeriodHours = cycleInt(a.cfg.MetricsPeriodHours, []int{1, 6, 12, 24, 72})
			a.statusMsg = fmt.Sprintf("Metrics period set to %d hours.", a.cfg.MetricsPeriodHours)
		case "metrics_ttl":
			a.cfg.MetricsCacheTTL = cycleInt(a.cfg.MetricsCacheTTL, []int{5, 15, 30, 60})
			a.statusMsg = fmt.Sprintf("Metrics cache TTL set to %d minutes.", a.cfg.MetricsCacheTTL)
		}
		if err := config.Save(a.cfg); err != nil {
			a.statusMsg = fmt.Sprintf("Settings save failed: %v", err)
		}
		a.settingsList.SetItems(buildSettingsItems(a.cfg))
		return a, tea.Batch(cmds...)
	}
	var cmd tea.Cmd
	a.settingsList, cmd = a.settingsList.Update(msg)
	return a, cmd
}

func buildSettingsItems(cfg config.AppConfig) []list.Item {
	return []list.Item{
		settingsItem{
			key:         "backend",
			title:       "Backend mode",
			description: "Cycle provider execution mode.",
			value:       strings.ToUpper(orFallback(cfg.Backend, "cli")),
		},
		settingsItem{
			key:         "global_search",
			title:       "Find indexed resources",
			description: "Enable scoped find panels such as find-vms, find-dbs, and find-k8s.",
			toggle:      true,
			enabled:     cfg.GlobalSearch,
		},
		settingsItem{key: "discover_on_start", title: "Discover contexts on startup", description: "Scan cloud CLIs during app launch. Disable for fastest startup.", toggle: true, enabled: cfg.DiscoverOnStart},
		settingsItem{
			key:         "vm_indexing",
			title:       "Index VMs on startup",
			description: "Prefetch VM lists for all discovered contexts when CloudManager starts.",
			toggle:      true,
			enabled:     cfg.PrefetchOnStart && config.PrefetchesResource(cfg, "vms"),
		},
		settingsItem{key: "db_indexing", title: "Index databases", description: "Allow dashboard database summaries to refresh from provider data.", toggle: true, enabled: config.PrefetchesResource(cfg, "databases")},
		settingsItem{key: "storage_indexing", title: "Index storage", description: "Allow storage buckets/accounts to refresh into the local resource index.", toggle: true, enabled: config.PrefetchesResource(cfg, "storage")},
		settingsItem{key: "hide_kubernetes_nodes", title: "Hide Kubernetes nodes", description: "Hide managed cluster worker VMs from compute lists and dashboard by default.", toggle: true, enabled: cfg.HideKubernetesNodes},
		settingsItem{key: "vm_index_persistence", title: "Persist VM index", description: "Save the VM search index locally between app starts.", toggle: true, enabled: cfg.VMIndexPersistence},
		settingsItem{key: "vm_index_ttl", title: "VM index cache TTL", description: "Cycle how long persisted VM records remain searchable.", value: fmt.Sprintf("%d hr", cfg.VMIndexCacheTTL)},
		settingsItem{key: "resource_index_persistence", title: "Persist resource index", description: "Save index-all results locally between app starts.", toggle: true, enabled: cfg.ResourceIndexPersistence},
		settingsItem{key: "resource_index_ttl", title: "Resource index cache TTL", description: "Cycle how long indexed disks, DBs, clusters, networks, firewalls, and storage stay searchable.", value: fmt.Sprintf("%d hr", cfg.ResourceIndexCacheTTL)},
		settingsItem{key: "dashboard_theme", title: "Dashboard theme", description: "Cycle the home dashboard visual style.", value: config.SanitizeDashboardTheme(cfg.DashboardTheme)},
		settingsItem{key: "provider_login", title: "Provider CLI login", description: "Run native AWS, GCP, Azure, or DigitalOcean login flows."},
		settingsItem{key: "azure_subscriptions", title: "Import Azure subscriptions", description: "Pick visible az subscriptions and save them as CloudManager contexts."},
		settingsItem{key: "add_provider", title: "Add profile/context", description: "Create a provider context with auth mode and persistence policy."},
		settingsItem{key: "add_host", title: "Add manual host", description: "Add an SSH/RDP host without cloud provider API credentials."},
		settingsItem{key: "refresh_all_index", title: "Index all resources now", description: "Fetch VMs, DBs, clusters, disks, snapshots, networks, firewalls, and storage where supported."},
		settingsItem{key: "refresh_vm_index", title: "Refresh VM index now", description: "Fetch VMs across contexts and update the local index cache."},
		settingsItem{key: "prefetch_concurrency", title: "Prefetch concurrency", description: "Cycle startup indexing worker count.", value: fmt.Sprintf("%d", cfg.PrefetchConcurrency)},
		settingsItem{key: "cache_ttl", title: "Resource cache TTL", description: "Cycle resource cache lifetime.", value: fmt.Sprintf("%d min", cfg.CacheTTL)},
		settingsItem{key: "billing", title: "Billing features", description: "Enable cost-related actions and data.", toggle: true, enabled: cfg.BillingEnabled},
		settingsItem{key: "billing_ttl", title: "Billing cache TTL", description: "Cycle billing cache lifetime.", value: fmt.Sprintf("%d min", cfg.BillingCacheTTL)},
		settingsItem{key: "metrics", title: "Metrics features", description: "Enable metrics columns and fetches.", toggle: true, enabled: cfg.MetricsEnabled},
		settingsItem{key: "metrics_period", title: "Metrics period", description: "Cycle metric lookback period.", value: fmt.Sprintf("%d hr", cfg.MetricsPeriodHours)},
		settingsItem{key: "metrics_ttl", title: "Metrics cache TTL", description: "Cycle metrics cache lifetime.", value: fmt.Sprintf("%d min", cfg.MetricsCacheTTL)},
		settingsItem{key: "credentials", title: "Profiles / Contexts", description: "Add, use, login, edit, remove, discover, or back up contexts.", value: fmt.Sprintf("%d", len(cfg.CloudContexts))},
		settingsItem{key: "backup", title: "Back up config", description: "Copy the current CloudManager config before credential changes."},
	}
}

func cycleInt(current int, values []int) int {
	for _, value := range values {
		if current < value {
			return value
		}
	}
	if len(values) == 0 {
		return current
	}
	return values[0]
}

func cycleString(current string, values []string) string {
	if len(values) == 0 {
		return current
	}
	current = strings.ToLower(strings.TrimSpace(current))
	for i, value := range values {
		if current == value {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

func orFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func settingStatus(label string, enabled bool) string {
	if enabled {
		return label + " enabled."
	}
	return label + " disabled."
}

func ensureResourceSetting(resources []string, resource string, enabled bool) []string {
	seen := make(map[string]bool, len(resources)+1)
	var out []string
	for _, item := range resources {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" || normalized == resource {
			continue
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	if enabled && !seen[resource] {
		out = append(out, resource)
	}
	if len(out) == 0 {
		return []string{"vms"}
	}
	return out
}

func (a App) handleCredentialKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.editCredential {
		return a.handleCredentialFormKeys(msg)
	}
	if a.confirmCredentialDelete {
		return a.handleCredentialDeleteConfirmKeys(msg)
	}
	switch msg.String() {
	case "esc":
		a.showCredentials = false
		a.statusMsg = "Profiles / contexts closed."
		return a, nil
	case "a":
		a.startCredentialEdit(-1)
		return a, textinput.Blink
	case "l":
		return a.loginSelectedCredential()
	case "u":
		return a.useSelectedCredential()
	case "r":
		a.statusMsg = "Discovering provider contexts..."
		return a, fetchDiscoveredContextsCmd()
	case "z", "Z":
		a.statusMsg = "Loading Azure subscriptions..."
		return a, fetchAzureSubscriptionsCmd()
	case "e", "enter":
		item, ok := a.selectedCredentialItem()
		if !ok {
			a.statusMsg = "No profile/context selected."
			return a, nil
		}
		a.startCredentialEdit(item.index)
		return a, textinput.Blink
	case "d":
		item, ok := a.selectedCredentialItem()
		if !ok {
			a.statusMsg = "No profile/context selected."
			return a, nil
		}
		a.confirmCredentialDelete = true
		a.credentialDeleteIndex = item.index
		a.statusMsg = fmt.Sprintf("Remove profile/context %q? y to confirm, n to cancel.", item.ctx.ContextName)
		return a, nil
	case "p":
		a.openProviderLogin()
		return a, nil
	case "b":
		path, err := config.BackupConfig()
		if err != nil {
			a.statusMsg = fmt.Sprintf("Backup failed: %v", err)
		} else if path == "" {
			a.statusMsg = "No config file exists yet."
		} else {
			a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
		}
		return a, nil
	}
	var cmd tea.Cmd
	a.credentialList, cmd = a.credentialList.Update(msg)
	return a, cmd
}

func (a App) handleFindScopeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showFindPicker = false
		a.statusMsg = "Find canceled."
		return a, nil
	case "enter":
		selected := a.findScopeList.SelectedItem()
		if selected == nil {
			a.statusMsg = "No find scope selected."
			return a, nil
		}
		item, ok := selected.(findScopeItem)
		if !ok {
			return a, nil
		}
		a.showFindPicker = false
		return a.openFind(item.scope, "")
	}
	var cmd tea.Cmd
	a.findScopeList, cmd = a.findScopeList.Update(msg)
	return a, cmd
}

func (a App) handleAzureSubscriptionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showAzureSubscriptions = false
		a.openCredentials()
		a.statusMsg = "Azure subscription import canceled."
		return a, nil
	case " ":
		idx := a.azureSubList.Index()
		item, ok := a.selectedAzureSubscriptionItem()
		if !ok {
			a.statusMsg = "No Azure subscription selected."
			return a, nil
		}
		item.selected = !item.selected
		cmd := a.azureSubList.SetItem(idx, item)
		return a, cmd
	case "a", "A":
		items := a.azureSubList.Items()
		if len(items) == 0 {
			a.statusMsg = "No Azure subscriptions to select."
			return a, nil
		}
		allSelected := true
		for _, raw := range items {
			item, ok := raw.(azureSubscriptionItem)
			if !ok || !item.selected {
				allSelected = false
				break
			}
		}
		for i, raw := range items {
			item, ok := raw.(azureSubscriptionItem)
			if !ok {
				continue
			}
			item.selected = !allSelected
			_ = a.azureSubList.SetItem(i, item)
		}
		if allSelected {
			a.statusMsg = "Cleared Azure subscription selection."
		} else {
			a.statusMsg = "Selected all Azure subscriptions."
		}
		return a, nil
	case "enter":
		return a.importSelectedAzureSubscriptions()
	}
	var cmd tea.Cmd
	a.azureSubList, cmd = a.azureSubList.Update(msg)
	return a, cmd
}

func (a App) handleContextDiscoveryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showContextDiscovery = false
		a.openCredentials()
		a.statusMsg = "Context discovery import canceled."
		return a, nil
	case " ":
		idx := a.discoveryList.Index()
		item, ok := a.selectedDiscoveryContextItem()
		if !ok {
			a.statusMsg = "No discovered context selected."
			return a, nil
		}
		item.selected = !item.selected
		cmd := a.discoveryList.SetItem(idx, item)
		return a, cmd
	case "a", "A":
		items := a.discoveryList.Items()
		if len(items) == 0 {
			a.statusMsg = "No discovered contexts to select."
			return a, nil
		}
		allSelected := true
		for _, raw := range items {
			item, ok := raw.(discoveryContextItem)
			if !ok || !item.selected {
				allSelected = false
				break
			}
		}
		for i, raw := range items {
			item, ok := raw.(discoveryContextItem)
			if !ok {
				continue
			}
			item.selected = !allSelected
			_ = a.discoveryList.SetItem(i, item)
		}
		if allSelected {
			a.statusMsg = "Cleared discovered context selection."
		} else {
			a.statusMsg = "Selected all discovered contexts."
		}
		return a, nil
	case "enter":
		return a.importSelectedDiscoveredContexts()
	}
	var cmd tea.Cmd
	a.discoveryList, cmd = a.discoveryList.Update(msg)
	return a, cmd
}

func (a App) handleCredentialDeleteConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		a.confirmCredentialDelete = false
		a.credentialDeleteIndex = -1
		a.statusMsg = "Remove canceled."
		return a, nil
	case "y", "Y":
		idx := a.credentialDeleteIndex
		a.confirmCredentialDelete = false
		a.credentialDeleteIndex = -1
		if idx < 0 || idx >= len(a.cfg.CloudContexts) {
			a.statusMsg = "Profile/context no longer exists."
			return a, nil
		}
		removed := config.SanitizeManagedCloudContext(a.cfg.CloudContexts[idx])
		if path, err := config.BackupConfig(); err != nil {
			a.statusMsg = fmt.Sprintf("Backup failed; remove canceled: %v", err)
			return a, nil
		} else if path != "" {
			a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
		}
		a.cfg.CloudContexts = append(a.cfg.CloudContexts[:idx], a.cfg.CloudContexts[idx+1:]...)
		if strings.EqualFold(a.cfg.CurrentContext, removed.ContextName) {
			a.cfg.CurrentContext = ""
			a.activeCtx = core.CloudContext{}
		}
		if err := config.Save(a.cfg); err != nil {
			a.statusMsg = fmt.Sprintf("Remove failed: %v", err)
			return a, nil
		}
		a.credentialList.SetItems(buildCredentialItems(a.cfg))
		a.statusMsg = fmt.Sprintf("Profile/context %q removed.", removed.ContextName)
		return a, fetchContextsCmd(false)
	}
	a.statusMsg = "Confirm remove with y, or cancel with n/Esc."
	return a, nil
}

func (a App) handleProviderLoginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showProviderLogin = false
		a.statusMsg = "Provider login closed."
		return a, nil
	case "enter":
		item, ok := a.selectedProviderLoginItem()
		if !ok {
			a.statusMsg = "No provider login selected."
			return a, nil
		}
		if !item.available {
			a.statusMsg = item.reason
			return a, nil
		}
		a.showProviderLogin = false
		a.statusMsg = fmt.Sprintf("Running %s login...", item.provider)
		cmd := exec.Command(item.command[0], item.command[1:]...)
		return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return providerLoginCompleteMsg{provider: item.provider, err: err}
		})
	}
	var cmd tea.Cmd
	a.loginList, cmd = a.loginList.Update(msg)
	return a, cmd
}

func (a App) handleHostFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.showHostForm = false
		a.statusMsg = "Manual host add canceled."
		return a, nil
	case "tab", "down":
		a.moveHostFocus(1)
		return a, nil
	case "shift+tab", "up":
		a.moveHostFocus(-1)
		return a, nil
	case "enter", "ctrl+j", "ctrl+m", "f2":
		return a.saveManualHost()
	}
	var cmd tea.Cmd
	a.hostInputs[a.hostInputFocus], cmd = a.hostInputs[a.hostInputFocus].Update(msg)
	return a, cmd
}

func (a *App) moveHostFocus(delta int) {
	if len(a.hostInputs) == 0 {
		return
	}
	a.hostInputs[a.hostInputFocus].Blur()
	a.hostInputFocus = (a.hostInputFocus + delta + len(a.hostInputs)) % len(a.hostInputs)
	a.hostInputs[a.hostInputFocus].Focus()
}

func (a App) saveManualHost() (App, tea.Cmd) {
	host := config.SanitizeManualHost(config.ManualHost{
		Name:          strings.TrimSpace(a.hostInputs[0].Value()),
		Provider:      "Manual",
		Host:          strings.TrimSpace(a.hostInputs[1].Value()),
		Username:      strings.TrimSpace(a.hostInputs[2].Value()),
		Connection:    strings.TrimSpace(a.hostInputs[3].Value()),
		SSHConfigHost: strings.TrimSpace(a.hostInputs[4].Value()),
		KeyPath:       strings.TrimSpace(a.hostInputs[5].Value()),
		Tags:          splitCSV(a.hostInputs[6].Value()),
	})
	if host.Host == "" {
		a.statusMsg = "Host/IP is required."
		return a, nil
	}
	if host.Name == "" {
		host.Name = host.Host
	}
	if path, err := config.BackupConfig(); err != nil {
		a.statusMsg = fmt.Sprintf("Backup failed; host save canceled: %v", err)
		return a, nil
	} else if path != "" {
		a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
	}
	a.cfg.ManualHosts = append(a.cfg.ManualHosts, host)
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Manual host save failed: %v", err)
		return a, nil
	}
	a.showHostForm = false
	a.statusMsg = fmt.Sprintf("Manual host %s saved.", host.Name)
	a.indexManualHosts()
	return a, fetchContextsCmd(false)
}

func (a App) selectedProviderLoginItem() (providerLoginItem, bool) {
	selected := a.loginList.SelectedItem()
	if selected == nil {
		return providerLoginItem{}, false
	}
	item, ok := selected.(providerLoginItem)
	return item, ok
}

func buildProviderLoginItems() []list.Item {
	definitions := []providerLoginItem{
		{
			provider:    "Azure",
			title:       "Azure: az login",
			description: "Browser/device-code login. Discovers all visible subscriptions after completion.",
			command:     []string{"az", "login", "--use-device-code"},
		},
		{
			provider:    "GCP",
			title:       "GCP: gcloud auth login --no-browser",
			description: "Terminal-safe user login for gcloud commands.",
			command:     []string{"gcloud", "auth", "login", "--no-browser"},
		},
		{
			provider:    "GCP",
			title:       "GCP: application-default login",
			description: "ADC login for SDK-style calls.",
			command:     []string{"gcloud", "auth", "application-default", "login"},
		},
		{
			provider:    "AWS",
			title:       "AWS: configure SSO",
			description: "Interactive AWS SSO profile setup.",
			command:     []string{"aws", "configure", "sso"},
		},
		{
			provider:    "AWS",
			title:       "AWS: configure access keys",
			description: "Interactive static credential setup.",
			command:     []string{"aws", "configure"},
		},
		{
			provider:    "AWS",
			title:       "AWS: SSO login",
			description: "Login using the default/profile configured by AWS CLI.",
			command:     []string{"aws", "sso", "login"},
		},
		{
			provider:    "DigitalOcean",
			title:       "DigitalOcean: doctl auth init",
			description: "Interactive token setup for doctl.",
			command:     []string{"doctl", "auth", "init"},
		},
	}
	items := make([]list.Item, 0, len(definitions))
	for _, item := range definitions {
		if len(item.command) == 0 {
			continue
		}
		if _, err := exec.LookPath(item.command[0]); err != nil {
			item.available = false
			item.reason = fmt.Sprintf("%s CLI not found in PATH", item.command[0])
		} else {
			item.available = true
		}
		items = append(items, item)
	}
	return items
}

func (a *App) startCredentialEdit(index int) {
	a.editCredential = true
	a.credentialEditIndex = index
	a.credentialInputFocus = 0
	ctx := config.ManagedCloudContext{
		Provider:              "AWS",
		AuthMode:              config.AuthModeNativeCLI,
		CredentialPersistence: config.CredentialPersistenceNativeCLI,
		Regions:               []string{"us-east-1"},
	}
	if index >= 0 && index < len(a.cfg.CloudContexts) {
		ctx = config.SanitizeManagedCloudContext(a.cfg.CloudContexts[index])
	}
	values := []string{
		ctx.ContextName,
		ctx.Provider,
		ctx.AccountID,
		ctx.AccountName,
		ctx.Tenant,
		config.SanitizeAuthMode(ctx.AuthMode),
		config.SanitizeCredentialPersistence(ctx.CredentialPersistence, ctx.AuthMode),
		ctx.CredentialProfile,
		strings.Join(ctx.Regions, ","),
	}
	placeholders := []string{
		"eng",
		"AWS/GCP/Azure/DigitalOcean",
		"account/project/subscription id",
		"display name",
		"tenant id or domain",
		"native-cli | jit-session | awsume | vault | manual",
		"memory | keychain | native-cli | vault | none",
		"profile/auth reference",
		"us-east-1,us-west-2",
	}
	a.credentialInputs = make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.SetValue(value)
		input.Placeholder = placeholders[i]
		input.Width = 48
		if i == 0 {
			input.Focus()
		}
		a.credentialInputs[i] = input
	}
	a.statusMsg = "Editing managed context. Auth modes: native-cli, jit-session, awsume, vault, manual."
}

func (a App) handleCredentialFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.editCredential = false
		a.statusMsg = "Credential edit canceled."
		return a, nil
	case "tab", "down":
		a.moveCredentialFocus(1)
		return a, nil
	case "shift+tab", "up":
		a.moveCredentialFocus(-1)
		return a, nil
	case "enter":
		return a.saveCredentialEdit()
	}
	var cmd tea.Cmd
	a.credentialInputs[a.credentialInputFocus], cmd = a.credentialInputs[a.credentialInputFocus].Update(msg)
	return a, cmd
}

func (a *App) moveCredentialFocus(delta int) {
	if len(a.credentialInputs) == 0 {
		return
	}
	a.credentialInputs[a.credentialInputFocus].Blur()
	a.credentialInputFocus = (a.credentialInputFocus + delta + len(a.credentialInputs)) % len(a.credentialInputs)
	a.credentialInputs[a.credentialInputFocus].Focus()
}

func (a App) saveCredentialEdit() (App, tea.Cmd) {
	managed := config.ManagedCloudContext{
		ContextName:           strings.TrimSpace(a.credentialInputs[0].Value()),
		Provider:              strings.TrimSpace(a.credentialInputs[1].Value()),
		AccountID:             strings.TrimSpace(a.credentialInputs[2].Value()),
		AccountName:           strings.TrimSpace(a.credentialInputs[3].Value()),
		Tenant:                strings.TrimSpace(a.credentialInputs[4].Value()),
		AuthMode:              strings.TrimSpace(a.credentialInputs[5].Value()),
		CredentialPersistence: strings.TrimSpace(a.credentialInputs[6].Value()),
		CredentialProfile:     strings.TrimSpace(a.credentialInputs[7].Value()),
		Regions:               splitCSV(a.credentialInputs[8].Value()),
	}
	managed = config.SanitizeManagedCloudContext(managed)
	if managed.Provider == "" || (managed.AccountID == "" && managed.AccountName == "") {
		a.statusMsg = "Provider and account ID/name are required."
		return a, nil
	}
	if path, err := config.BackupConfig(); err != nil {
		a.statusMsg = fmt.Sprintf("Backup failed; save canceled: %v", err)
		return a, nil
	} else if path != "" {
		a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
	}
	if a.credentialEditIndex >= 0 && a.credentialEditIndex < len(a.cfg.CloudContexts) {
		a.cfg.CloudContexts[a.credentialEditIndex] = managed
	} else {
		a.cfg.CloudContexts = append(a.cfg.CloudContexts, managed)
	}
	if strings.TrimSpace(a.cfg.CurrentContext) == "" {
		a.cfg.CurrentContext = managed.ContextName
	}
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Credential save failed: %v", err)
		return a, nil
	}
	a.editCredential = false
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	a.statusMsg = fmt.Sprintf("Profile/context %q saved.", managed.ContextName)
	return a, fetchContextsCmd(false)
}

func (a App) useSelectedCredential() (App, tea.Cmd) {
	item, ok := a.selectedCredentialItem()
	if !ok {
		a.statusMsg = "No profile/context selected."
		return a, nil
	}
	ctx := config.SanitizeManagedCloudContext(item.ctx)
	if ctx.ContextName == "" {
		a.statusMsg = "Selected profile/context has no name."
		return a, nil
	}
	a.cfg.CurrentContext = ctx.ContextName
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Set current failed: %v", err)
		return a, nil
	}
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	if active, ok := config.CurrentCloudContext(a.cfg); ok {
		a.activeCtx = active
	}
	a.statusMsg = fmt.Sprintf("Current profile/context: %s", ctx.ContextName)
	return a, fetchContextsCmd(false)
}

func (a App) loginSelectedCredential() (App, tea.Cmd) {
	item, ok := a.selectedCredentialItem()
	if !ok {
		a.statusMsg = "No profile/context selected."
		return a, nil
	}
	ctx := config.SanitizeManagedCloudContext(item.ctx)
	command, err := managedContextLoginCommand(ctx)
	if err != nil {
		a.statusMsg = err.Error()
		return a, nil
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		a.statusMsg = fmt.Sprintf("%s CLI not found in PATH", command[0])
		return a, nil
	}
	a.cfg.CurrentContext = ctx.ContextName
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Set current before login failed: %v", err)
		return a, nil
	}
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	a.showCredentials = false
	a.statusMsg = fmt.Sprintf("Running login for %s...", ctx.ContextName)
	cmd := exec.Command(command[0], command[1:]...)
	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return providerLoginCompleteMsg{provider: ctx.Provider, contextName: ctx.ContextName, err: err}
	})
}

func (a App) selectedAzureSubscriptionItem() (azureSubscriptionItem, bool) {
	selected := a.azureSubList.SelectedItem()
	if selected == nil {
		return azureSubscriptionItem{}, false
	}
	item, ok := selected.(azureSubscriptionItem)
	return item, ok
}

func (a App) selectedDiscoveryContextItem() (discoveryContextItem, bool) {
	selected := a.discoveryList.SelectedItem()
	if selected == nil {
		return discoveryContextItem{}, false
	}
	item, ok := selected.(discoveryContextItem)
	return item, ok
}

func (a App) importSelectedDiscoveredContexts() (App, tea.Cmd) {
	var selected []core.CloudContext
	for _, raw := range a.discoveryList.Items() {
		item, ok := raw.(discoveryContextItem)
		if !ok || !item.selected {
			continue
		}
		selected = append(selected, item.ctx)
	}
	if len(selected) == 0 {
		a.statusMsg = "Select one or more discovered contexts first."
		return a, nil
	}
	if path, err := config.BackupConfig(); err != nil {
		a.statusMsg = fmt.Sprintf("Backup failed; discovery import canceled: %v", err)
		return a, nil
	} else if path != "" {
		a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
	}
	added, updated := upsertDiscoveredContexts(&a.cfg, selected)
	if strings.TrimSpace(a.cfg.CurrentContext) == "" && len(selected) > 0 {
		first := managedContextFromCloudContext(selected[0])
		a.cfg.CurrentContext = first.ContextName
	}
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Discovery import failed: %v", err)
		return a, nil
	}
	a.showContextDiscovery = false
	a.openCredentials()
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	a.statusMsg = fmt.Sprintf("Imported discovered contexts: %d added, %d updated.", added, updated)
	return a, fetchContextsCmd(false)
}

func (a App) importSelectedAzureSubscriptions() (App, tea.Cmd) {
	var selected []core.CloudContext
	for _, raw := range a.azureSubList.Items() {
		item, ok := raw.(azureSubscriptionItem)
		if !ok || !item.selected {
			continue
		}
		selected = append(selected, item.ctx)
	}
	if len(selected) == 0 {
		a.statusMsg = "Select one or more Azure subscriptions first."
		return a, nil
	}
	if path, err := config.BackupConfig(); err != nil {
		a.statusMsg = fmt.Sprintf("Backup failed; Azure import canceled: %v", err)
		return a, nil
	} else if path != "" {
		a.statusMsg = fmt.Sprintf("Backed up config to %s", path)
	}
	added, updated := upsertAzureContexts(&a.cfg, selected)
	if strings.TrimSpace(a.cfg.CurrentContext) == "" && len(selected) > 0 {
		first := azureManagedContextFromCloudContext(selected[0])
		a.cfg.CurrentContext = first.ContextName
	}
	if err := config.Save(a.cfg); err != nil {
		a.statusMsg = fmt.Sprintf("Azure import failed: %v", err)
		return a, nil
	}
	a.showAzureSubscriptions = false
	a.openCredentials()
	a.credentialList.SetItems(buildCredentialItems(a.cfg))
	a.statusMsg = fmt.Sprintf("Imported Azure subscriptions: %d added, %d updated.", added, updated)
	return a, fetchContextsCmd(false)
}

func upsertAzureContexts(cfg *config.AppConfig, contexts []core.CloudContext) (added, updated int) {
	for _, ctx := range contexts {
		managed := azureManagedContextFromCloudContext(ctx)
		if managed.AccountID == "" {
			continue
		}
		if idx := findManagedContextByProviderAccount(cfg.CloudContexts, "Azure", managed.AccountID); idx >= 0 {
			existing := config.SanitizeManagedCloudContext(cfg.CloudContexts[idx])
			if existing.ContextName != "" {
				managed.ContextName = existing.ContextName
			}
			if existing.CredentialProfile != "" {
				managed.CredentialProfile = existing.CredentialProfile
			}
			if existing.AuthMode != "" {
				managed.AuthMode = existing.AuthMode
			}
			if existing.CredentialPersistence != "" {
				managed.CredentialPersistence = existing.CredentialPersistence
			}
			cfg.CloudContexts[idx] = config.SanitizeManagedCloudContext(managed)
			updated++
			continue
		}
		cfg.CloudContexts = append(cfg.CloudContexts, managed)
		added++
	}
	return added, updated
}

func upsertDiscoveredContexts(cfg *config.AppConfig, contexts []core.CloudContext) (added, updated int) {
	for _, ctx := range contexts {
		managed := managedContextFromCloudContext(ctx)
		if managed.Provider == "" || (managed.AccountID == "" && managed.AccountName == "") {
			continue
		}
		if idx := findManagedContextForDiscovered(cfg.CloudContexts, managed); idx >= 0 {
			existing := config.SanitizeManagedCloudContext(cfg.CloudContexts[idx])
			if existing.ContextName != "" {
				managed.ContextName = existing.ContextName
			}
			if existing.AuthMode != "" {
				managed.AuthMode = existing.AuthMode
			}
			if existing.CredentialPersistence != "" {
				managed.CredentialPersistence = existing.CredentialPersistence
			}
			managed.Regions = mergeRegions(existing.Regions, managed.Regions)
			cfg.CloudContexts[idx] = config.SanitizeManagedCloudContext(managed)
			updated++
			continue
		}
		cfg.CloudContexts = append(cfg.CloudContexts, managed)
		added++
	}
	return added, updated
}

func managedContextFromCloudContext(ctx core.CloudContext) config.ManagedCloudContext {
	return config.SanitizeManagedCloudContext(config.ManagedCloudContext{
		ContextName:           ctx.ContextName,
		Provider:              ctx.Provider,
		AccountID:             ctx.AccountID,
		AccountName:           ctx.AccountName,
		Tenant:                ctx.Tenant,
		AuthMode:              config.AuthModeNativeCLI,
		CredentialPersistence: config.CredentialPersistenceNativeCLI,
		CredentialProfile:     ctx.CredentialProfile,
		Regions:               []string{orFallback(ctx.Region, "global")},
	})
}

func azureManagedContextFromCloudContext(ctx core.CloudContext) config.ManagedCloudContext {
	return config.SanitizeManagedCloudContext(config.ManagedCloudContext{
		ContextName:           ctx.ContextName,
		Provider:              "Azure",
		AccountID:             ctx.AccountID,
		AccountName:           ctx.AccountName,
		Tenant:                ctx.Tenant,
		AuthMode:              config.AuthModeNativeCLI,
		CredentialPersistence: config.CredentialPersistenceNativeCLI,
		CredentialProfile:     ctx.CredentialProfile,
		Regions:               []string{orFallback(ctx.Region, "global")},
	})
}

func findManagedContextByProviderAccount(contexts []config.ManagedCloudContext, provider, accountID string) int {
	for i, ctx := range contexts {
		ctx = config.SanitizeManagedCloudContext(ctx)
		if strings.EqualFold(ctx.Provider, provider) && strings.EqualFold(ctx.AccountID, accountID) {
			return i
		}
	}
	return -1
}

func findManagedContextForDiscovered(contexts []config.ManagedCloudContext, target config.ManagedCloudContext) int {
	target = config.SanitizeManagedCloudContext(target)
	for i, ctx := range contexts {
		ctx = config.SanitizeManagedCloudContext(ctx)
		if !strings.EqualFold(ctx.Provider, target.Provider) {
			continue
		}
		if target.AccountID != "" && strings.EqualFold(ctx.AccountID, target.AccountID) {
			if target.ContextName == "" || ctx.ContextName == "" || strings.EqualFold(ctx.ContextName, target.ContextName) || strings.EqualFold(ctx.CredentialProfile, target.CredentialProfile) {
				return i
			}
		}
		if target.ContextName != "" && strings.EqualFold(ctx.ContextName, target.ContextName) {
			return i
		}
	}
	return -1
}

func mergeRegions(left, right []string) []string {
	seen := map[string]bool{}
	var merged []string
	for _, value := range append(append([]string{}, left...), right...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		merged = append(merged, value)
	}
	return merged
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (a App) selectedCredentialItem() (credentialItem, bool) {
	selected := a.credentialList.SelectedItem()
	if selected == nil {
		return credentialItem{}, false
	}
	item, ok := selected.(credentialItem)
	return item, ok
}

func buildCredentialItems(cfg config.AppConfig) []list.Item {
	items := make([]list.Item, 0, len(cfg.CloudContexts))
	for i, ctx := range cfg.CloudContexts {
		sanitized := config.SanitizeManagedCloudContext(ctx)
		items = append(items, credentialItem{
			index:   i,
			ctx:     sanitized,
			current: strings.EqualFold(cfg.CurrentContext, sanitized.ContextName),
		})
	}
	return items
}

func buildAzureSubscriptionItems(cfg config.AppConfig, contexts []core.CloudContext) []list.Item {
	items := make([]list.Item, 0, len(contexts))
	for _, ctx := range contexts {
		if !strings.EqualFold(ctx.Provider, "Azure") {
			continue
		}
		existing := findManagedContextByProviderAccount(cfg.CloudContexts, "Azure", ctx.AccountID) >= 0
		items = append(items, azureSubscriptionItem{
			ctx:      ctx,
			selected: !existing,
			existing: existing,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].(azureSubscriptionItem)
		right := items[j].(azureSubscriptionItem)
		if left.existing != right.existing {
			return !left.existing
		}
		return strings.ToLower(left.ctx.DisplayName()) < strings.ToLower(right.ctx.DisplayName())
	})
	return items
}

func buildDiscoveryContextItems(cfg config.AppConfig, contexts []core.CloudContext) []list.Item {
	items := make([]list.Item, 0, len(contexts))
	seen := make(map[string]bool, len(contexts))
	for _, ctx := range contexts {
		managed := managedContextFromCloudContext(ctx)
		if managed.Provider == "" {
			continue
		}
		key := strings.ToLower(strings.Join([]string{
			managed.Provider,
			managed.ContextName,
			managed.AccountID,
			managed.AccountName,
			managed.Tenant,
			managed.CredentialProfile,
			strings.Join(managed.Regions, ","),
		}, "|"))
		if seen[key] {
			continue
		}
		seen[key] = true
		existing := findManagedContextForDiscovered(cfg.CloudContexts, managed) >= 0
		items = append(items, discoveryContextItem{
			ctx:      ctx,
			selected: false,
			existing: existing,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].(discoveryContextItem)
		right := items[j].(discoveryContextItem)
		if left.existing != right.existing {
			return !left.existing
		}
		if left.ctx.Provider != right.ctx.Provider {
			return left.ctx.Provider < right.ctx.Provider
		}
		return strings.ToLower(left.ctx.DisplayName()) < strings.ToLower(right.ctx.DisplayName())
	})
	return items
}

func managedContextLoginCommand(ctx config.ManagedCloudContext) ([]string, error) {
	switch config.SanitizeManagedCloudContext(ctx).Provider {
	case "Azure":
		args := []string{"az", "login", "--use-device-code"}
		if strings.TrimSpace(ctx.Tenant) != "" {
			args = append(args, "--tenant", strings.TrimSpace(ctx.Tenant))
		}
		return args, nil
	case "AWS":
		args := []string{"aws", "sso", "login"}
		if strings.TrimSpace(ctx.CredentialProfile) != "" {
			args = append(args, "--profile", strings.TrimSpace(ctx.CredentialProfile))
		}
		return args, nil
	case "GCP":
		return []string{"gcloud", "auth", "login", "--no-browser"}, nil
	case "DigitalOcean":
		return []string{"doctl", "auth", "init"}, nil
	default:
		return nil, fmt.Errorf("login is not implemented for provider %s", ctx.Provider)
	}
}

func managedContextLabel(ctx config.ManagedCloudContext) string {
	if strings.TrimSpace(ctx.AccountID) != "" && strings.TrimSpace(ctx.AccountName) != "" && ctx.AccountID != ctx.AccountName {
		return fmt.Sprintf("%s (%s)", ctx.AccountID, ctx.AccountName)
	}
	if strings.TrimSpace(ctx.AccountID) != "" {
		return ctx.AccountID
	}
	if strings.TrimSpace(ctx.AccountName) != "" {
		return ctx.AccountName
	}
	return "unnamed"
}

func (a App) handleGlobalSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.findSortHeader.Active {
		switch msg.String() {
		case "esc", "down":
			a.findSortHeader.Deactivate()
		case "left", "h":
			a.findSortHeader.Move(-1, a.globalSearchTable.Columns())
		case "right", "l":
			a.findSortHeader.Move(1, a.globalSearchTable.Columns())
		case "enter":
			column := a.findSortHeader.SelectedColumn(a.globalSearchTable.Columns())
			a.findSortColumn, a.findSortAsc = ToggleSortColumn(a.findSortColumn, a.findSortAsc, column)
			a.statusMsg = fmt.Sprintf("Sorted Find by %s (%s).", a.findSortColumn, SortDirectionLabel(a.findSortAsc))
		}
		a.resizeGlobalSearch()
		a.refreshGlobalSearchResults()
		return a, nil
	}
	if a.showFindSort {
		switch msg.String() {
		case "esc":
			a.showFindSort = false
			a.globalSearchTable.Focus()
			return a, nil
		case "enter":
			selected, ok := a.findSortList.SelectedItem().(SortColumnItem)
			if !ok {
				return a, nil
			}
			a.findSortColumn, a.findSortAsc = ToggleSortColumn(a.findSortColumn, a.findSortAsc, selected.Name)
			a.refreshGlobalSearchResults()
			a.showFindSort = false
			a.statusMsg = fmt.Sprintf("Sorted Find by %s (%s).", a.findSortColumn, SortDirectionLabel(a.findSortAsc))
			return a, nil
		default:
			var cmd tea.Cmd
			a.findSortList, cmd = a.findSortList.Update(msg)
			return a, cmd
		}
	}
	switch msg.String() {
	case "esc":
		if a.globalSearchInput.Focused() {
			a.globalSearchInput.Blur()
			a.globalSearchTable.Focus()
			return a, nil
		}
		a.showGlobalSearch = false
		return a, nil
	case "enter":
		return a.openSelectedFindResult()
	case "/":
		a.globalSearchInput.Focus()
		a.globalSearchTable.Blur()
		return a, textinput.Blink
	case "S":
		a.findSortHeader.Activate(a.globalSearchTable.Columns())
		a.globalSearchInput.Blur()
		a.globalSearchTable.Focus()
		a.resizeGlobalSearch()
		return a, nil
	case "up":
		if a.globalSearchTable.Cursor() == 0 && !a.globalSearchInput.Focused() {
			a.findSortHeader.Activate(a.globalSearchTable.Columns())
			a.resizeGlobalSearch()
			return a, nil
		}
	case "K":
		if a.findScope == findScopeVMs || a.findScope == findScopeAll {
			a.toggleKubernetesNodes()
			return a, nil
		}
	}
	var cmd tea.Cmd
	if a.globalSearchInput.Focused() {
		a.globalSearchInput, cmd = a.globalSearchInput.Update(msg)
		a.refreshGlobalSearchResults()
	} else {
		a.globalSearchTable, cmd = a.globalSearchTable.Update(msg)
	}
	return a, cmd
}

func (a App) openSelectedGlobalVM() (App, tea.Cmd) {
	if len(a.findRows) == 0 && len(a.vmSearchRows) > 0 {
		a.findRows = vmRecordsToFindRecords(a.vmSearchRows)
	}
	return a.openSelectedFindResult()
}

func (a App) openSelectedFindResult() (App, tea.Cmd) {
	idx := a.globalSearchTable.Cursor()
	if idx < 0 && len(a.findRows) > 0 {
		idx = 0
	}
	if idx < 0 || idx >= len(a.findRows) {
		a.statusMsg = "No find result selected."
		return a, nil
	}
	rec := a.findRows[idx]
	a.showGlobalSearch = false
	a.globalSearchInput.Blur()
	a.activeCtx = rec.Context
	if !a.forceRootTabByCapability(rec.Capability) {
		a.statusMsg = fmt.Sprintf("%s view is not registered.", rec.Kind)
		return a, nil
	}
	a.focus = focusMain
	a.showCmdBar = false
	a.applyKubernetesNodeVisibility()
	topView := a.viewStack[len(a.viewStack)-1]
	initCmd := topView.Init(a.activeCtx, a.mainContentWidth(), a.mainContentHeight(), a.showSidebar)
	if searchable, ok := topView.(interface{ SetSearchQuery(string) }); ok {
		searchable.SetSearchQuery(rec.Filter)
	}
	a.statusMsg = fmt.Sprintf("Opening %s %s in %s %s.", strings.ToLower(rec.Kind), rec.Name, rec.Context.Provider, rec.Context.Region)
	return a, initCmd
}

func (a *App) forceRootTabByCapability(cap providers.Capability) bool {
	selected := tabForCapability(a.resourceViews, cap)
	if selected == "" {
		a.activeTab = ""
		a.viewStack = nil
		return false
	}
	registered, ok := a.resourceViews[selected]
	if !ok {
		a.activeTab = ""
		a.viewStack = nil
		return false
	}
	a.activeTab = selected
	a.viewStack = []View{registered.View}
	return true
}

func tabForCapability(views map[string]registeredView, cap providers.Capability) string {
	selected := ""
	for tabIndex, view := range views {
		if view.Capability != cap {
			continue
		}
		if selected == "" || tabIndex < selected {
			selected = tabIndex
		}
	}
	return selected
}

func (a *App) refreshGlobalSearchResults() {
	query := a.globalSearchInput.Value()
	var rows []findRecord
	if a.findScope == findScopeVMs {
		vmRows := searchVMRecords(a.visibleVMIndexRecords(), query)
		a.vmSearchRows = vmRows
		rows = vmRecordsToFindRecords(vmRows)
	} else if a.findScope == findScopeHosts {
		vmRows := searchVMRecords(filterVMRecordsByProvider(a.visibleVMIndexRecords(), "Manual"), query)
		a.vmSearchRows = vmRows
		rows = vmRecordsToFindRecords(vmRows)
	} else {
		rows = a.findRecords(a.findScope, query)
	}
	SortByColumn(rows, a.findSortColumn, a.findSortAsc, findField)
	a.findRows = rows
	a.globalSearchTable.SetRows(mapFindRows(rows, a.globalSearchTable.Columns()))
	if len(rows) == 0 {
		a.globalSearchTable.SetCursor(0)
		return
	}
	if cursor := a.globalSearchTable.Cursor(); cursor >= len(rows) {
		a.globalSearchTable.SetCursor(len(rows) - 1)
	}
}

func (a *App) resizeGlobalSearch() {
	width := a.fullScreenContentWidth()
	height := a.fullScreenContentHeight() - 5
	if width < 40 {
		width = 40
	}
	if height < 5 {
		height = 5
	}
	columns := fitGlobalSearchColumns(width)
	columns = DecorateSortColumns(columns, a.findSortHeader, a.findSortColumn, a.findSortAsc)
	a.globalSearchTable.SetRows(nil)
	a.globalSearchTable.SetColumns(columns)
	a.globalSearchTable.SetWidth(TableViewportWidth(width))
	a.globalSearchTable.SetHeight(height)
	a.findSortList.SetSize(width, height)
	a.globalSearchInput.Width = width - 8
	if a.globalSearchInput.Width < 20 {
		a.globalSearchInput.Width = 20
	}
	a.globalSearchTable.SetRows(mapFindRows(a.findRows, columns))
}

func (a *App) indexVMs(ctx core.CloudContext, vms []core.VM) {
	if a.vmIndex == nil {
		a.vmIndex = make(map[string]vmSearchRecord)
	}
	vms = config.ApplyResourceTagsToVMs(a.cfg, ctx, vms)
	vms = iac.ApplyTerraformToVMs(a.cfg.TerraformStatePaths, ctx, vms)
	now := time.Now()
	for _, vm := range vms {
		key := vmIndexKey(ctx, vm)
		a.vmIndex[key] = vmSearchRecord{VM: vm, Context: ctx, SeenAt: now}
	}
	a.persistVMIndex()
}

func (a *App) indexManualHosts() {
	if a.vmIndex == nil {
		a.vmIndex = make(map[string]vmSearchRecord)
	}
	a.pruneManualHostsFromIndex()
	ctx := core.CloudContext{
		Provider:        "Manual",
		AccountID:       "manual-hosts",
		AccountName:     "Manual Hosts",
		Region:          "global",
		AuthMode:        config.AuthModeManual,
		CredentialScope: config.CredentialPersistenceNone,
	}
	now := time.Now()
	for _, vm := range iac.ApplyTerraformToVMs(a.cfg.TerraformStatePaths, ctx, config.ApplyResourceTagsToVMs(a.cfg, ctx, hostinventory.ToVMs(a.cfg))) {
		a.vmIndex[vmIndexKey(ctx, vm)] = vmSearchRecord{VM: vm, Context: ctx, SeenAt: now}
	}
	a.refreshGlobalSearchResults()
}

func (a *App) pruneManualHostsFromIndex() {
	for key, rec := range a.vmIndex {
		if rec.Context.Provider == "Manual" {
			delete(a.vmIndex, key)
		}
	}
}

func (a *App) indexClusters(ctx core.CloudContext, clusters []core.Cluster) {
	if a.clusterIndex == nil {
		a.clusterIndex = make(map[string]clusterIndexRecord)
	}
	clusters = config.ApplyResourceTagsToClusters(a.cfg, ctx, clusters)
	clusters = iac.ApplyTerraformToClusters(a.cfg.TerraformStatePaths, ctx, clusters)
	now := time.Now()
	for _, cluster := range clusters {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(cluster.ID, cluster.Name))
		a.clusterIndex[key] = clusterIndexRecord{Cluster: cluster, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexDatabases(ctx core.CloudContext, databases []core.Database) {
	if a.databaseIndex == nil {
		a.databaseIndex = make(map[string]databaseIndexRecord)
	}
	databases = config.ApplyResourceTagsToDatabases(a.cfg, ctx, databases)
	databases = iac.ApplyTerraformToDatabases(a.cfg.TerraformStatePaths, ctx, databases)
	now := time.Now()
	for _, db := range databases {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(db.ID, db.Name))
		a.databaseIndex[key] = databaseIndexRecord{Database: db, Context: ctx, SeenAt: now}
	}
	stats := a.databaseDashboardStats()
	logging.Infof("component=ui event=database_index_updated provider=%s account=%s region=%s added=%d total=%d ready=%d down=%d other=%d contexts=%d", ctx.Provider, ctx.AccountID, ctx.Region, len(databases), stats.Total, stats.Running, stats.Stopped, stats.Other, stats.IndexedContexts)
}

func (a *App) indexResourceSummary(ctx core.CloudContext, resource string, count, extra int) {
	record := resourceSummaryRecord{Count: count, Extra: extra}
	key := ctx.CacheKey()
	switch resource {
	case "disks":
		if a.diskSummaryIndex == nil {
			a.diskSummaryIndex = make(map[string]resourceSummaryRecord)
		}
		a.diskSummaryIndex[key] = record
	case "snapshots":
		if a.snapshotSummaryIndex == nil {
			a.snapshotSummaryIndex = make(map[string]resourceSummaryRecord)
		}
		a.snapshotSummaryIndex[key] = record
	case "networks":
		if a.networkSummaryIndex == nil {
			a.networkSummaryIndex = make(map[string]resourceSummaryRecord)
		}
		a.networkSummaryIndex[key] = record
	case "firewalls":
		if a.firewallSummaryIndex == nil {
			a.firewallSummaryIndex = make(map[string]resourceSummaryRecord)
		}
		a.firewallSummaryIndex[key] = record
	case "storage":
		if a.storageSummaryIndex == nil {
			a.storageSummaryIndex = make(map[string]resourceSummaryRecord)
		}
		a.storageSummaryIndex[key] = record
	}
}

func (a *App) indexResourceSummaryMsg(msg ResourceSummaryUpdateMsg) {
	switch msg.Resource {
	case "disks":
		a.indexDisks(msg.Ctx, msg.Disks)
	case "snapshots":
		a.indexSnapshots(msg.Ctx, msg.Snapshots)
	case "networks":
		a.indexNetworks(msg.Ctx, msg.Networks)
		a.indexSubnets(msg.Ctx, msg.Subnets)
	case "firewalls":
		a.indexFirewalls(msg.Ctx, msg.SecurityGroups)
	case "storage":
		a.indexStorage(msg.Ctx, msg.StorageBuckets)
	}
	a.indexResourceSummary(msg.Ctx, msg.Resource, msg.Count, msg.Extra)
}

func (a *App) indexDisks(ctx core.CloudContext, disks []core.Disk) {
	if a.diskIndex == nil {
		a.diskIndex = make(map[string]diskIndexRecord)
	}
	disks = config.ApplyResourceTagsToDisks(a.cfg, ctx, disks)
	now := time.Now()
	for _, disk := range disks {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(disk.ID, disk.Name))
		a.diskIndex[key] = diskIndexRecord{Disk: disk, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexSnapshots(ctx core.CloudContext, snapshots []core.Snapshot) {
	if a.snapshotIndex == nil {
		a.snapshotIndex = make(map[string]snapshotIndexRecord)
	}
	snapshots = config.ApplyResourceTagsToSnapshots(a.cfg, ctx, snapshots)
	now := time.Now()
	for _, snap := range snapshots {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(snap.ID, snap.Name))
		a.snapshotIndex[key] = snapshotIndexRecord{Snapshot: snap, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexNetworks(ctx core.CloudContext, networks []core.Network) {
	if a.networkIndex == nil {
		a.networkIndex = make(map[string]networkIndexRecord)
	}
	networks = config.ApplyResourceTagsToNetworks(a.cfg, ctx, networks)
	now := time.Now()
	for _, network := range networks {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(network.ID, network.Name))
		a.networkIndex[key] = networkIndexRecord{Network: network, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexSubnets(ctx core.CloudContext, subnets []core.Subnet) {
	if a.subnetIndex == nil {
		a.subnetIndex = make(map[string]subnetIndexRecord)
	}
	subnets = config.ApplyResourceTagsToSubnets(a.cfg, ctx, subnets)
	now := time.Now()
	for _, subnet := range subnets {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(subnet.ID, subnet.Name))
		a.subnetIndex[key] = subnetIndexRecord{Subnet: subnet, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexFirewalls(ctx core.CloudContext, groups []core.SecurityGroup) {
	if a.firewallIndex == nil {
		a.firewallIndex = make(map[string]firewallIndexRecord)
	}
	groups = config.ApplyResourceTagsToSecurityGroups(a.cfg, ctx, groups)
	now := time.Now()
	for _, group := range groups {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(group.ID, group.Name))
		a.firewallIndex[key] = firewallIndexRecord{Group: group, Context: ctx, SeenAt: now}
	}
}

func (a *App) indexStorage(ctx core.CloudContext, buckets []core.StorageBucket) {
	if a.storageIndex == nil {
		a.storageIndex = make(map[string]storageIndexRecord)
	}
	buckets = config.ApplyResourceTagsToStorageBuckets(a.cfg, ctx, buckets)
	now := time.Now()
	for _, bucket := range buckets {
		key := fmt.Sprintf("%s|%s", ctx.CacheKey(), orFallback(bucket.ID, bucket.Name))
		a.storageIndex[key] = storageIndexRecord{Bucket: bucket, Context: ctx, SeenAt: now}
	}
}

func (a *App) applyCloudManagerTagsToResourceIndexes() {
	for key, rec := range a.clusterIndex {
		rec.Cluster = config.ApplyResourceTagsToClusters(a.cfg, rec.Context, []core.Cluster{rec.Cluster})[0]
		a.clusterIndex[key] = rec
	}
	for key, rec := range a.databaseIndex {
		rec.Database = config.ApplyResourceTagsToDatabases(a.cfg, rec.Context, []core.Database{rec.Database})[0]
		a.databaseIndex[key] = rec
	}
	for key, rec := range a.diskIndex {
		rec.Disk = config.ApplyResourceTagsToDisks(a.cfg, rec.Context, []core.Disk{rec.Disk})[0]
		a.diskIndex[key] = rec
	}
	for key, rec := range a.snapshotIndex {
		rec.Snapshot = config.ApplyResourceTagsToSnapshots(a.cfg, rec.Context, []core.Snapshot{rec.Snapshot})[0]
		a.snapshotIndex[key] = rec
	}
	for key, rec := range a.networkIndex {
		rec.Network = config.ApplyResourceTagsToNetworks(a.cfg, rec.Context, []core.Network{rec.Network})[0]
		a.networkIndex[key] = rec
	}
	for key, rec := range a.subnetIndex {
		rec.Subnet = config.ApplyResourceTagsToSubnets(a.cfg, rec.Context, []core.Subnet{rec.Subnet})[0]
		a.subnetIndex[key] = rec
	}
	for key, rec := range a.firewallIndex {
		rec.Group = config.ApplyResourceTagsToSecurityGroups(a.cfg, rec.Context, []core.SecurityGroup{rec.Group})[0]
		a.firewallIndex[key] = rec
	}
	for key, rec := range a.storageIndex {
		rec.Bucket = config.ApplyResourceTagsToStorageBuckets(a.cfg, rec.Context, []core.StorageBucket{rec.Bucket})[0]
		a.storageIndex[key] = rec
	}
}

func vmIndexKey(ctx core.CloudContext, vm core.VM) string {
	return fmt.Sprintf("%s|%s", ctx.CacheKey(), vm.ID)
}

func (a App) persistVMIndex() {
	if !a.cfg.VMIndexPersistence || len(a.vmIndex) == 0 {
		return
	}
	if err := saveVMIndexCache(a.vmIndex); err != nil {
		logging.Warnf("component=ui event=vm_index_cache_save_failed err=%v", err)
	}
}

func (a App) persistResourceIndex() {
	if !a.cfg.ResourceIndexPersistence || a.resourceIndexSize() == 0 {
		return
	}
	if err := saveResourceIndexCache(a); err != nil {
		logging.Warnf("component=ui event=resource_index_cache_save_failed err=%v", err)
	}
}

func (a App) resourceIndexSize() int {
	return len(a.clusterIndex) + len(a.databaseIndex) + len(a.diskIndex) + len(a.snapshotIndex) + len(a.networkIndex) + len(a.subnetIndex) + len(a.firewallIndex) + len(a.storageIndex)
}

func searchVMIndex(index map[string]vmSearchRecord, query string) []vmSearchRecord {
	records := make([]vmSearchRecord, 0, len(index))
	for _, rec := range index {
		records = append(records, rec)
	}
	return searchVMRecords(records, query)
}

func searchVMRecords(records []vmSearchRecord, query string) []vmSearchRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]vmSearchRecord, 0, len(records))
	for _, rec := range records {
		if vmRecordMatchesQuery(rec, query) {
			results = append(results, rec)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		left, right := results[i], results[j]
		for _, cmp := range []int{
			strings.Compare(left.Context.Provider, right.Context.Provider),
			strings.Compare(left.Context.DisplayName(), right.Context.DisplayName()),
			strings.Compare(left.Context.Region, right.Context.Region),
			strings.Compare(strings.ToLower(left.VM.Name), strings.ToLower(right.VM.Name)),
			strings.Compare(strings.ToLower(left.VM.ID), strings.ToLower(right.VM.ID)),
		} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
	return results
}

func vmRecordMatchesQuery(rec vmSearchRecord, query string) bool {
	switch query {
	case "":
		return true
	case "has:public-ip", "public-ip:true", "public_ip:true":
		return hasUsablePublicIP(rec.VM.PublicIP)
	default:
		return strings.Contains(vmSearchBlob(rec), query)
	}
}

func vmSearchBlob(rec vmSearchRecord) string {
	vm := rec.VM
	ctx := rec.Context
	parts := []string{
		vm.Name, vm.ID, vm.PrivateIP, vm.PublicIP, vm.Labels, vm.Network,
		vm.Subnet, vm.SecurityGroups, vm.ResourceGroup, vm.Zone, vm.State,
		ctx.Provider, ctx.AccountID, ctx.AccountName, ctx.Region, ctx.AuthRef(),
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (a App) findRecords(scope findScope, query string) []findRecord {
	var records []findRecord
	switch scope {
	case findScopeDisks:
		records = a.diskFindRecords()
	case findScopeSnapshots:
		records = a.snapshotFindRecords()
	case findScopeDatabases:
		records = a.databaseFindRecords()
	case findScopeK8s:
		records = a.k8sFindRecords()
	case findScopeNetworks:
		records = a.networkFindRecords()
	case findScopeSubnets:
		records = a.subnetFindRecords()
	case findScopeFirewalls:
		records = a.firewallFindRecords()
	case findScopeStorage:
		records = a.storageFindRecords()
	case findScopeHosts:
		records = vmRecordsToFindRecords(filterVMRecordsByProvider(a.visibleVMIndexRecords(), "Manual"))
	case findScopeAll:
		records = append(records, vmRecordsToFindRecords(a.visibleVMIndexRecords())...)
		records = append(records, a.diskFindRecords()...)
		records = append(records, a.snapshotFindRecords()...)
		records = append(records, a.databaseFindRecords()...)
		records = append(records, a.k8sFindRecords()...)
		records = append(records, a.networkFindRecords()...)
		records = append(records, a.subnetFindRecords()...)
		records = append(records, a.firewallFindRecords()...)
		records = append(records, a.storageFindRecords()...)
	default:
		records = vmRecordsToFindRecords(a.visibleVMIndexRecords())
	}
	return searchFindRecords(records, query)
}

func (a App) diskFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.diskIndex))
	for _, rec := range a.diskIndex {
		disk := rec.Disk
		id := orFallback(disk.ID, disk.Name)
		records = append(records, findRecord{
			Kind:       "DISK",
			Name:       disk.Name,
			ID:         id,
			Location:   disk.Zone,
			Status:     disk.State,
			Match:      strings.Join(nonEmptyStrings(disk.Type, disk.Zone, disk.AttachedToVM, disk.ResourceGroup, disk.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityDisks,
			Filter:     id,
		})
	}
	return records
}

func (a App) snapshotFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.snapshotIndex))
	for _, rec := range a.snapshotIndex {
		snap := rec.Snapshot
		id := orFallback(snap.ID, snap.Name)
		records = append(records, findRecord{
			Kind:       "SNAP",
			Name:       snap.Name,
			ID:         id,
			Location:   snap.Zone,
			Status:     snap.State,
			Match:      strings.Join(nonEmptyStrings(snap.SourceDiskName, snap.SourceDiskID, snap.Zone, snap.ResourceGroup, snap.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilitySnapshots,
			Filter:     id,
		})
	}
	return records
}

func (a App) databaseFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.databaseIndex))
	for _, rec := range a.databaseIndex {
		db := rec.Database
		id := orFallback(db.ID, db.Name)
		records = append(records, findRecord{
			Kind:       "DB",
			Name:       db.Name,
			ID:         id,
			Location:   db.Region,
			Status:     db.Status,
			Match:      strings.Join(nonEmptyStrings(db.Engine, db.Version, db.Size, db.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityDatabases,
			Filter:     id,
		})
	}
	return records
}

func (a App) k8sFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.clusterIndex))
	for _, rec := range a.clusterIndex {
		cluster := rec.Cluster
		id := orFallback(cluster.ID, cluster.Name)
		records = append(records, findRecord{
			Kind:       "K8S",
			Name:       cluster.Name,
			ID:         id,
			Location:   cluster.Location,
			Status:     cluster.Status,
			Match:      strings.Join(nonEmptyStrings(cluster.Version, cluster.NodeCount, cluster.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityClusters,
			Filter:     id,
		})
	}
	return records
}

func (a App) networkFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.networkIndex))
	for _, rec := range a.networkIndex {
		network := rec.Network
		id := orFallback(network.ID, network.Name)
		records = append(records, findRecord{
			Kind:       "NET",
			Name:       network.Name,
			ID:         id,
			Location:   network.Region,
			Status:     network.State,
			Match:      strings.Join(nonEmptyStrings(network.CIDRBlock, network.Region, network.ResourceGroup, network.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityNetworks,
			Filter:     id,
		})
	}
	return records
}

func (a App) subnetFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.subnetIndex))
	for _, rec := range a.subnetIndex {
		subnet := rec.Subnet
		id := orFallback(subnet.ID, subnet.Name)
		records = append(records, findRecord{
			Kind:       "SUBNET",
			Name:       subnet.Name,
			ID:         id,
			Location:   orFallback(subnet.AvailabilityZone, subnet.Region),
			Status:     subnet.State,
			Match:      strings.Join(nonEmptyStrings(subnet.CIDRBlock, subnet.AvailabilityZone, subnet.NetworkName, subnet.NetworkID, subnet.Region, subnet.ResourceGroup, subnet.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityNetworks,
			Filter:     "subnet:" + id,
		})
	}
	return records
}

func (a App) firewallFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.firewallIndex))
	for _, rec := range a.firewallIndex {
		group := rec.Group
		id := orFallback(group.ID, group.Name)
		records = append(records, findRecord{
			Kind:       "SG",
			Name:       group.Name,
			ID:         id,
			Location:   group.Region,
			Status:     firewallFindStatus(group),
			Match:      strings.Join(nonEmptyStrings(group.Description, group.NetworkName, group.NetworkID, group.Region, group.ResourceGroup, group.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityFirewalls,
			Filter:     id,
		})
	}
	return records
}

func (a App) storageFindRecords() []findRecord {
	records := make([]findRecord, 0, len(a.storageIndex))
	for _, rec := range a.storageIndex {
		bucket := rec.Bucket
		id := orFallback(bucket.ID, bucket.Name)
		records = append(records, findRecord{
			Kind:       "STORE",
			Name:       bucket.Name,
			ID:         id,
			Location:   bucket.Region,
			Status:     bucket.Access,
			Match:      strings.Join(nonEmptyStrings(bucket.ProviderType, bucket.Region, bucket.StorageClass, bucket.Encrypted, bucket.Versioning, bucket.ResourceGroup, bucket.Labels), " "),
			Context:    rec.Context,
			Capability: providers.CapabilityStorage,
			Filter:     id,
		})
	}
	return records
}

func firewallFindStatus(group core.SecurityGroup) string {
	if group.HasAuditRisk() {
		return "open ingress"
	}
	return fmt.Sprintf("%d in / %d out", group.InboundRuleCount, group.OutboundRuleCount)
}

func vmRecordsToFindRecords(records []vmSearchRecord) []findRecord {
	out := make([]findRecord, 0, len(records))
	for _, rec := range records {
		vm := rec.VM
		kind := "VM"
		capability := providers.CapabilityVMs
		if strings.EqualFold(rec.Context.Provider, "Manual") {
			kind = "HOST"
			capability = providers.CapabilityHosts
		}
		id := orFallback(vm.ID, vm.Name)
		out = append(out, findRecord{
			Kind:       kind,
			Name:       vm.Name,
			ID:         id,
			PublicIP:   vm.PublicIP,
			Location:   vm.Zone,
			Status:     vm.State,
			Match:      strings.Join(nonEmptyStrings(vm.PrivateIP, vm.PublicIP, vm.Zone, vm.Network, vm.Subnet, vm.SecurityGroups, vm.Labels), " "),
			Context:    rec.Context,
			Capability: capability,
			Filter:     id,
		})
	}
	return out
}

func findRecordsToVMRecords(records []findRecord) []vmSearchRecord {
	out := make([]vmSearchRecord, 0, len(records))
	for _, rec := range records {
		if rec.Kind != "VM" && rec.Kind != "HOST" {
			continue
		}
		out = append(out, vmSearchRecord{
			Context: rec.Context,
			VM: core.VM{
				Name:     rec.Name,
				ID:       rec.ID,
				PublicIP: rec.PublicIP,
				Zone:     rec.Location,
				State:    rec.Status,
			},
		})
	}
	return out
}

func filterVMRecordsByProvider(records []vmSearchRecord, provider string) []vmSearchRecord {
	var out []vmSearchRecord
	for _, rec := range records {
		if strings.EqualFold(rec.Context.Provider, provider) {
			out = append(out, rec)
		}
	}
	return out
}

func searchFindRecords(records []findRecord, query string) []findRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	var results []findRecord
	for _, rec := range records {
		if query == "" || strings.Contains(findRecordBlob(rec), query) {
			results = append(results, rec)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		left, right := results[i], results[j]
		for _, cmp := range []int{
			strings.Compare(left.Kind, right.Kind),
			strings.Compare(left.Context.Provider, right.Context.Provider),
			strings.Compare(left.Context.DisplayName(), right.Context.DisplayName()),
			strings.Compare(findRecordLocation(left), findRecordLocation(right)),
			strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name)),
			strings.Compare(strings.ToLower(left.ID), strings.ToLower(right.ID)),
		} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
	return results
}

func findRecordBlob(rec findRecord) string {
	return strings.ToLower(strings.Join([]string{
		rec.Kind, rec.Name, rec.ID, rec.Status, findRecordLocation(rec), rec.Match,
	}, " "))
}

func findRecordLocation(rec findRecord) string {
	if value := strings.TrimSpace(rec.Location); value != "" && value != "-" {
		return value
	}
	return rec.Context.Region
}

func nonEmptyStrings(values ...string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "-" {
			out = append(out, value)
		}
	}
	return out
}

func globalSearchColumns() []table.Column {
	return []table.Column{
		{Title: "Type", Width: 10},
		{Title: "Name", Width: 24},
		{Title: "ID", Width: 20},
		{Title: "Public IP", Width: 15},
		{Title: "Provider", Width: 10},
		{Title: "Context", Width: 18},
		{Title: "Location", Width: 14},
		{Title: "Status", Width: 12},
		{Title: "Match", Width: 28},
	}
}

func fitGlobalSearchColumns(width int) []table.Column {
	cols, _, _, _ := VisibleColumnsForWidth(globalSearchColumns(), TableViewportWidth(width), 0)
	if len(cols) == 0 {
		return []table.Column{{Title: "Name", Width: width - 2}}
	}
	return cols
}

func mapVMSearchRows(records []vmSearchRecord, columns []table.Column) []table.Row {
	return mapFindRows(vmRecordsToFindRecords(records), columns)
}

func mapFindRows(records []findRecord, columns []table.Column) []table.Row {
	rows := make([]table.Row, 0, len(records))
	for _, rec := range records {
		row := make([]string, 0, len(columns))
		for _, col := range columns {
			row = append(row, TruncateText(findField(rec, col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	return rows
}

func findField(rec findRecord, col string) string {
	col = NormalizeSortColumnTitle(col)
	switch col {
	case "Type":
		return rec.Kind
	case "Name":
		return rec.Name
	case "ID":
		return rec.ID
	case "Public IP":
		return orFallback(rec.PublicIP, "-")
	case "Provider":
		return rec.Context.Provider
	case "Context":
		return rec.Context.DisplayName()
	case "Region", "Location":
		return findRecordLocation(rec)
	case "Status":
		return rec.Status
	case "Match":
		return rec.Match
	default:
		return "-"
	}
}

func vmSearchField(rec vmSearchRecord, col string) string {
	switch col {
	case "Provider":
		return rec.Context.Provider
	case "Account":
		return rec.Context.DisplayName()
	case "Region", "Location":
		if zone := strings.TrimSpace(rec.VM.Zone); zone != "" && zone != "-" {
			return zone
		}
		return rec.Context.Region
	case "Name":
		return rec.VM.Name
	case "Instance ID":
		return rec.VM.ID
	case "Private IP":
		return rec.VM.PrivateIP
	case "Public IP":
		return rec.VM.PublicIP
	case "State":
		return rec.VM.State
	default:
		return "-"
	}
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

func (a App) renderHelpView() string {
	title := TitleStyle.Render("Help")
	searchWidth := a.fullScreenContentWidth() - 2
	if searchWidth < 20 {
		searchWidth = 20
	}
	search := lipgloss.NewStyle().
		Width(searchWidth).
		MaxWidth(searchWidth).
		Render(a.helpSearchInput.View())
	body := a.helpView.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, search, body)
	mainView := a.renderShellPane(content, true)
	footer := renderFooter(a.width, fmt.Sprintf("↑↓ Scroll • / Filter • Esc Clear/Close • F1/? Toggle • :help | Mode:%s | %s", a.backendMode(), a.statusMsg))
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

func (a App) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.helpSearchInput.Focused() {
		switch msg.String() {
		case "esc":
			a.helpSearchInput.SetValue("")
			a.helpSearchInput.Blur()
			a.refreshHelpContent()
			a.helpView.GotoTop()
			a.statusMsg = "Help filter cleared."
			return a, nil
		case "enter":
			a.helpSearchInput.Blur()
			return a, nil
		}
		var cmd tea.Cmd
		a.helpSearchInput, cmd = a.helpSearchInput.Update(msg)
		a.refreshHelpContent()
		a.helpView.GotoTop()
		return a, cmd
	}
	switch msg.String() {
	case "esc", "q", "?", "f1":
		if msg.String() == "esc" && strings.TrimSpace(a.helpSearchInput.Value()) != "" {
			a.helpSearchInput.SetValue("")
			a.refreshHelpContent()
			a.helpView.GotoTop()
			a.statusMsg = "Help filter cleared."
			return a, nil
		}
		a.showHelp = false
		a.statusMsg = "Closed help."
		return a, nil
	case "/":
		a.helpSearchInput.Focus()
		return a, textinput.Blink
	}
	var cmd tea.Cmd
	a.helpView, cmd = a.helpView.Update(msg)
	return a, cmd
}

func (a *App) refreshHelpContent() {
	a.helpView.SetContent(a.helpContent())
}

type helpSection struct {
	title string
	lines []string
}

func (a App) helpContent() string {
	sections := []helpSection{}
	if len(a.viewStack) > 0 {
		topView := a.viewStack[len(a.viewStack)-1]
		if help := strings.TrimSpace(topView.ShortHelp()); help != "" {
			sections = append(sections, helpSection{title: "Current View", lines: []string{topView.Title() + ": " + help}})
		}
	}
	sections = append(sections,
		helpSection{title: "Everywhere", lines: []string{
			"?: Help",
			"F1: Help",
			"1-9: Resource tabs",
			"↑/↓ or k/j: Move",
			"Enter: Select or open action menu",
			"Esc: Back or close panel",
			"q: Quit when at the top level",
			"Tab: Switch sidebar/main focus",
			"b: Toggle sidebar",
			"H: Home dashboard",
			"K: Toggle Kubernetes worker nodes",
			"g: Find resources",
			",: Settings",
			"L: Logs",
			":logs: Open application logs",
			"B: Toggle CLI/SDK backend",
			":: Command bar",
		}},
		helpSection{title: "Help", lines: []string{
			"/: Filter shortcuts",
			"Enter: Keep filter and return to scrolling",
			"Esc: Clear filter, then close help",
		}},
		helpSection{title: "Resource Lists", lines: []string{
			"/: Filter current list",
			"r: Refresh current view",
			"←/→: Pan wide tables",
			"↑ at first row: Focus column headers",
			"←/→ on headers: Choose column",
			"Enter on header: Sort or reverse sort",
			"C: Configure visible columns",
			"t: CloudManager tag, where supported",
		}},
		helpSection{title: "VMs", lines: []string{
			"s: SSH/access picker",
			"d: Describe",
			"c: Cost/details copy when detail pane is open",
			"f: FinOps actions",
		}},
		helpSection{title: "Firewalls", lines: []string{
			"Enter: Open firewall actions",
			"a: Add rule in rule view",
			"i: Add current public IP in rule view",
			"e: Edit rule in rule view",
			"ctrl+d: Delete rule in rule view",
		}},
		helpSection{title: "Profiles And Contexts", lines: []string{
			":login: Provider CLI login",
			":discover: Scan local cloud CLIs for contexts",
			":creds: Manage profiles/contexts",
			":add-provider: Add a managed provider context",
			":add-host: Add a manual SSH/RDP host",
			":ctx <name>: Switch context",
		}},
		helpSection{title: "Find And Index", lines: []string{
			":find-vms, :find-dbs, :find-k8s, :find-storage, :find-hosts, :find-all",
			":index: Refresh VM index",
			":index-db: Refresh database index",
			":index-storage: Refresh storage index",
			":index-all: Refresh all supported resource indexes",
			":summary: Refresh dashboard summaries",
			":export-public-endpoints [path]: Export known indexed public endpoints",
		}},
		helpSection{title: "Tab Map", lines: []string{
			"1 VMs, 2 Disks, 3 Snapshots, 4 Firewalls, 5 Clusters",
			"6 Databases, 7 Networks, 8 Storage, 9 Hosts",
		}},
	)

	var b strings.Builder
	query := strings.ToLower(strings.TrimSpace(a.helpSearchInput.Value()))
	matches := 0
	for _, section := range sections {
		filtered := filteredHelpLines(section, query)
		if len(filtered) == 0 {
			continue
		}
		matches += len(filtered)
		writeHelpSection(&b, section.title, filtered)
	}
	if b.Len() == 0 {
		b.WriteString(fmt.Sprintf("No shortcuts match %q.", a.helpSearchInput.Value()))
	} else if query != "" {
		b.WriteString(fmt.Sprintf("\n\n%d match(es) for %q.", matches, a.helpSearchInput.Value()))
	}
	return lipgloss.NewStyle().Width(a.helpView.Width).Render(strings.TrimSpace(b.String()))
}

func filteredHelpLines(section helpSection, query string) []string {
	if query == "" {
		return section.lines
	}
	if strings.Contains(strings.ToLower(section.title), query) {
		return section.lines
	}
	var lines []string
	for _, line := range section.lines {
		if strings.Contains(strings.ToLower(line), query) {
			lines = append(lines, line)
		}
	}
	return lines
}

func writeHelpSection(b *strings.Builder, title string, lines []string) {
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(title)
	b.WriteString("\n")
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
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
		return a, fetchContextsCmd(false)
	}
	var cmd tea.Cmd
	a.configList, cmd = a.configList.Update(msg)
	return a, cmd
}

// --- Commands ---

func (a *App) startStartupVMPrefetch() []tea.Cmd {
	if !a.cfg.PrefetchOnStart || !config.PrefetchesResource(a.cfg, "vms") {
		return nil
	}
	if a.cfg.VMIndexPersistence && !a.vmIndexCacheChecked {
		a.statusMsg = "Loaded contexts. Checking VM index cache..."
		return nil
	}
	if a.cfg.VMIndexPersistence && len(a.vmIndex) > 0 {
		a.statusMsg = fmt.Sprintf("Using %d cached VM index records. Use :index to refresh.", len(a.vmIndex))
		logging.Infof("component=ui event=vm_prefetch_skipped reason=warm_cache count=%d", len(a.vmIndex))
		return nil
	}
	return a.startVMPrefetch()
}

func (a *App) startVMPrefetch() []tea.Cmd {
	if !a.cfg.PrefetchOnStart || !config.PrefetchesResource(a.cfg, "vms") {
		return nil
	}
	return a.startVMIndexRefresh()
}

func (a *App) startVMIndexRefresh() []tea.Cmd {
	var queue []core.CloudContext
	for _, ctx := range a.allContexts {
		if providers.Supports(ctx.Provider, providers.CapabilityVMs) {
			queue = append(queue, ctx)
		}
	}
	a.vmPrefetchQueue = queue
	a.vmPrefetchTotal = len(queue)
	a.vmPrefetchDone = 0
	a.vmPrefetchRunning = 0
	if len(queue) == 0 {
		return nil
	}
	a.statusMsg = fmt.Sprintf("Indexing VMs: 0/%d contexts", len(queue))
	logging.Infof("component=ui event=vm_prefetch_start contexts=%d concurrency=%d", len(queue), prefetchConcurrency(a.cfg))
	return a.continueVMPrefetch()
}

func (a *App) startDashboardSummaryRefresh() []tea.Cmd {
	cfg := a.cfg
	var cmds []tea.Cmd
	for _, ctx := range a.allContexts {
		if providers.Supports(ctx.Provider, providers.CapabilityClusters) {
			cmds = append(cmds, fetchClusterSummaryCmd(cfg, ctx))
		}
		if config.PrefetchesResource(cfg, "databases") && providers.Supports(ctx.Provider, providers.CapabilityDatabases) {
			cmds = append(cmds, fetchDatabaseSummaryCmd(cfg, ctx))
		}
		if config.PrefetchesResource(cfg, "storage") && providers.Supports(ctx.Provider, providers.CapabilityStorage) {
			cmds = append(cmds, fetchStorageSummaryCmd(cfg, ctx))
		}
	}
	return cmds
}

func (a *App) startAllIndexRefresh() []tea.Cmd {
	cmds := a.startVMIndexRefresh()
	cfg := a.cfg
	for _, ctx := range a.allContexts {
		if providers.Supports(ctx.Provider, providers.CapabilityClusters) {
			cmds = append(cmds, fetchClusterSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilityDatabases) {
			cmds = append(cmds, fetchDatabaseSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilityDisks) {
			cmds = append(cmds, fetchDiskSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilitySnapshots) {
			cmds = append(cmds, fetchSnapshotSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilityNetworks) {
			cmds = append(cmds, fetchNetworkSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilityFirewalls) {
			cmds = append(cmds, fetchFirewallSummaryCmd(cfg, ctx))
		}
		if providers.Supports(ctx.Provider, providers.CapabilityStorage) {
			cmds = append(cmds, fetchStorageSummaryCmd(cfg, ctx))
		}
	}
	return cmds
}

func (a *App) startDatabaseIndexRefresh() []tea.Cmd {
	cfg := a.cfg
	var cmds []tea.Cmd
	for _, ctx := range a.allContexts {
		if providers.Supports(ctx.Provider, providers.CapabilityDatabases) {
			cmds = append(cmds, fetchDatabaseSummaryCmd(cfg, ctx))
		}
	}
	return cmds
}

func (a *App) startStorageIndexRefresh() []tea.Cmd {
	cfg := a.cfg
	var cmds []tea.Cmd
	for _, ctx := range a.allContexts {
		if providers.Supports(ctx.Provider, providers.CapabilityStorage) {
			cmds = append(cmds, fetchStorageSummaryCmd(cfg, ctx))
		}
	}
	return cmds
}

func (a *App) continueVMPrefetch() []tea.Cmd {
	limit := prefetchConcurrency(a.cfg)
	var cmds []tea.Cmd
	for a.vmPrefetchRunning < limit && len(a.vmPrefetchQueue) > 0 {
		ctx := a.vmPrefetchQueue[0]
		a.vmPrefetchQueue = a.vmPrefetchQueue[1:]
		a.vmPrefetchRunning++
		cmds = append(cmds, fetchVMIndexCmd(a.cfg, ctx))
	}
	return cmds
}

func prefetchConcurrency(cfg config.AppConfig) int {
	if cfg.PrefetchConcurrency <= 0 {
		return 4
	}
	if cfg.PrefetchConcurrency > 16 {
		return 16
	}
	return cfg.PrefetchConcurrency
}

func fetchVMIndexCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		rows, err := provider.FetchVMs(context.Background(), cloudCtx)
		return vmPrefetchMsg{ctx: cloudCtx, vms: rows, err: err}
	}
}

func fetchClusterSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		clusterProvider, ok := provider.(providers.ClusterProvider)
		if !ok {
			return ClusterIndexUpdateMsg{Ctx: cloudCtx, Err: fmt.Errorf("clusters not supported by provider backend")}
		}
		rows, err := clusterProvider.FetchClusters(context.Background(), cloudCtx)
		return ClusterIndexUpdateMsg{Ctx: cloudCtx, Clusters: rows, Err: err}
	}
}

func fetchDatabaseSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		dbProvider, ok := provider.(providers.DatabaseProvider)
		if !ok {
			return DatabaseIndexUpdateMsg{Ctx: cloudCtx, Err: fmt.Errorf("databases not supported by provider backend")}
		}
		rows, err := dbProvider.FetchDatabases(context.Background(), cloudCtx)
		return DatabaseIndexUpdateMsg{Ctx: cloudCtx, Databases: rows, Err: err}
	}
}

func fetchDiskSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		diskProvider, ok := provider.(providers.DiskProvider)
		if !ok {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "disks", Err: fmt.Errorf("disks not supported by provider backend")}
		}
		rows, err := diskProvider.FetchDisks(context.Background(), cloudCtx)
		return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "disks", Count: len(rows), Disks: rows, Err: err}
	}
}

func fetchSnapshotSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		snapshotProvider, ok := provider.(providers.SnapshotProvider)
		if !ok {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "snapshots", Err: fmt.Errorf("snapshots not supported by provider backend")}
		}
		rows, err := snapshotProvider.FetchSnapshots(context.Background(), cloudCtx)
		return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "snapshots", Count: len(rows), Snapshots: rows, Err: err}
	}
}

func fetchNetworkSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		networkProvider, ok := provider.(providers.NetworkProvider)
		if !ok {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "networks", Err: fmt.Errorf("networks not supported by provider backend")}
		}
		rows, err := networkProvider.FetchNetworks(context.Background(), cloudCtx)
		if err != nil {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "networks", Err: err}
		}
		subnetRows, subnetErr := networkProvider.FetchSubnets(context.Background(), cloudCtx)
		if subnetErr != nil {
			logging.Warnf("component=ui event=subnet_summary_failed provider=%s context=%s err=%v", cloudCtx.Provider, cloudCtx.DisplayName(), subnetErr)
		}
		subnets := 0
		for _, row := range rows {
			subnets += row.SubnetCount
		}
		if len(subnetRows) > 0 {
			subnets = len(subnetRows)
		}
		return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "networks", Count: len(rows), Extra: subnets, Networks: rows, Subnets: subnetRows}
	}
}

func fetchFirewallSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		firewallProvider, ok := provider.(providers.FirewallProvider)
		if !ok {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "firewalls", Err: fmt.Errorf("firewalls not supported by provider backend")}
		}
		rows, err := firewallProvider.FetchSecurityGroups(context.Background(), cloudCtx)
		return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "firewalls", Count: len(rows), SecurityGroups: rows, Err: err}
	}
}

func fetchStorageSummaryCmd(cfg config.AppConfig, cloudCtx core.CloudContext) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		storageProvider, ok := provider.(providers.StorageProvider)
		if !ok {
			return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "storage", Err: fmt.Errorf("storage not supported by provider backend")}
		}
		rows, err := storageProvider.FetchStorageBuckets(context.Background(), cloudCtx)
		return ResourceSummaryUpdateMsg{Ctx: cloudCtx, Resource: "storage", Count: len(rows), StorageBuckets: rows, Err: err}
	}
}

func loadVMIndexCacheCmd(cfg config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		if !cfg.VMIndexPersistence {
			return vmIndexCacheLoadMsg{}
		}
		ttl := time.Duration(cfg.VMIndexCacheTTL) * time.Hour
		index, total, err := loadVMIndexCache(ttl)
		return vmIndexCacheLoadMsg{index: index, total: total, err: err}
	}
}

func loadResourceIndexCacheCmd(cfg config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		if !cfg.ResourceIndexPersistence {
			return resourceIndexCacheLoadMsg{}
		}
		ttl := time.Duration(cfg.ResourceIndexCacheTTL) * time.Hour
		cache, err := loadResourceIndexCache(ttl)
		return resourceIndexCacheLoadMsg{cache: cache, err: err}
	}
}

func fetchAzureSubscriptionsCmd() tea.Cmd {
	return func() tea.Msg {
		contexts, warnings := providers.DiscoverContexts("Azure")
		return azureSubscriptionLoadMsg{contexts: contexts, warnings: warnings}
	}
}

func fetchDiscoveredContextsCmd() tea.Cmd {
	return func() tea.Msg {
		var contexts []core.CloudContext
		var warnings []string
		for _, registered := range providers.RegisteredProviders() {
			providerName := registered.Metadata.DisplayName
			ctxs, providerWarnings := providers.DiscoverContexts(providerName)
			contexts = append(contexts, ctxs...)
			warnings = append(warnings, providerWarnings...)
		}
		return contextDiscoveryLoadMsg{contexts: contexts, warnings: warnings}
	}
}

func fetchContextsCmd(discover bool) tea.Cmd {
	return func() tea.Msg {
		logging.Infof("component=ui event=contexts_load_start")
		cfg := config.Load()
		var allCtx []core.CloudContext
		var discovered []core.CloudContext
		var warnings []string

		allCtx = append(allCtx, config.ManagedContexts(cfg)...)

		if discover {
			for _, registered := range providers.RegisteredProviders() {
				providerName := registered.Metadata.DisplayName
				if config.HasManagedContextsForProvider(cfg, providerName) {
					continue
				}
				ctxs, providerWarnings := providers.DiscoverContexts(providerName)
				discovered = append(discovered, ctxs...)
				warnings = append(warnings, providerWarnings...)
			}
		} else if len(allCtx) == 0 {
			warnings = append(warnings, "Context discovery skipped for fast startup. Run :discover to scan local cloud CLIs.")
		}

		if discover {
			if len(allCtx) == 0 {
				allCtx = append(allCtx, discovered...)
			} else if len(discovered) > 0 {
				warnings = append(warnings, "Discovered provider contexts were not imported automatically. Run :discover to choose what to onboard.")
			}
		}

		// Restore mock fallback if no contexts found
		if discover && len(allCtx) == 0 {
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
		return contextLoadMsg{tree: tree, contexts: allCtx, warnings: warnings}
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
