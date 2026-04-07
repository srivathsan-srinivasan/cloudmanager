package snapshots

import (
	"context"
	"fmt"
	"strings"
	"time"

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

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneColumnConfig
	paneSortConfig
	paneConfirm
	paneCreateDisk
)

// --- Bubble Tea messages ---

type snapFetchMsg struct {
	requestKey string
	snaps      []core.Snapshot
	err        error
}
type commandCompleteMsg struct {
	output string
	err    error
}
type describeCompleteMsg struct {
	output string
	err    error
}

type cacheEntry struct {
	snaps     []core.Snapshot
	timestamp time.Time
}

// --- list item adapters ---

type actionItem struct {
	title       string
	desc        string
	isDangerous bool
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

// --- SnapshotsView ---

type SnapshotsView struct {
	snaps            table.Model
	actions          list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	createDiskInput  textinput.Model
	activePane       int

	snapData     []core.Snapshot
	visibleSnaps []core.Snapshot
	snapCache    map[string]cacheEntry
	tableCols    []table.Column
	cfg          *config.AppConfig

	activeCtx      core.CloudContext
	sortColumn     string
	sortAsc        bool
	columnOffset   int
	canScrollLeft  bool
	canScrollRight bool
	isSearching    bool
	loading        bool
	breadcrumbs    string
	statusMsg      string
	notSupported   bool

	pendingAction actionItem
	pendingSnap   core.Snapshot

	width, height int
	showSidebar   bool
	requestKey    string
}

func New(cfg *config.AppConfig) *SnapshotsView {
	// Actions list
	var actionItems []list.Item
	for _, a := range core.SnapshotActions() {
		actionItems = append(actionItems, actionItem{title: a.Title, desc: a.Description, isDangerous: a.Dangerous})
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actionList := list.New(actionItems, actionDelegate, 30, 15)
	actionList.Title = "Snapshot Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	// Column config
	var colItems []list.Item
	for _, c := range core.DefaultSnapshotColumns {
		selected := false
		for _, cfgCol := range cfg.SnapshotColumns {
			if cfgCol == c {
				selected = true
				break
			}
		}
		colItems = append(colItems, columnItem{name: c, selected: selected})
	}
	colList := list.New(colItems, list.NewDefaultDelegate(), 0, 0)
	colList.Title = "Configure Snapshot Columns (Space to toggle, Enter to save, Esc to cancel)"
	colList.SetShowStatusBar(false)

	// Sort config
	var sItems []list.Item
	for _, c := range core.DefaultSnapshotColumns {
		sItems = append(sItems, sortItem{name: c})
	}
	sortList := list.New(sItems, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = "Sort Snapshots by (Enter to select, Esc to cancel)"
	sortList.SetShowStatusBar(false)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search snapshots..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	createDiskInput := textinput.New()
	createDiskInput.Placeholder = "Enter new disk name..."
	createDiskInput.Prompt = "> "
	createDiskInput.CharLimit = 64
	createDiskInput.Width = 30

	snapTable, tableCols, _, canScrollLeft, canScrollRight := createSnapTable(*cfg, 80, 0)

	return &SnapshotsView{
		snaps: snapTable, actions: actionList,
		columnConfigList: colList, sortList: sortList,
		descView: vp, searchInput: searchInput, createDiskInput: createDiskInput,
		activePane: paneTable, tableCols: tableCols,
		cfg: cfg, snapCache: make(map[string]cacheEntry),
		sortColumn: "Name", sortAsc: true,
		canScrollLeft: canScrollLeft, canScrollRight: canScrollRight,
		breadcrumbs: "Select a context to view snapshots",
	}
}

func (v *SnapshotsView) Title() string { return "Snapshots" }

func (v *SnapshotsView) ShortHelp() string {
	return "\u2191\u2193: Navigate \u2022 \u2190\u2192: Pan \u2022 Enter: Actions \u2022 /: Search \u2022 S: Sort \u2022 C: Columns \u2022 r: Refresh"
}

func (v *SnapshotsView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneCreateDisk || v.activePane == paneColumnConfig || v.activePane == paneSortConfig || v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneDescribe
}

func (v *SnapshotsView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.snapData = nil
	v.visibleSnaps = nil
	v.requestKey = ctx.CacheKey()
	v.columnOffset = 0
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s", ctx.Provider, ctx.DisplayName(), ctx.Region)
	v.loading = true
	v.notSupported = false

	if !providers.Supports(ctx.Provider, providers.CapabilitySnapshots) {
		v.notSupported = true
		v.loading = false
		v.statusMsg = fmt.Sprintf("Snapshots not supported for %s", ctx.Provider)
		return nil
	}

	v.statusMsg = fmt.Sprintf("Fetching snapshots for %s...", ctx.DisplayName())
	v.refreshTable()
	v.snaps.Focus()
	return v.fetchSnapsCmd(false)
}

func (v *SnapshotsView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.refreshTable()
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.columnConfigList.SetSize(width-4, height-4)
	v.sortList.SetSize(width-4, height-4)
}

func (v *SnapshotsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
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
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneColumnConfig, paneSortConfig, paneCreateDisk:
				v.activePane = paneTable
				return v, nil
			case paneConfirm:
				v.activePane = paneActions
				return v, nil
			}
		}
		switch v.activePane {
		case paneActions:
			_, cmd = v.handleActionKeys(msg)
		case paneConfirm:
			_, cmd = v.handleConfirmKeys(msg)
		case paneDescribe:
			_, cmd = v.handleDescribeKeys(msg)
		case paneColumnConfig:
			_, cmd = v.handleColumnConfigKeys(msg)
		case paneSortConfig:
			_, cmd = v.handleSortConfigKeys(msg)
		case paneTable:
			_, cmd = v.handleTableKeys(msg)
		case paneCreateDisk:
			_, cmd = v.handleCreateDiskKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case snapFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.snapData = msg.snaps
		if msg.err != nil {
			applog.Errorf("component=snapshots event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.visibleSnaps = nil
			v.snaps.SetRows([]table.Row{})
		} else {
			applog.Infof("component=snapshots event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(msg.snaps))
			sortSnaps(v.snapData, v.sortColumn, v.sortAsc)
			v.syncVisibleRows()
			v.statusMsg = fmt.Sprintf("Loaded %d snapshots.", len(msg.snaps))
		}

	case commandCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = msg.output
			v.loading = true
			return v, v.fetchSnapsCmd(true)
		}

	case describeCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.descView.SetContent(msg.output)
			v.activePane = paneDescribe
			v.statusMsg = "Viewing details (Esc to close, Up/Down to scroll)."
		}

	case tea.WindowSizeMsg:
		v.Resize(msg.Width, msg.Height, v.showSidebar)
	}

	// Route to active component
	switch v.activePane {
	case paneTable:
		v.snaps, cmd = v.snaps.Update(msg)
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
	case paneCreateDisk:
		v.createDiskInput, cmd = v.createDiskInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *SnapshotsView) Render() string {
	header := ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)

	if v.notSupported {
		msg := lipgloss.NewStyle().Padding(2).Foreground(ui.Alert).Render("Switch to SDK backend (press 'c') for snapshot management.")
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", msg), v.width, v.height)
	}

	tableContent := v.snaps.View()

	if v.loading {
		tableContent = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading snapshots...")
	} else if v.isSearching || v.searchInput.Value() != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(v.searchInput.View()),
			tableContent,
		)
	}

	switch v.activePane {
	case paneColumnConfig:
		return ui.ClampToWindow(v.columnConfigList.View(), v.width, v.height)
	case paneSortConfig:
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	case paneDescribe:
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	case paneActions:
		overlay := ui.OverlayStyle.Render(v.actions.View())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	case paneConfirm:
		confirmMsg := fmt.Sprintf("Are you sure you want to %s snapshot %s?", v.pendingAction.title, v.pendingSnap.Name)
		confirmStyle := ui.OverlayStyle.Copy().BorderForeground(ui.Alert).Padding(1, 2).Width(50)
		confirmView := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ui.Alert).Bold(true).Render("⚠️  CONFIRM ACTION"),
			"\n", lipgloss.NewStyle().Align(lipgloss.Center).Render(confirmMsg),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Confirm \u2022 Esc: Cancel"),
		)
		overlay := confirmStyle.Render(confirmView)
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	case paneCreateDisk:
		createMsg := fmt.Sprintf("Enter new disk name to create from snapshot %s:", v.pendingSnap.Name)
		createStyle := ui.OverlayStyle.Copy().Padding(1, 2).Width(60)
		createView := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Create Disk"),
			"\n", lipgloss.NewStyle().Render(createMsg),
			"\n", v.createDiskInput.View(),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Submit \u2022 Esc: Cancel"),
		)
		overlay := createStyle.Render(createView)
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", tableContent), v.width, v.height)
}

