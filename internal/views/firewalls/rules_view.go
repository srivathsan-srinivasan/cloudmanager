package firewalls

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	applog "github.com/srivathsan-srinivasan/cloudmanager/internal/logging"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/ui"
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

type ruleEditField struct {
	prompt      string
	placeholder string
	value       func(core.FirewallRule) string
	apply       func(*core.FirewallRule, string) error
}

var getFirewallProvider = providers.GetProvider

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

	editFields []ruleEditField
	editInputs []textinput.Model
	focusIndex int
	formError  string
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
	for _, a := range core.FirewallRuleActionsForProvider(group.Provider) {
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
	return "\u2191\u2193: Nav \u2022 \u2190\u2192: Pan \u2022 a: Add \u2022 e: Edit Rule \u2022 d: Describe \u2022 ctrl+d: Delete \u2022 x: Toggle \u2022 Enter: Menu \u2022 /: Search"
}

func (v *RulesView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneDescribe || v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneEditRule || v.activePane == paneAddRule
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
	v.syncActionItems()
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
	if v.activePane == paneEditRule {
		v.syncEditInputLayout()
	}
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
		case paneAddRule:
			_, cmd = v.handleAddKeys(msg)
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
		if msg.err != nil {
			applog.Errorf("component=firewall_rules event=action_failed err=%v", msg.err)
			switch v.pendingAction.title {
			case "Edit":
				v.formError = msg.err.Error()
				v.activePane = paneEditRule
			case "Add":
				v.formError = msg.err.Error()
				v.activePane = paneAddRule
			default:
				v.descView.SetContent(msg.err.Error())
				v.activePane = paneDescribe
			}
		} else {
			applog.Infof("component=firewall_rules event=action_success msg=%s", msg.msg)
			v.formError = ""
			v.activePane = paneTable
			cmds = append(cmds, v.fetchRulesCmd())
		}

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
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(v.rules.View()), overlay)
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
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
		body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(v.rules.View()), overlay)
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	}

	if v.activePane == paneEditRule || v.activePane == paneAddRule {
		if v.activePane == paneAddRule {
			var b strings.Builder
			b.WriteString(lipgloss.NewStyle().Foreground(ui.Highlight).Bold(true).Render("➕ ADD FIREWALL RULE\n\n"))
			for i := range v.editInputs {
				b.WriteString(v.editInputs[i].View())
				if i < len(v.editInputs)-1 {
					b.WriteString("\n\n")
				}
			}
			b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(ui.Subtle).Render("Tab/Shift+Tab: Navigate • Enter: Submit • Esc: Cancel"))
			overlay := ui.OverlayStyle.Copy().Width(50).Padding(1, 2).Render(b.String())
			body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().MaxWidth(v.width-55).Render(v.rules.View()), overlay)
			return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
		}
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", v.renderEditRuleForm()), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
}

func (v *RulesView) refreshTable() {
	if v.width == 0 {
		return
	}

	availWidth := v.width
	if v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneAddRule {
		availWidth = v.width - 55
		if availWidth < 40 {
			availWidth = 40
		}
	}

	cursor := v.rules.Cursor()
	focused := v.rules.Focused()
	tbl, cols, nextOffset, canScrollLeft, canScrollRight := createFirewallRulesTable(availWidth, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(availWidth))
	rows := mapFirewallRulesToRows(v.visibleRules, cols)
	tbl.SetRows(rows)
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

func (v *RulesView) handleEditKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		v.formError = ""
		v.refreshTable()
		return v, nil
	case "tab", "shift+tab", "up", "down":
		v.shiftEditFocus(msg.String() == "up" || msg.String() == "shift+tab")
		return v, v.focusEditInput()
	case "enter":
		if v.focusIndex < len(v.editInputs)-1 {
			v.shiftEditFocus(false)
			return v, v.focusEditInput()
		}
		return v, v.submitEditForm()
	case "f2", "ctrl+s":
		return v, v.submitEditForm()
	}

	var cmd tea.Cmd
	v.editInputs[v.focusIndex], cmd = v.editInputs[v.focusIndex].Update(msg)
	v.formError = ""
	return v, cmd
}

