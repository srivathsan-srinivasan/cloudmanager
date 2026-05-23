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

	"github.com/vyoogam/cloudmanager/internal/clipboard"
	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	applog "github.com/vyoogam/cloudmanager/internal/logging"
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/ui"
)

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneColumnConfig
	paneSortConfig
	paneConfirm
	paneEditRule
	paneAddRule
)

type securityGroupFetchMsg struct {
	requestKey string
	groups     []core.SecurityGroup
	err        error
}

type clipboardCompleteMsg struct {
	err error
}

type actionItem struct {
	title string
	desc  string
}

func (i actionItem) Title() string       { return i.title }
func (i actionItem) Description() string { return i.desc }
func (i actionItem) FilterValue() string { return i.title }

type columnItem struct {
	name     string
	selected bool
}

func (i columnItem) Title() string {
	if i.selected {
		return "[x] " + i.name
	}
	return "[ ] " + i.name
}
func (i columnItem) Description() string { return "Column" }
func (i columnItem) FilterValue() string { return i.name }

type sortItem struct{ name string }

func (i sortItem) Title() string       { return i.name }
func (i sortItem) Description() string { return "Sort by this column" }
func (i sortItem) FilterValue() string { return i.name }

type FirewallsView struct {
	groups           table.Model
	actions          list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	activePane       int

	groupData      []core.SecurityGroup
	visibleGroups  []core.SecurityGroup
	activeCtx      core.CloudContext
	cfg            *config.AppConfig
	loading        bool
	notSupported   bool
	isSearching    bool
	breadcrumbs    string
	requestKey     string
	width, height  int
	showSidebar    bool
	pendingGroup   core.SecurityGroup
	displayColumns []table.Column
	sortColumn     string
	sortAsc        bool
	sortHeader     ui.HeaderSortState
	columnOffset   int
	canScrollLeft  bool
	canScrollRight bool
	filterTerms    []string
	filterLabel    string
	copyableText   string
	detailURL      string
}

var getProvider = providers.GetProvider

func New(cfg *config.AppConfig) *FirewallsView {
	return NewFiltered(cfg, nil, "")
}

