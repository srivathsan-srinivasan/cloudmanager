package databases

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
	"github.com/vyoogam/cloudmanager/internal/iac"
	applog "github.com/vyoogam/cloudmanager/internal/logging"
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/ui"
	"github.com/vyoogam/cloudmanager/internal/views/tagging"
)

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneTag
	paneSortConfig
)

type dbsFetchMsg struct {
	requestKey string
	databases  []core.Database
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

type DatabasesView struct {
	table          table.Model
	actions        list.Model
	sortList       list.Model
	descView       viewport.Model
	tagInput       textinput.Model
	activeCtx      core.CloudContext
	dbData         []core.Database
	visibleRows    []core.Database
	tableCols      []table.Column
	cfg            *config.AppConfig
	width, height  int
	loading        bool
	statusMsg      string
	searchQuery    string
	requestKey     string
	activePane     int
	copyableText   string
	detailURL      string
	pending        core.Database
	canScrollLeft  bool
	canScrollRight bool
	columnOffset   int
	sortColumn     string
	sortAsc        bool
	sortHeader     ui.HeaderSortState
}

func New(cfg *config.AppConfig) *DatabasesView {
	items := make([]list.Item, 0, len(core.DatabaseActions()))
	for _, action := range core.DatabaseActions() {
		items = append(items, actionItem{title: action.Title, desc: action.Description})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	actions := list.New(items, delegate, 42, 10)
	actions.Title = "Database Actions"
	actions.SetShowStatusBar(false)
	actions.SetFilteringEnabled(false)

	sortList := ui.NewSortList("Sort Databases by (Enter to select, Esc to cancel)", databaseSortColumns())

	descView := viewport.New(80, 20)
	descView.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	tagInput := textinput.New()
	tagInput.Placeholder = "comma separated tags..."
	tagInput.Prompt = "tags> "
	tagInput.CharLimit = 160
	tagInput.Width = 42

	return &DatabasesView{
		cfg: cfg, actions: actions, sortList: sortList, descView: descView, tagInput: tagInput,
		sortAsc: true,
	}
}

func (v *DatabasesView) Title() string { return "Databases" }
func (v *DatabasesView) ShortHelp() string {
	return "↑↓: Navigate • ↑ at top: Columns • Enter: Sort/Actions • t: Tag • r: Refresh"
}
func (v *DatabasesView) IsInputActive() bool {
	return v.sortHeader.Active || v.activePane == paneActions || v.activePane == paneDescribe || v.activePane == paneTag || v.activePane == paneSortConfig
}

func (v *DatabasesView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *DatabasesView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.requestKey = ctx.CacheKey()
	v.activePane = paneTable
	v.copyableText = ""
	v.detailURL = ""
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching databases for %s...", ctx.DisplayName())
	v.refreshTable()
	return v.fetchCmd()
}

func (v *DatabasesView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
	v.sortList.SetSize(width-4, height-4)
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.refreshTable()
}

func (v *DatabasesView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.sortHeader.Active {
			return v.handleHeaderSortKeys(msg)
		}
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneTag, paneSortConfig:
				v.activePane = paneTable
				v.tagInput.Blur()
				return v, nil
			}
		}
		switch v.activePane {
		case paneActions:
			return v.handleActionKeys(msg)
		case paneDescribe:
			if msg.String() == "c" || msg.String() == "C" {
				return v.copyText(v.copyableText)
			}
			if msg.String() == "o" || msg.String() == "O" {
				return v.openConsole(v.detailURL)
			}
			v.descView, cmd = v.descView.Update(msg)
			return v, cmd
		case paneTag:
			return v.handleTagKeys(msg)
		case paneSortConfig:
			return v.handleSortConfigKeys(msg)
		}
		switch msg.String() {
		case "r":
			v.loading = true
			v.statusMsg = "Refreshing databases..."
			return v, v.fetchCmd()
		case "enter":
			db, ok := v.selectedDatabase()
			if !ok {
				return v, nil
			}
			v.actions.Title = fmt.Sprintf("Actions: %s", db.Name)
			v.activePane = paneActions
			return v, nil
		case "t":
			return v.openTag()
		case "S":
			v.sortHeader.Activate(v.tableCols)
			v.refreshTable()
			return v, nil
		case "up":
			if v.table.Cursor() == 0 {
				v.sortHeader.Activate(v.tableCols)
				v.refreshTable()
				return v, nil
			}
		}
		v.table, cmd = v.table.Update(msg)
		return v, cmd

	case dbsFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.dbData = iac.ApplyTerraformToDatabases(v.cfg.TerraformStatePaths, v.activeCtx, msg.databases)
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			applog.Errorf("component=databases event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
		} else {
			v.dbData = config.ApplyResourceTagsToDatabases(*v.cfg, v.activeCtx, v.dbData)
			ready, down, other := databaseStatusCounts(v.dbData)
			v.statusMsg = fmt.Sprintf("Loaded %d databases (%d ready, %d down, %d other).", len(v.dbData), ready, down, other)
			applog.Infof("component=databases event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d ready=%d down=%d other=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(v.dbData), ready, down, other)
			indexCtx := v.activeCtx
			indexDatabases := v.dbData
			cmd = func() tea.Msg {
				return ui.DatabaseIndexUpdateMsg{Ctx: indexCtx, Databases: indexDatabases}
			}
		}
		v.refreshTable()
	case clipboardCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Copy failed: %v", msg.err)
		} else {
			v.statusMsg = "Copied to clipboard."
		}
	case ui.BrowserOpenMsg:
		if msg.Err != nil {
			v.statusMsg = fmt.Sprintf("Open failed: %v", msg.Err)
		} else {
			v.statusMsg = "Opened provider console."
		}
	}
	return v, cmd
}