func (v *RulesView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	rule, ok := v.selectedRule()

	switch msg.String() {
	case "enter":
		if !ok {
			return v, nil
		}
		v.syncActionItems()
		v.activePane = paneActions
		v.refreshTable()
	case "a":
		if v.blockFirewallMutationIfNeeded("Add", core.FirewallRule{}) {
			return v, nil
		}
		v.setupAddForm()
		v.refreshTable()
		return v, textinput.Blink
	case "d":
		if !ok {
			return v, nil
		}
		v.descView.SetContent(core.DescribeFirewallRule(rule))
		v.activePane = paneDescribe
		v.refreshTable()
	case "e":
		if !ok {
			return v, nil
		}
		if v.blockFirewallMutationIfNeeded("Edit", rule) {
			return v, nil
		}
		v.setupEditForm(rule)
		v.refreshTable()
		applog.Infof("component=firewall_rules event=edit_opened provider=%s account=%s region=%s mode=%s rule=%s", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), rule.ID)
		return v, v.focusEditInput()
	case "ctrl+d":
		if !ok {
			return v, nil
		}
		if v.blockFirewallMutationIfNeeded("Delete", rule) {
			return v, nil
		}
		v.pendingAction = actionItem{title: "Delete", desc: "Delete this firewall rule"}
		v.pendingRule = rule
		v.activePane = paneConfirm
		v.refreshTable()
	case "x":
		if !ok {
			return v, nil
		}
		if v.blockFirewallMutationIfNeeded("Enable/Disable", rule) {
			return v, nil
		}
		// Determine toggle action
		actionTitle := "Disable"
		if strings.Contains(strings.ToLower(rule.Description), "disabled") || strings.Contains(strings.ToLower(rule.Action), "disabled") {
			actionTitle = "Enable"
		}
		v.pendingAction = actionItem{title: actionTitle, desc: actionTitle + " this firewall rule"}
		v.pendingRule = rule
		v.activePane = paneConfirm
		v.refreshTable()
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
	v.pendingRule = editableFirewallRule(rule)
	v.pendingRule.OriginalRule = &rule
	v.pendingAction = actionItem{title: "Edit", desc: "Edit rule"}
	v.buildEditFormInputs(v.pendingRule)
	v.formError = ""
	v.activePane = paneEditRule
	return v, v.focusEditInput()
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
		v.refreshTable()
	}
	return v, nil
}

func (v *RulesView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		v.refreshTable()
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
			v.refreshTable()
			return v, nil
		}

		if item.title == "Edit" {
			if v.blockFirewallMutationIfNeeded(item.title, rule) {
				return v, nil
			}
			v.setupEditForm(rule)
			v.refreshTable()
			return v, v.focusEditInput()
		}

		if v.blockFirewallMutationIfNeeded(item.title, rule) {
			return v, nil
		}
		v.pendingAction = item
		v.pendingRule = rule
		v.activePane = paneConfirm
		v.refreshTable()
	}
	return v, nil
}

