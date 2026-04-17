package firewalls

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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

type ruleActionCompleteMsg struct {
	requestKey string
	msg        string
	err        error
}

type RulesView struct {
	rules       table.Model
	descView    viewport.Model
	searchInput textinput.Model
	actions     list.Model
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

	pendingAction actionItem
	pendingRule   core.FirewallRule

	editInputs    []textinput.Model
	focusIndex    int
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

	var actionItems []list.Item
	for _, a := range core.FirewallRuleActions() {
		actionItems = append(actionItems, actionItem{title: a.Title, desc: a.Description})
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actionList := list.New(actionItems, actionDelegate, 30, 15)
	actionList.Title = "Rule Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	tbl, cols, _, canScrollLeft, canScrollRight := createFirewallRulesTable(80, 0)
	return &RulesView{
		rules:          tbl,
		descView:       descView,
		searchInput:    searchInput,
		actions:        actionList,
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
	return "\u2191\u2193: Nav \u2022 \u2190\u2192: Pan \u2022 e: Edit \u2022 d: Describe \u2022 ctrl+d: Delete \u2022 x: Toggle \u2022 Enter: Menu \u2022 /: Search"
}

func (v *RulesView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneDescribe || v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneEditRule
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
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
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
		case paneActions:
			_, cmd = v.handleActionKeys(msg)
		case paneConfirm:
			_, cmd = v.handleConfirmKeys(msg)
		case paneEditRule:
			_, cmd = v.handleEditKeys(msg)
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

	case ruleActionCompleteMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.activePane = paneTable
		if msg.err != nil {
			applog.Errorf("component=firewall_rules event=action_failed err=%v", msg.err)
		} else {
			applog.Infof("component=firewall_rules event=action_success msg=%s", msg.msg)
		}
		cmds = append(cmds, v.fetchRulesCmd())

	case tea.WindowSizeMsg:
		v.Resize(msg.Width, msg.Height, v.showSidebar)
	}

	switch v.activePane {
	case paneTable:
		v.rules, cmd = v.rules.Update(msg)
		v.applySelectionStyle()
		cmds = append(cmds, cmd)
	case paneActions:
		v.actions, cmd = v.actions.Update(msg)
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

	if v.activePane == paneActions {
		overlay := ui.OverlayStyle.Render(v.actions.View())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}
	if v.activePane == paneConfirm {
		confirmMsg := fmt.Sprintf("Are you sure you want to %s rule %s?", v.pendingAction.title, v.pendingRule.ID)
		confirmStyle := ui.OverlayStyle.Copy().BorderForeground(ui.Alert).Padding(1, 2).Width(50)
		confirmView := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ui.Alert).Bold(true).Render("⚠️  CONFIRM ACTION"),
			"\n", lipgloss.NewStyle().Align(lipgloss.Center).Render(confirmMsg),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Confirm \u2022 Esc: Cancel"),
		)
		overlay := confirmStyle.Render(confirmView)
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}

	if v.activePane == paneEditRule {
		var b strings.Builder
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Highlight).Bold(true).Render("📝 EDIT FIREWALL RULE\n\n"))
		for i := range v.editInputs {
			b.WriteString(v.editInputs[i].View())
			if i < len(v.editInputs)-1 {
				b.WriteString("\n\n")
			}
		}
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(ui.Subtle).Render("Tab/Shift+Tab: Navigate • Enter: Submit • Esc: Cancel"))
		
		overlay := ui.OverlayStyle.Copy().Width(50).Padding(1, 2).Render(b.String())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
}

func (v *RulesView) handleEditKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "tab", "shift+tab", "up", "down":
		s := msg.String()
		if s == "up" || s == "shift+tab" {
			v.focusIndex--
		} else {
			v.focusIndex++
		}

		if v.focusIndex > len(v.editInputs)-1 {
			v.focusIndex = 0
		} else if v.focusIndex < 0 {
			v.focusIndex = len(v.editInputs) - 1
		}

		var cmds []tea.Cmd
		for i := 0; i <= len(v.editInputs)-1; i++ {
			if i == v.focusIndex {
				cmds = append(cmds, v.editInputs[i].Focus())
				v.editInputs[i].PromptStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
				v.editInputs[i].TextStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
			} else {
				v.editInputs[i].Blur()
				v.editInputs[i].PromptStyle = lipgloss.NewStyle()
				v.editInputs[i].TextStyle = lipgloss.NewStyle()
			}
		}
		return v, tea.Batch(cmds...)
	case "enter":
		if v.focusIndex == len(v.editInputs)-1 {
			// Submitted the form!
			updatedRule := *v.pendingRule.OriginalRule
			updatedRule.Protocol = v.editInputs[0].Value()
			updatedRule.PortRange = v.editInputs[1].Value()
			if v.pendingRule.Direction == "Inbound" {
				updatedRule.Source = v.editInputs[2].Value()
			} else {
				updatedRule.Destination = v.editInputs[2].Value()
			}
			
			v.pendingRule = updatedRule
			v.loading = true
			return v, v.executeRuleActionCmd()
		}
	}

	var cmd tea.Cmd
	v.editInputs[v.focusIndex], cmd = v.editInputs[v.focusIndex].Update(msg)
	return v, cmd
}

