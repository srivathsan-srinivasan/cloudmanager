package networks

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
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/ui"
	"github.com/vyoogam/cloudmanager/internal/views/vms"
)

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneSubnets
	paneSubnetActions
	paneSortConfig
)

type networkFetchMsg struct {
	requestKey string
	networks   []core.Network
	err        error
}

type subnetFetchMsg struct {
	requestKey string
	subnets    []core.Subnet
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

type NetworksView struct {
	networks      table.Model
	subnets       table.Model
	actions       list.Model
	subnetActions list.Model
	sortList      list.Model
	descView      viewport.Model
	searchInput   textinput.Model
	activePane    int

	networkData    []core.Network
	subnetData     []core.Subnet
	visibleRows    []core.Network
	visibleSubnets []core.Subnet
	activeCtx      core.CloudContext
	cfg            *config.AppConfig
	loading        bool
	notSupported   bool
	isSearching    bool
	breadcrumbs    string
	requestKey     string
	width, height  int
	showSidebar    bool
	pendingNetwork core.Network
	filterVPCID    string
	subnetSearch   string
	copyableText   string
	detailURL      string
	sortColumn     string
	sortAsc        bool
	subnetSortCol  string
	subnetSortAsc  bool
	sortHeader     ui.HeaderSortState
	subnetHeader   ui.HeaderSortState
}

func New(cfg *config.AppConfig) *NetworksView {
	var actionItems []list.Item
	for _, action := range core.NetworkActions() {
		actionItems = append(actionItems, actionItem{title: action.Title, desc: action.Description})
	}

	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actions := list.New(actionItems, actionDelegate, 30, 10)
	actions.Title = "Network Actions"
	actions.SetShowStatusBar(false)
	actions.SetFilteringEnabled(false)

	var subnetActionItems []list.Item
	for _, action := range core.SubnetActions() {
		subnetActionItems = append(subnetActionItems, actionItem{title: action.Title, desc: action.Description})
	}
	subnetActions := list.New(subnetActionItems, actionDelegate, 30, 10)
	subnetActions.Title = "Subnet Actions"
	subnetActions.SetShowStatusBar(false)
	subnetActions.SetFilteringEnabled(false)

	sortList := ui.NewSortList("Sort Networks by (Enter to select, Esc to cancel)", networkSortColumns())

	descView := viewport.New(80, 20)
	descView.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search networks..."
	searchInput.Prompt = "/ "

	// Networks table
	netCols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 18},
		{Title: "CIDR", Width: 18},
		{Title: "State", Width: 10},
		{Title: "Subnets", Width: 8},
		{Title: "Region", Width: 12},
	}
	netTbl := table.New(table.WithColumns(netCols), table.WithFocused(true))
	netTbl.SetStyles(ui.DefaultTableStyles())

	// Subnets table
	subCols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 18},
		{Title: "CIDR", Width: 18},
		{Title: "AZ", Width: 12},
		{Title: "Network", Width: 15},
		{Title: "Available IPs", Width: 12},
	}
	subTbl := table.New(table.WithColumns(subCols), table.WithFocused(true))
	subTbl.SetStyles(ui.DefaultTableStyles())

	return &NetworksView{
		networks:      netTbl,
		subnets:       subTbl,
		actions:       actions,
		subnetActions: subnetActions,
		sortList:      sortList,
		descView:      descView,
		searchInput:   searchInput,
		activePane:    paneTable,
		cfg:           cfg,
		breadcrumbs:   "Select a context to view networks",
		sortAsc:       true,
		subnetSortAsc: true,
	}
}

func (v *NetworksView) Title() string { return "Networks" }

func (v *NetworksView) ShortHelp() string {
	if v.activePane == paneSubnets {
		return "\u2191\u2193: Navigate \u2022 ↑ at top: Columns \u2022 Enter: Sort/Actions \u2022 Esc: Back"
	}
	return "\u2191\u2193: Navigate \u2022 ↑ at top: Columns \u2022 Enter: Sort/Actions \u2022 /: Search \u2022 r: Refresh"
}

