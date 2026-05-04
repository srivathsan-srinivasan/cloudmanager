package vms

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/ui"
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

func TestKubernetesNodesHiddenByDefaultWhenConfigured(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns, HideKubernetesNodes: true}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.vmData = []core.VM{
		{Name: "api", ID: "vm-1", State: "running"},
		{Name: "gke-prod-pool-abc", ID: "vm-2", State: "running", Labels: "goog-gke-node=true, cloud.google.com/gke-nodepool=pool-a"},
	}

	view.refreshTable()
	view.syncVisibleRows()

	if len(view.visibleVMs) != 1 || view.visibleVMs[0].ID != "vm-1" {
		t.Fatalf("expected only non-kubernetes VM visible, got %+v", view.visibleVMs)
	}
	if !strings.Contains(view.Render(), "K8s nodes hidden: 1") {
		t.Fatalf("expected hidden-node banner, got:\n%s", view.Render())
	}
}

func TestKubernetesNodesCanBeShownWithToggle(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns, HideKubernetesNodes: true}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.vmData = []core.VM{
		{Name: "api", ID: "vm-1", State: "running"},
		{Name: "ip-10-0-0-1", ID: "vm-2", State: "running", Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot"},
	}
	view.refreshTable()
	view.syncVisibleRows()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	next := updated.(*VMsView)

	if len(next.visibleVMs) != 2 {
		t.Fatalf("expected kubernetes node to be visible after toggle, got %+v", next.visibleVMs)
	}
	if !strings.Contains(next.statusMsg, "Showing Kubernetes") {
		t.Fatalf("expected toggle status, got %q", next.statusMsg)
	}
}

func TestTagSelectedVMSavesCloudManagerTagOverlay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "us-central1"}
	view.vmData = []core.VM{{Name: "vf-web-1", ID: "gce-1", PublicIP: "203.0.113.10"}}
	view.refreshTable()
	view.syncVisibleRows()

	updated, cmd := view.handleTableKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view = updated.(*VMsView)
	if cmd == nil || view.activePane != paneTag {
		t.Fatalf("expected tag pane and blink command, pane=%d cmd=%v", view.activePane, cmd)
	}
	view.tagInput.SetValue("VFWEB, prod")
	updated, _ = view.handleTagKeys(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*VMsView)

	if len(cfg.ResourceTags) != 1 {
		t.Fatalf("expected saved CloudManager resource tag, got %+v", cfg.ResourceTags)
	}
	if !strings.Contains(view.vmData[0].Labels, "cm:VFWEB") {
		t.Fatalf("expected VM labels to include CloudManager tag, got %q", view.vmData[0].Labels)
	}
	view.searchInput.SetValue("VFWEB")
	view.syncVisibleRows()
	if len(view.visibleVMs) != 1 || view.visibleVMs[0].ID != "gce-1" {
		t.Fatalf("expected CloudManager tag to be searchable, got %+v", view.visibleVMs)
	}
}

func TestInitResetsTransientSelectionState(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.activePane = paneActions
	view.isSearching = true
	view.searchInput.SetValue("stale-query")
	view.copyableText = "stale"

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
	if view.copyableText != "" {
		t.Fatalf("expected init to clear copyable text, got %q", view.copyableText)
	}
}

func TestDescribePaneRendersBorderlessCopyableText(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.Resize(80, 20, false)
	command := "gcloud compute ssh web-1 --tunnel-through-iap"

	view.showCopyableDetail("SSH CONNECTION FAILED\n\n" + command)
	rendered := view.Render()

	if !strings.Contains(rendered, command) {
		t.Fatalf("expected rendered detail to contain command, got:\n%s", rendered)
	}
	for _, border := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(rendered, border) {
			t.Fatalf("expected copyable detail pane to be borderless, found %q in:\n%s", border, rendered)
		}
	}
	if view.copyableText == "" {
		t.Fatal("expected copyable text to be stored")
	}
}

func TestDescribeKeyStartsProviderDescribe(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "AWS", Region: "us-east-1"}
	view.vmData = []core.VM{{Name: "alpha", ID: "i-123", State: "running"}}
	view.refreshTable()
	view.syncVisibleRows()

	_, cmd := view.handleTableKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected describe key to start provider describe")
	}
	if !view.loading {
		t.Fatal("expected describe key to set loading")
	}
	if !strings.Contains(view.statusMsg, "Describing alpha") {
		t.Fatalf("expected describing status, got %q", view.statusMsg)
	}
}

