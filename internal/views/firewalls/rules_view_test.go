package firewalls

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
)

func TestFirewallRuleActionsHideGCPOnlyItemsOutsideGCP(t *testing.T) {
	awsActions := core.FirewallRuleActionsForProvider("AWS")
	for _, action := range awsActions {
		if action.Title == "Enable" || action.Title == "Disable" {
			t.Fatalf("unexpected GCP-only action %q in AWS actions", action.Title)
		}
	}

	gcpActions := core.FirewallRuleActionsForProvider("GCP")
	foundEnable := false
	foundDisable := false
	for _, action := range gcpActions {
		if action.Title == "Enable" {
			foundEnable = true
		}
		if action.Title == "Disable" {
			foundDisable = true
		}
	}
	if !foundEnable || !foundDisable {
		t.Fatalf("expected GCP actions to include enable/disable, got %#v", gcpActions)
	}
}

func TestFirewallMutationBlockedInCliMode(t *testing.T) {
	view := NewRules(&config.AppConfig{Backend: "cli"}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.activeCtx.Provider = "AWS"
	view.group = core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"}
	view.descView.SetContent("")

	blocked := view.blockFirewallMutationIfNeeded("Edit", core.FirewallRule{ID: "rule-1", Name: "rule-1"})
	if !blocked {
		t.Fatal("expected CLI mode firewall mutation to be blocked")
	}
	if view.activePane != paneDescribe {
		t.Fatalf("expected blocked mutation to open describe pane, got %d", view.activePane)
	}
	if !strings.Contains(view.descView.View(), "SDK mode") {
		t.Fatalf("expected SDK mode warning in describe pane, got:\n%s", view.descView.View())
	}
}

func TestGCPPolicyFirewallMutationBlocked(t *testing.T) {
	view := NewRules(&config.AppConfig{Backend: "sdk"}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.activeCtx.Provider = "GCP"
	view.group = core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"}
	view.descView.SetContent("")

	blocked := view.blockFirewallMutationIfNeeded("Edit", core.FirewallRule{
		ID:         "policy-900-0",
		Name:       "policy-900-0",
		ResourceID: "global-policy",
		NetworkID:  "prod-vpc",
		Provider:   "GCP",
	})
	if !blocked {
		t.Fatal("expected GCP policy-derived firewall mutation to be blocked")
	}
	if view.activePane != paneDescribe {
		t.Fatalf("expected blocked policy mutation to open describe pane, got %d", view.activePane)
	}
	if !strings.Contains(view.descView.View(), "read-only") {
		t.Fatalf("expected read-only warning in describe pane, got:\n%s", view.descView.View())
	}
}

func TestAddRuleEnterAdvancesBetweenFields(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.setupAddForm()

	updated, cmd := view.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(*RulesView)

	if next.focusIndex != 1 {
		t.Fatalf("expected enter to move focus to next add-rule field, got %d", next.focusIndex)
	}
	if next.activePane != paneAddRule {
		t.Fatalf("expected to remain in add-rule pane, got %d", next.activePane)
	}
	if cmd == nil {
		t.Fatal("expected enter on a non-final field to refocus the next input")
	}
}

func TestAddRuleEnterSubmitsOnFinalField(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.setupAddForm()
	view.focusIndex = len(view.editInputs) - 1

	updated, cmd := view.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(*RulesView)

	if !next.loading {
		t.Fatal("expected add-rule submit to start execution")
	}
	if cmd == nil {
		t.Fatal("expected add-rule submit to return an execution command")
	}
}

func TestEditRuleOpensTerminalForm(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.activeCtx.Provider = "AWS"
	view.ruleData = []core.FirewallRule{{
		ID:           "sgr-1",
		Name:         "sgr-1",
		Direction:    "Inbound",
		Protocol:     "tcp",
		PortRange:    "443",
		Source:       "0.0.0.0/0",
		Destination:  "web-sg",
		Action:       "Allow",
		Provider:     "AWS",
		ResourceID:   "sg-1",
		ResourceName: "web-sg",
	}}
	view.visibleRules = []core.FirewallRule{view.ruleData[0]}
	view.width = 120
	view.height = 24
	view.refreshTable()
	view.rules.SetCursor(0)
	view.rules.Focus()

	updated, cmd := view.handleTableKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	next := updated.(*RulesView)

	if next.activePane != paneEditRule {
		t.Fatalf("expected edit pane to open, got %d", next.activePane)
	}
	if len(next.editInputs) != 5 {
		t.Fatalf("expected compact AWS edit form, got %d fields", len(next.editInputs))
	}
	if next.editInputs[1].Value() != "tcp" {
		t.Fatalf("expected protocol field to be prefilled, got %q", next.editInputs[1].Value())
	}
	if next.editInputs[3].Value() != "0.0.0.0/0" {
		t.Fatalf("expected peer field to be prefilled, got %q", next.editInputs[3].Value())
	}
	if !strings.Contains(next.Render(), "Edit Security Group Rule") {
		t.Fatalf("expected rule edit title in render, got:\n%s", next.Render())
	}
	if cmd == nil {
		t.Fatal("expected edit hotkey to focus the first form input")
	}
}

func TestEditRuleFormSubmitExecutesFirewallAction(t *testing.T) {
	origFirewallLookup := getFirewallProvider
	origViewLookup := getProvider
	defer func() {
		getFirewallProvider = origFirewallLookup
		getProvider = origViewLookup
	}()

	var executed bool
	var gotAction string
	var gotRule core.FirewallRule

	mock := &providers.MockProvider{
		ExecuteFirewallActionFn: func(_ context.Context, action string, rule core.FirewallRule, _ core.CloudContext) (string, error) {
			executed = true
			gotAction = action
			gotRule = rule
			return "updated", nil
		},
		FetchFirewallRulesFn: func(_ context.Context, _ string, _ core.CloudContext) ([]core.FirewallRule, error) {
			return []core.FirewallRule{}, nil
		},
	}
	getFirewallProvider = func(config.AppConfig) providers.Provider { return mock }
	getProvider = func(config.AppConfig) providers.Provider { return mock }

	view := NewRules(&config.AppConfig{Backend: "sdk"}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc", Provider: "AWS"})
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "acct", Region: "us-east-1"}
	view.ruleData = []core.FirewallRule{{
		ID:           "sgr-1",
		Name:         "sgr-1",
		Direction:    "Inbound",
		Protocol:     "tcp",
		PortRange:    "443",
		Source:       "0.0.0.0/0",
		Destination:  "web-sg",
		Action:       "Allow",
		Provider:     "AWS",
		ResourceID:   "sg-1",
		ResourceName: "web-sg",
		OriginalRule: &core.FirewallRule{
			ID:           "sgr-1",
			Name:         "sgr-1",
			Direction:    "Inbound",
			Protocol:     "tcp",
			PortRange:    "443",
			Source:       "0.0.0.0/0",
			Destination:  "web-sg",
			Action:       "Allow",
			Provider:     "AWS",
			ResourceID:   "sg-1",
			ResourceName: "web-sg",
		},
	}}
	view.visibleRules = []core.FirewallRule{view.ruleData[0]}
	view.width = 120
	view.height = 24
	view.refreshTable()
	view.rules.SetCursor(0)
	view.rules.Focus()

	updated, cmd := view.handleTableKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	next := updated.(*RulesView)
	next.editInputs[1].SetValue("udp")
	next.editInputs[2].SetValue("53")
	next.editInputs[3].SetValue("10.0.0.0/8")
	next.editInputs[4].SetValue("dns access")

	if cmd == nil {
		t.Fatal("expected edit hotkey to focus the edit form")
	}

	updated2, cmd2 := next.handleEditKeys(tea.KeyMsg{Type: tea.KeyF2})
	if cmd2 == nil {
		t.Fatal("expected f2 to submit the edit form")
	}
	msg := cmd2()
	afterCmd, _ := updated2.(*RulesView).Update(msg)
	final := afterCmd.(*RulesView)

	if !executed {
		t.Fatal("expected JSON submit to execute the firewall provider")
	}
	if gotAction != "Edit" {
		t.Fatalf("expected Edit action, got %q", gotAction)
	}
	if gotRule.Protocol != "udp" {
		t.Fatalf("expected edited rule to carry updated protocol, got %#v", gotRule)
	}
	if gotRule.PortRange != "53" || gotRule.Source != "10.0.0.0/8" {
		t.Fatalf("expected edited rule to carry updated ports and peer, got %#v", gotRule)
	}
	if gotRule.Action != "Allow" {
		t.Fatalf("expected AWS SG edits to stay Allow, got %#v", gotRule)
	}
	if final.activePane != paneTable {
		t.Fatalf("expected edit completion to return to table, got %d", final.activePane)
	}
}

func TestEditRuleInvalidFormBlocksSubmit(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.activeCtx.Provider = "AWS"
	view.pendingRule = core.FirewallRule{
		ID:           "sgr-1",
		Direction:    "Inbound",
		Protocol:     "tcp",
		PortRange:    "443",
		Source:       "0.0.0.0/0",
		Destination:  "web-sg",
		Action:       "Allow",
		Provider:     "AWS",
		ResourceID:   "sg-1",
		ResourceName: "web-sg",
	}
	view.setupEditForm(view.pendingRule)
	view.editInputs[0].SetValue("sideways")

	updated, cmd := view.handleEditKeys(tea.KeyMsg{Type: tea.KeyF2})
	next := updated.(*RulesView)

	if cmd != nil {
		t.Fatal("expected invalid form input to block submit command")
	}
	if next.activePane != paneEditRule {
		t.Fatalf("expected to remain in edit pane after invalid input, got %d", next.activePane)
	}
	if !strings.Contains(next.formError, "direction") {
		t.Fatalf("expected validation error, got %q", next.formError)
	}
}

func TestEditRuleProviderErrorKeepsFormOpen(t *testing.T) {
	origFirewallLookup := getFirewallProvider
	origViewLookup := getProvider
	defer func() {
		getFirewallProvider = origFirewallLookup
		getProvider = origViewLookup
	}()

	mock := &providers.MockProvider{
		ExecuteFirewallActionFn: func(_ context.Context, action string, rule core.FirewallRule, _ core.CloudContext) (string, error) {
			return "", fmt.Errorf("aws rejected security group update")
		},
		FetchFirewallRulesFn: func(_ context.Context, _ string, _ core.CloudContext) ([]core.FirewallRule, error) {
			return []core.FirewallRule{}, nil
		},
	}
	getFirewallProvider = func(config.AppConfig) providers.Provider { return mock }
	getProvider = func(config.AppConfig) providers.Provider { return mock }

	view := NewRules(&config.AppConfig{Backend: "sdk"}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc", Provider: "AWS"})
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "acct", Region: "us-east-1"}
	view.setupEditForm(core.FirewallRule{
		ID:           "sgr-1",
		Direction:    "Inbound",
		Protocol:     "tcp",
		PortRange:    "443",
		Source:       "0.0.0.0/0",
		Destination:  "web-sg",
		Action:       "Allow",
		Provider:     "AWS",
		ResourceID:   "sg-1",
		ResourceName: "web-sg",
	})

	updated, cmd := view.handleEditKeys(tea.KeyMsg{Type: tea.KeyF2})
	if cmd == nil {
		t.Fatal("expected edit submit to return an execution command")
	}
	msg := cmd()
	afterCmd, _ := updated.(*RulesView).Update(msg)
	final := afterCmd.(*RulesView)

	if final.activePane != paneEditRule {
		t.Fatalf("expected failed provider save to keep edit pane open, got %d", final.activePane)
	}
	if !strings.Contains(final.formError, "aws rejected security group update") {
		t.Fatalf("expected provider error to remain visible, got %q", final.formError)
	}
}

func TestRulesEditPaneFitsWindow(t *testing.T) {
	view := NewRules(&config.AppConfig{}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc"})
	view.width = 96
	view.height = 22
	rule := core.FirewallRule{
		ID:           "sgr-123",
		ResourceID:   "sg-main",
		ResourceName: "web-sg",
		NetworkID:    "vpc-123",
		Provider:     "AWS",
		Direction:    "Inbound",
		Protocol:     "tcp",
		PortRange:    "443",
		Source:       "0.0.0.0/0",
		Destination:  "web-sg",
		Action:       "Allow",
		Description:  strings.Repeat("long ", 8),
	}

	view.setupEditForm(rule)
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
	if !strings.Contains(rendered, "Terminal edit for AWS security-group rules") {
		t.Fatalf("expected terminal edit helper text, got:\n%s", rendered)
	}
}

func TestAddRuleSubmitExecutesFirewallAction(t *testing.T) {
	origFirewallLookup := getFirewallProvider
	origViewLookup := getProvider
	defer func() {
		getFirewallProvider = origFirewallLookup
		getProvider = origViewLookup
	}()

	var executed bool
	var gotAction string
	var gotRule core.FirewallRule

	mock := &providers.MockProvider{
		ExecuteFirewallActionFn: func(_ context.Context, action string, rule core.FirewallRule, _ core.CloudContext) (string, error) {
			executed = true
			gotAction = action
			gotRule = rule
			return "added", nil
		},
		FetchFirewallRulesFn: func(_ context.Context, _ string, _ core.CloudContext) ([]core.FirewallRule, error) {
			return []core.FirewallRule{}, nil
		},
	}
	getFirewallProvider = func(config.AppConfig) providers.Provider { return mock }
	getProvider = func(config.AppConfig) providers.Provider { return mock }

	view := NewRules(&config.AppConfig{Backend: "sdk"}, core.SecurityGroup{Name: "prod-vpc", ID: "prod-vpc", Provider: "AWS"})
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "acct", Region: "us-east-1"}
	view.setupAddForm()
	view.editInputs[0].SetValue("Inbound")
	view.editInputs[1].SetValue("Allow")
	view.editInputs[2].SetValue("tcp")
	view.editInputs[3].SetValue("443")
	view.editInputs[4].SetValue("0.0.0.0/0")
	view.focusIndex = len(view.editInputs) - 1

	updated, cmd := view.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected add submit to return an execution command")
	}
	msg := cmd()
	afterCmd, _ := updated.(*RulesView).Update(msg)
	final := afterCmd.(*RulesView)

	if !executed {
		t.Fatal("expected add submit to execute the firewall provider")
	}
	if gotAction != "Add" {
		t.Fatalf("expected Add action, got %q", gotAction)
	}
	if gotRule.Protocol != "tcp" || gotRule.PortRange != "443" {
		t.Fatalf("unexpected add rule payload: %#v", gotRule)
	}
	if final.activePane != paneTable {
		t.Fatalf("expected add completion to return to table, got %d", final.activePane)
	}
}

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