func (v *NetworksView) IsInputActive() bool {
	return v.sortHeader.Active || v.subnetHeader.Active || v.isSearching || v.activePane == paneActions || v.activePane == paneDescribe || v.activePane == paneSortConfig
}

func (v *NetworksView) SetSearchQuery(query string) {
	query = strings.TrimSpace(query)
	v.isSearching = false
	v.searchInput.Blur()
	if subnetID, ok := strings.CutPrefix(query, "subnet:"); ok {
		v.activePane = paneSubnets
		v.filterVPCID = ""
		v.subnetSearch = strings.TrimSpace(subnetID)
		v.searchInput.SetValue("")
		v.syncVisibleSubnets()
		return
	}
	v.activePane = paneTable
	v.subnetSearch = ""
	v.searchInput.SetValue(query)
	v.syncVisibleRows()
}

func (v *NetworksView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.activePane = paneTable
	v.isSearching = false
	v.searchInput.SetValue("")
	v.subnetSearch = ""
	v.copyableText = ""
	v.detailURL = ""
	v.requestKey = ctx.CacheKey()
	v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s \u203A Networks", ctx.Provider, ctx.DisplayName(), ctx.Region)
	v.loading = true
	v.notSupported = !providers.Supports(ctx.Provider, providers.CapabilityNetworks)

	if v.notSupported {
		v.loading = false
		return nil
	}
	return v.fetchNetworksCmd()
}

func (v *NetworksView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.showSidebar = showSidebar
	v.networks.SetWidth(ui.TableViewportWidth(width))
	v.networks.SetHeight(ui.TableHeight(height))
	v.subnets.SetWidth(ui.TableViewportWidth(width))
	v.subnets.SetHeight(ui.TableHeight(height))
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
	v.subnetActions.SetSize(50, ui.ActionListHeight(len(v.subnetActions.Items()), height))
	v.sortList.SetSize(width-4, height-4)
	v.descView.Width = width - 4
	v.descView.Height = height - 4
}

func (v *NetworksView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.sortHeader.Active || v.subnetHeader.Active {
			return v.handleHeaderSortKeys(msg)
		}
		if v.isSearching {
			return v.handleSearchKeys(msg)
		}
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneSortConfig:
				v.activePane = paneTable
				return v, nil
			case paneSubnetActions:
				v.activePane = paneSubnets
				return v, nil
			case paneSubnets:
				v.activePane = paneTable
				v.filterVPCID = ""
				v.breadcrumbs = strings.Split(v.breadcrumbs, " \u203A Subnets")[0]
				return v, nil
			}
		}
		if v.activePane == paneDescribe {
			switch msg.String() {
			case "c", "C":
				if strings.TrimSpace(v.copyableText) == "" {
					return v, nil
				}
				return v, copyTextCmd(v.copyableText)
			case "o", "O":
				return v.openConsole(v.detailURL)
			}
		}

		switch v.activePane {
		case paneActions, paneSubnetActions:
			_, cmd = v.handleActionKeys(msg)
		case paneSortConfig:
			_, cmd = v.handleSortConfigKeys(msg)
		case paneSubnets:
			_, cmd = v.handleSubnetTableKeys(msg)
		default:
			_, cmd = v.handleTableKeys(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case networkFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		if msg.err != nil {
			v.networkData = nil
		} else {
			v.networkData = msg.networks
			v.syncVisibleRows()
		}

	case subnetFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		if msg.err == nil {
			v.subnetData = msg.subnets
			v.syncVisibleSubnets()
		}

	case clipboardCompleteMsg:
		// Copy is intentionally silent in this view; keep the detail pane unchanged.
	case ui.BrowserOpenMsg:
		// Browser-open feedback is silent in this view.
	}

	switch v.activePane {
	case paneTable:
		v.networks, cmd = v.networks.Update(msg)
		cmds = append(cmds, cmd)
	case paneActions:
		v.actions, cmd = v.actions.Update(msg)
		cmds = append(cmds, cmd)
	case paneDescribe:
		v.descView, cmd = v.descView.Update(msg)
		cmds = append(cmds, cmd)
	case paneSortConfig:
		v.sortList, cmd = v.sortList.Update(msg)
		cmds = append(cmds, cmd)
	case paneSubnets:
		v.subnets, cmd = v.subnets.Update(msg)
		cmds = append(cmds, cmd)
	case paneSubnetActions:
		v.subnetActions, cmd = v.subnetActions.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

func (v *NetworksView) Render() string {
	header := ui.BreadcrumbStyle.Render(ui.TruncateText(v.breadcrumbs, v.width-2))

	if v.notSupported {
		msg := lipgloss.NewStyle().Padding(2).Foreground(ui.Alert).Render(fmt.Sprintf("Networks are not supported for %s.", v.activeCtx.Provider))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", msg), v.width, v.height)
	}

	var body string
	if v.loading {
		body = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading networks...")
	} else {
		switch v.activePane {
		case paneSubnets:
			body = ui.ColorizeOperationalStates(v.subnets.View())
		default:
			body = ui.ColorizeOperationalStates(v.networks.View())
			if v.isSearching || v.searchInput.Value() != "" {
				body = lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Padding(1, 2).Render(v.searchInput.View()),
					body,
				)
			}
		}
	}

	switch v.activePane {
	case paneActions:
		overlay := ui.OverlayStyle.Render(v.actions.View())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	case paneSubnetActions:
		overlay := ui.OverlayStyle.Render(v.subnetActions.View())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	case paneDescribe:
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	case paneSortConfig:
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	default:
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", body), v.width, v.height)
	}
}

func (v *NetworksView) handleTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v.networks.SelectedRow() != nil {
			v.activePane = paneActions
		}
	case "/":
		v.isSearching = true
		v.searchInput.Focus()
	case "r":
		v.loading = true
		return v, v.fetchNetworksCmd()
	case "S":
		v.sortHeader.Activate(networkColumns())
		v.syncVisibleRows()
	case "up":
		if v.networks.Cursor() == 0 {
			v.sortHeader.Activate(networkColumns())
			v.syncVisibleRows()
		}
	}
	return v, nil
}