func TestProviderNativeAccessLabelAWS(t *testing.T) {
	ssm := providerNativeAccessLabel("AWS", []string{"aws", "--no-cli-pager", "ssm", "start-session", "--target", "i-123"})
	if ssm != "AWS SSM Session Manager" {
		t.Fatalf("expected SSM label, got %q", ssm)
	}

	eic := providerNativeAccessLabel("AWS", []string{"aws", "--no-cli-pager", "ec2-instance-connect", "ssh", "--instance-id", "i-123"})
	if eic != "AWS EC2 Instance Connect" {
		t.Fatalf("expected EC2 Instance Connect label, got %q", eic)
	}
}

func TestAccessResolvedOpensPickerAndSelectsCommand(t *testing.T) {
	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.Resize(100, 30, false)

	updated, _ := view.Update(accessResolvedMsg{
		vm: core.VM{Name: "alpha", ID: "i-123"},
		methods: []core.AccessMethod{
			{
				ID:        "direct-ssh",
				Kind:      "ssh",
				Label:     "Direct SSH",
				Command:   []string{"ssh", "203.0.113.10"},
				CopyText:  "ssh 203.0.113.10",
				Available: true,
			},
		},
	})
	next := updated.(*VMsView)

	if next.activePane != paneAccess {
		t.Fatalf("expected access picker pane, got %d", next.activePane)
	}
	method, ok := next.selectedAccessMethod()
	if !ok {
		t.Fatal("expected selected access method")
	}
	if method.CopyText != "ssh 203.0.113.10" {
		t.Fatalf("expected direct ssh command selected, got %+v", method)
	}
	if !strings.Contains(next.Render(), "Direct SSH") {
		t.Fatalf("expected rendered access picker, got:\n%s", next.Render())
	}
}

func TestPrivateKeyAccessUsesKeyDropdownAndUsername(t *testing.T) {
	home := t.TempDir()
	keyDir := filepath.Join(home, "sshkeys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "prod.pem"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLOUDMANAGER_SSH_KEY_DIRS", keyDir)

	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.Resize(120, 30, false)
	view.activePane = paneAccess
	view.pendingVM = core.VM{Name: "alpha", PublicIP: "203.0.113.10", PrivateIP: "10.0.0.5"}
	view.setAccessItems([]core.AccessMethod{{
		ID:        "private-key-ssh",
		Kind:      "private_key_picker",
		Label:     "Private key SSH (1 keys)",
		Available: true,
	}})

	_, cmd := view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected status command when opening key picker")
	}
	if view.accessMode != "keys" {
		t.Fatalf("expected key picker mode, got %q", view.accessMode)
	}
	if _, ok := view.currentPrivateKeyMethod(); ok {
		t.Fatal("expected no runnable key command before selecting a key")
	}
	_, _ = view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if !view.keyDropdownOpen {
		t.Fatal("expected key dropdown to open")
	}
	_, _ = view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if view.keyDropdownOpen {
		t.Fatal("expected key dropdown to close after selection")
	}
	if filepath.Base(view.selectedSSHKey) != "prod.pem" {
		t.Fatalf("expected prod.pem selected, got %q", view.selectedSSHKey)
	}
	method, ok := view.currentPrivateKeyMethod()
	if !ok || !strings.Contains(method.CopyText, "ubuntu@203.0.113.10") {
		t.Fatalf("expected default ubuntu public IP command, got %+v", method)
	}

	_, _ = view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if !view.editingAccessUser {
		t.Fatal("expected username edit mode")
	}
	view.accessUserInput.SetValue("ec2-user")
	_, _ = view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyEnter})
	method, ok = view.currentPrivateKeyMethod()
	if !ok || !strings.Contains(method.CopyText, "ec2-user@203.0.113.10") {
		t.Fatalf("expected edited username in command, got %+v", method)
	}
}