func (v *RulesView) handleConfirmKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		v.activePane = paneTable
		v.refreshTable()
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
		mode := strings.ToUpper(strings.TrimSpace(cfg.Backend))
		fieldName := ""
		if action == "Edit" {
			fieldName = "FORM"
		}
		applog.Infof("component=firewall_rules event=action_start provider=%s account=%s region=%s mode=%s action=%s field=%s rule=%s", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, action, fieldName, rule.ID)
		if strings.EqualFold(cfg.Backend, "cli") && isFirewallMutationAction(action) {
			err := fmt.Errorf("firewall updates require SDK mode; switch to SDK and retry")
			applog.Warnf("component=firewall_rules event=action_blocked provider=%s account=%s region=%s mode=%s action=%s field=%s rule=%s err=%v", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, action, fieldName, rule.ID, err)
			return ruleActionCompleteMsg{requestKey: requestKey, err: err}
		}
		provider := getFirewallProvider(cfg)
		firewallProvider, ok := provider.(providers.FirewallProvider)
		if !ok {
			applog.Errorf("component=firewall_rules event=action_failed provider=%s account=%s region=%s mode=%s action=%s field=%s rule=%s err=firewall provider not implemented", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, action, fieldName, rule.ID)
			return ruleActionCompleteMsg{requestKey: requestKey, err: fmt.Errorf("FirewallProvider not implemented")}
		}
		msg, err := firewallProvider.ExecuteFirewallAction(context.Background(), action, rule, activeCtx)
		if err != nil {
			applog.Errorf("component=firewall_rules event=action_failed provider=%s account=%s region=%s mode=%s action=%s field=%s rule=%s err=%v", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, action, fieldName, rule.ID, err)
		}
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

func (v *RulesView) renderEditRuleForm() string {
	panelWidth := v.editFormPanelWidth()
	bodyWidth := v.editFormBodyWidth()

	title := "Edit Firewall Rule"
	if v.isAWSSecurityGroupRuleEditor() {
		title = "Edit Security Group Rule"
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Highlight).Bold(true).Render(title))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Subtle).Render(v.editFormNote()))
	b.WriteString("\n\n")
	b.WriteString(v.renderRuleFormMeta(bodyWidth))
	b.WriteString("\n\n")
	for i := range v.editInputs {
		b.WriteString(v.editInputs[i].View())
		if i < len(v.editInputs)-1 {
			b.WriteString("\n\n")
		}
	}
	if strings.TrimSpace(v.formError) != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Alert).Render(v.formError))
	}
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Subtle).Render("Tab/Shift+Tab: Navigate • Enter: Next/Save • F2/Ctrl+S: Save • Esc: Cancel"))
	return ui.OverlayStyle.Copy().Width(panelWidth).Padding(1, 2).Render(b.String())
}