func NewFiltered(cfg *config.AppConfig, filterTerms []string, filterLabel string) *FirewallsView {
	var actionItems []list.Item
	for _, action := range core.SecurityGroupActions() {
		actionItems = append(actionItems, actionItem{title: action.Title, desc: action.Description})
	}

	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actions := list.New(actionItems, actionDelegate, 30, 12)
	actions.Title = "Security Group Actions"
	actions.SetShowStatusBar(false)
	actions.SetFilteringEnabled(false)

	var columnItems []list.Item
	selectedColumns := cfg.FirewallColumns
	if len(selectedColumns) == 0 {
		selectedColumns = core.DefaultSecurityGroupColumns
	}
	for _, column := range core.DefaultSecurityGroupColumns {
		selected := false
		for _, cfgCol := range selectedColumns {
			if cfgCol == column {
				selected = true
				break
			}
		}
		columnItems = append(columnItems, columnItem{name: column, selected: selected})
	}
	columnConfigList := list.New(columnItems, list.NewDefaultDelegate(), 0, 0)
	columnConfigList.Title = "Configure Firewall Columns (Space to toggle, Enter to save, Esc to cancel)"
	columnConfigList.SetShowStatusBar(false)
	columnConfigList.SetFilteringEnabled(false)

	var sortItems []list.Item
	for _, column := range core.DefaultSecurityGroupColumns {
		sortItems = append(sortItems, sortItem{name: column})
	}
	sortList := list.New(sortItems, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = "Sort Firewalls by (Enter to select, Esc to cancel)"
	sortList.SetShowStatusBar(false)
	sortList.SetFilteringEnabled(false)

	descView := viewport.New(80, 20)
	descView.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search security groups..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	tbl, cols, _, canScrollLeft, canScrollRight := createSecurityGroupTable(*cfg, 80, 0)
	return &FirewallsView{
		groups:           tbl,
		actions:          actions,
		columnConfigList: columnConfigList,
		sortList:         sortList,
		descView:         descView,
		searchInput:      searchInput,
		activePane:       paneTable,
		cfg:              cfg,
		breadcrumbs:      "Select a context to view firewalls",
		displayColumns:   cols,
		sortColumn:       "Name",
		sortAsc:          true,
		canScrollLeft:    canScrollLeft,
		canScrollRight:   canScrollRight,
		filterTerms:      normalizeFilterTerms(filterTerms),
		filterLabel:      filterLabel,
	}
}

func (v *FirewallsView) Title() string { return "Firewalls" }

func (v *FirewallsView) ShortHelp() string {
	return "\u2191\u2193: Navigate \u2022 ↑ at top: Columns \u2022 \u2190\u2192: Pan \u2022 Enter: Sort/Actions \u2022 /: Search \u2022 C: Columns \u2022 r: Refresh"
}

func (v *FirewallsView) IsInputActive() bool {
	return v.sortHeader.Active || v.isSearching || v.activePane == paneActions || v.activePane == paneDescribe || v.activePane == paneColumnConfig || v.activePane == paneSortConfig
}

func (v *FirewallsView) SetSearchQuery(query string) {
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue(strings.TrimSpace(query))
	v.searchInput.Blur()
	v.syncVisibleRows()
}

func (v *FirewallsView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.groupData = nil
	v.visibleGroups = nil
	v.copyableText = ""
	v.detailURL = ""
	v.requestKey = ctx.CacheKey()
	v.columnOffset = 0
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s \u203A Firewalls", ctx.Provider, ctx.DisplayName(), ctx.Region)
	if v.filterLabel != "" {
		v.breadcrumbs = fmt.Sprintf("%s \u203A %s", v.breadcrumbs, v.filterLabel)
	}
	v.loading = true
	v.notSupported = !providers.Supports(ctx.Provider, providers.CapabilityFirewalls)
	v.refreshTable()
	v.groups.Focus()

	if v.notSupported {
		v.loading = false
		return nil
	}
	return v.fetchGroupsCmd()
}

func (v *FirewallsView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.refreshTable()
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.columnConfigList.SetSize(width-4, height-4)
	v.sortList.SetSize(width-4, height-4)
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
}

func (v *FirewallsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.notSupported {
			return v, nil
		}
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		if v.sortHeader.Active {
			return v.handleHeaderSortKeys(msg)
		}
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneColumnConfig, paneSortConfig:
				v.activePane = paneTable
				return v, nil
			}
		}
		switch v.activePane {
		case paneActions:
			_, cmd = v.handleActionKeys(msg)
		case paneDescribe:
			_, cmd = v.handleDescribeKeys(msg)
		case paneColumnConfig:
			_, cmd = v.handleColumnConfigKeys(msg)
		case paneSortConfig:
			_, cmd = v.handleSortConfigKeys(msg)
		default:
			_, cmd = v.handleTableKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case securityGroupFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.groupData = msg.groups
		if msg.err != nil {
			applog.Errorf("component=firewalls event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
			v.visibleGroups = nil
			v.groups.SetRows([]table.Row{})
		} else {
			applog.Infof("component=firewalls event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(msg.groups))
			sortSecurityGroups(v.groupData, v.sortColumn, v.sortAsc)
			v.syncVisibleRows()
		}

	case clipboardCompleteMsg:
		// The rules view shares this message type; keep security-group copy silent.
	case ui.BrowserOpenMsg:
		if msg.Err != nil {
			v.copyableText = fmt.Sprintf("Open failed: %v", msg.Err)
		}

	case tea.WindowSizeMsg:
		v.Resize(msg.Width, msg.Height, v.showSidebar)
	}

	switch v.activePane {
	case paneTable:
		v.groups, cmd = v.groups.Update(msg)
		v.applySelectionStyle()
		cmds = append(cmds, cmd)
	case paneActions:
		v.actions, cmd = v.actions.Update(msg)
		cmds = append(cmds, cmd)
	case paneDescribe:
		v.descView, cmd = v.descView.Update(msg)
		cmds = append(cmds, cmd)
	case paneColumnConfig:
		v.columnConfigList, cmd = v.columnConfigList.Update(msg)
		cmds = append(cmds, cmd)
	case paneSortConfig:
		v.sortList, cmd = v.sortList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *FirewallsView) Render() string {
	header := ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)

	if v.notSupported {
		msg := lipgloss.NewStyle().Padding(2).Foreground(ui.Alert).Render(fmt.Sprintf("Firewalls are not supported for %s.", v.activeCtx.Provider))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", msg), v.width, v.height)
	}

	body := ui.ColorizeOperationalStates(v.groups.View())
	if v.loading {
		body = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading security groups...")
	} else if v.isSearching || v.searchInput.Value() != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(v.searchInput.View()),
			body,
		)
	}

	switch v.activePane {
	case paneActions:
		overlay := ui.OverlayStyle.Render(v.actions.View())
		bodyWithOverlay := lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyWithOverlay), v.width, v.height)
	case paneDescribe:
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	case paneColumnConfig:
		return ui.ClampToWindow(v.columnConfigList.View(), v.width, v.height)
	case paneSortConfig:
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	default:
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	}
}