func TestPrivateKeyAccessRenderDoesNotBleedVMTable(t *testing.T) {
	home := t.TempDir()
	keyDir := filepath.Join(home, "sshkeys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "prod.pem"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLOUDMANAGER_SSH_KEY_DIRS", keyDir)

	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.Resize(120, 30, false)
	view.activePane = paneAccess
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountName: "main", Region: "ap-south-1"}
	view.breadcrumbs = "AWS > main > ap-south-1"
	view.vmData = []core.VM{{Name: "alpha", ID: "i-123", Type: "t3.large", State: "running", PublicIP: "203.0.113.10", PrivateIP: "10.0.0.5"}}
	view.syncVisibleRows()
	view.pendingVM = view.vmData[0]
	view.openPrivateKeyPicker()

	rendered := view.Render()
	if strings.Contains(rendered, "Instance ID") || strings.Contains(rendered, "t3.large") {
		t.Fatalf("expected private-key form to render without VM table bleed, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Private key SSH") || !strings.Contains(rendered, "key> Select key") {
		t.Fatalf("expected private-key form content, got:\n%s", rendered)
	}
}

func TestAccessCopyCopiesOnlySelectedCommand(t *testing.T) {
	originalCopy := copyToClipboard
	defer func() { copyToClipboard = originalCopy }()

	var copied string
	copyToClipboard = func(text string) error {
		copied = text
		return nil
	}

	cfg := config.AppConfig{VMColumns: config.DefaultVMColumns}
	view := New(&cfg)
	view.Resize(100, 30, false)
	view.activePane = paneAccess
	view.pendingVM = core.VM{Name: "alpha", ID: "i-123"}
	view.accessMethods.SetItems([]list.Item{
		accessItem{method: core.AccessMethod{
			ID:        "direct-ssh",
			Kind:      "ssh",
			Label:     "Direct SSH",
			Command:   []string{"ssh", "203.0.113.10"},
			CopyText:  "ssh 203.0.113.10",
			Available: true,
		}},
	})

	_, cmd := view.handleAccessKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	msg := cmd()
	clip, ok := msg.(clipboardCompleteMsg)
	if !ok {
		t.Fatalf("expected clipboard message, got %T", msg)
	}
	if clip.err != nil {
		t.Fatalf("copy failed: %v", clip.err)
	}
	if copied != "ssh 203.0.113.10" {
		t.Fatalf("expected only selected command copied, got %q", copied)
	}
	if view.copyableText != "ssh 203.0.113.10" {
		t.Fatalf("expected borderless fallback text to be only command, got %q", view.copyableText)
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
	updated, _ := view.Update(msg)
	view = updated.(*VMsView)

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
	if cmd == nil {
		t.Fatal("expected VM index update command after fetch")
	}
	msg := cmd()
	if _, ok := msg.(ui.VMIndexUpdateMsg); !ok {
		t.Fatalf("expected VM index update command, got %T", msg)
	}
	if got := next.vmData[0].MonthlyCost; got != "-" {
		t.Fatalf("expected placeholder cost without auto-fetch, got %q", got)
	}
	if strings.Contains(next.statusMsg, "Fetching costs") {
		t.Fatalf("expected status not to auto-fetch costs, got %q", next.statusMsg)
	}
}

func TestCostGuideForAWSIncludesCliFormats(t *testing.T) {
	guide := formatShellCommand(buildCostCommand(core.VM{Name: "alpha", ID: "i-123"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "aws-main-9431",
	}, config.AppConfig{}))

	if !strings.Contains(guide, "aws --no-cli-pager ce get-cost-and-usage") {
		t.Fatal("expected AWS CLI cost command in cost guide")
	}
	if !strings.Contains(guide, "--profile aws-main-9431") {
		t.Fatal("expected auth profile in AWS cost command")
	}
	if !strings.Contains(guide, "--output table") {
		t.Fatal("expected table output variant in AWS cost guide")
	}
}

func TestFetchResultDoesNotAutoTriggerCostLookupWhenEnabled(t *testing.T) {
	cfg := config.AppConfig{
		VMColumns:      config.DefaultVMColumns,
		BillingEnabled: true,
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
	if cmd == nil {
		t.Fatal("expected VM index update command after fetch")
	}
	msg := cmd()
	if _, ok := msg.(ui.VMIndexUpdateMsg); !ok {
		t.Fatalf("expected VM index update command, got %T", msg)
	}
	if got := next.vmData[0].MonthlyCost; got != "-" {
		t.Fatalf("expected placeholder cost without auto-fetch, got %q", got)
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
