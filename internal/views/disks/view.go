package disks

import (
	"context"
	"fmt"
	"strconv"
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
	paneResize
)

// --- Bubble Tea messages ---

type diskFetchMsg struct {
	requestKey string
	disks      []core.Disk
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
	disks     []core.Disk
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

// --- DisksView ---

// DisksView implements ui.View for the Disk list table.
type DisksView struct {
	disks            table.Model
	actions          list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	resizeInput      textinput.Model
	activePane       int

	diskData     []core.Disk
	visibleDisks []core.Disk
	diskCache    map[string]cacheEntry
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
	pendingDisk   core.Disk

	width, height int
	showSidebar   bool
	requestKey    string
}

// New creates a new DisksView.
func New(cfg *config.AppConfig) *DisksView {
	// Actions list
	var actionItems []list.Item
	for _, a := range core.DiskActions() {
		actionItems = append(actionItems, actionItem{title: a.Title, desc: a.Description, isDangerous: a.Dangerous})
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actionList := list.New(actionItems, actionDelegate, 30, 15)
	actionList.Title = "Disk Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	// Column config
	var colItems []list.Item
	for _, c := range core.DefaultDiskColumns {
		selected := false
		for _, cfgCol := range cfg.DiskColumns {
			if cfgCol == c {
				selected = true
				break
			}
		}
		colItems = append(colItems, columnItem{name: c, selected: selected})
	}
	colList := list.New(colItems, list.NewDefaultDelegate(), 0, 0)
	colList.Title = "Configure Disk Columns (Space to toggle, Enter to save, Esc to cancel)"
	colList.SetShowStatusBar(false)

	// Sort config
	var sItems []list.Item
	for _, c := range core.DefaultDiskColumns {
		sItems = append(sItems, sortItem{name: c})
	}
	sortList := list.New(sItems, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = "Sort Disks by (Enter to select, Esc to cancel)"
	sortList.SetShowStatusBar(false)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search disks..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	resizeInput := textinput.New()
	resizeInput.Placeholder = "Enter new size in GB..."
	resizeInput.Prompt = "> "
	resizeInput.CharLimit = 10
	resizeInput.Width = 20

	diskTable, tableCols, _, canScrollLeft, canScrollRight := createDiskTable(*cfg, 80, 0)

	return &DisksView{
		disks: diskTable, actions: actionList,
		columnConfigList: colList, sortList: sortList,
		descView: vp, searchInput: searchInput, resizeInput: resizeInput,
		activePane: paneTable, tableCols: tableCols,
		cfg: cfg, diskCache: make(map[string]cacheEntry),
		sortColumn: "Name", sortAsc: true,
		canScrollLeft: canScrollLeft, canScrollRight: canScrollRight,
		breadcrumbs: "Select a context to view disks",
	}
}

func (v *DisksView) Title() string { return "Disks" }

func (v *DisksView) ShortHelp() string {
	return "\u2191\u2193: Navigate \u2022 \u2190\u2192: Pan \u2022 Enter: Actions \u2022 /: Search \u2022 S: Sort \u2022 C: Columns \u2022 r: Refresh"
}

func (v *DisksView) IsInputActive() bool {
	return v.isSearching || v.activePane == paneResize || v.activePane == paneColumnConfig || v.activePane == paneSortConfig || v.activePane == paneActions || v.activePane == paneConfirm || v.activePane == paneDescribe
}

func (v *DisksView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.diskData = nil
	v.visibleDisks = nil
	v.requestKey = ctx.CacheKey()
	v.columnOffset = 0
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s", ctx.Provider, ctx.DisplayName(), ctx.Region)
	v.loading = true
	v.notSupported = false

	if !providers.Supports(ctx.Provider, providers.CapabilityDisks) {
		v.notSupported = true
		v.loading = false
		v.statusMsg = fmt.Sprintf("Disks not supported for %s", ctx.Provider)
		return nil
	}

	v.statusMsg = fmt.Sprintf("Fetching disks for %s...", ctx.DisplayName())
	v.refreshTable()
	v.disks.Focus()
	return v.fetchDisksCmd(false)
}

func (v *DisksView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.refreshTable()
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.columnConfigList.SetSize(width-4, height-4)
	v.sortList.SetSize(width-4, height-4)
}

func (v *DisksView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.notSupported {
			if msg.String() == "c" {
				// We don't change config here directly usually, it is done globally,
				// but user should go to config or we can just ignore it or send a msg to global state.
			}
			return v, nil
		}
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneColumnConfig, paneSortConfig, paneResize:
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
		case paneResize:
			_, cmd = v.handleResizeKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case diskFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.diskData = msg.disks
		if msg.err != nil {
			applog.Errorf("component=disks event=fetch_failed provider=%s account=%s region=%s mode=%s err=%v", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), msg.err)
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			v.visibleDisks = nil
			v.disks.SetRows([]table.Row{})
		} else {
			applog.Infof("component=disks event=fetch_completed provider=%s account=%s region=%s mode=%s count=%d", v.activeCtx.Provider, v.activeCtx.AccountID, v.activeCtx.Region, strings.ToUpper(v.cfg.Backend), len(msg.disks))
			sortDisks(v.diskData, v.sortColumn, v.sortAsc)
			v.syncVisibleRows()
			v.statusMsg = fmt.Sprintf("Loaded %d disks.", len(msg.disks))
		}

	case commandCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = msg.output
			v.loading = true
			return v, v.fetchDisksCmd(true)
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
		v.disks, cmd = v.disks.Update(msg)
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
	case paneResize:
		v.resizeInput, cmd = v.resizeInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *DisksView) Render() string {
	header := ui.AppendScrollHint(ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2)), v.canScrollLeft, v.canScrollRight, v.width)

	if v.notSupported {
		msg := lipgloss.NewStyle().Padding(2).Foreground(ui.Alert).Render("Switch to SDK backend (press 'c') for disk management.")
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", msg), v.width, v.height)
	}

	tableContent := v.disks.View()

	if v.loading {
		tableContent = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading disks...")
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
		confirmMsg := fmt.Sprintf("Are you sure you want to %s disk %s?", v.pendingAction.title, v.pendingDisk.Name)
		confirmStyle := ui.OverlayStyle.Copy().BorderForeground(ui.Alert).Padding(1, 2).Width(50)
		confirmView := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ui.Alert).Bold(true).Render("⚠️  CONFIRM ACTION"),
			"\n", lipgloss.NewStyle().Align(lipgloss.Center).Render(confirmMsg),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Confirm \u2022 Esc: Cancel"),
		)
		overlay := confirmStyle.Render(confirmView)
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	case paneResize:
		resizeMsg := fmt.Sprintf("Current size is %d GB. Enter new size for %s:", v.pendingDisk.SizeGB, v.pendingDisk.Name)
		resizeStyle := ui.OverlayStyle.Copy().Padding(1, 2).Width(50)
		resizeView := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Resize Disk"),
			"\n", lipgloss.NewStyle().Render(resizeMsg),
			"\n", v.resizeInput.View(),
			"\n", lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Submit \u2022 Esc: Cancel"),
		)
		overlay := resizeStyle.Render(resizeView)
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", tableContent), v.width, v.height)
}

