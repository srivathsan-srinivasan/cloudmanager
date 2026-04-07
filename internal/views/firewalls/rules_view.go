package firewalls

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
)

type firewallRulesFetchMsg struct {
	requestKey string
	rules      []core.FirewallRule
	err        error
}

type RulesView struct {
	rules       table.Model
	descView    viewport.Model
	searchInput textinput.Model
	activePane  int

	ruleData       []core.FirewallRule
	visibleRules   []core.FirewallRule
	group          core.SecurityGroup
	activeCtx      core.CloudContext
	cfg            *config.AppConfig
	loading        bool
	isSearching    bool
	breadcrumbs    string
	requestKey     string
	width, height  int
	showSidebar    bool
	displayCols    []table.Column
	columnOffset   int
	canScrollLeft  bool
	canScrollRight bool
}

func NewRules(cfg *config.AppConfig, group core.SecurityGroup) *RulesView {
	descView := viewport.New(80, 20)
	descView.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search firewall rules..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	tbl, cols, _, canScrollLeft, canScrollRight := createFirewallRulesTable(80, 0)
	return &RulesView{
		rules:          tbl,
		descView:       descView,
		searchInput:    searchInput,
		group:          group,
		cfg:            cfg,
		activePane:     paneTable,
		displayCols:    cols,
		canScrollLeft:  canScrollLeft,
		canScrollRight: canScrollRight,
	}
}

func (v *RulesView) Title() string { return "Firewall Rules" }

func (v *RulesView) ShortHelp() string {
	return "\u2191\u2193: Navigate \u2022 \u2190\u2192: Pan \u2022 Enter: Describe \u2022 /: Search \u2022 Esc: Back"
}

func (v *RulesView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneDescribe
}

func (v *RulesView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.ruleData = nil
	v.visibleRules = nil
	v.columnOffset = 0
	v.requestKey = fmt.Sprintf("%s:%s", ctx.CacheKey(), v.group.ID)
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s", v.group.Name, v.group.ID)
	v.loading = true
	v.refreshTable()
	v.rules.Focus()
	return v.fetchRulesCmd()
}

func (v *RulesView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.refreshTable()
	v.descView.Width = width - 4
	v.descView.Height = height - 4
}

func (v *RulesView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		if msg.String() == "esc" && v.activePane == paneDescribe {
			v.activePane = paneTable
			return v, nil
		}
		switch v.activePane {
		case paneDescribe:
			_, cmd = v.handleDescribeKeys(msg)
		default:
			_, cmd = v.handleTableKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case firewallRulesFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.ruleData = msg.rules
		if msg.err != nil {
			applog.Errorf("component=firewall_rules event=fetch_failed provider=%s account=%s region=%s mode=%s group=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), v.group.ID, msg.err)
			v.visibleRules = nil
			v.rules.SetRows([]table.Row{})
		} else {
			applog.Infof("component=firewall_rules event=fetch_completed provider=%s account=%s region=%s mode=%s group=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), v.group.ID, len(msg.rules))
			v.syncVisibleRows()
		}

	case tea.WindowSizeMsg:
		v.Resize(msg.Width, msg.Height, v.showSidebar)
	}

	switch v.activePane {
	case paneTable:
		v.rules, cmd = v.rules.Update(msg)
		v.applySelectionStyle()
		cmds = append(cmds, cmd)
	case paneDescribe:
		v.descView, cmd = v.descView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *RulesView) Render() string {
	header := ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)
	body := v.rules.View()
	if v.loading {
		body = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading firewall rules...")
	} else if v.isSearching || v.searchInput.Value() != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(v.searchInput.View()),
			body,
		)
	}
	if v.activePane == paneDescribe {
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	}
	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
}

func (v *RulesView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		rule, ok := v.selectedRule()
		if !ok {
			return v, nil
		}
		v.descView.SetContent(core.DescribeFirewallRule(rule))
		v.activePane = paneDescribe
	case "/":
		v.isSearching = true
		v.searchInput.Focus()
	case "left", "h":
		if v.columnOffset > 0 {
			v.columnOffset--
			v.refreshTable()
		}
	case "right", "l":
		if v.canScrollRight {
			v.columnOffset++
			v.refreshTable()
		}
	case "r":
		v.loading = true
		return v, v.fetchRulesCmd()
	}
	return v, nil
}

func (v *RulesView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		v.isSearching = false
		v.searchInput.Blur()
		v.syncVisibleRows()
		return v, nil
	}
	var cmd tea.Cmd
	v.searchInput, cmd = v.searchInput.Update(msg)
	v.syncVisibleRows()
	return v, cmd
}

func (v *RulesView) handleDescribeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() == "esc" {
		v.activePane = paneTable
	}
	return v, nil
}

