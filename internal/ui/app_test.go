package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
)

type mockView struct {
	rendered         string
	title            string
	lastResizeWidth  int
	lastResizeHeight int
	lastShowSidebar  bool
	sawWindowMsg     bool
	searchQuery      string
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

func (m *mockView) Render() string { return m.rendered }
func (m *mockView) Title() string {
	if m.title != "" {
		return m.title
	}
	return "Mock"
}
func (m *mockView) ShortHelp() string { return "" }
func (m *mockView) Resize(width, height int, showSidebar bool) {
	m.lastResizeWidth = width
	m.lastResizeHeight = height
	m.lastShowSidebar = showSidebar
}
func (m *mockView) IsInputActive() bool { return false }
func (m *mockView) SetSearchQuery(query string) {
	m.searchQuery = query
}

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
	if !strings.Contains(rendered, "CPU:") || !strings.Contains(rendered, "Mem:") {
		t.Fatalf("expected process usage in footer, got:\n%s", rendered)
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

func TestGlobalVMSearchMatchesIdentityAndIPs(t *testing.T) {
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app.indexVMs(ctx, []core.VM{
		{Name: "api-1", ID: "i-aaa", PrivateIP: "10.0.1.5", PublicIP: "203.0.113.5", State: "running"},
		{Name: "worker", ID: "i-bbb", PrivateIP: "10.0.2.9", PublicIP: "-", State: "stopped"},
	})

	app.globalSearchInput.SetValue("203.0.113.5")
	app.refreshGlobalSearchResults()
	if len(app.vmSearchRows) != 1 || app.vmSearchRows[0].VM.ID != "i-aaa" {
		t.Fatalf("expected public IP search to find i-aaa, got %+v", app.vmSearchRows)
	}

	app.globalSearchInput.SetValue("i-bbb")
	app.refreshGlobalSearchResults()
	if len(app.vmSearchRows) != 1 || app.vmSearchRows[0].VM.Name != "worker" {
		t.Fatalf("expected instance ID search to find worker, got %+v", app.vmSearchRows)
	}
}

func TestGlobalVMSearchMatchesCloudManagerTags(t *testing.T) {
	ctx := core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "us-central1"}
	app := NewApp(config.AppConfig{
		GlobalSearch: true,
		ResourceTags: []config.ResourceTag{
			{Provider: "GCP", AccountID: "project-a", ResourceID: "gce-1", Tags: []string{"VFWEB"}},
		},
	}, "1.0.0", "today")
	app.indexVMs(ctx, []core.VM{{Name: "web", ID: "gce-1"}})

	app.globalSearchInput.SetValue("VFWEB")
	app.refreshGlobalSearchResults()

	if len(app.vmSearchRows) != 1 || app.vmSearchRows[0].VM.ID != "gce-1" {
		t.Fatalf("expected CloudManager tag search to find gce-1, got %+v", app.vmSearchRows)
	}
	if !strings.Contains(app.vmSearchRows[0].VM.Labels, "cm:VFWEB") {
		t.Fatalf("expected indexed VM labels to include CloudManager tag, got %q", app.vmSearchRows[0].VM.Labels)
	}
}

func TestFindResourcesMatchesCloudManagerTagsForNonVMResources(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app := NewApp(config.AppConfig{
		ResourceTags: []config.ResourceTag{
			{Provider: "AWS", AccountID: "1234", ResourceKind: "Disk", ResourceID: "vol-1", Tags: []string{"VFWEB"}},
			{Provider: "AWS", AccountID: "1234", ResourceKind: "Storage", ResourceID: "bucket-1", Tags: []string{"ARCHIVE"}},
		},
	}, "1.0.0", "today")

	app.indexDisks(ctx, []core.Disk{{Name: "data", ID: "vol-1"}})
	app.indexStorage(ctx, []core.StorageBucket{{Name: "logs", ID: "bucket-1"}})

	diskRows := app.findRecords(findScopeDisks, "VFWEB")
	if len(diskRows) != 1 || diskRows[0].ID != "vol-1" {
		t.Fatalf("expected disk tag search to find vol-1, got %+v", diskRows)
	}
	storageRows := app.findRecords(findScopeStorage, "ARCHIVE")
	if len(storageRows) != 1 || storageRows[0].ID != "bucket-1" {
		t.Fatalf("expected storage tag search to find bucket-1, got %+v", storageRows)
	}
}

func TestFindScopePickerOpensWithG(t *testing.T) {
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.showSplash = false

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	updated := model.(App)

	if !updated.showFindPicker {
		t.Fatal("expected g to open find scope picker")
	}
}

func TestFindOpensWithTableFocused(t *testing.T) {
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false

	updated, _ := app.openFind(findScopeVMs, "")

	if updated.globalSearchInput.Focused() {
		t.Fatal("expected find input to start blurred")
	}
	if !updated.globalSearchTable.Focused() {
		t.Fatal("expected find table to start focused")
	}
}

func TestFindSlashFocusesFilterInput(t *testing.T) {
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app, _ = app.openFind(findScopeVMs, "")

	model, _ := app.handleGlobalSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated := model.(App)

	if !updated.globalSearchInput.Focused() {
		t.Fatal("expected / to focus find input")
	}
	if updated.globalSearchTable.Focused() {
		t.Fatal("expected table to blur while editing find input")
	}
}

func TestFindDatabasesUsesDatabaseIndex(t *testing.T) {
	dbView := &mockView{title: "Databases", rendered: "dbs"}
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.RegisterView("6", providers.CapabilityDatabases, dbView)
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexDatabases(ctx, []core.Database{
		{Name: "orders", ID: "db-orders", Engine: "postgres", Status: "available"},
		{Name: "archive", ID: "db-archive", Engine: "mysql", Status: "stopped"},
	})

	updated, _ := app.openFind(findScopeDatabases, "orders")

	if updated.findScope != findScopeDatabases || len(updated.findRows) != 1 || updated.findRows[0].ID != "db-orders" {
		t.Fatalf("expected one database find result, scope=%s rows=%+v", updated.findScope, updated.findRows)
	}
	updated, _ = updated.openSelectedFindResult()
	if updated.activeTab != "6" {
		t.Fatalf("expected databases tab, got %q", updated.activeTab)
	}
	if dbView.searchQuery != "db-orders" {
		t.Fatalf("expected database view filter, got %q", dbView.searchQuery)
	}
}

func TestFindK8sUsesClusterIndex(t *testing.T) {
	clusterView := &mockView{title: "Clusters", rendered: "clusters"}
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.RegisterView("5", providers.CapabilityClusters, clusterView)
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexClusters(ctx, []core.Cluster{
		{Name: "eks-prod", ID: "cluster-prod", Status: "ACTIVE", NodeCount: "3"},
		{Name: "eks-stage", ID: "cluster-stage", Status: "ACTIVE", NodeCount: "1"},
	})

	updated, _ := app.openFind(findScopeK8s, "prod")

	if updated.findScope != findScopeK8s || len(updated.findRows) != 1 || updated.findRows[0].ID != "cluster-prod" {
		t.Fatalf("expected one k8s find result, scope=%s rows=%+v", updated.findScope, updated.findRows)
	}
	updated, _ = updated.openSelectedFindResult()
	if updated.activeTab != "5" {
		t.Fatalf("expected clusters tab, got %q", updated.activeTab)
	}
	if clusterView.searchQuery != "cluster-prod" {
		t.Fatalf("expected cluster view filter, got %q", clusterView.searchQuery)
	}
}

func TestResizeGlobalSearchWithExistingRowsDoesNotPanic(t *testing.T) {
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 90
	app.height = 24
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app.vmSearchRows = []vmSearchRecord{{Context: ctx, VM: core.VM{Name: "api-1", ID: "i-aaa", PrivateIP: "10.0.0.5", PublicIP: "203.0.113.10", State: "running"}}}
	app.globalSearchTable.SetRows(mapVMSearchRows(app.vmSearchRows, app.globalSearchTable.Columns()))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resizeGlobalSearch panicked with existing rows: %v", r)
		}
	}()

	app.resizeGlobalSearch()
}