func (v *NetworksView) handleSubnetTableKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v.subnets.SelectedRow() != nil {
			v.activePane = paneSubnetActions
		}
	case "S":
		v.subnetHeader.Activate(subnetColumns())
		v.syncVisibleSubnets()
	case "up":
		if v.subnets.Cursor() == 0 {
			v.subnetHeader.Activate(subnetColumns())
			v.syncVisibleSubnets()
		}
	}
	return v, nil
}

func (v *NetworksView) handleHeaderSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if v.subnetHeader.Active {
		cols := subnetColumns()
		switch msg.String() {
		case "esc", "down":
			v.subnetHeader.Deactivate()
		case "left", "h":
			v.subnetHeader.Move(-1, cols)
		case "right", "l":
			v.subnetHeader.Move(1, cols)
		case "enter":
			column := v.subnetHeader.SelectedColumn(cols)
			v.subnetSortCol, v.subnetSortAsc = ui.ToggleSortColumn(v.subnetSortCol, v.subnetSortAsc, column)
		}
		v.syncVisibleSubnets()
		return v, nil
	}
	cols := networkColumns()
	switch msg.String() {
	case "esc", "down":
		v.sortHeader.Deactivate()
	case "left", "h":
		v.sortHeader.Move(-1, cols)
	case "right", "l":
		v.sortHeader.Move(1, cols)
	case "enter":
		column := v.sortHeader.SelectedColumn(cols)
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, column)
	}
	v.syncVisibleRows()
	return v, nil
}

func (v *NetworksView) handleSortConfigKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() != "enter" {
		return v, nil
	}
	selected, ok := v.sortList.SelectedItem().(ui.SortColumnItem)
	if !ok {
		return v, nil
	}
	if v.filterVPCID != "" || strings.Contains(v.sortList.Title, "Subnets") {
		v.subnetSortCol, v.subnetSortAsc = ui.ToggleSortColumn(v.subnetSortCol, v.subnetSortAsc, selected.Name)
		v.syncVisibleSubnets()
		v.activePane = paneSubnets
		return v, nil
	}
	v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, selected.Name)
	v.syncVisibleRows()
	v.activePane = paneTable
	return v, nil
}

