package firewalls

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/ui"
)

func TestRenderWithManySecurityGroupsFitsViewWidth(t *testing.T) {
	view := New(&config.AppConfig{})
	view.width = 90
	view.height = 20
	view.breadcrumbs = "AWS › 9431 (main) › us-east-2 › Firewalls"

	for i := 0; i < 100; i++ {
		view.groupData = append(view.groupData, core.SecurityGroup{
			Name:              "web-tier-security-group",
			ID:                "sg-0123456789abcdef0",
			NetworkID:         "vpc-0123456789abcdef0",
			InboundRuleCount:  4,
			OutboundRuleCount: 2,
			AttachedResources: 3,
			Description:       "frontend access control",
		})
	}

	view.refreshTable()

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

func TestViewRulesActionPushesRulesView(t *testing.T) {
	view := New(&config.AppConfig{})
	view.width = 100
	view.height = 30
	view.groupData = []core.SecurityGroup{
		{Name: "web", ID: "sg-1", NetworkID: "vpc-1"},
	}
	view.refreshTable()
	view.syncVisibleRows()
	view.groups.Focus()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(*FirewallsView)
	if next.activePane != paneActions {
		t.Fatalf("expected actions pane, got %d", next.activePane)
	}

	updated, cmd := next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected push view command")
	}

	msg := cmd()
	push, ok := msg.(ui.PushViewMsg)
	if !ok {
		t.Fatalf("expected PushViewMsg, got %T", msg)
	}
	if _, ok := push.View.(*RulesView); !ok {
		t.Fatalf("expected pushed view to be *RulesView, got %T", push.View)
	}

	final := updated.(*FirewallsView)
	if final.activePane != paneTable {
		t.Fatalf("expected pane to return to table before drill-down, got %d", final.activePane)
	}
}

func TestHorizontalPanChangesVisibleFirewallColumns(t *testing.T) {
	cfg := config.AppConfig{FirewallColumns: []string{"Name", "ID", "Network", "Inbound Rules", "Outbound Rules", "Attached", "Description"}}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.refreshTable()

	initialFirst := view.displayColumns[0].Title
	if !view.canScrollRight {
		t.Fatal("expected horizontal scrolling to be available in a narrow firewall view")
	}

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*FirewallsView)

	if next.columnOffset != 1 {
		t.Fatalf("expected column offset 1 after panning right, got %d", next.columnOffset)
	}
	if next.displayColumns[0].Title == initialFirst {
		t.Fatalf("expected visible columns to change after panning, still starts with %q", next.displayColumns[0].Title)
	}
}

func TestFirewallActionsOverlayFitsWindow(t *testing.T) {
	view := New(&config.AppConfig{})
	view.width = 80
	view.height = 20
	view.groupData = []core.SecurityGroup{{Name: "web", ID: "sg-1"}}
	view.refreshTable()
	view.syncVisibleRows()
	view.groups.Focus()
	view.activePane = paneActions

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

func TestRenderPrefixesRiskySecurityGroups(t *testing.T) {
	view := New(&config.AppConfig{})
	view.width = 100
	view.height = 20
	view.groupData = []core.SecurityGroup{
		{Name: "public-ssh", ID: "sg-1", HasOpenSSH: true},
	}
	view.refreshTable()
	view.syncVisibleRows()

	rendered := view.Render()
	if !strings.Contains(rendered, "⚠ public-ssh") {
		t.Fatalf("expected risky group warning prefix in render, got:\n%s", rendered)
	}
}

func TestRenderRiskyGroupsFitsWidth(t *testing.T) {
	groups := []core.SecurityGroup{
		{
			Name:              "vuln-lab-access-from-dev3-with-a-very-long-name",
			ID:                "sg-0fa816c43428123456789",
			NetworkID:         "vpc-0c70259ee123456789",
			InboundRuleCount:  5,
			OutboundRuleCount: 1,
			AttachedResources: 13,
			Description:       "This is all deliberately long to verify risky colored rows do not leak ANSI fragments into the fixed-width table",
			HasOpenSSH:        true,
		},
	}
	cols := []table.Column{
		{Title: "Name", Width: 18},
		{Title: "ID", Width: 16},
		{Title: "Network", Width: 14},
		{Title: "Inbound Rules", Width: 13},
		{Title: "Outbound Rules", Width: 14},
		{Title: "Attached", Width: 8},
		{Title: "Description", Width: 24},
	}
	rows := mapSecurityGroupsToRows(groups, cols)
	if len(rows) != 1 {
		t.Fatalf("expected one rendered row, got %d", len(rows))
	}
	for i, cell := range rows[0] {
		if lipgloss.Width(cell) > cols[i].Width {
			t.Fatalf("cell %q exceeds width: got %d want <= %d", cols[i].Title, lipgloss.Width(cell), cols[i].Width)
		}
		if strings.Contains(cell, "\x1b[") {
			t.Fatalf("cell %q leaked ANSI escape sequence: %q", cols[i].Title, cell)
		}
	}
}

func TestSetSearchQueryFiltersSecurityGroups(t *testing.T) {
	view := New(&config.AppConfig{})
	view.width = 100
	view.height = 30
	view.groupData = []core.SecurityGroup{
		{Name: "alpha", ID: "sg-1"},
		{Name: "beta", ID: "sg-2"},
	}
	view.refreshTable()

	view.SetSearchQuery("sg-2")

	if len(view.visibleGroups) != 1 || view.visibleGroups[0].ID != "sg-2" {
		t.Fatalf("expected search handoff to filter to sg-2, got %+v", view.visibleGroups)
	}
}

func TestNewFilteredLimitsVisibleGroups(t *testing.T) {
	view := NewFiltered(&config.AppConfig{}, []string{"sg-2"}, "VM: web-1")
	view.width = 100
	view.height = 20
	view.groupData = []core.SecurityGroup{
		{Name: "alpha", ID: "sg-1"},
		{Name: "beta", ID: "sg-2"},
	}
	view.refreshTable()
	view.syncVisibleRows()

	if len(view.visibleGroups) != 1 {
		t.Fatalf("expected one visible filtered group, got %d", len(view.visibleGroups))
	}
	if view.visibleGroups[0].ID != "sg-2" {
		t.Fatalf("expected sg-2 to remain after filtering, got %s", view.visibleGroups[0].ID)
	}
}