func TestOpenSelectedGlobalVMInitializesVMTabWithFilter(t *testing.T) {
	mv := &mockView{title: "VMs", rendered: "vms"}
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.showSidebar = false
	app.RegisterView("1", providers.CapabilityVMs, mv)
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app.vmSearchRows = []vmSearchRecord{{Context: ctx, VM: core.VM{Name: "api-1", ID: "i-aaa"}}}
	app.globalSearchTable.SetRows(mapVMSearchRows(app.vmSearchRows, app.globalSearchTable.Columns()))

	updated, _ := app.openSelectedGlobalVM()
	if updated.activeCtx.CacheKey() != ctx.CacheKey() {
		t.Fatalf("expected active context to change to search result, got %+v", updated.activeCtx)
	}
	if mv.searchQuery != "i-aaa" {
		t.Fatalf("expected VM view search filter to be instance ID, got %q", mv.searchQuery)
	}
}

func TestOpenSelectedGlobalVMForcesVMRootView(t *testing.T) {
	vmRoot := &mockView{title: "VMs", rendered: "vms"}
	drilldown := &mockView{title: "Firewalls", rendered: "firewalls"}
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.showSidebar = false
	app.RegisterView("1", providers.CapabilityVMs, vmRoot)
	app.viewStack = []View{vmRoot, drilldown}
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app.vmSearchRows = []vmSearchRecord{{Context: ctx, VM: core.VM{Name: "api-1", ID: "i-aaa"}}}
	app.globalSearchTable.SetRows(mapVMSearchRows(app.vmSearchRows, app.globalSearchTable.Columns()))

	updated, _ := app.openSelectedGlobalVM()

	if len(updated.viewStack) != 1 || updated.viewStack[0].Title() != "VMs" {
		t.Fatalf("expected global search to land on VM root view, got stack=%d title=%q", len(updated.viewStack), updated.viewStack[len(updated.viewStack)-1].Title())
	}
	if vmRoot.searchQuery != "i-aaa" {
		t.Fatalf("expected VM root search filter to be instance ID, got %q", vmRoot.searchQuery)
	}
	if drilldown.searchQuery != "" {
		t.Fatalf("expected drilldown view not to receive search query, got %q", drilldown.searchQuery)
	}
}

func TestSettingsToggleEnablesVMStartupIndexing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(config.AppConfig{
		GlobalSearch:        true,
		PrefetchResources:   []string{"vms"},
		PrefetchConcurrency: 4,
	}, "1.0.0", "today")
	app.openSettings()
	for i, item := range app.settingsList.Items() {
		if item.(settingsItem).key == "vm_indexing" {
			app.settingsList.Select(i)
			break
		}
	}

	model, _ := app.handleSettingsKeys(tea.KeyMsg{Type: tea.KeySpace})
	updated := model.(App)

	if !updated.cfg.PrefetchOnStart {
		t.Fatal("expected settings toggle to enable startup VM indexing")
	}
	if !config.PrefetchesResource(updated.cfg, "vms") {
		t.Fatalf("expected VM resource to remain enabled, got %#v", updated.cfg.PrefetchResources)
	}
	if !strings.Contains(updated.statusMsg, "enabled") {
		t.Fatalf("expected enabled status, got %q", updated.statusMsg)
	}
}

func TestSettingsToggleEnablesContextDiscoveryOnStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.openSettings()
	for i, item := range app.settingsList.Items() {
		if item.(settingsItem).key == "discover_on_start" {
			app.settingsList.Select(i)
			break
		}
	}

	model, _ := app.handleSettingsKeys(tea.KeyMsg{Type: tea.KeySpace})
	updated := model.(App)

	if !updated.cfg.DiscoverOnStart {
		t.Fatal("expected settings toggle to enable startup context discovery")
	}
	if !strings.Contains(updated.statusMsg, "enabled") {
		t.Fatalf("expected enabled status, got %q", updated.statusMsg)
	}
}

func TestLoginCommandOpensProviderLoginPicker(t *testing.T) {
	app := NewApp(config.AppConfig{}, "1.0.0", "today")

	updated, _ := app.handleCommand("login")

	if !updated.showProviderLogin {
		t.Fatal("expected :login to open provider login picker")
	}
	if len(updated.loginList.Items()) == 0 {
		t.Fatal("expected provider login options")
	}
}

func TestAddHostCommandOpensManualHostForm(t *testing.T) {
	app := NewApp(config.AppConfig{}, "1.0.0", "today")

	updated, _ := app.handleCommand("add-host")

	if !updated.showHostForm {
		t.Fatal("expected :add-host to open manual host form")
	}
	if len(updated.hostInputs) == 0 {
		t.Fatal("expected manual host inputs")
	}
}

func TestSaveManualHostPersistsConfigAndIndexesHost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(config.AppConfig{GlobalSearch: true, VMIndexPersistence: false}, "1.0.0", "today")
	app.openHostForm()
	app.hostInputs[0].SetValue("hetzner-web")
	app.hostInputs[1].SetValue("203.0.113.10")
	app.hostInputs[2].SetValue("root")
	app.hostInputs[5].SetValue("~/.ssh/hetzner")
	app.hostInputs[6].SetValue("prod,hetzner")

	updated, _ := app.saveManualHost()

	if len(updated.cfg.ManualHosts) != 1 {
		t.Fatalf("expected one saved manual host, got %+v", updated.cfg.ManualHosts)
	}
	if updated.showHostForm {
		t.Fatal("expected manual host form to close after save")
	}
	found := false
	for _, rec := range updated.vmIndex {
		if rec.Context.Provider == "Manual" && rec.VM.Name == "hetzner-web" && rec.VM.PublicIP == "203.0.113.10" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected manual host in VM search index, got %+v", updated.vmIndex)
	}
}