func (v *RulesView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	rule, ok := v.selectedRule()

	switch msg.String() {
	case "enter":
		if !ok {
			return v, nil
		}
		v.activePane = paneActions
	case "d":
		if !ok { return v, nil }
		v.descView.SetContent(core.DescribeFirewallRule(rule))
		v.activePane = paneDescribe
	case "e":
		if !ok { return v, nil }
		return v.setupEditForm(rule)
	case "ctrl+d":
		if !ok { return v, nil }
		v.pendingAction = actionItem{title: "Delete", desc: "Delete this firewall rule"}
		v.pendingRule = rule
		v.activePane = paneConfirm
	case "x":
		if !ok { return v, nil }
		// Determine toggle action
		actionTitle := "Disable"
		if strings.Contains(strings.ToLower(rule.Description), "disabled") || strings.Contains(strings.ToLower(rule.Action), "disabled") {
			actionTitle = "Enable"
		}
		v.pendingAction = actionItem{title: actionTitle, desc: actionTitle + " this firewall rule"}
		v.pendingRule = rule
		v.activePane = paneConfirm
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

func (v *RulesView) setupEditForm(rule core.FirewallRule) (ui.View, tea.Cmd) {
	v.pendingRule = rule
	v.pendingRule.OriginalRule = &rule
	v.pendingAction = actionItem{title: "Edit", desc: "Edit rule"}

	i1 := textinput.New()
	i1.Prompt = "Protocol: "
	i1.Placeholder = "tcp, udp, all"
	i1.SetValue(rule.Protocol)
	i1.Focus()
	i1.PromptStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
	i1.TextStyle = lipgloss.NewStyle().Foreground(ui.Highlight)

	i2 := textinput.New()
	i2.Prompt = "Ports: "
	i2.Placeholder = "e.g. 80, 443, 80-90, all"
	i2.SetValue(rule.PortRange)

	i3 := textinput.New()
	if rule.Direction == "Inbound" {
		i3.Prompt = "Source IP/CIDR: "
		i3.SetValue(rule.Source)
	} else {
		i3.Prompt = "Dest IP/CIDR: "
		i3.SetValue(rule.Destination)
	}
	i3.Placeholder = "0.0.0.0/0"

	v.editInputs = []textinput.Model{i1, i2, i3}
	v.focusIndex = 0
	v.activePane = paneEditRule
	return v, textinput.Blink
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

func (v *RulesView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
	case "enter":
		if v.actions.SelectedItem() == nil {
			return v, nil
		}
		item := v.actions.SelectedItem().(actionItem)
		rule, ok := v.selectedRule()
		if !ok {
			return v, nil
		}

		if item.title == "Describe" {
			v.descView.SetContent(core.DescribeFirewallRule(rule))
			v.activePane = paneDescribe
			return v, nil
		}

		if item.title == "Edit" {
			return v.setupEditForm(rule)
		}

		v.pendingAction = item
		v.pendingRule = rule
		v.activePane = paneConfirm
	}
	return v, nil
}

func (v *RulesView) handleConfirmKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		v.activePane = paneTable
	case "enter", "y":
		v.loading = true
		return v, v.executeRuleActionCmd()
	}
	return v, nil
}

func (v *RulesView) executeRuleActionCmd() tea.Cmd {
	requestKey := v.requestKey
	activeCtx := v.activeCtx
	action := v.pendingAction.title
	rule := v.pendingRule
	cfg := *v.cfg
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		firewallProvider, ok := provider.(providers.FirewallProvider)
		if !ok {
			return ruleActionCompleteMsg{requestKey: requestKey, err: fmt.Errorf("FirewallProvider not implemented")}
		}
		msg, err := firewallProvider.ExecuteFirewallAction(context.Background(), action, rule, activeCtx)
		return ruleActionCompleteMsg{requestKey: requestKey, msg: msg, err: err}
	}
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