func (v *DatabasesView) Render() string {
	content := ui.ColorizeOperationalStates(v.table.View())
	if v.loading {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading databases...")
	} else if len(v.visibleRows) == 0 {
		content = v.emptyState()
	}
	header := ui.BreadcrumbStyle.Render(fmt.Sprintf("%s \u203A %s \u203A Databases", v.activeCtx.Provider, v.activeCtx.DisplayName()))
	switch v.activePane {
	case paneDescribe:
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	case paneActions:
		overlay := ui.OverlayStyle.Render(v.actions.View())
		bodyWithOverlay := lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyWithOverlay), v.width, v.height)
	case paneTag:
		tagStyle := ui.OverlayStyle.Copy().Padding(1, 2).Width(60)
		tagView := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("CloudManager Tags"),
			"",
			lipgloss.NewStyle().Render(fmt.Sprintf("Add local tags to %s:", v.pending.Name)),
			"",
			v.tagInput.View(),
			"",
			lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Save • Esc: Cancel"),
		)
		overlay := tagStyle.Render(tagView)
		bodyWithOverlay := lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyWithOverlay), v.width, v.height)
	case paneSortConfig:
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	}
	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", content), v.width, v.height)
}

func (v *DatabasesView) emptyState() string {
	width := v.width - 4
	if width < 20 {
		width = 20
	}
	style := lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Width(width)
	if strings.HasPrefix(v.statusMsg, "Error:") {
		return style.Render(fmt.Sprintf("Database fetch failed for %s.\n\n%s\n\nPress r to retry. Press L for logs.", v.activeCtx.DisplayName(), strings.TrimPrefix(v.statusMsg, "Error: ")))
	}
	if strings.TrimSpace(v.searchQuery) != "" {
		return style.Render(fmt.Sprintf("No databases match %q in %s.\n\nPress / to change the filter, or Esc if the filter is active.", v.searchQuery, v.activeCtx.DisplayName()))
	}
	return style.Render(fmt.Sprintf("No databases found for %s.\n\nThe dashboard database total is global indexed/cache data across contexts. This tab only shows databases for the selected context. Use g or :find-dbs to search the global index, or switch context and press r.", v.activeCtx.DisplayName()))
}