func (v *RulesView) fetchRulesCmd() tea.Cmd {
	requestKey := v.requestKey
	groupID := v.group.ID
	activeCtx := v.activeCtx
	provider := getProvider(*v.cfg)
	mode := strings.ToUpper(v.cfg.Backend)
	return func() tea.Msg {
		applog.Infof("component=firewall_rules event=fetch_start provider=%s account=%s region=%s mode=%s group=%s", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, groupID)
		firewallProvider, ok := provider.(providers.FirewallProvider)
		if !ok {
			return firewallRulesFetchMsg{requestKey: requestKey, err: fmt.Errorf("FirewallProvider not implemented")}
		}
		rules, err := firewallProvider.FetchFirewallRules(context.Background(), groupID, activeCtx)
		return firewallRulesFetchMsg{requestKey: requestKey, rules: rules, err: err}
	}
}

func (v *RulesView) refreshTable() {
	if v.width == 0 {
		return
	}
	cursor := v.rules.Cursor()
	focused := v.rules.Focused()
	tbl, cols, nextOffset, canScrollLeft, canScrollRight := createFirewallRulesTable(v.width, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	rows := mapFirewallRulesToRows(v.visibleRowsSource(), cols)
	tbl.SetRows(rows)
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	if cursor >= 0 && cursor < len(rows) {
		tbl.SetCursor(cursor)
	}
	if focused {
		tbl.Focus()
	}
	v.rules = tbl
	v.displayCols = cols
	v.applySelectionStyle()
}

func (v *RulesView) syncVisibleRows() {
	v.visibleRules = filterFirewallRules(v.ruleData, v.searchInput.Value())
	rows := mapFirewallRulesToRows(v.visibleRules, v.displayCols)
	v.rules.SetRows(rows)
	if len(rows) == 0 {
		v.rules.SetCursor(0)
		v.applySelectionStyle()
		return
	}
	if cursor := v.rules.Cursor(); cursor >= len(rows) {
		v.rules.SetCursor(len(rows) - 1)
	}
	v.applySelectionStyle()
}

func (v *RulesView) visibleRowsSource() []core.FirewallRule {
	if v.visibleRules != nil {
		return v.visibleRules
	}
	return filterFirewallRules(v.ruleData, v.searchInput.Value())
}

func (v *RulesView) selectedRule() (core.FirewallRule, bool) {
	cursor := v.rules.Cursor()
	if cursor < 0 || cursor >= len(v.visibleRules) {
		return core.FirewallRule{}, false
	}
	return v.visibleRules[cursor], true
}

func (v *RulesView) applySelectionStyle() {
	styles := ui.DefaultTableStyles()
	if rule, ok := v.selectedRule(); ok && core.IsRuleRisky(rule) {
		styles = ui.AlertSelectedTableStyles()
	}
	v.rules.SetStyles(styles)
}

func filterFirewallRules(rules []core.FirewallRule, query string) []core.FirewallRule {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		filtered := make([]core.FirewallRule, len(rules))
		copy(filtered, rules)
		return filtered
	}

	var filtered []core.FirewallRule
	for _, rule := range rules {
		if strings.Contains(strings.ToLower(rule.Direction), normalized) ||
			strings.Contains(strings.ToLower(rule.Protocol), normalized) ||
			strings.Contains(strings.ToLower(rule.PortRange), normalized) ||
			strings.Contains(strings.ToLower(rule.Source), normalized) ||
			strings.Contains(strings.ToLower(rule.Destination), normalized) ||
			strings.Contains(strings.ToLower(rule.Description), normalized) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func mapFirewallRulesToRows(rules []core.FirewallRule, cols []table.Column) []table.Row {
	rows := make([]table.Row, 0, len(rules))
	for _, rule := range rules {
		row := make(table.Row, 0, len(cols))
		direction := rule.GetField("Direction")
		action := rule.GetField("Action")
		switch {
		case core.IsRuleRisky(rule):
			direction = "⚠ " + direction
		case strings.EqualFold(rule.Action, "Deny"):
			action = "⊘ " + action
		}
		for _, col := range cols {
			value := rule.GetField(col.Title)
			switch col.Title {
			case "Direction":
				value = direction
			case "Action":
				value = action
			}
			row = append(row, ui.TruncateText(value, col.Width))
		}
		rows = append(rows, row)
	}
	return rows
}

func createFirewallRulesTable(availableWidth, offset int) (table.Model, []table.Column, int, bool, bool) {
	cols := []table.Column{
		{Title: "Direction", Width: 10},
		{Title: "Protocol", Width: 9},
		{Title: "Ports", Width: 8},
		{Title: "Source", Width: 22},
		{Title: "Destination", Width: 22},
		{Title: "Action", Width: 8},
		{Title: "Description", Width: 22},
	}
	return ui.NewResourceTable(cols, availableWidth, offset, "Direction")
}