func (v *NetworksView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() == "enter" {
		if v.activePane == paneActions {
			return v.handleNetworkAction()
		} else if v.activePane == paneSubnetActions {
			return v.handleSubnetAction()
		}
	}
	return v, nil
}

func (v *NetworksView) handleNetworkAction() (ui.View, tea.Cmd) {
	idx := v.networks.Cursor()
	if idx < 0 || idx >= len(v.visibleRows) {
		return v, nil
	}
	net := v.visibleRows[idx]
	action := v.actions.SelectedItem().(actionItem).title

	switch action {
	case "View Subnets":
		v.activePane = paneSubnets
		v.filterVPCID = net.ID
		v.subnetSearch = ""
		v.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A Subnets", v.breadcrumbs, net.ID)
		v.syncVisibleSubnets()
		if len(v.subnetData) == 0 {
			return v, v.fetchSubnetsCmd()
		}
		return v, nil
	case "Describe":
		v.detailURL = core.NetworkConsoleURL(v.activeCtx, net)
		content := fmt.Sprintf("Network: %s\nID: %s\nCIDR: %s\nState: %s\nRegion: %s\nLabels: %s",
			net.Name, net.ID, net.CIDRBlock, net.State, net.Region, net.Labels)
		v.copyableText = core.DetailWithConsoleURL(content, v.detailURL)
		v.descView.SetContent(v.copyableText)
		v.activePane = paneDescribe
		return v, nil
	case "Open Console":
		return v.openConsole(core.NetworkConsoleURL(v.activeCtx, net))
	}
	return v, nil
}

func (v *NetworksView) handleSubnetAction() (ui.View, tea.Cmd) {
	idx := v.subnets.Cursor()
	if idx < 0 || idx >= len(v.visibleSubnets) {
		return v, nil
	}
	sub := v.visibleSubnets[idx]
	action := v.subnetActions.SelectedItem().(actionItem).title

	switch action {
	case "View VMs":
		v.activePane = paneSubnets
		return v, func() tea.Msg {
			return ui.PushViewMsg{
				View: vms.NewFiltered(v.cfg, []string{sub.ID, sub.Name}, sub.ID),
				Ctx:  v.activeCtx,
			}
		}
	case "Describe":
		v.detailURL = core.SubnetConsoleURL(v.activeCtx, sub)
		content := fmt.Sprintf("Subnet: %s\nID: %s\nCIDR: %s\nAZ: %s\nNetwork: %s (%s)\nState: %s\nRegion: %s",
			sub.Name, sub.ID, sub.CIDRBlock, sub.AvailabilityZone, sub.NetworkName, sub.NetworkID, sub.State, sub.Region)
		v.copyableText = core.DetailWithConsoleURL(content, v.detailURL)
		v.descView.SetContent(v.copyableText)
		v.activePane = paneDescribe
		return v, nil
	case "Open Console":
		return v.openConsole(core.SubnetConsoleURL(v.activeCtx, sub))
	}
	return v, nil
}

func (v *NetworksView) handleSearchKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
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

func (v *NetworksView) fetchNetworksCmd() tea.Cmd {
	requestKey := v.requestKey
	activeCtx := v.activeCtx
	return func() tea.Msg {
		provider := providers.GetProvider(*v.cfg)
		netProvider, ok := provider.(providers.NetworkProvider)
		if !ok {
			return networkFetchMsg{requestKey: requestKey, err: fmt.Errorf("NetworkProvider not implemented")}
		}
		nets, err := netProvider.FetchNetworks(context.Background(), activeCtx)
		return networkFetchMsg{requestKey: requestKey, networks: nets, err: err}
	}
}

func (v *NetworksView) fetchSubnetsCmd() tea.Cmd {
	requestKey := v.requestKey
	activeCtx := v.activeCtx
	return func() tea.Msg {
		provider := providers.GetProvider(*v.cfg)
		netProvider, ok := provider.(providers.NetworkProvider)
		if !ok {
			return subnetFetchMsg{requestKey: requestKey, err: fmt.Errorf("NetworkProvider not implemented")}
		}
		subs, err := netProvider.FetchSubnets(context.Background(), activeCtx)
		return subnetFetchMsg{requestKey: requestKey, subnets: subs, err: err}
	}
}