func (v *DatabasesView) refreshTable() {
	if v.width == 0 {
		return
	}
	cursor := v.table.Cursor()
	var cols []table.Column

	columnNames := v.cfg.DatabaseColumns
	if len(columnNames) == 0 {
		columnNames = core.DefaultDatabaseColumns
	}

	for _, c := range columnNames {
		cols = append(cols, table.Column{Title: c, Width: 15})
	}
	tbl, visibleCols, _, _, _ := ui.NewResourceTable(cols, v.width, 0, "Name")
	ui.SortByColumn(v.dbData, v.sortColumn, v.sortAsc, func(db core.Database, column string) string {
		return db.GetField(column)
	})

	var rows []table.Row
	v.visibleRows = v.visibleRows[:0]
	for _, db := range v.dbData {
		if !databaseMatchesQuery(db, v.searchQuery) {
			continue
		}
		v.visibleRows = append(v.visibleRows, db)
		var row []string
		for _, col := range visibleCols {
			row = append(row, ui.TruncateText(db.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	tbl.SetRows(rows)
	tbl.SetColumns(ui.DecorateSortColumns(visibleCols, v.sortHeader, v.sortColumn, v.sortAsc))
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	if len(rows) > 0 {
		if cursor >= len(rows) {
			cursor = len(rows) - 1
		}
		if cursor < 0 {
			cursor = 0
		}
		tbl.SetCursor(cursor)
	}
	tbl.Focus()
	v.table = tbl
	v.tableCols = visibleCols
}

func (v *DatabasesView) handleHeaderSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "down":
		v.sortHeader.Deactivate()
	case "left", "h":
		v.sortHeader.Move(-1, v.tableCols)
	case "right", "l":
		v.sortHeader.Move(1, v.tableCols)
	case "enter":
		column := v.sortHeader.SelectedColumn(v.tableCols)
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, column)
		v.statusMsg = fmt.Sprintf("Sorted by %s (%s).", v.sortColumn, ui.SortDirectionLabel(v.sortAsc))
	}
	v.refreshTable()
	return v, nil
}

func (v *DatabasesView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() != "enter" {
		var cmd tea.Cmd
		v.sortList, cmd = v.sortList.Update(msg)
		return v, cmd
	}
	selected, ok := v.sortList.SelectedItem().(ui.SortColumnItem)
	if !ok {
		return v, nil
	}
	v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, selected.Name)
	ui.SortByColumn(v.dbData, v.sortColumn, v.sortAsc, func(db core.Database, column string) string {
		return db.GetField(column)
	})
	v.refreshTable()
	v.activePane = paneTable
	v.statusMsg = fmt.Sprintf("Sorted by %s (%s).", v.sortColumn, ui.SortDirectionLabel(v.sortAsc))
	return v, nil
}

func (v *DatabasesView) visibleOrDefaultColumns() []table.Column {
	if len(v.tableCols) > 0 {
		return v.tableCols
	}
	return databaseSortColumns()
}

func databaseSortColumns() []table.Column {
	cols := make([]table.Column, 0, len(core.DefaultDatabaseColumns))
	for _, col := range core.DefaultDatabaseColumns {
		cols = append(cols, table.Column{Title: col})
	}
	return cols
}

func sortItemsForColumns(columns []table.Column) []list.Item {
	items := make([]list.Item, 0, len(columns))
	for _, col := range columns {
		items = append(items, ui.SortColumnItem{Name: col.Title})
	}
	return items
}

func (v *DatabasesView) selectedDatabase() (core.Database, bool) {
	cursor := v.table.Cursor()
	if cursor < 0 || cursor >= len(v.visibleRows) {
		return core.Database{}, false
	}
	return v.visibleRows[cursor], true
}

func (v *DatabasesView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() != "enter" {
		var cmd tea.Cmd
		v.actions, cmd = v.actions.Update(msg)
		return v, cmd
	}
	db, ok := v.selectedDatabase()
	if !ok || v.actions.SelectedItem() == nil {
		return v, nil
	}
	action := v.actions.SelectedItem().(actionItem).title
	switch action {
	case "Describe":
		return v.showDetails(core.DescribeDatabase(v.activeCtx, db), core.DatabaseConsoleURL(v.activeCtx, db)), nil
	case "Copy ID":
		return v.copyText(firstNonEmpty(db.ID, db.Name))
	case "Copy Console URL":
		consoleURL := core.DatabaseConsoleURL(v.activeCtx, db)
		if strings.TrimSpace(consoleURL) == "" {
			v.statusMsg = "No console URL available for this database."
			return v, nil
		}
		return v.copyText(consoleURL)
	case "Open Console":
		return v.openConsole(core.DatabaseConsoleURL(v.activeCtx, db))
	case "Tag":
		return v.openTag()
	}
	return v, nil
}