// --- Key handlers ---

func (v *SnapshotsView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v.snaps.SelectedRow() != nil {
			v.activePane = paneActions
		}
	case "/":
		v.isSearching = true
		v.searchInput.Focus()
		v.statusMsg = "Search (Enter/Esc to apply)"
	case "left", "h":
		if v.columnOffset > 0 {
			v.columnOffset--
			v.refreshTable()
			v.statusMsg = "Scrolled columns left."
		}
	case "right", "l":
		if v.canScrollRight {
			v.columnOffset++
			v.refreshTable()
			v.statusMsg = "Scrolled columns right."
		}
	case "r":
		v.loading = true
		v.statusMsg = fmt.Sprintf("Refreshing snapshots for %s...", v.activeCtx.DisplayName())
		return v, v.fetchSnapsCmd(true)
	case "S":
		v.activePane = paneSortConfig
	case "C":
		v.activePane = paneColumnConfig
	}
	return v, nil
}

func (v *SnapshotsView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		v.isSearching = false
		v.searchInput.Blur()
		v.syncVisibleRows()
		v.statusMsg = "Search applied."
		return v, nil
	}
	var cmd tea.Cmd
	v.searchInput, cmd = v.searchInput.Update(msg)
	v.syncVisibleRows()
	return v, cmd
}