func (v *FirewallsView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	sg, ok := v.selectedGroup()

	switch msg.String() {
	case "enter":
		if !ok {
			return v, nil
		}
		v.actions.Title = fmt.Sprintf("Actions: %s", sg.Name)
		v.activePane = paneActions
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
	case "up":
		if v.groups.Cursor() == 0 {
			v.sortHeader.Activate(v.displayColumns)
			v.refreshTable()
		}
	case "S":
		v.sortHeader.Activate(v.displayColumns)
		v.refreshTable()
	case "C":
		v.activePane = paneColumnConfig
	case "r":
		v.loading = true
		return v, v.fetchGroupsCmd()
	}
	return v, nil
}

func (v *FirewallsView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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

func (v *FirewallsView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		group, ok := v.selectedGroup()
		if !ok {
			return v, nil
		}

		switch v.actions.SelectedItem().(actionItem).title {
		case "View Rules":
			v.activePane = paneTable
			return v, func() tea.Msg {
				return ui.PushViewMsg{
					View: NewRules(v.cfg, group),
					Ctx:  v.activeCtx,
				}
			}
		case "Describe":
			v.pendingGroup = group
			v.detailURL = core.SecurityGroupConsoleURL(v.activeCtx, group)
			v.copyableText = core.DetailWithConsoleURL(core.DescribeSecurityGroup(group), v.detailURL)
			v.descView.SetContent(v.copyableText)
			v.activePane = paneDescribe
			return v, nil
		case "Open Console":
			return v.openConsole(core.SecurityGroupConsoleURL(v.activeCtx, group))
		}
	}
	return v, nil
}

func (v *FirewallsView) handleDescribeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
	case "c", "C":
		if strings.TrimSpace(v.copyableText) == "" {
			return v, nil
		}
		return v, copyTextCmd(v.copyableText)
	case "o", "O":
		return v.openConsole(v.detailURL)
	}
	return v, nil
}

func (v *FirewallsView) handleColumnConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case " ":
		selected := v.columnConfigList.SelectedItem()
		if selected != nil {
			idx := v.columnConfigList.Index()
			item := selected.(columnItem)
			item.selected = !item.selected
			v.columnConfigList.SetItem(idx, item)
		}
		return v, nil
	case "enter":
		var selectedColumns []string
		for _, item := range v.columnConfigList.Items() {
			column := item.(columnItem)
			if column.selected {
				selectedColumns = append(selectedColumns, column.name)
			}
		}
		if len(selectedColumns) == 0 {
			selectedColumns = core.DefaultSecurityGroupColumns
		}
		v.cfg.FirewallColumns = selectedColumns
		config.Save(*v.cfg)
		v.refreshTable()
		v.syncVisibleRows()
		v.activePane = paneTable
		return v, nil
	}
	return v, nil
}