func (v *RulesView) renderRuleFormMeta(width int) string {
	fields := []ruleEditField{
		{prompt: "Provider:", value: func(core.FirewallRule) string { return orFallback(v.pendingRule.Provider, "-") }},
		{prompt: "Rule ID:", value: func(core.FirewallRule) string { return orFallback(v.pendingRule.ID, "-") }},
		{prompt: "Resource:", value: func(core.FirewallRule) string {
			return orFallback(v.pendingRule.ResourceName, v.pendingRule.ResourceID)
		}},
		{prompt: "Network:", value: func(core.FirewallRule) string { return orFallback(v.pendingRule.NetworkID, "-") }},
	}

	lines := make([]string, 0, len(fields))
	labelStyle := lipgloss.NewStyle().Foreground(ui.Subtle)
	for _, field := range fields {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render(field.prompt), ui.TruncateText(field.value(v.pendingRule), width-18)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (v *RulesView) editFormPanelWidth() int {
	width := v.width - 2
	if width < 56 {
		width = 56
	}
	return width
}

func (v *RulesView) editFormBodyWidth() int {
	width := v.editFormPanelWidth() - 8
	if width < 40 {
		width = 40
	}
	return width
}

func (v *RulesView) editFormNote() string {
	if v.isAWSSecurityGroupRuleEditor() {
		return "Terminal edit for AWS security-group rules. Peer accepts CIDR, sg-..., or pl-.... Comma-separated ports create separate AWS SG rules."
	}
	return "Terminal edit for firewall rules. Resource identity is preserved automatically."
}

func (v *RulesView) buildEditFormInputs(rule core.FirewallRule) {
	v.editFields = v.editFieldsForRule(rule)
	v.editInputs = make([]textinput.Model, 0, len(v.editFields))
	for idx, field := range v.editFields {
		input := textinput.New()
		input.Prompt = field.prompt
		input.Placeholder = field.placeholder
		input.SetValue(field.value(rule))
		input.CharLimit = 256
		input.Width = v.editFormBodyWidth()
		if idx == 0 {
			input.Focus()
			input.PromptStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
			input.TextStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
		}
		v.editInputs = append(v.editInputs, input)
	}
	v.focusIndex = 0
}

func (v *RulesView) editFieldsForRule(rule core.FirewallRule) []ruleEditField {
	if v.isAWSSecurityGroupRuleEditorForRule(rule) {
		return []ruleEditField{
			{
				prompt:      "Direction: ",
				placeholder: "Inbound or Outbound",
				value:       func(rule core.FirewallRule) string { return defaultDirection(rule.Direction) },
				apply: func(rule *core.FirewallRule, value string) error {
					direction, err := normalizeDirectionValue(value)
					if err != nil {
						return err
					}
					rule.Direction = direction
					rule.Action = "Allow"
					return nil
				},
			},
			{
				prompt:      "Protocol: ",
				placeholder: "tcp, udp, icmp, all",
				value:       func(rule core.FirewallRule) string { return defaultProtocol(rule.Protocol) },
				apply: func(rule *core.FirewallRule, value string) error {
					rule.Protocol = normalizeProtocolValue(value)
					return nil
				},
			},
			{
				prompt:      "Ports: ",
				placeholder: "22, 443, 80-90, all",
				value:       func(rule core.FirewallRule) string { return defaultPorts(rule.PortRange) },
				apply: func(rule *core.FirewallRule, value string) error {
					rule.PortRange = normalizePortRangeValue(value)
					return nil
				},
			},
			{
				prompt:      "Peer: ",
				placeholder: "0.0.0.0/0, sg-..., or pl-...",
				value:       func(rule core.FirewallRule) string { return awsEditablePeerValue(rule) },
				apply: func(rule *core.FirewallRule, value string) error {
					peer := cleanEditableValue(value)
					if peer == "" {
						return fmt.Errorf("peer is required")
					}
					resourceName := orFallback(rule.ResourceName, v.group.Name, rule.ResourceID)
					if strings.EqualFold(strings.TrimSpace(rule.Direction), core.Outbound) {
						rule.Source = resourceName
						rule.Destination = peer
					} else {
						rule.Source = peer
						rule.Destination = resourceName
					}
					rule.Action = "Allow"
					return nil
				},
			},
			{
				prompt:      "Description: ",
				placeholder: "optional",
				value:       func(rule core.FirewallRule) string { return cleanEditableValue(rule.Description) },
				apply: func(rule *core.FirewallRule, value string) error {
					rule.Description = cleanEditableValue(value)
					return nil
				},
			},
		}
	}

	return []ruleEditField{
		{
			prompt:      "Direction: ",
			placeholder: "Inbound or Outbound",
			value:       func(rule core.FirewallRule) string { return defaultDirection(rule.Direction) },
			apply: func(rule *core.FirewallRule, value string) error {
				direction, err := normalizeDirectionValue(value)
				if err != nil {
					return err
				}
				rule.Direction = direction
				return nil
			},
		},
		{
			prompt:      "Action: ",
			placeholder: "Allow or Deny",
			value:       func(rule core.FirewallRule) string { return defaultAction(rule.Action) },
			apply: func(rule *core.FirewallRule, value string) error {
				action, err := normalizeActionValue(value)
				if err != nil {
					return err
				}
				rule.Action = action
				return nil
			},
		},
		{
			prompt:      "Protocol: ",
			placeholder: "tcp, udp, icmp, all",
			value:       func(rule core.FirewallRule) string { return defaultProtocol(rule.Protocol) },
			apply: func(rule *core.FirewallRule, value string) error {
				rule.Protocol = normalizeProtocolValue(value)
				return nil
			},
		},
		{
			prompt:      "Ports: ",
			placeholder: "22, 443, 80-90, all",
			value:       func(rule core.FirewallRule) string { return defaultPorts(rule.PortRange) },
			apply: func(rule *core.FirewallRule, value string) error {
				rule.PortRange = normalizePortRangeValue(value)
				return nil
			},
		},
		{
			prompt:      "Source: ",
			placeholder: "source CIDR / selector",
			value:       func(rule core.FirewallRule) string { return cleanEditableValue(rule.Source) },
			apply: func(rule *core.FirewallRule, value string) error {
				rule.Source = cleanEditableValue(value)
				return nil
			},
		},
		{
			prompt:      "Destination: ",
			placeholder: "destination CIDR / selector",
			value:       func(rule core.FirewallRule) string { return cleanEditableValue(rule.Destination) },
			apply: func(rule *core.FirewallRule, value string) error {
				rule.Destination = cleanEditableValue(value)
				return nil
			},
		},
		{
			prompt:      "Description: ",
			placeholder: "optional",
			value:       func(rule core.FirewallRule) string { return cleanEditableValue(rule.Description) },
			apply: func(rule *core.FirewallRule, value string) error {
				rule.Description = cleanEditableValue(value)
				return nil
			},
		},
		{
			prompt:      "Priority: ",
			placeholder: "0",
			value:       func(rule core.FirewallRule) string { return priorityValue(rule.Priority) },
			apply: func(rule *core.FirewallRule, value string) error {
				priority, err := parsePriorityValue(value)
				if err != nil {
					return err
				}
				rule.Priority = priority
				return nil
			},
		},
	}
}

func (v *RulesView) submitEditForm() tea.Cmd {
	updatedRule := v.pendingRule
	for idx, field := range v.editFields {
		if err := field.apply(&updatedRule, v.editInputs[idx].Value()); err != nil {
			v.formError = err.Error()
			applog.Errorf("component=firewall_rules event=edit_form_invalid provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), err)
			return nil
		}
	}
	updatedRule.OriginalRule = v.pendingRule.OriginalRule
	v.pendingRule = editableFirewallRule(updatedRule)
	v.loading = true
	v.formError = ""
	applog.Infof("component=firewall_rules event=edit_submit provider=%s account=%s region=%s mode=%s field=form rule=%s", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), updatedRule.ID)
	return v.executeRuleActionCmd()
}

func (v *RulesView) syncEditInputLayout() {
	width := v.editFormBodyWidth()
	for i := range v.editInputs {
		v.editInputs[i].Width = width
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (v *RulesView) syncActionItems() {
	actions := make([]list.Item, 0, len(core.FirewallRuleActionsForProvider(v.activeCtx.Provider)))
	for _, a := range core.FirewallRuleActionsForProvider(v.activeCtx.Provider) {
		actions = append(actions, actionItem{title: a.Title, desc: a.Description})
	}
	v.actions.SetItems(actions)
}

func (v *RulesView) blockFirewallMutationIfNeeded(action string, rule core.FirewallRule) bool {
	if reason := firewallMutationReadOnlyReason(rule); reason != "" {
		target := rule.GetName()
		if target == "" {
			target = v.group.Name
		}
		applog.Warnf("component=firewall_rules event=action_blocked provider=%s account=%s region=%s mode=%s action=%s target=%s err=%s", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), action, target, reason)
		v.descView.SetContent(reason)
		v.activePane = paneDescribe
		v.refreshTable()
		return true
	}

	if !strings.EqualFold(strings.TrimSpace(v.cfg.Backend), "cli") {
		return false
	}

	target := rule.GetName()
	if target == "" {
		target = v.group.Name
	}

	msg := fmt.Sprintf(
		"%s requires SDK mode.\n\nCurrent mode: %s\nProvider: %s\nTarget: %s\n\nPress B to switch to SDK mode and retry.",
		action,
		strings.ToUpper(strings.TrimSpace(v.cfg.Backend)),
		v.activeCtx.Provider,
		target,
	)
	applog.Warnf("component=firewall_rules event=action_blocked provider=%s account=%s region=%s mode=%s action=%s target=%s err=firewall updates require SDK mode", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), action, target)
	v.descView.SetContent(msg)
	v.activePane = paneDescribe
	v.refreshTable()
	return true
}

func isFirewallMutationAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "add", "edit", "delete", "enable", "disable", "enable/disable":
		return true
	default:
		return false
	}
}