func (v *SnapshotsView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		if snap, ok := v.selectedSnapshot(); ok {
			action := v.actions.SelectedItem().(actionItem)

			if action.isDangerous {
				v.pendingAction = action
				v.pendingSnap = snap
				v.activePane = paneConfirm
				return v, nil
			}
			if action.title == "Create Disk" {
				v.pendingAction = action
				v.pendingSnap = snap
				v.activePane = paneCreateDisk
				v.createDiskInput.SetValue("")
				v.createDiskInput.Focus()
				return v, nil
			}
			if action.title == "Describe" {
				v.activePane = paneDescribe
				return v, func() tea.Msg {
					return describeCompleteMsg{output: core.DescribeSnapshot(snap)}
				}
			}

			// Catch-all
			v.activePane = paneTable
			v.statusMsg = fmt.Sprintf("Executing %s on %s...", action.title, snap.Name)
			return v, executeSnapActionCmd(action.title, snap, v.activeCtx, v.cfg)
		}
	}
	return v, nil
}

func (v *SnapshotsView) handleConfirmKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Executing %s on %s...", v.pendingAction.title, v.pendingSnap.Name)
		return v, executeSnapActionCmd(v.pendingAction.title, v.pendingSnap, v.activeCtx, v.cfg)
	}
	return v, nil
}

func (v *SnapshotsView) handleCreateDiskKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		v.createDiskInput.Blur()
		return v, nil
	case "enter":
		val := strings.TrimSpace(v.createDiskInput.Value())
		if val == "" {
			v.statusMsg = "Disk name cannot be empty."
			return v, nil
		}

		v.activePane = paneTable
		v.createDiskInput.Blur()
		v.statusMsg = fmt.Sprintf("Executing %s on %s...", v.pendingAction.title, v.pendingSnap.Name)

		actionStr := fmt.Sprintf("Create Disk:%s", val)

		return v, executeSnapActionCmd(actionStr, v.pendingSnap, v.activeCtx, v.cfg)
	}
	return v, nil
}

