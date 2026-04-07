package firewalls

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
)

func TestRulesViewSelectedRowFitsWidth(t *testing.T) {
	rules := []core.FirewallRule{
		{
			ID:          "rule-1",
			Direction:   "Inbound",
			Protocol:    "all",
			PortRange:   "All",
			Source:      "0.0.0.0/0, tag:prod-natgw, secure:env/prod",
			Destination: "tag:web, tag:api, network:prod-vpc, sa:web@project.iam.gserviceaccount.com",
			Action:      "Allow",
			Description: "This is a deliberately long firewall description to verify the selected row stays clipped inside the fixed window",
		},
	}
	cols := []table.Column{
		{Title: "Direction", Width: 10},
		{Title: "Protocol", Width: 9},
		{Title: "Ports", Width: 8},
		{Title: "Source", Width: 22},
		{Title: "Destination", Width: 22},
		{Title: "Action", Width: 8},
		{Title: "Description", Width: 22},
	}
	rows := mapFirewallRulesToRows(rules, cols)
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
	if !strings.Contains(rows[0][0], "⚠ ") {
		t.Fatalf("expected risky rule marker in direction column, got %q", rows[0][0])
	}
}

func TestDenyRulesUsePlainTextMarker(t *testing.T) {
	rules := []core.FirewallRule{{
		ID:          "rule-1",
		Direction:   "Outbound",
		Protocol:    "tcp",
		PortRange:   "443",
		Source:      "10.0.0.0/8",
		Destination: "0.0.0.0/0",
		Action:      "Deny",
		Description: "blocked egress",
	}}
	cols := []table.Column{
		{Title: "Direction", Width: 10},
		{Title: "Protocol", Width: 9},
		{Title: "Ports", Width: 8},
		{Title: "Source", Width: 22},
		{Title: "Destination", Width: 22},
		{Title: "Action", Width: 8},
		{Title: "Description", Width: 22},
	}
	rows := mapFirewallRulesToRows(rules, cols)
	if len(rows) != 1 {
		t.Fatalf("expected one rendered row, got %d", len(rows))
	}
	if !strings.Contains(rows[0][5], "⊘ ") {
		t.Fatalf("expected plain-text deny marker in action column, got %q", rows[0][5])
	}
	if strings.Contains(rows[0][5], "\x1b[") {
		t.Fatalf("deny action leaked ANSI escape sequence: %q", rows[0][5])
	}
}

func TestHorizontalPanChangesVisibleRuleColumns(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.width = 90
	view.height = 20
	view.refreshTable()

	initialFirst := view.displayCols[0].Title
	if !view.canScrollRight {
		t.Fatal("expected horizontal scrolling to be available in a narrow rules view")
	}

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*RulesView)

	if next.columnOffset != 1 {
		t.Fatalf("expected column offset 1 after panning right, got %d", next.columnOffset)
	}
	if next.displayCols[0].Title == initialFirst {
		t.Fatalf("expected visible columns to change after panning, still starts with %q", next.displayCols[0].Title)
	}
}

func TestRulesDescribePaneFitsWindow(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.width = 80
	view.height = 20
	view.ruleData = []core.FirewallRule{{
		ID:          "rule-1",
		Direction:   "Inbound",
		Protocol:    "tcp",
		PortRange:   "443",
		Source:      "0.0.0.0/0",
		Destination: "tag:web,network:prod-vpc",
		Action:      "Allow",
		Description: strings.Repeat("very long description ", 8),
	}}
	view.refreshTable()
	view.syncVisibleRows()
	view.rules.Focus()
	view.activePane = paneDescribe
	view.descView.SetContent(core.DescribeFirewallRule(view.ruleData[0]))

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