func TestIndexManualHostsPrunesRemovedHosts(t *testing.T) {
	app := NewApp(config.AppConfig{
		VMIndexPersistence: false,
		ManualHosts: []config.ManualHost{
			{Name: "old", Host: "203.0.113.10", Username: "root"},
		},
	}, "1.0.0", "today")
	app.indexManualHosts()
	if len(app.vmIndex) != 1 {
		t.Fatalf("expected one indexed manual host, got %+v", app.vmIndex)
	}

	app.cfg.ManualHosts = nil
	app.indexManualHosts()
	if len(app.vmIndex) != 0 {
		t.Fatalf("expected removed manual host to be pruned, got %+v", app.vmIndex)
	}
}

func TestManualHostFormSavesOnAlternateEnterKeys(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "enter", key: tea.KeyMsg{Type: tea.KeyEnter}},
		{name: "ctrl-j", key: tea.KeyMsg{Type: tea.KeyCtrlJ}},
		{name: "ctrl-m", key: tea.KeyMsg{Type: tea.KeyCtrlM}},
		{name: "f2", key: tea.KeyMsg{Type: tea.KeyF2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			app := NewApp(config.AppConfig{GlobalSearch: true, VMIndexPersistence: false}, "1.0.0", "today")
			app.openHostForm()
			app.hostInputs[0].SetValue("web")
			app.hostInputs[1].SetValue("203.0.113.10")
			app.hostInputs[2].SetValue("ubuntu")

			model, _ := app.handleHostFormKeys(tc.key)
			updated := model.(App)

			if len(updated.cfg.ManualHosts) != 1 {
				t.Fatalf("expected host to save via %s, got %+v", tc.name, updated.cfg.ManualHosts)
			}
			if updated.showHostForm {
				t.Fatalf("expected form to close after %s", tc.name)
			}
		})
	}
}

func TestOpenSelectedGlobalManualHostInitializesHostsTabWithFilter(t *testing.T) {
	hostsView := &mockView{title: "Hosts", rendered: "hosts"}
	app := NewApp(config.AppConfig{GlobalSearch: true}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.showSidebar = false
	app.RegisterView("8", providers.CapabilityHosts, hostsView)
	ctx := core.CloudContext{Provider: "Manual", AccountID: "manual-hosts", AccountName: "Manual Hosts", Region: "global"}
	app.vmSearchRows = []vmSearchRecord{{Context: ctx, VM: core.VM{Name: "hetzner-web", ID: "hetzner-web", PublicIP: "203.0.113.10"}}}
	app.globalSearchTable.SetRows(mapVMSearchRows(app.vmSearchRows, app.globalSearchTable.Columns()))

	updated, _ := app.openSelectedGlobalVM()

	if updated.activeTab != "8" {
		t.Fatalf("expected Hosts tab to be active, got %q", updated.activeTab)
	}
	if hostsView.searchQuery != "hetzner-web" {
		t.Fatalf("expected Hosts view search filter to be manual host ID, got %q", hostsView.searchQuery)
	}
}

func TestProviderLoginItemsIncludeAzureDeviceCode(t *testing.T) {
	items := buildProviderLoginItems()
	for _, raw := range items {
		item := raw.(providerLoginItem)
		if item.provider == "Azure" {
			got := strings.Join(item.command, " ")
			if got != "az login --use-device-code" {
				t.Fatalf("expected Azure device-code login, got %q", got)
			}
			return
		}
	}
	t.Fatal("expected Azure login item")
}

func TestProviderLoginItemsUseNoBrowserForGCloudUserLogin(t *testing.T) {
	items := buildProviderLoginItems()
	for _, raw := range items {
		item := raw.(providerLoginItem)
		if item.title == "GCP: gcloud auth login --no-browser" {
			got := strings.Join(item.command, " ")
			if got != "gcloud auth login --no-browser" {
				t.Fatalf("expected gcloud no-browser login, got %q", got)
			}
			return
		}
	}
	t.Fatal("expected gcloud no-browser login item")
}

func TestVMIndexCacheRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	index := map[string]vmSearchRecord{
		"one": {Context: ctx, VM: core.VM{Name: "api", ID: "i-123", PrivateIP: "10.0.0.5"}, SeenAt: time.Now()},
	}
	if err := saveVMIndexCache(index); err != nil {
		t.Fatalf("save index cache: %v", err)
	}
	loaded, total, err := loadVMIndexCache(24 * time.Hour)
	if err != nil {
		t.Fatalf("load index cache: %v", err)
	}
	if total != 1 || len(loaded) != 1 {
		t.Fatalf("expected one cached record, total=%d len=%d", total, len(loaded))
	}
	found := false
	for _, rec := range loaded {
		if rec.VM.ID == "i-123" && rec.Context.AccountName == "prod" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cached VM record, got %+v", loaded)
	}
}

func TestResourceIndexCacheRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app := NewApp(config.AppConfig{
		ResourceIndexPersistence: true,
		ResourceIndexCacheTTL:    24,
	}, "1.0.0", "today")
	now := time.Now()
	app.clusterIndex["cluster"] = clusterIndexRecord{Context: ctx, Cluster: core.Cluster{Name: "eks-prod", ID: "eks-1"}, SeenAt: now}
	app.databaseIndex["db"] = databaseIndexRecord{Context: ctx, Database: core.Database{Name: "orders", ID: "db-1"}, SeenAt: now}
	app.diskIndex["disk"] = diskIndexRecord{Context: ctx, Disk: core.Disk{Name: "data", ID: "vol-1"}, SeenAt: now}
	app.snapshotIndex["snap"] = snapshotIndexRecord{Context: ctx, Snapshot: core.Snapshot{Name: "snap-data", ID: "snap-1"}, SeenAt: now}
	app.networkIndex["net"] = networkIndexRecord{Context: ctx, Network: core.Network{Name: "prod-vpc", ID: "vpc-1"}, SeenAt: now}
	app.subnetIndex["subnet"] = subnetIndexRecord{Context: ctx, Subnet: core.Subnet{Name: "web-a", ID: "subnet-1"}, SeenAt: now}
	app.firewallIndex["sg"] = firewallIndexRecord{Context: ctx, Group: core.SecurityGroup{Name: "web-sg", ID: "sg-1"}, SeenAt: now}
	app.storageIndex["bucket"] = storageIndexRecord{Context: ctx, Bucket: core.StorageBucket{Name: "logs", ID: "bucket-1"}, SeenAt: now}
	app.diskSummaryIndex[ctx.CacheKey()] = resourceSummaryRecord{Count: 1}
	app.networkSummaryIndex[ctx.CacheKey()] = resourceSummaryRecord{Count: 1, Extra: 1}
	app.storageSummaryIndex[ctx.CacheKey()] = resourceSummaryRecord{Count: 1}

	if err := saveResourceIndexCache(app); err != nil {
		t.Fatalf("save resource index cache: %v", err)
	}
	loaded, err := loadResourceIndexCache(24 * time.Hour)
	if err != nil {
		t.Fatalf("load resource index cache: %v", err)
	}
	if loaded.Total != 11 {
		t.Fatalf("expected eleven cached resource/summarized records, got %d", loaded.Total)
	}
	if len(loaded.Databases) != 1 || len(loaded.Disks) != 1 || len(loaded.Subnets) != 1 || len(loaded.Storage) != 1 {
		t.Fatalf("expected DB/disk/subnet/storage cache records, got db=%d disk=%d subnet=%d storage=%d", len(loaded.Databases), len(loaded.Disks), len(loaded.Subnets), len(loaded.Storage))
	}
	if loaded.DiskSummaries[ctx.CacheKey()].Count != 1 || loaded.NetworkSummaries[ctx.CacheKey()].Extra != 1 || loaded.StorageSummaries[ctx.CacheKey()].Count != 1 {
		t.Fatalf("expected cached summaries, got disks=%+v networks=%+v storage=%+v", loaded.DiskSummaries, loaded.NetworkSummaries, loaded.StorageSummaries)
	}
}

func TestResourceIndexCacheLoadWithSummariesOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	app := NewApp(config.AppConfig{
		ResourceIndexPersistence: true,
		ResourceIndexCacheTTL:    24,
	}, "1.0.0", "today")
	app.diskSummaryIndex[ctx.CacheKey()] = resourceSummaryRecord{Count: 7}

	if err := saveResourceIndexCache(app); err != nil {
		t.Fatalf("save resource index cache: %v", err)
	}
	loaded, err := loadResourceIndexCache(24 * time.Hour)
	if err != nil {
		t.Fatalf("load resource index cache: %v", err)
	}
	if loaded.Total != 1 || loaded.DiskSummaries[ctx.CacheKey()].Count != 7 {
		t.Fatalf("expected summary-only cache to load, total=%d summaries=%+v", loaded.Total, loaded.DiskSummaries)
	}
}