func (v *SnapshotsView) handleDescribeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() == "esc" {
		v.activePane = paneTable
	}
	return v, nil
}

func (v *SnapshotsView) handleColumnConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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
			c := item.(columnItem)
			if c.selected {
				selectedColumns = append(selectedColumns, c.name)
			}
		}
		if len(selectedColumns) == 0 {
			selectedColumns = core.DefaultSnapshotColumns
		}
		v.cfg.SnapshotColumns = selectedColumns
		config.Save(*v.cfg)
		v.refreshTable()
		v.activePane = paneTable
		v.statusMsg = "Columns saved."
		return v, nil
	}
	return v, nil
}

func (v *SnapshotsView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		selected := v.sortList.SelectedItem()
		if selected != nil {
			sItem := selected.(sortItem)
			if v.sortColumn == sItem.name {
				v.sortAsc = !v.sortAsc
			} else {
				v.sortColumn = sItem.name
				v.sortAsc = true
			}
			sortSnaps(v.snapData, v.sortColumn, v.sortAsc)
			v.syncVisibleRows()
			v.activePane = paneTable
			dir := "asc"
			if !v.sortAsc {
				dir = "desc"
			}
			v.statusMsg = fmt.Sprintf("Sorted by %s (%s)", v.sortColumn, dir)
		}
		return v, nil
	}
	return v, nil
}

// --- Table helpers ---

func (v *SnapshotsView) refreshTable() {
	if v.width == 0 {
		return
	}
	cursor := v.snaps.Cursor()
	focused := v.snaps.Focused()
	newSnaps, newCols, nextOffset, canScrollLeft, canScrollRight := createSnapTable(*v.cfg, v.width, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	newSnaps.SetHeight(ui.TableHeight(v.height))
	newSnaps.SetWidth(ui.TableViewportWidth(v.width))
	rows := mapSnapsToRows(v.visibleRowsSource(), newCols)
	newSnaps.SetRows(rows)
	if cursor >= 0 && cursor < len(rows) {
		newSnaps.SetCursor(cursor)
	}
	if focused {
		newSnaps.Focus()
	}
	v.snaps = newSnaps
	v.tableCols = newCols
}

func (v *SnapshotsView) syncVisibleRows() {
	v.visibleSnaps = filterSnaps(v.snapData, v.searchInput.Value())
	rows := mapSnapsToRows(v.visibleSnaps, v.tableCols)
	v.snaps.SetRows(rows)
	if len(rows) == 0 {
		v.snaps.SetCursor(0)
		return
	}
	if cursor := v.snaps.Cursor(); cursor >= len(rows) {
		v.snaps.SetCursor(len(rows) - 1)
	}
}

func (v *SnapshotsView) visibleRowsSource() []core.Snapshot {
	if v.visibleSnaps != nil {
		return v.visibleSnaps
	}
	return filterSnaps(v.snapData, v.searchInput.Value())
}

func (v *SnapshotsView) selectedSnapshot() (core.Snapshot, bool) {
	cursor := v.snaps.Cursor()
	if cursor < 0 || cursor >= len(v.visibleSnaps) {
		return core.Snapshot{}, false
	}
	return v.visibleSnaps[cursor], true
}

func filterSnaps(snaps []core.Snapshot, query string) []core.Snapshot {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		filtered := make([]core.Snapshot, len(snaps))
		copy(filtered, snaps)
		return filtered
	}

	var filtered []core.Snapshot
	for _, snap := range snaps {
		if strings.Contains(strings.ToLower(snap.Name), normalizedQuery) ||
			strings.Contains(strings.ToLower(snap.ID), normalizedQuery) ||
			strings.Contains(strings.ToLower(snap.SourceDiskID), normalizedQuery) ||
			strings.Contains(strings.ToLower(snap.SourceDiskName), normalizedQuery) ||
			strings.Contains(strings.ToLower(snap.Description), normalizedQuery) ||
			strings.Contains(strings.ToLower(snap.Labels), normalizedQuery) {
			filtered = append(filtered, snap)
		}
	}
	return filtered
}