func (v *NetworksView) syncVisibleRows() {
	query := strings.ToLower(v.searchInput.Value())
	var filtered []core.Network
	for _, n := range v.networkData {
		if query == "" || strings.Contains(strings.ToLower(n.Name), query) || strings.Contains(strings.ToLower(n.ID), query) {
			filtered = append(filtered, n)
		}
	}
	v.visibleRows = filtered
	ui.SortByColumn(v.visibleRows, v.sortColumn, v.sortAsc, func(n core.Network, column string) string {
		return networkField(n, column)
	})
	var rows []table.Row
	for _, n := range v.visibleRows {
		rows = append(rows, table.Row{n.Name, n.ID, n.CIDRBlock, n.State, fmt.Sprintf("%d", n.SubnetCount), n.Region})
	}
	v.networks.SetRows(rows)
	v.networks.SetColumns(ui.DecorateSortColumns(networkColumns(), v.sortHeader, v.sortColumn, v.sortAsc))
}

func (v *NetworksView) syncVisibleSubnets() {
	query := strings.ToLower(strings.TrimSpace(v.subnetSearch))
	var filtered []core.Subnet
	for _, s := range v.subnetData {
		matchesNetwork := v.filterVPCID == "" || s.NetworkID == v.filterVPCID
		blob := strings.ToLower(strings.Join([]string{s.Name, s.ID, s.CIDRBlock, s.AvailabilityZone, s.NetworkID, s.NetworkName, s.Region, s.ResourceGroup, s.Labels}, " "))
		matchesQuery := query == "" || strings.Contains(blob, query)
		if matchesNetwork && matchesQuery {
			filtered = append(filtered, s)
		}
	}
	v.visibleSubnets = filtered
	ui.SortByColumn(v.visibleSubnets, v.subnetSortCol, v.subnetSortAsc, func(s core.Subnet, column string) string {
		return subnetField(s, column)
	})
	var rows []table.Row
	for _, s := range v.visibleSubnets {
		rows = append(rows, table.Row{s.Name, s.ID, s.CIDRBlock, s.AvailabilityZone, s.NetworkName, fmt.Sprintf("%d", s.AvailableIPs)})
	}
	v.subnets.SetRows(rows)
	v.subnets.SetColumns(ui.DecorateSortColumns(subnetColumns(), v.subnetHeader, v.subnetSortCol, v.subnetSortAsc))
}

func networkSortColumns() []table.Column {
	return networkColumns()
}

func subnetSortColumns() []table.Column {
	return subnetColumns()
}

func networkColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 18},
		{Title: "CIDR", Width: 18},
		{Title: "State", Width: 10},
		{Title: "Subnets", Width: 8},
		{Title: "Region", Width: 12},
	}
}

func subnetColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 18},
		{Title: "CIDR", Width: 18},
		{Title: "AZ", Width: 12},
		{Title: "Network", Width: 15},
		{Title: "Available IPs", Width: 12},
	}
}

func networkField(n core.Network, column string) string {
	switch column {
	case "Name":
		return n.Name
	case "ID":
		return n.ID
	case "CIDR":
		return n.CIDRBlock
	case "State":
		return n.State
	case "Subnets":
		return fmt.Sprintf("%d", n.SubnetCount)
	case "Region":
		return n.Region
	default:
		return ""
	}
}

func subnetField(s core.Subnet, column string) string {
	switch column {
	case "Name":
		return s.Name
	case "ID":
		return s.ID
	case "CIDR":
		return s.CIDRBlock
	case "AZ":
		return s.AvailabilityZone
	case "Network":
		return s.NetworkName
	case "Available IPs":
		return fmt.Sprintf("%d", s.AvailableIPs)
	default:
		return ""
	}
}

func copyTextCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardCompleteMsg{err: clipboard.Write(text)}
	}
}

func (v *NetworksView) openConsole(consoleURL string) (ui.View, tea.Cmd) {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		return v, nil
	}
	return v, ui.OpenURLCmd(consoleURL)
}
