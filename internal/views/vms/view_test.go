package vms

import (
	"context"
	"strings"
	"testing"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSelectedVMUsesVisibleRows(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.vmData = []core.VM{
		{Name: "alpha", ID: "vm-1"},
		{Name: "beta", ID: "vm-2"},
	}

	view.refreshTable()
	view.searchInput.SetValue("beta")
	view.syncVisibleRows()

	selected, ok := view.selectedVM()
	if !ok {
		t.Fatal("expected a selected VM")
	}
	if selected.ID != "vm-2" {
		t.Fatalf("expected filtered selection to return vm-2, got %s", selected.ID)
	}
}

func TestSyncVisibleRowsClampsCursor(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.vmData = []core.VM{
		{Name: "alpha", ID: "vm-1"},
		{Name: "beta", ID: "vm-2"},
	}

	view.refreshTable()
	view.syncVisibleRows()
	view.vms.SetCursor(1)

	view.searchInput.SetValue("alpha")
	view.syncVisibleRows()

	if got := view.vms.Cursor(); got != 0 {
		t.Fatalf("expected cursor to clamp to 0 after filtering, got %d", got)
	}
	selected, ok := view.selectedVM()
	if !ok {
		t.Fatal("expected a selected VM after filtering")
	}
	if selected.ID != "vm-1" {
		t.Fatalf("expected cursor to point at vm-1 after clamping, got %s", selected.ID)
	}
}

func TestInitResetsTransientSelectionState(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.activePane = paneActions
	view.isSearching = true
	view.searchInput.SetValue("stale-query")

	view.Init(core.CloudContext{
		Provider:    "AWS",
		AccountName: "prod",
		Region:      "us-east-1",
	}, 100, 30, true)

	if view.activePane != paneTable {
		t.Fatalf("expected init to reset pane to table, got %d", view.activePane)
	}
	if view.isSearching {
		t.Fatal("expected init to exit search mode")
	}
	if got := view.searchInput.Value(); got != "" {
		t.Fatalf("expected init to clear the search query, got %q", got)
	}
}

func TestStaleFetchResultIsIgnored(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{
		Provider:    "AWS",
		AccountID:   "9431",
		AccountName: "main",
		Region:      "ap-south-1",
	}
	view.requestKey = view.activeCtx.CacheKey()
	view.loading = true
	view.vmData = []core.VM{{Name: "current", ID: "vm-current"}}
	view.refreshTable()
	view.syncVisibleRows()

	updated, _ := view.Update(vmFetchMsg{
		requestKey: "stale-request",
		vms:        []core.VM{{Name: "stale", ID: "vm-stale"}},
	})

	next := updated.(*VMsView)
	if !next.loading {
		t.Fatal("expected stale fetch result to leave loading state unchanged")
	}
	if len(next.vmData) != 1 || next.vmData[0].ID != "vm-current" {
		t.Fatalf("expected stale fetch result to be ignored, got %+v", next.vmData)
	}
}

func TestRenderWithManyVMsFitsViewWidth(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.activeCtx = core.CloudContext{
		Provider:    "AWS",
		AccountID:   "1193",
		AccountName: "poc",
		Region:      "us-east-2",
	}
	view.requestKey = view.activeCtx.CacheKey()
	view.breadcrumbs = "AWS › 1193 (poc) › us-east-2"

	for i := 0; i < 100; i++ {
		view.vmData = append(view.vmData, core.VM{
			Name:        "fc-poc-openvpn-node-with-a-long-name",
			ID:          "i-0123456789abcdef0",
			Type:        "t3a.2xlarge",
			State:       "running",
			PrivateIP:   "10.230.136.100",
			PublicIP:    "3.142.166.100",
			Network:     "vpc-0b6b2deadbeef",
			Subnet:      "subnet-0b6b2deadbeef",
			Labels:      "USED_FOR=fc-poc-openvpn-node",
			MonthlyCost: "$0.00",
			CostTrend:   "+0.0%",
		})
	}

	view.refreshTable()
	view.syncVisibleRows()

	rendered := strings.TrimRight(view.Render(), "\n")
	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		if lipgloss.Width(line) > view.width {
			t.Fatalf("rendered line exceeds view width: got %d want <= %d\n%s", lipgloss.Width(line), view.width, line)
		}
	}
	if len(lines) > view.height {
		t.Fatalf("rendered view exceeds height: got %d want <= %d", len(lines), view.height)
	}
}

func TestHorizontalPanChangesVisibleColumns(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.refreshTable()

	initialFirst := view.tableCols[0].Title
	if !view.canScrollRight {
		t.Fatal("expected horizontal scrolling to be available in a narrow view")
	}

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*VMsView)

	if next.columnOffset != 1 {
		t.Fatalf("expected column offset 1 after panning right, got %d", next.columnOffset)
	}
	if next.tableCols[0].Title == initialFirst {
		t.Fatalf("expected visible columns to change after panning, still starts with %q", next.tableCols[0].Title)
	}
}

func TestSortVMsKeepsRunningInstancesOnTop(t *testing.T) {
	vms := []core.VM{
		{Name: "zeta", ID: "vm-3", State: "stopped"},
		{Name: "alpha", ID: "vm-1", State: "running"},
		{Name: "beta", ID: "vm-2", State: "terminated"},
	}

	sortVMs(vms, "Name", true)

	if vms[0].ID != "vm-1" {
		t.Fatalf("expected running VM first, got %s", vms[0].ID)
	}
}