func firewallMutationReadOnlyReason(rule core.FirewallRule) string {
	if !strings.EqualFold(strings.TrimSpace(rule.Provider), "GCP") {
		return ""
	}
	resourceID := strings.TrimSpace(rule.ResourceID)
	networkID := strings.TrimSpace(rule.NetworkID)
	if resourceID != "" && networkID != "" && resourceID != networkID {
		return "This GCP effective firewall policy rule is read-only here.\n\nEdit the source firewall policy instead of the network-scoped firewall table row."
	}
	return ""
}

func editableFirewallRule(rule core.FirewallRule) core.FirewallRule {
	rule.Direction = cleanEditableValue(rule.Direction)
	rule.Protocol = cleanEditableValue(rule.Protocol)
	rule.PortRange = cleanEditableValue(rule.PortRange)
	rule.Source = cleanEditableValue(rule.Source)
	rule.Destination = cleanEditableValue(rule.Destination)
	rule.Action = cleanEditableValue(rule.Action)
	rule.Description = cleanEditableValue(rule.Description)
	return rule
}

func cleanEditableValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}

func defaultDirection(value string) string {
	value = cleanEditableValue(value)
	if value == "" {
		return core.Inbound
	}
	return value
}

func defaultProtocol(value string) string {
	value = cleanEditableValue(value)
	if value == "" {
		return "tcp"
	}
	return value
}