// --- Key handlers ---

func (v *DisksView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v.disks.SelectedRow() != nil {
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
		v.statusMsg = fmt.Sprintf("Refreshing disks for %s...", v.activeCtx.DisplayName())
		return v, v.fetchDisksCmd(true)
	case "S":
		v.activePane = paneSortConfig
	case "C":
		v.activePane = paneColumnConfig
	}
	return v, nil
}

func (v *DisksView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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

func (v *DisksView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		if disk, ok := v.selectedDisk(); ok {
			action := v.actions.SelectedItem().(actionItem)

			if action.isDangerous {
				v.pendingAction = action
				v.pendingDisk = disk
				v.activePane = paneConfirm
				return v, nil
			}
			if action.title == "Resize" {
				v.pendingAction = action
				v.pendingDisk = disk
				v.activePane = paneResize
				v.resizeInput.SetValue("")
				v.resizeInput.Focus()
				return v, nil
			}
			if action.title == "Describe" {
				v.activePane = paneDescribe
				return v, func() tea.Msg {
					return describeCompleteMsg{output: core.DescribeDisk(disk)}
				}
			}

			// Other actions (Detach, Create Snapshot)
			v.activePane = paneTable
			v.statusMsg = fmt.Sprintf("Executing %s on %s...", action.title, disk.Name)
			return v, executeDiskActionCmd(action.title, disk, v.activeCtx, v.cfg)
		}
	}
	return v, nil
}