func TestHomeDashboardSummarizesVMIndex(t *testing.T) {
	app := NewApp(config.AppConfig{VMIndexCacheTTL: 24}, "1.0.0", "today")
	now := time.Now()
	app.vmIndex = map[string]vmSearchRecord{
		"aws-run": {
			Context: core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"},
			VM:      core.VM{Name: "api", ID: "i-1", State: "running", PublicIP: "203.0.113.10"},
			SeenAt:  now,
		},
		"aws-stop": {
			Context: core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"},
			VM:      core.VM{Name: "worker", ID: "i-2", State: "stopped", PublicIP: "-"},
			SeenAt:  now,
		},
		"gcp-run": {
			Context: core.CloudContext{Provider: "GCP", AccountID: "project", AccountName: "project", Region: "us-central1"},
			VM:      core.VM{Name: "batch", ID: "g-1", State: "RUNNING", PublicIP: ""},
			SeenAt:  now.Add(-2 * time.Hour),
		},
	}

	stats := app.vmDashboardStats()
	if stats.Total != 3 || stats.Running != 2 || stats.Stopped != 1 || stats.WithPublicIP != 1 {
		t.Fatalf("unexpected dashboard stats: %+v", stats)
	}
	if stats.IndexedContexts != 2 {
		t.Fatalf("expected two indexed contexts, got %d", stats.IndexedContexts)
	}
	rendered := app.renderHomePanel(120, 30)
	for _, expected := range []string{"Indexed VMs", "Running", "Stopped", "Public IPs", "VMs by provider: AWS 2 | GCP 1"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected dashboard to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestDashboardSummarizesDatabasesAndKubernetes(t *testing.T) {
	app := NewApp(config.AppConfig{VMIndexCacheTTL: 24}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexDatabases(ctx, []core.Database{
		{Name: "orders", ID: "db-1", Status: "available"},
		{Name: "archive", ID: "db-2", Status: "stopped"},
		{Name: "unknown", ID: "db-3", Status: "maintenance"},
	})
	app.indexClusters(ctx, []core.Cluster{{Name: "prod", ID: "eks-prod", Status: "ACTIVE", NodeCount: "2"}})
	app.indexVMs(ctx, []core.VM{
		{Name: "ip-10-0-0-1", ID: "i-1", Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot"},
		{Name: "ip-10-0-0-2", ID: "i-2", Labels: "eks:cluster-name=prod, eks:nodegroup-name=static"},
	})

	dbStats := app.databaseDashboardStats()
	if dbStats.Total != 3 || dbStats.Running != 1 || dbStats.Stopped != 1 || dbStats.Other != 1 {
		t.Fatalf("unexpected database stats: %+v", dbStats)
	}
	k8sStats := app.kubernetesDashboardStats()
	if k8sStats.Clusters != 1 || k8sStats.Pools != 2 || k8sStats.Nodes != 2 {
		t.Fatalf("unexpected k8s stats: %+v", k8sStats)
	}
	rendered := app.renderHomePanel(120, 35)
	for _, expected := range []string{"DBs 1 up / 1 down", "1:2:2", "clusters:pools:nodes"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected dashboard to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestDashboardSummarizesInfrastructureSignals(t *testing.T) {
	app := NewApp(config.AppConfig{
		VMIndexCacheTTL: 24,
		DashboardWidgets: []string{
			"disks",
			"snapshots",
			"networks",
			"subnets",
			"firewalls",
			"terraform",
			"manual_hosts",
		},
	}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexVMs(ctx, []core.VM{
		{Name: "api", ID: "i-1", Network: "vpc-1", Subnet: "subnet-a", SecurityGroups: "sg-web,sg-ssh", Labels: "iac:terraform"},
		{Name: "worker", ID: "i-2", Network: "vpc-1", Subnet: "subnet-b", SecurityGroups: "sg-web"},
	})
	app.indexVMs(core.CloudContext{Provider: "Manual"}, []core.VM{{Name: "manual", ID: "host-1"}})
	app.indexResourceSummary(ctx, "disks", 7, 0)
	app.indexResourceSummary(ctx, "snapshots", 9, 0)

	stats := app.infrastructureDashboardStats()
	if stats.Disks != 7 || stats.Snapshots != 9 || stats.Networks != 1 || stats.Subnets != 2 || stats.SecurityGroups != 2 || stats.TerraformManaged != 1 || stats.ManualHosts != 1 {
		t.Fatalf("unexpected infra stats: %+v", stats)
	}
	rendered := app.renderHomePanel(120, 35)
	for _, expected := range []string{"Disks indexed", "Snapshots indexed", "Networks seen", "Subnets seen", "security groups seen: 2", "Terraform managed", "Manual hosts"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected dashboard to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestDashboardHonorsConfiguredWidgets(t *testing.T) {
	app := NewApp(config.AppConfig{
		VMIndexCacheTTL: 24,
		DashboardWidgets: []string{
			"running_vms",
			"kubernetes",
		},
	}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexVMs(ctx, []core.VM{
		{Name: "api", ID: "i-1", State: "running"},
		{Name: "ip-10-0-0-1", ID: "i-2", State: "running", Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot"},
	})

	rendered := app.renderHomePanel(120, 30)

	for _, expected := range []string{"Running VMs", "K8s"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected configured dashboard widget %q, got:\n%s", expected, rendered)
		}
	}
	for _, hidden := range []string{"Indexed VMs", "Public IPs", "Backend"} {
		if strings.Contains(rendered, hidden) {
			t.Fatalf("did not expect unconfigured dashboard widget %q, got:\n%s", hidden, rendered)
		}
	}
}

func TestSelectableDashboardOpensScopedFind(t *testing.T) {
	app := NewApp(config.AppConfig{
		GlobalSearch:     true,
		VMIndexCacheTTL:  24,
		DashboardWidgets: []string{"indexed_vms", "databases"},
	}, "1.0.0", "today")
	app.width = 120
	app.height = 40
	app.showSplash = false
	app.focus = focusMain
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.indexDatabases(ctx, []core.Database{{Name: "orders", ID: "db-orders", Status: "available"}})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	app = model.(App)
	if app.dashboardCursor != 1 {
		t.Fatalf("expected dashboard cursor to move to second widget, got %d", app.dashboardCursor)
	}
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)
	if !app.showGlobalSearch || app.findScope != findScopeDatabases {
		t.Fatalf("expected database find to open, show=%t scope=%s", app.showGlobalSearch, app.findScope)
	}
	if len(app.findRows) != 1 || app.findRows[0].ID != "db-orders" {
		t.Fatalf("expected database find result, got %+v", app.findRows)
	}
}

func TestSelectableDashboardResourceCardsOpenFindScopes(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	cases := []struct {
		name       string
		widget     string
		summary    ResourceSummaryUpdateMsg
		wantScope  findScope
		wantResult string
	}{
		{
			name:       "disks",
			widget:     "disks",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "disks", Count: 1, Disks: []core.Disk{{Name: "data", ID: "vol-1", State: "in-use"}}},
			wantScope:  findScopeDisks,
			wantResult: "vol-1",
		},
		{
			name:       "snapshots",
			widget:     "snapshots",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "snapshots", Count: 1, Snapshots: []core.Snapshot{{Name: "snap-data", ID: "snap-1", State: "completed"}}},
			wantScope:  findScopeSnapshots,
			wantResult: "snap-1",
		},
		{
			name:       "networks",
			widget:     "networks",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "networks", Count: 1, Networks: []core.Network{{Name: "prod-vpc", ID: "vpc-1", State: "available"}}},
			wantScope:  findScopeNetworks,
			wantResult: "vpc-1",
		},
		{
			name:       "subnets",
			widget:     "subnets",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "networks", Count: 1, Extra: 1, Networks: []core.Network{{Name: "prod-vpc", ID: "vpc-1"}}, Subnets: []core.Subnet{{Name: "web-a", ID: "subnet-1", State: "available"}}},
			wantScope:  findScopeSubnets,
			wantResult: "subnet-1",
		},
		{
			name:       "security groups",
			widget:     "firewalls",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "firewalls", Count: 1, SecurityGroups: []core.SecurityGroup{{Name: "web-sg", ID: "sg-1", Description: "web"}}},
			wantScope:  findScopeFirewalls,
			wantResult: "sg-1",
		},
		{
			name:       "storage",
			widget:     "storage",
			summary:    ResourceSummaryUpdateMsg{Ctx: ctx, Resource: "storage", Count: 1, StorageBuckets: []core.StorageBucket{{Name: "logs", ID: "bucket-1", Access: "private"}}},
			wantScope:  findScopeStorage,
			wantResult: "bucket-1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp(config.AppConfig{
				GlobalSearch:     true,
				VMIndexCacheTTL:  24,
				DashboardWidgets: []string{tc.widget},
			}, "1.0.0", "today")
			app.width = 120
			app.height = 40
			app.showSplash = false
			app.focus = focusMain
			app.indexResourceSummaryMsg(tc.summary)

			model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
			app = model.(App)
			if !app.showGlobalSearch || app.findScope != tc.wantScope {
				t.Fatalf("expected %s find to open, show=%t scope=%s", tc.wantScope, app.showGlobalSearch, app.findScope)
			}
			if len(app.findRows) != 1 || app.findRows[0].ID != tc.wantResult {
				t.Fatalf("expected find result %q, got %+v", tc.wantResult, app.findRows)
			}
		})
	}
}

func TestHomeDashboardArrowKeysWorkAfterStartupFocus(t *testing.T) {
	app := NewApp(config.AppConfig{
		GlobalSearch:     true,
		VMIndexCacheTTL:  24,
		DashboardWidgets: []string{"indexed_vms", "databases"},
	}, "1.0.0", "today")
	app.width = 120
	app.height = 40
	app.showSplash = false

	if app.focus != focusMain {
		t.Fatalf("expected dashboard to start focused on main, got %d", app.focus)
	}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	app = model.(App)
	if app.dashboardCursor != 1 {
		t.Fatalf("expected right arrow to move dashboard cursor, got %d", app.dashboardCursor)
	}
}

func TestHomeDashboardArrowKeysWorkWithInitialViewStack(t *testing.T) {
	app := NewApp(config.AppConfig{
		GlobalSearch:     true,
		VMIndexCacheTTL:  24,
		DashboardWidgets: []string{"indexed_vms", "databases"},
	}, "1.0.0", "today")
	app.width = 120
	app.height = 40
	app.showSplash = false
	app.focus = focusMain
	app.activeCtx = core.CloudContext{}
	app.viewStack = []View{&mockView{rendered: "vms", title: "VMs"}}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	app = model.(App)
	if app.dashboardCursor != 1 {
		t.Fatalf("expected right arrow to move dashboard cursor on Home even with initial viewStack, got %d", app.dashboardCursor)
	}
}

func TestDashboardThemeUsesHeavyRule(t *testing.T) {
	app := NewApp(config.AppConfig{
		VMIndexCacheTTL: 24,
		DashboardTheme:  "btop",
	}, "1.0.0", "today")

	rendered := app.renderHomePanel(120, 30)

	if !strings.Contains(rendered, "━") {
		t.Fatalf("expected btop dashboard theme to render heavy rule, got:\n%s", rendered)
	}
}

func TestDashboardCardsDoNotOverflowSmallWidths(t *testing.T) {
	app := NewApp(config.AppConfig{
		VMIndexCacheTTL: 24,
		DashboardWidgets: []string{
			"indexed_vms",
			"running_vms",
			"databases",
			"kubernetes",
			"storage",
		},
	}, "1.0.0", "today")

	rendered := app.renderHomePanel(44, 30)
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 44 {
			t.Fatalf("expected dashboard line width <= 44, got %d for %q\n%s", lipgloss.Width(line), line, rendered)
		}
	}
}