func (v *DatabasesView) handleTagKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		tags := tagging.SplitInput(v.tagInput.Value())
		if err := tagging.Save(v.cfg, v.activeCtx, v.pending, tags); err != nil {
			v.statusMsg = fmt.Sprintf("Tag save failed: %v", err)
			return v, nil
		}
		v.dbData = config.ApplyResourceTagsToDatabases(*v.cfg, v.activeCtx, v.dbData)
		v.refreshTable()
		v.tagInput.Blur()
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Tagged %s with %s.", v.pending.Name, strings.Join(tags, ","))
		indexCtx := v.activeCtx
		indexDatabases := v.dbData
		return v, func() tea.Msg {
			return ui.DatabaseIndexUpdateMsg{Ctx: indexCtx, Databases: indexDatabases}
		}
	}
	var cmd tea.Cmd
	v.tagInput, cmd = v.tagInput.Update(msg)
	return v, cmd
}

func (v *DatabasesView) openTag() (ui.View, tea.Cmd) {
	db, ok := v.selectedDatabase()
	if !ok {
		return v, nil
	}
	v.pending = db
	v.tagInput.SetValue("")
	v.tagInput.Focus()
	v.activePane = paneTag
	v.statusMsg = fmt.Sprintf("Tag %s with CloudManager-only tags.", db.Name)
	return v, textinput.Blink
}

func (v *DatabasesView) showDetails(content, consoleURL string) ui.View {
	v.detailURL = strings.TrimSpace(consoleURL)
	v.copyableText = content
	v.descView.SetContent(v.copyableText)
	v.descView.GotoTop()
	v.activePane = paneDescribe
	v.statusMsg = "Viewing details (c copy, o open, Esc close)."
	return v
}

func (v *DatabasesView) copyText(text string) (ui.View, tea.Cmd) {
	text = strings.TrimSpace(text)
	if text == "" {
		v.statusMsg = "Nothing to copy."
		return v, nil
	}
	v.statusMsg = "Copying to clipboard..."
	return v, func() tea.Msg {
		return clipboardCompleteMsg{err: clipboard.Write(text)}
	}
}

func (v *DatabasesView) openConsole(consoleURL string) (ui.View, tea.Cmd) {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		v.statusMsg = "No provider console URL available."
		return v, nil
	}
	v.statusMsg = "Opening provider console..."
	return v, ui.OpenURLCmd(consoleURL)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func databaseMatchesQuery(db core.Database, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join([]string{
		db.Name, db.ID, db.Engine, db.Version, db.Status, db.Region, db.Size, db.Labels,
	}, " ")), query)
}

func (v *DatabasesView) fetchCmd() tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	cfg := *v.cfg
	return func() tea.Msg {
		applog.Infof("component=databases event=fetch_start provider=%s account=%s region=%s mode=%s", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, strings.ToUpper(strings.TrimSpace(cfg.Backend)))
		provider := providers.GetProvider(cfg)
		dbProvider, ok := provider.(providers.DatabaseProvider)
		if !ok {
			return dbsFetchMsg{requestKey: requestKey, err: fmt.Errorf("databases not supported by provider backend")}
		}
		dbs, err := dbProvider.FetchDatabases(context.Background(), activeCtx)
		return dbsFetchMsg{requestKey: requestKey, databases: dbs, err: err}
	}
}

func databaseStatusCounts(databases []core.Database) (ready, down, other int) {
	for _, db := range databases {
		switch databaseStateClass(db.Status) {
		case "ready":
			ready++
		case "down":
			down++
		default:
			other++
		}
	}
	return ready, down, other
}

func databaseStateClass(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	for _, token := range []string{"available", "running", "ready", "online", "active", "succeeded"} {
		if strings.Contains(normalized, token) {
			return "ready"
		}
	}
	for _, token := range []string{"stopped", "terminated", "deleted", "suspended", "failed", "unavailable"} {
		if strings.Contains(normalized, token) {
			return "down"
		}
	}
	return "other"
}