func (v *DisksView) handleConfirmKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		return v, nil
	case "enter":
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Executing %s on %s...", v.pendingAction.title, v.pendingDisk.Name)
		return v, executeDiskActionCmd(v.pendingAction.title, v.pendingDisk, v.activeCtx, v.cfg)
	}
	return v, nil
}

func (v *DisksView) handleResizeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.activePane = paneTable
		v.resizeInput.Blur()
		return v, nil
	case "enter":
		val := v.resizeInput.Value()
		newSize, err := strconv.Atoi(val)
		if err != nil || newSize <= v.pendingDisk.SizeGB {
			v.statusMsg = fmt.Sprintf("Invalid size. Must be an integer greater than %d.", v.pendingDisk.SizeGB)
			return v, nil
		}

		v.activePane = paneTable
		v.resizeInput.Blur()
		v.statusMsg = fmt.Sprintf("Executing %s on %s...", v.pendingAction.title, v.pendingDisk.Name)

		// Typically resize requires modifying the action string or passing it differently.
		// For now we'll pass the new size in the action string, e.g., "Resize 100"
		// The actual execution cmd expects "action" so let's format it.
		actionStr := fmt.Sprintf("Resize:%d", newSize)

		return v, executeDiskActionCmd(actionStr, v.pendingDisk, v.activeCtx, v.cfg)
	}
	return v, nil
}

func (v *DisksView) handleDescribeKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() == "esc" {
		v.activePane = paneTable
	}
	return v, nil
}

func (v *DisksView) handleColumnConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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
			selectedColumns = core.DefaultDiskColumns
		}
		v.cfg.DiskColumns = selectedColumns
		config.Save(*v.cfg)
		v.refreshTable()
		v.activePane = paneTable
		v.statusMsg = "Columns saved."
		return v, nil
	}
	return v, nil
}

func (v *DisksView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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
			sortDisks(v.diskData, v.sortColumn, v.sortAsc)
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

func (v *DisksView) refreshTable() {
	if v.width == 0 {
		return
	}
	cursor := v.disks.Cursor()
	focused := v.disks.Focused()
	newDisks, newCols, nextOffset, canScrollLeft, canScrollRight := createDiskTable(*v.cfg, v.width, v.columnOffset)
	v.columnOffset = nextOffset
	v.canScrollLeft = canScrollLeft
	v.canScrollRight = canScrollRight
	newDisks.SetHeight(ui.TableHeight(v.height))
	newDisks.SetWidth(ui.TableViewportWidth(v.width))
	rows := mapDisksToRows(v.visibleRowsSource(), newCols)
	newDisks.SetRows(rows)
	if cursor >= 0 && cursor < len(rows) {
		newDisks.SetCursor(cursor)
	}
	if focused {
		newDisks.Focus()
	}
	v.disks = newDisks
	v.tableCols = newCols
}

func (v *DisksView) syncVisibleRows() {
	v.visibleDisks = filterDisks(v.diskData, v.searchInput.Value())
	rows := mapDisksToRows(v.visibleDisks, v.tableCols)
	v.disks.SetRows(rows)
	if len(rows) == 0 {
		v.disks.SetCursor(0)
		return
	}
	if cursor := v.disks.Cursor(); cursor >= len(rows) {
		v.disks.SetCursor(len(rows) - 1)
	}
}

func (v *DisksView) visibleRowsSource() []core.Disk {
	if v.visibleDisks != nil {
		return v.visibleDisks
	}
	return filterDisks(v.diskData, v.searchInput.Value())
}

func (v *DisksView) selectedDisk() (core.Disk, bool) {
	cursor := v.disks.Cursor()
	if cursor < 0 || cursor >= len(v.visibleDisks) {
		return core.Disk{}, false
	}
	return v.visibleDisks[cursor], true
}

func filterDisks(disks []core.Disk, query string) []core.Disk {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		filtered := make([]core.Disk, len(disks))
		copy(filtered, disks)
		return filtered
	}

	var filtered []core.Disk
	for _, disk := range disks {
		if strings.Contains(strings.ToLower(disk.Name), normalizedQuery) ||
			strings.Contains(strings.ToLower(disk.ID), normalizedQuery) ||
			strings.Contains(strings.ToLower(disk.AttachedToVM), normalizedQuery) ||
			strings.Contains(strings.ToLower(disk.Labels), normalizedQuery) {
			filtered = append(filtered, disk)
		}
	}
	return filtered
}