func TestDashboardAndGlobalSearchHideKubernetesNodes(t *testing.T) {
	app := NewApp(config.AppConfig{HideKubernetesNodes: true, GlobalSearch: true, VMIndexCacheTTL: 24}, "1.0.0", "today")
	app.showKubernetesNodes = false
	now := time.Now()
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.vmIndex = map[string]vmSearchRecord{
		"vm":   {Context: ctx, VM: core.VM{Name: "api", ID: "i-1", State: "running"}, SeenAt: now},
		"node": {Context: ctx, VM: core.VM{Name: "ip-10-0-0-1", ID: "i-2", State: "running", Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot"}, SeenAt: now},
	}

	stats := app.vmDashboardStats()
	if stats.Total != 1 || stats.Running != 1 {
		t.Fatalf("expected dashboard to exclude k8s node, got %+v", stats)
	}
	app.globalSearchInput.SetValue("")
	app.refreshGlobalSearchResults()
	if len(app.vmSearchRows) != 1 || app.vmSearchRows[0].VM.ID != "i-1" {
		t.Fatalf("expected global search to exclude k8s node, got %+v", app.vmSearchRows)
	}

	app.showKubernetesNodes = true
	app.refreshGlobalSearchResults()
	if len(app.vmSearchRows) != 2 {
		t.Fatalf("expected global search to include k8s node after toggle, got %+v", app.vmSearchRows)
	}
}

func TestStartupPrefetchWaitsForVMIndexCache(t *testing.T) {
	app := NewApp(config.AppConfig{
		PrefetchOnStart:     true,
		PrefetchResources:   []string{"vms"},
		PrefetchConcurrency: 1,
		VMIndexPersistence:  true,
	}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}

	model, cmd := app.Update(contextLoadMsg{
		tree:     BuildContextTree([]core.CloudContext{ctx}),
		contexts: []core.CloudContext{ctx},
	})
	updated := model.(App)

	if cmd != nil {
		t.Fatal("expected startup prefetch to wait for VM index cache check")
	}
	if updated.vmPrefetchTotal != 0 {
		t.Fatalf("expected no prefetch before cache check, got total=%d", updated.vmPrefetchTotal)
	}
	if !strings.Contains(updated.statusMsg, "Checking VM index cache") {
		t.Fatalf("expected cache-check status, got %q", updated.statusMsg)
	}
}

func TestStartupPrefetchSkipsWhenVMIndexCacheLoaded(t *testing.T) {
	app := NewApp(config.AppConfig{
		PrefetchOnStart:     true,
		PrefetchResources:   []string{"vms"},
		PrefetchConcurrency: 1,
		VMIndexPersistence:  true,
	}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	model, _ := app.Update(contextLoadMsg{
		tree:     BuildContextTree([]core.CloudContext{ctx}),
		contexts: []core.CloudContext{ctx},
	})
	app = model.(App)

	model, cmd := app.Update(vmIndexCacheLoadMsg{
		index: map[string]vmSearchRecord{
			"one": {Context: ctx, VM: core.VM{Name: "api", ID: "i-1"}, SeenAt: time.Now()},
		},
		total: 1,
	})
	updated := model.(App)

	if cmd != nil {
		t.Fatal("expected warm VM index cache to skip startup prefetch")
	}
	if updated.vmPrefetchTotal != 0 {
		t.Fatalf("expected no prefetch with warm cache, got total=%d", updated.vmPrefetchTotal)
	}
	if !strings.Contains(updated.statusMsg, "cached VM index") {
		t.Fatalf("expected cached index status, got %q", updated.statusMsg)
	}
}

func TestStartupPrefetchRunsWhenVMIndexCacheEmpty(t *testing.T) {
	app := NewApp(config.AppConfig{
		PrefetchOnStart:     true,
		PrefetchResources:   []string{"vms"},
		PrefetchConcurrency: 1,
		VMIndexPersistence:  true,
	}, "1.0.0", "today")
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	model, _ := app.Update(contextLoadMsg{
		tree:     BuildContextTree([]core.CloudContext{ctx}),
		contexts: []core.CloudContext{ctx},
	})
	app = model.(App)

	model, cmd := app.Update(vmIndexCacheLoadMsg{})
	updated := model.(App)

	if cmd == nil {
		t.Fatal("expected empty VM index cache to trigger startup prefetch")
	}
	if updated.vmPrefetchTotal != 1 {
		t.Fatalf("expected one context to prefetch, got %d", updated.vmPrefetchTotal)
	}
}

func TestDatabaseIndexingSettingsToggleEnablesResource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(config.AppConfig{
		PrefetchResources: []string{"vms"},
	}, "1.0.0", "today")
	app.openSettings()
	for i, item := range app.settingsList.Items() {
		if item.(settingsItem).key == "db_indexing" {
			app.settingsList.Select(i)
			break
		}
	}

	model, _ := app.handleSettingsKeys(tea.KeyMsg{Type: tea.KeySpace})
	updated := model.(App)

	if !config.PrefetchesResource(updated.cfg, "databases") {
		t.Fatalf("expected database indexing resource to be enabled, got %#v", updated.cfg.PrefetchResources)
	}
}

func TestDashboardSummaryRefreshRespectsDatabaseIndexSetting(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app := NewApp(config.AppConfig{PrefetchResources: []string{"vms"}}, "1.0.0", "today")
	app.allContexts = []core.CloudContext{ctx}

	if cmds := app.startDashboardSummaryRefresh(); len(cmds) != 1 {
		t.Fatalf("expected only cluster summary command when databases disabled, got %d", len(cmds))
	}

	app.cfg.PrefetchResources = []string{"vms", "databases"}
	if cmds := app.startDashboardSummaryRefresh(); len(cmds) != 2 {
		t.Fatalf("expected cluster and database summary commands when databases enabled, got %d", len(cmds))
	}

	app.cfg.PrefetchResources = []string{"vms", "databases", "storage"}
	if cmds := app.startDashboardSummaryRefresh(); len(cmds) != 3 {
		t.Fatalf("expected cluster, database, and storage summary commands when storage enabled, got %d", len(cmds))
	}
}

func TestIndexAllSchedulesSupportedResourceIndexes(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app := NewApp(config.AppConfig{PrefetchConcurrency: 4}, "1.0.0", "today")
	app.allContexts = []core.CloudContext{ctx}

	cmds := app.startAllIndexRefresh()
	if len(cmds) != 8 {
		t.Fatalf("expected VM, cluster, database, disk, snapshot, network, firewall, and storage index commands, got %d", len(cmds))
	}
}

func TestHomeCommandReturnsToDashboard(t *testing.T) {
	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1"}
	app.focus = focusSidebar
	app.viewStack = []View{&mockView{rendered: "vms"}}

	updated, _ := app.handleCommand("dashboard")

	if updated.activeCtx.Provider != "" {
		t.Fatalf("expected active context to be cleared, got %+v", updated.activeCtx)
	}
	if len(updated.viewStack) != 0 {
		t.Fatalf("expected dashboard to clear view stack, got %d", len(updated.viewStack))
	}
	if updated.focus != focusMain {
		t.Fatalf("expected dashboard focus on main, got %d", updated.focus)
	}
}

func TestCredentialEditBacksUpAndUpdatesManagedContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{
		Backend: "cli",
		CloudContexts: []config.ManagedCloudContext{
			{Provider: "AWS", AccountID: "1111", AccountName: "old", CredentialProfile: "old-profile", Regions: []string{"us-east-1"}},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	app := NewApp(cfg, "1.0.0", "today")
	app.openCredentials()
	app.startCredentialEdit(0)
	app.credentialInputs[3].SetValue("prod")
	app.credentialInputs[7].SetValue("prod-profile")

	updated, _ := app.saveCredentialEdit()

	if updated.cfg.CloudContexts[0].AccountName != "prod" {
		t.Fatalf("expected account name update, got %+v", updated.cfg.CloudContexts[0])
	}
	matches, err := filepath.Glob(filepath.Join(home, ".cloudmanager.backup-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one config backup, got %d", len(matches))
	}
}

func TestAddProviderCommandOpensManagedContextForm(t *testing.T) {
	app := NewApp(config.AppConfig{}, "1.0.0", "today")

	updated, _ := app.handleCommand("add-provider")

	if !updated.showCredentials || !updated.editCredential {
		t.Fatalf("expected add-provider to open credential form, showCredentials=%t edit=%t", updated.showCredentials, updated.editCredential)
	}
	if len(updated.credentialInputs) != 9 {
		t.Fatalf("expected auth-aware provider form, got %d inputs", len(updated.credentialInputs))
	}
	if got := updated.credentialInputs[5].Value(); got != config.AuthModeNativeCLI {
		t.Fatalf("expected native-cli default auth mode, got %q", got)
	}
}

func TestCredentialEditSavesAuthModeAndPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{Backend: "cli"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	app := NewApp(cfg, "1.0.0", "today")
	app.startCredentialEdit(-1)
	app.credentialInputs[0].SetValue("prod-aws")
	app.credentialInputs[1].SetValue("AWS")
	app.credentialInputs[2].SetValue("1111")
	app.credentialInputs[3].SetValue("prod")
	app.credentialInputs[5].SetValue(config.AuthModeAWSume)
	app.credentialInputs[6].SetValue(config.CredentialPersistenceMemory)
	app.credentialInputs[7].SetValue("prod-admin")
	app.credentialInputs[8].SetValue("us-east-1")

	updated, _ := app.saveCredentialEdit()

	if len(updated.cfg.CloudContexts) != 1 {
		t.Fatalf("expected one managed context, got %+v", updated.cfg.CloudContexts)
	}
	got := updated.cfg.CloudContexts[0]
	if got.AuthMode != config.AuthModeAWSume || got.CredentialPersistence != config.CredentialPersistenceMemory {
		t.Fatalf("expected awsume/memory auth metadata, got %+v", got)
	}
}

func TestUseSelectedCredentialSetsCurrentContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{
		Backend: "cli",
		CloudContexts: []config.ManagedCloudContext{
			{ContextName: "eng", Provider: "Azure", AccountID: "sub-1", AccountName: "Engineering", Regions: []string{"global"}},
			{ContextName: "prod", Provider: "AWS", AccountID: "1111", AccountName: "Prod", Regions: []string{"us-east-1"}},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	app := NewApp(cfg, "1.0.0", "today")
	app.openCredentials()
	app.credentialList.Select(1)

	updated, _ := app.useSelectedCredential()
	loaded := config.Load()

	if updated.cfg.CurrentContext != "prod" || loaded.CurrentContext != "prod" {
		t.Fatalf("expected prod current context, updated=%q loaded=%q", updated.cfg.CurrentContext, loaded.CurrentContext)
	}
	item, ok := updated.credentialList.Items()[1].(credentialItem)
	if !ok || !item.current {
		t.Fatalf("expected selected item to be marked current, got %#v", updated.credentialList.Items()[1])
	}
}

func TestCredentialDeleteRequiresConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{
		Backend:        "cli",
		CurrentContext: "eng",
		CloudContexts:  []config.ManagedCloudContext{{ContextName: "eng", Provider: "Azure", AccountID: "sub-1", Regions: []string{"global"}}},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	app := NewApp(cfg, "1.0.0", "today")
	app.openCredentials()

	model, _ := app.handleCredentialKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	pending := model.(App)
	if !pending.confirmCredentialDelete || len(pending.cfg.CloudContexts) != 1 {
		t.Fatalf("expected pending confirmation without deletion, got confirm=%t contexts=%d", pending.confirmCredentialDelete, len(pending.cfg.CloudContexts))
	}

	model, _ = pending.handleCredentialKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := model.(App)
	if len(updated.cfg.CloudContexts) != 0 || updated.cfg.CurrentContext != "" {
		t.Fatalf("expected confirmed delete to remove current context, got %+v current=%q", updated.cfg.CloudContexts, updated.cfg.CurrentContext)
	}
}

func TestManagedContextLoginCommandUsesSelectedProfile(t *testing.T) {
	azure, err := managedContextLoginCommand(config.ManagedCloudContext{Provider: "Azure", Tenant: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(azure, " "); got != "az login --use-device-code --tenant example.com" {
		t.Fatalf("unexpected Azure login command: %q", got)
	}

	aws, err := managedContextLoginCommand(config.ManagedCloudContext{Provider: "AWS", CredentialProfile: "prod-admin"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(aws, " "); got != "aws sso login --profile prod-admin" {
		t.Fatalf("unexpected AWS login command: %q", got)
	}
}

func TestAzureSubscriptionItemsMarkExistingContexts(t *testing.T) {
	cfg := config.AppConfig{
		CloudContexts: []config.ManagedCloudContext{
			{ContextName: "eng", Provider: "Azure", AccountID: "sub-1", AccountName: "Engineering", Regions: []string{"global"}},
		},
	}
	items := buildAzureSubscriptionItems(cfg, []core.CloudContext{
		{Provider: "Azure", ContextName: "engineering", AccountID: "sub-1", AccountName: "Engineering", Tenant: "tenant-a", Region: "global"},
		{Provider: "Azure", ContextName: "payg", AccountID: "sub-2", AccountName: "Pay-As-You-Go", Tenant: "tenant-a", Region: "global"},
	})

	if len(items) != 2 {
		t.Fatalf("expected two Azure subscription items, got %d", len(items))
	}
	first := items[0].(azureSubscriptionItem)
	second := items[1].(azureSubscriptionItem)
	if first.existing || !first.selected || first.ctx.AccountID != "sub-2" {
		t.Fatalf("expected new subscription first and selected by default, got %+v", first)
	}
	if !second.existing || second.selected || second.ctx.AccountID != "sub-1" {
		t.Fatalf("expected existing subscription marked saved and unselected, got %+v", second)
	}
}

func TestImportSelectedAzureSubscriptionsPreservesFriendlyNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{
		Backend:        "cli",
		CurrentContext: "eng",
		CloudContexts: []config.ManagedCloudContext{
			{ContextName: "eng", Provider: "Azure", AccountID: "sub-1", AccountName: "Old Engineering", Tenant: "tenant-old", Regions: []string{"global"}},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	app := NewApp(cfg, "1.0.0", "today")
	app.showAzureSubscriptions = true
	app.azureSubList.SetItems([]list.Item{
		azureSubscriptionItem{
			selected: true,
			existing: true,
			ctx: core.CloudContext{
				Provider:    "Azure",
				ContextName: "azure-sponsorship-engineering",
				AccountID:   "sub-1",
				AccountName: "Azure Sponsorship - Engineering",
				Tenant:      "tenant-new",
				Region:      "global",
			},
		},
		azureSubscriptionItem{
			selected: true,
			ctx: core.CloudContext{
				Provider:    "Azure",
				ContextName: "payg",
				AccountID:   "sub-2",
				AccountName: "Pay-As-You-Go",
				Tenant:      "tenant-new",
				Region:      "global",
			},
		},
	})

	updated, _ := app.importSelectedAzureSubscriptions()

	if len(updated.cfg.CloudContexts) != 2 {
		t.Fatalf("expected two Azure contexts, got %+v", updated.cfg.CloudContexts)
	}
	if updated.cfg.CloudContexts[0].ContextName != "eng" {
		t.Fatalf("expected re-import to preserve friendly name, got %+v", updated.cfg.CloudContexts[0])
	}
	if updated.cfg.CloudContexts[0].AccountName != "Azure Sponsorship - Engineering" || updated.cfg.CloudContexts[0].Tenant != "tenant-new" {
		t.Fatalf("expected re-import to refresh subscription metadata, got %+v", updated.cfg.CloudContexts[0])
	}
	if updated.cfg.CloudContexts[1].ContextName != "payg" || updated.cfg.CloudContexts[1].AccountID != "sub-2" {
		t.Fatalf("expected new payg context, got %+v", updated.cfg.CloudContexts[1])
	}
	if updated.cfg.CurrentContext != "eng" {
		t.Fatalf("expected current context to remain eng, got %q", updated.cfg.CurrentContext)
	}
}

func TestDiscoveryPickerDefaultsUnselectedAndImportsOnlySelected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.AppConfig{
		Backend: "cli",
		CloudContexts: []config.ManagedCloudContext{
			{ContextName: "existing", Provider: "GCP", AccountID: "project-old", AccountName: "project-old", Regions: []string{"global"}},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	app := NewApp(cfg, "1.0.0", "today")
	contexts := []core.CloudContext{
		{Provider: "GCP", ContextName: "project-old", AccountID: "project-old", AccountName: "project-old", Region: "global"},
		{Provider: "GCP", ContextName: "project-new", AccountID: "project-new", AccountName: "project-new", Region: "global"},
		{Provider: "Azure", ContextName: "eng", AccountID: "sub-1", AccountName: "Engineering", Tenant: "tenant-1", Region: "global"},
	}
	items := buildDiscoveryContextItems(cfg, contexts)
	if len(items) != 3 {
		t.Fatalf("expected 3 discovered items, got %d", len(items))
	}
	for _, raw := range items {
		item := raw.(discoveryContextItem)
		if item.selected {
			t.Fatalf("expected discovered item to default unselected, got %+v", item)
		}
	}
	app.showContextDiscovery = true
	app.discoveryList.SetItems(items)
	for i, raw := range app.discoveryList.Items() {
		item := raw.(discoveryContextItem)
		if item.ctx.AccountID == "project-new" {
			item.selected = true
			_ = app.discoveryList.SetItem(i, item)
		}
	}

	updated, _ := app.importSelectedDiscoveredContexts()

	if len(updated.cfg.CloudContexts) != 2 {
		t.Fatalf("expected only selected new context to import, got %+v", updated.cfg.CloudContexts)
	}
	if updated.cfg.CloudContexts[1].AccountID != "project-new" {
		t.Fatalf("expected project-new import, got %+v", updated.cfg.CloudContexts[1])
	}
}

func TestFilteredContextEnterActivatesMatchingLeaf(t *testing.T) {
	mv := &mockView{rendered: "vms"}
	app := NewApp(config.AppConfig{}, "1.0.0", "today")
	app.width = 120
	app.height = 30
	app.showSplash = false
	app.RegisterView("1", providers.CapabilityVMs, mv)
	contexts := []core.CloudContext{
		{Provider: "AWS", AccountID: "1111", AccountName: "prod", Region: "us-east-1", CredentialProfile: "prod-admin"},
		{Provider: "AWS", AccountID: "2222", AccountName: "stage", Region: "us-west-2", CredentialProfile: "stage-admin"},
	}
	app.rootNodes = BuildContextTree(contexts)
	app.contexts.SetItems(BuildFlatList(app.rootNodes))
	app.contexts.SetFilterText("stage")
	app.contexts.SetFilterState(list.FilterApplied)

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(App)

	if updated.focus != focusMain {
		t.Fatalf("expected focus to move to main, got %d", updated.focus)
	}
	if updated.activeCtx.AccountName != "stage" || updated.activeCtx.Region != "us-west-2" {
		t.Fatalf("expected stage context to activate, got %+v", updated.activeCtx)
	}
}