func TestSortVMsByCostStillKeepsRunningInstancesOnTop(t *testing.T) {
	vms := []core.VM{
		{Name: "expensive-stopped", ID: "vm-2", State: "stopped", MonthlyCost: "$500.00"},
		{Name: "cheap-running", ID: "vm-1", State: "running", MonthlyCost: "$1.00"},
	}

	sortVMs(vms, "Cost", false)

	if vms[0].ID != "vm-1" {
		t.Fatalf("expected running VM first even when sorting by cost desc, got %s", vms[0].ID)
	}
}

func TestEnrichCostCmdUsesProviderCostsAndCache(t *testing.T) {
	originalGetProvider := getProvider
	defer func() { getProvider = originalGetProvider }()

	callCount := 0
	getProvider = func(cfg config.AppConfig) providers.Provider {
		return &providers.MockProvider{
			FetchResourceCostFn: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
				callCount++
				return &core.ResourceCost{
					ResourceID:        resourceID,
					CurrentMonthCost:  12.5,
					PreviousMonthCost: 10,
				}, nil
			},
		}
	}

	cfg := config.AppConfig{
		VMColumns:       config.DefaultVMColumns,
		BillingEnabled:  true,
		BillingCacheTTL: 30,
	}
	view := New(&cfg)
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "9431", AccountName: "main", Region: "us-east-2"}
	view.requestKey = view.activeCtx.CacheKey()

	input := []core.VM{{Name: "alpha", ID: "i-123", State: "running"}}
	msg := view.enrichCostCmd(input)().(vmCostEnrichedMsg)

	if got := msg.vms[0].MonthlyCost; got != "$12.50" {
		t.Fatalf("expected monthly cost to be rendered, got %q", got)
	}
	if got := msg.vms[0].CostTrend; got != "+25.0%" {
		t.Fatalf("expected cost trend to be rendered, got %q", got)
	}
	if callCount != 1 {
		t.Fatalf("expected one cost lookup, got %d", callCount)
	}

	cachedMsg := view.enrichCostCmd(msg.vms)().(vmCostEnrichedMsg)
	if got := cachedMsg.vms[0].MonthlyCost; got != "$12.50" {
		t.Fatalf("expected cached monthly cost to persist, got %q", got)
	}
	if callCount != 1 {
		t.Fatalf("expected cached cost lookup to avoid extra provider calls, got %d", callCount)
	}
}

func TestFetchResultDoesNotAutoTriggerCostLookupWhenDisabled(t *testing.T) {
	cfg := config.AppConfig{
		VMColumns:      config.DefaultVMColumns,
		BillingEnabled: false,
	}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{
		Provider:    "AWS",
		AccountID:   "9431",
		AccountName: "main",
		Region:      "us-east-2",
	}
	view.requestKey = view.activeCtx.CacheKey()

	updated, cmd := view.Update(vmFetchMsg{
		requestKey: view.requestKey,
		vms: []core.VM{
			{Name: "alpha", ID: "i-123", State: "running"},
		},
	})

	next := updated.(*VMsView)
	if cmd != nil {
		t.Fatal("expected no automatic cost enrichment command after fetch")
	}
	if got := next.vmData[0].MonthlyCost; got != "-" {
		t.Fatalf("expected placeholder cost without auto-fetch, got %q", got)
	}
	if strings.Contains(next.statusMsg, "Fetching costs") {
		t.Fatalf("expected status not to auto-fetch costs, got %q", next.statusMsg)
	}
}

func TestCostGuideForAWSIncludesCliFormats(t *testing.T) {
	guide := costGuide(core.VM{Name: "alpha", ID: "i-123"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "aws-main-9431",
	}, config.AppConfig{})

	if !strings.Contains(guide, "aws ce get-cost-and-usage") {
		t.Fatal("expected AWS CLI cost command in cost guide")
	}
	if !strings.Contains(guide, "--profile aws-main-9431") {
		t.Fatal("expected auth profile in AWS cost command")
	}
	if !strings.Contains(guide, "--output json") {
		t.Fatal("expected JSON output variant in AWS cost guide")
	}
	if !strings.Contains(guide, "--output table") {
		t.Fatal("expected table output variant in AWS cost guide")
	}
}

func TestViewFirewallsActionPushesFilteredFirewallsView(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "9431", AccountName: "main", Region: "us-east-2"}
	view.vmData = []core.VM{{Name: "web-1", ID: "i-123", SecurityGroups: "sg-1,sg-2"}}
	view.refreshTable()
	view.syncVisibleRows()
	view.actions.Select(4)

	updated, cmd := view.handleActionKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected push view command")
	}

	msg := cmd()
	push, ok := msg.(ui.PushViewMsg)
	if !ok {
		t.Fatalf("expected PushViewMsg, got %T", msg)
	}
	if push.View.Title() != "Firewalls" {
		t.Fatalf("expected Firewalls view, got %s", push.View.Title())
	}

	next := updated.(*VMsView)
	if next.activePane != paneTable {
		t.Fatalf("expected VM view to return to table pane, got %d", next.activePane)
	}
}