func createSnapTable(cfg config.AppConfig, availableWidth int, offset int) (table.Model, []table.Column, int, bool, bool) {
	columns := buildSnapshotColumns(cfg)
	return ui.NewResourceTable(columns, availableWidth, offset, "Name")
}

func buildSnapshotColumns(cfg config.AppConfig) []table.Column {
	preferredWidths := map[string]int{
		"Name": 22, "ID": 18, "State": 12, "Size (GB)": 12,
		"Source Disk": 18, "Created At": 18, "Zone": 14,
		"Resource Group": 18, "Description": 24, "Labels": 24,
	}
	var columns []table.Column
	for _, col := range cfg.SnapshotColumns {
		width := preferredWidths[col]
		if width == 0 {
			width = 12
		}
		columns = append(columns, table.Column{Title: col, Width: width})
	}
	return columns
}

func mapSnapsToRows(snaps []core.Snapshot, columns []table.Column) []table.Row {
	var rows []table.Row
	for _, snap := range snaps {
		var row []string
		for _, col := range columns {
			row = append(row, ui.TruncateText(snap.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	return rows
}

func sortSnaps(snaps []core.Snapshot, column string, asc bool) {
	colName := column
	compare := func(i, j int) bool {
		valI := snaps[i].GetField(colName)
		valJ := snaps[j].GetField(colName)
		if asc {
			return strings.Compare(valI, valJ) < 0
		}
		return strings.Compare(valI, valJ) > 0
	}
	sortSlice(snaps, compare)
}

func sortSlice(snaps []core.Snapshot, less func(i, j int) bool) {
	for i := 1; i < len(snaps); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			snaps[j], snaps[j-1] = snaps[j-1], snaps[j]
		}
	}
}

// --- Commands ---

func (v *SnapshotsView) fetchSnapsCmd(force bool) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	mode := strings.ToUpper(v.cfg.Backend)
	return func() tea.Msg {
		applog.Infof("component=snapshots event=fetch_start provider=%s account=%s region=%s mode=%s force=%t", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, force)
		cacheKey := activeCtx.CacheKey()
		ttl := time.Duration(v.cfg.CacheTTL) * time.Minute
		if !force {
			if entry, ok := v.snapCache[cacheKey]; ok {
				if time.Since(entry.timestamp) < ttl {
					return snapFetchMsg{requestKey: requestKey, snaps: entry.snaps}
				}
			}
		}
		provider := providers.GetProvider(*v.cfg)
		dp, ok := provider.(providers.SnapshotProvider)
		if !ok {
			return snapFetchMsg{requestKey: requestKey, err: fmt.Errorf("SnapshotProvider not implemented")}
		}
		rows, err := dp.FetchSnapshots(context.Background(), activeCtx)
		if err != nil {
			return snapFetchMsg{requestKey: requestKey, err: err}
		}
		v.snapCache[cacheKey] = cacheEntry{snaps: rows, timestamp: time.Now()}
		return snapFetchMsg{requestKey: requestKey, snaps: rows}
	}
}

func executeSnapActionCmd(action string, snap core.Snapshot, cloudCtx core.CloudContext, cfg *config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(*cfg)
		dp, ok := provider.(providers.SnapshotProvider)
		if !ok {
			return commandCompleteMsg{err: fmt.Errorf("SnapshotProvider not implemented")}
		}
		output, err := dp.ExecuteSnapshotAction(context.Background(), action, snap, cloudCtx)
		if action == "Describe" {
			return describeCompleteMsg{output: output, err: err}
		}
		return commandCompleteMsg{output: output, err: err}
	}
}