func (v *FirewallsView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		selected := v.sortList.SelectedItem()
		if selected != nil {
			item := selected.(sortItem)
			if v.sortColumn == item.name {
				v.sortAsc = !v.sortAsc
			} else {
				v.sortColumn = item.name
				v.sortAsc = true
			}
			sortSecurityGroups(v.groupData, v.sortColumn, v.sortAsc)
			v.syncVisibleRows()
			v.activePane = paneTable
		}
		return v, nil
	}
	return v, nil
}

func (v *FirewallsView) fetchGroupsCmd() tea.Cmd {
	requestKey := v.requestKey
	activeCtx := v.activeCtx
	provider := getProvider(*v.cfg)
	mode := strings.ToUpper(v.cfg.Backend)
	return func() tea.Msg {
		applog.Infof("component=firewalls event=fetch_start provider=%s account=%s region=%s mode=%s", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode)
		firewallProvider, ok := provider.(providers.FirewallProvider)
		if !ok {
			return securityGroupFetchMsg{requestKey: requestKey, err: fmt.Errorf("FirewallProvider not implemented")}
		}
		groups, err := firewallProvider.FetchSecurityGroups(context.Background(), activeCtx)
		return securityGroupFetchMsg{requestKey: requestKey, groups: groups, err: err}
	}
}

func (v *FirewallsView) refreshTable() {
	if v.width == 0 {
		return
	}
	cursor := v.groups.Cursor()
	focused := v.groups.Focused()
	tbl, cols, nextOffset, canScrollLeft, canScrollRight := createSecurityGroupTable(*v.cfg, v.width, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	rows := mapSecurityGroupsToRows(v.visibleRowsSource(), cols)
	tbl.SetRows(rows)
	tbl.SetColumns(ui.DecorateSortColumns(cols, v.sortHeader, v.sortColumn, v.sortAsc))
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	if cursor >= 0 && cursor < len(rows) {
		tbl.SetCursor(cursor)
	}
	if focused {
		tbl.Focus()
	}
	v.groups = tbl
	v.displayColumns = cols
	v.applySelectionStyle()
}

func (v *FirewallsView) handleHeaderSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "down":
		v.sortHeader.Deactivate()
	case "left", "h":
		if !v.sortHeader.Move(-1, v.displayColumns) && v.columnOffset > 0 {
			v.columnOffset--
			v.refreshTable()
			v.sortHeader.Index = len(v.displayColumns) - 1
		}
	case "right", "l":
		if !v.sortHeader.Move(1, v.displayColumns) && v.canScrollRight {
			v.columnOffset++
			v.sortHeader.Index = 0
			v.refreshTable()
		}
	case "enter":
		column := v.sortHeader.SelectedColumn(v.displayColumns)
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, column)
		sortSecurityGroups(v.groupData, v.sortColumn, v.sortAsc)
		v.syncVisibleRows()
	}
	v.refreshTable()
	return v, nil
}

func (v *FirewallsView) syncVisibleRows() {
	v.visibleGroups = filterSecurityGroups(v.groupData, v.searchInput.Value(), v.filterTerms)
	rows := mapSecurityGroupsToRows(v.visibleGroups, v.displayColumns)
	v.groups.SetRows(rows)
	if len(rows) == 0 {
		v.groups.SetCursor(0)
		v.applySelectionStyle()
		return
	}
	if cursor := v.groups.Cursor(); cursor >= len(rows) {
		v.groups.SetCursor(len(rows) - 1)
	}
	v.applySelectionStyle()
}

func (v *FirewallsView) applySelectionStyle() {
	styles := ui.DefaultTableStyles()
	if group, ok := v.selectedGroup(); ok && group.HasAuditRisk() {
		styles = ui.AlertSelectedTableStyles()
	}
	v.groups.SetStyles(styles)
}

func (v *FirewallsView) visibleRowsSource() []core.SecurityGroup {
	if v.visibleGroups != nil {
		return v.visibleGroups
	}
	return filterSecurityGroups(v.groupData, v.searchInput.Value(), v.filterTerms)
}