func createDiskTable(cfg config.AppConfig, availableWidth int, offset int) (table.Model, []table.Column, int, bool, bool) {
	columns := buildDiskColumns(cfg)
	return ui.NewResourceTable(columns, availableWidth, offset, "Name")
}

func buildDiskColumns(cfg config.AppConfig) []table.Column {
	preferredWidths := map[string]int{
		"Name": 22, "ID": 18, "State": 12, "Size (GB)": 12,
		"Type": 12, "Attached To": 18, "Zone": 14, "Encrypted": 10,
		"IOPS": 10, "Throughput": 12, "Created At": 18, "Resource Group": 18, "Labels": 24,
	}
	var columns []table.Column
	for _, col := range cfg.DiskColumns {
		width := preferredWidths[col]
		if width == 0 {
			width = 12
		}
		columns = append(columns, table.Column{Title: col, Width: width})
	}
	return columns
}

func mapDisksToRows(disks []core.Disk, columns []table.Column) []table.Row {
	var rows []table.Row
	for _, disk := range disks {
		var row []string
		for _, col := range columns {
			row = append(row, ui.TruncateText(disk.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	return rows
}

func sortDisks(disks []core.Disk, column string, asc bool) {
	colName := column
	compare := func(i, j int) bool {
		valI := disks[i].GetField(colName)
		valJ := disks[j].GetField(colName)
		if asc {
			return strings.Compare(valI, valJ) < 0
		}
		return strings.Compare(valI, valJ) > 0
	}
	sortSlice(disks, compare)
}

func sortSlice(disks []core.Disk, less func(i, j int) bool) {
	for i := 1; i < len(disks); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			disks[j], disks[j-1] = disks[j-1], disks[j]
		}
	}
}

// --- Commands ---

func (v *DisksView) fetchDisksCmd(force bool) tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	mode := strings.ToUpper(v.cfg.Backend)
	return func() tea.Msg {
		applog.Infof("component=disks event=fetch_start provider=%s account=%s region=%s mode=%s force=%t", activeCtx.Provider, activeCtx.AccountID, activeCtx.Region, mode, force)
		cacheKey := activeCtx.CacheKey()
		ttl := time.Duration(v.cfg.CacheTTL) * time.Minute
		if !force {
			if entry, ok := v.diskCache[cacheKey]; ok {
				if time.Since(entry.timestamp) < ttl {
					return diskFetchMsg{requestKey: requestKey, disks: entry.disks}
				}
			}
		}
		provider := providers.GetProvider(*v.cfg)
		dp, ok := provider.(providers.DiskProvider)
		if !ok {
			return diskFetchMsg{requestKey: requestKey, err: fmt.Errorf("DiskProvider not implemented")}
		}
		rows, err := dp.FetchDisks(context.Background(), activeCtx)
		if err != nil {
			return diskFetchMsg{requestKey: requestKey, err: err}
		}
		v.diskCache[cacheKey] = cacheEntry{disks: rows, timestamp: time.Now()}
		return diskFetchMsg{requestKey: requestKey, disks: rows}
	}
}

func executeDiskActionCmd(action string, disk core.Disk, cloudCtx core.CloudContext, cfg *config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		provider := providers.GetProvider(*cfg)
		dp, ok := provider.(providers.DiskProvider)
		if !ok {
			return commandCompleteMsg{err: fmt.Errorf("DiskProvider not implemented")}
		}
		output, err := dp.ExecuteDiskAction(context.Background(), action, disk, cloudCtx)
		if action == "Describe" {
			return describeCompleteMsg{output: output, err: err}
		}
		return commandCompleteMsg{output: output, err: err}
	}
}