func defaultPorts(value string) string {
	value = cleanEditableValue(value)
	if value == "" {
		return "all"
	}
	return value
}

func defaultAction(value string) string {
	value = cleanEditableValue(value)
	if value == "" {
		return "Allow"
	}
	return value
}

func priorityValue(priority int) string {
	if priority <= 0 {
		return ""
	}
	return strconv.Itoa(priority)
}

func parsePriorityValue(value string) (int, error) {
	value = cleanEditableValue(value)
	if value == "" {
		return 0, nil
	}
	priority, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("priority must be a number")
	}
	return priority, nil
}

func normalizeDirectionValue(value string) (string, error) {
	switch strings.ToLower(cleanEditableValue(value)) {
	case "inbound", "ingress", "in":
		return core.Inbound, nil
	case "outbound", "egress", "out":
		return core.Outbound, nil
	default:
		return "", fmt.Errorf("direction must be Inbound or Outbound")
	}
}

func normalizeActionValue(value string) (string, error) {
	switch strings.ToLower(cleanEditableValue(value)) {
	case "allow":
		return "Allow", nil
	case "deny":
		return "Deny", nil
	default:
		return "", fmt.Errorf("action must be Allow or Deny")
	}
}

func normalizeProtocolValue(value string) string {
	value = strings.ToLower(cleanEditableValue(value))
	if value == "" {
		return "tcp"
	}
	return value
}

func normalizePortRangeValue(value string) string {
	value = cleanEditableValue(value)
	if value == "" {
		return "all"
	}
	return value
}

func awsEditablePeerValue(rule core.FirewallRule) string {
	if strings.EqualFold(strings.TrimSpace(rule.Direction), core.Outbound) {
		return cleanEditableValue(rule.Destination)
	}
	return cleanEditableValue(rule.Source)
}

func (v *RulesView) isAWSSecurityGroupRuleEditor() bool {
	if strings.EqualFold(strings.TrimSpace(v.activeCtx.Provider), "AWS") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(v.pendingRule.Provider), "AWS")
}

func (v *RulesView) isAWSSecurityGroupRuleEditorForRule(rule core.FirewallRule) bool {
	if strings.EqualFold(strings.TrimSpace(rule.Provider), "AWS") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(v.activeCtx.Provider), "AWS")
}