func (v *FirewallsView) selectedGroup() (core.SecurityGroup, bool) {
	cursor := v.groups.Cursor()
	if cursor < 0 || cursor >= len(v.visibleGroups) {
		return core.SecurityGroup{}, false
	}
	return v.visibleGroups[cursor], true
}

func filterSecurityGroups(groups []core.SecurityGroup, query string, filters []string) []core.SecurityGroup {
	normalized := strings.ToLower(strings.TrimSpace(query))
	var filtered []core.SecurityGroup
	for _, group := range groups {
		if !matchesSecurityGroupFilters(group, filters) {
			continue
		}
		if normalized == "" {
			filtered = append(filtered, group)
			continue
		}
		if strings.Contains(strings.ToLower(group.Name), normalized) ||
			strings.Contains(strings.ToLower(group.ID), normalized) ||
			strings.Contains(strings.ToLower(group.NetworkID), normalized) ||
			strings.Contains(strings.ToLower(group.NetworkName), normalized) ||
			strings.Contains(strings.ToLower(group.Description), normalized) ||
			strings.Contains(strings.ToLower(group.Labels), normalized) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func mapSecurityGroupsToRows(groups []core.SecurityGroup, cols []table.Column) []table.Row {
	rows := make([]table.Row, 0, len(groups))
	for _, group := range groups {
		row := make(table.Row, 0, len(cols))
		nameValue := group.GetField("Name")
		if group.HasAuditRisk() {
			nameValue = "⚠ " + nameValue
		}
		for _, col := range cols {
			value := group.GetField(col.Title)
			if col.Title == "Name" {
				value = nameValue
			}
			row = append(row, ui.TruncateText(value, col.Width))
		}
		rows = append(rows, row)
	}
	return rows
}

func createSecurityGroupTable(cfg config.AppConfig, availableWidth, offset int) (table.Model, []table.Column, int, bool, bool) {
	cols := buildSecurityGroupColumns(cfg)
	return ui.NewResourceTable(cols, availableWidth, offset, "Name")
}

func buildSecurityGroupColumns(cfg config.AppConfig) []table.Column {
	preferredWidths := map[string]int{
		"Name": 18, "ID": 16, "Network": 14, "Inbound Rules": 13,
		"Outbound Rules": 14, "Attached": 8, "Description": 24,
	}
	selectedColumns := cfg.FirewallColumns
	if len(selectedColumns) == 0 {
		selectedColumns = core.DefaultSecurityGroupColumns
	}
	cols := make([]table.Column, 0, len(selectedColumns))
	for _, column := range selectedColumns {
		width := preferredWidths[column]
		if width == 0 {
			width = 12
		}
		cols = append(cols, table.Column{Title: column, Width: width})
	}
	return cols
}

func normalizeFilterTerms(filters []string) []string {
	seen := make(map[string]bool, len(filters))
	var normalized []string
	for _, filter := range filters {
		value := strings.ToLower(strings.TrimSpace(filter))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func matchesSecurityGroupFilters(group core.SecurityGroup, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	candidates := []string{
		strings.ToLower(strings.TrimSpace(group.ID)),
		strings.ToLower(strings.TrimSpace(group.Name)),
		strings.ToLower(strings.TrimSpace(group.NetworkID)),
		strings.ToLower(strings.TrimSpace(group.NetworkName)),
	}
	for _, filter := range filters {
		for _, candidate := range candidates {
			if filter != "" && candidate == filter {
				return true
			}
		}
	}
	return false
}

func sortSecurityGroups(groups []core.SecurityGroup, column string, asc bool) {
	ui.SortByColumn(groups, column, asc, func(group core.SecurityGroup, column string) string {
		return group.GetField(column)
	})
}

func copyTextCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardCompleteMsg{err: clipboard.Write(text)}
	}
}

func (v *FirewallsView) openConsole(consoleURL string) (ui.View, tea.Cmd) {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		return v, nil
	}
	return v, ui.OpenURLCmd(consoleURL)
}