func orFallback(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "-" {
			return value
		}
	}
	return "-"
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

func (v *RulesView) setupAddForm() (ui.View, tea.Cmd) {
	v.pendingAction = actionItem{title: "Add", desc: "Add new rule"}
	v.pendingRule = core.FirewallRule{
		ResourceID: v.group.ID,
		Provider:   v.activeCtx.Provider,
	}

	i1 := textinput.New()
	i1.Prompt = "Direction: "
	i1.Placeholder = "Inbound or Outbound"
	i1.SetValue("Inbound")
	i1.Focus()
	i1.PromptStyle = lipgloss.NewStyle().Foreground(ui.Highlight)
	i1.TextStyle = lipgloss.NewStyle().Foreground(ui.Highlight)

	i2 := textinput.New()
	i2.Prompt = "Action: "
	i2.Placeholder = "Allow or Deny"
	i2.SetValue("Allow")

	i3 := textinput.New()
	i3.Prompt = "Protocol: "
	i3.Placeholder = "tcp, udp, icmp, all"
	i3.SetValue("tcp")

	i4 := textinput.New()
	i4.Prompt = "Ports: "
	i4.Placeholder = "e.g. 80, 443, 80-90, all"

	i5 := textinput.New()
	i5.Prompt = "Source/Dest IP (CIDR): "
	i5.Placeholder = "0.0.0.0/0"
	i5.SetValue("0.0.0.0/0")

	v.editInputs = []textinput.Model{i1, i2, i3, i4, i5}
	v.focusIndex = 0
	v.activePane = paneAddRule
	return v, textinput.Blink
}

func (v *RulesView) handleAddKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		v.refreshTable()
		return v, nil
	case "tab", "shift+tab", "up", "down":
		v.shiftEditFocus(msg.String() == "up" || msg.String() == "shift+tab")
		return v, v.focusEditInput()
	case "enter":
		if v.focusIndex < len(v.editInputs)-1 {
			v.shiftEditFocus(false)
			return v, v.focusEditInput()
		}

		if v.focusIndex == len(v.editInputs)-1 {
			dir := "Inbound"
			if strings.EqualFold(strings.TrimSpace(v.editInputs[0].Value()), "outbound") {
				dir = "Outbound"
			}
			act := "Allow"
			if strings.EqualFold(strings.TrimSpace(v.editInputs[1].Value()), "deny") {
				act = "Deny"
			}

			newRule := core.FirewallRule{
				ResourceID: v.group.ID,
				Provider:   v.activeCtx.Provider,
				Direction:  dir,
				Action:     act,
				Protocol:   strings.TrimSpace(v.editInputs[2].Value()),
				PortRange:  strings.TrimSpace(v.editInputs[3].Value()),
			}

			ipVal := strings.TrimSpace(v.editInputs[4].Value())
			if dir == "Inbound" {
				newRule.Source = ipVal
			} else {
				newRule.Destination = ipVal
			}

			v.pendingRule = newRule
			v.loading = true
			applog.Infof("component=firewall_rules event=add_submit provider=%s account=%s region=%s mode=%s rule=%s", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(strings.TrimSpace(v.cfg.Backend)), newRule.ResourceID)
			return v, v.executeRuleActionCmd()
		}
	}

	var cmd tea.Cmd
	v.editInputs[v.focusIndex], cmd = v.editInputs[v.focusIndex].Update(msg)
	return v, cmd
}

func (v *RulesView) shiftEditFocus(reverse bool) {
	if len(v.editInputs) == 0 {
		return
	}
	if reverse {
		v.focusIndex--
	} else {
		v.focusIndex++
	}

	if v.focusIndex > len(v.editInputs)-1 {
		v.focusIndex = 0
	} else if v.focusIndex < 0 {
		v.focusIndex = len(v.editInputs) - 1
	}
}

func (v *RulesView) focusEditInput() tea.Cmd {
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
	return tea.Batch(cmds...)
}
