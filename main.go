package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	paneContexts = iota
	paneVMs
	paneActions
	paneConfig
	paneColumnConfig
	paneDescribe
	paneSortConfig
	paneSplash
)

// Styling
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	alert     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle)

	activeStyle = baseStyle.
			BorderForeground(highlight)

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			MarginLeft(2).
			MarginBottom(1)

	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(special).
			Bold(true).
			Padding(0, 1)

	statusLineStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(1, 2)
)

// Data Models
type contextItem struct {
	provider    string
	accountID   string
	accountName string
	region      string
}

func (i contextItem) Title() string       { return fmt.Sprintf("%s - %s", i.provider, i.accountName) }
func (i contextItem) Description() string { return fmt.Sprintf("%s / %s", i.accountID, i.region) }
func (i contextItem) FilterValue() string { return i.Title() + " " + i.Description() }

type actionItem struct {
	title string
	desc  string
}

func (i actionItem) Title() string       { return i.title }
func (i actionItem) Description() string { return i.desc }
func (i actionItem) FilterValue() string { return i.title }

type gcpProjectItem struct {
	projectId string
	selected  bool
}

func (i gcpProjectItem) Title() string {
	if i.selected {
		return "[x] " + i.projectId
	}
	return "[ ] " + i.projectId
}
func (i gcpProjectItem) Description() string { return "GCP Project" }
func (i gcpProjectItem) FilterValue() string { return i.projectId }

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

type sortItem struct {
	name string
}

func (i sortItem) Title() string { return i.name }
func (i sortItem) Description() string { return "Sort by this column" }
func (i sortItem) FilterValue() string { return i.name }

// Main Model
type model struct {
	contexts         list.Model
	vms              table.Model
	actions          list.Model
	configList       list.Model
	columnConfigList list.Model
	sortList         list.Model
	descView         viewport.Model
	searchInput      textinput.Model
	activePane       int

	breadcrumbs string
	statusMsg   string

	width  int
	height int

	loading     bool
	showSidebar bool
	rootNodes   []*treeNode
	vmData      []VM
	activeCtx   contextItem

	sortColumn string
	sortAsc    bool
	tableCols  []table.Column
	isSearching bool
}

// Msgs for asynchronous operations
type vmFetchMsg struct {
	vms []VM
	err error
}

type contextLoadMsg struct {
	tree []*treeNode
}

type commandCompleteMsg struct {
	output string
	err    error
}

type gcpProjectFetchMsg struct {
	items []list.Item
}

type sshCompleteMsg struct {
	err error
}

type cacheEntry struct {
	vms       []VM
	timestamp time.Time
}

var vmCache = make(map[string]cacheEntry)
const cacheTTL = 5 * time.Minute

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		if max > 1 {
			return string(runes[:max-1]) + "…"
		}
		return string(runes[:max])
	}
	return s
}

func createVMTable(cfg AppConfig, availableWidth int) (table.Model, []table.Column) {
	var columns []table.Column
	if availableWidth <= 0 {
		availableWidth = 100 // fallback
	}
	
	tableWidth := availableWidth - 4 // Account for borders and padding
	if tableWidth < 20 {
		tableWidth = 20
	}

	weights := make(map[string]int)
	totalWeight := 0
	
	for _, col := range cfg.VMColumns {
		w := 10
		switch col {
		case "Name", "Instance ID", "Labels":
			w = 20
		case "Type", "Network", "Subnet", "Zone", "Resource Group":
			w = 12
		case "State":
			w = 8
		case "Private IP", "Public IP":
			w = 15
		}
		weights[col] = w
		totalWeight += w
	}

	for _, col := range cfg.VMColumns {
		colWidth := (weights[col] * tableWidth) / totalWeight
		if colWidth < 4 {
			colWidth = 4
		}
		columns = append(columns, table.Column{Title: col, Width: colWidth})
	}

	vmTable := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	vmTable.SetStyles(s)

	return vmTable, columns
}

func sortVMs(vms []VM, column string, asc bool) {
	sort.Slice(vms, func(i, j int) bool {
		var valI, valJ string
		switch column {
		case "Name":
			valI, valJ = vms[i].Name, vms[j].Name
		case "Instance ID":
			valI, valJ = vms[i].ID, vms[j].ID
		case "Type":
			valI, valJ = vms[i].Type, vms[j].Type
		case "State":
			valI, valJ = vms[i].State, vms[j].State
		case "Private IP":
			valI, valJ = vms[i].PrivateIP, vms[j].PrivateIP
		case "Public IP":
			valI, valJ = vms[i].PublicIP, vms[j].PublicIP
		case "Zone":
			valI, valJ = vms[i].Zone, vms[j].Zone
		case "Resource Group":
			valI, valJ = vms[i].ResourceGroup, vms[j].ResourceGroup
		case "Network":
			valI, valJ = vms[i].Network, vms[j].Network
		case "Subnet":
			valI, valJ = vms[i].Subnet, vms[j].Subnet
		case "Labels":
			valI, valJ = vms[i].Labels, vms[j].Labels
		default:
			valI, valJ = vms[i].Name, vms[j].Name
		}
		
		if asc {
			return strings.Compare(valI, valJ) < 0
		}
		return strings.Compare(valI, valJ) > 0
	})
}

func mapVMsToRows(vms []VM, columns []table.Column) []table.Row {
	var rows []table.Row
	for _, vm := range vms {
		var row []string
		for _, col := range columns {
			var val string
			switch col.Title {
			case "Name":
				val = vm.Name
			case "Instance ID":
				val = vm.ID
			case "Type":
				val = vm.Type
			case "State":
				val = vm.State
			case "Private IP":
				val = vm.PrivateIP
			case "Public IP":
				val = vm.PublicIP
			case "Zone":
				val = vm.Zone
			case "Resource Group":
				val = vm.ResourceGroup
			case "Network":
				val = vm.Network
			case "Subnet":
				val = vm.Subnet
			case "Labels":
				val = vm.Labels
			default:
				val = "-"
			}
			row = append(row, truncate(val, col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	return rows
}

func refreshVMTable(m model) model {
	if m.width == 0 {
		return m
	}
	cfg := loadAppConfig()
	mainWidth := m.width - 2
	if m.showSidebar {
		mainWidth = m.width - (m.width / 3) - 2
	}
	
	h := m.height - 4
	cursor := m.vms.Cursor()
	focused := m.vms.Focused()
	
	newVMs, newCols := createVMTable(cfg, mainWidth)
	newVMs.SetHeight(h - 5)
	newVMs.SetRows(mapVMsToRows(m.vmData, newCols))
	
	if cursor >= 0 && cursor < len(m.vmData) {
		newVMs.SetCursor(cursor)
	}
	if focused {
		newVMs.Focus()
	}
	
	m.vms = newVMs
	m.tableCols = newCols
	return m
}

func initialModel() model {
	// 1. Contexts List
	ctxList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	ctxList.Title = "Cloud Contexts"
	ctxList.SetShowStatusBar(false)
	ctxList.SetFilteringEnabled(true)

	// 2. VMs Table
	cfg := loadAppConfig()
	vmTable, tableCols := createVMTable(cfg, 0)

	// 3. Actions List (Dropdown)
	actionItems := []list.Item{
		actionItem{"Start", "Start the virtual machine"},
		actionItem{"Stop", "Gracefully stop the virtual machine"},
		actionItem{"Restart", "Reboot the virtual machine"},
		actionItem{"Terminate", "Permanently delete the virtual machine"},
		actionItem{"Describe", "Show full resource details"},
		actionItem{"SSH", "Connect to the instance via SSH"},
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	
	actionList := list.New(actionItems, actionDelegate, 30, 15)
	actionList.Title = "Instance Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	// 4. Config List
	configDelegate := list.NewDefaultDelegate()
	configList := list.New([]list.Item{}, configDelegate, 0, 0)
	configList.Title = "Configure GCP Projects (Space to toggle, Enter to save, Esc to cancel)"
	configList.SetShowStatusBar(false)

	// 5. Column Config List
	allColumns := []string{"Name", "Instance ID", "Type", "State", "Private IP", "Public IP", "Network", "Subnet", "Zone", "Resource Group", "Labels"}
	var colItems []list.Item
	for _, c := range allColumns {
		selected := false
		for _, cfgCol := range cfg.VMColumns {
			if cfgCol == c {
				selected = true
				break
			}
		}
		colItems = append(colItems, columnItem{name: c, selected: selected})
	}
	colList := list.New(colItems, list.NewDefaultDelegate(), 0, 0)
	colList.Title = "Configure VM Columns (Space to toggle, Enter to save, Esc to cancel)"
	colList.SetShowStatusBar(false)

	// 6. Sort Config List
	var sItems []list.Item
	for _, c := range allColumns {
		sItems = append(sItems, sortItem{name: c})
	}
	sortList := list.New(sItems, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = "Sort VMs by (Enter to select, Esc to cancel)"
	sortList.SetShowStatusBar(false)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(highlight).
		PaddingRight(2)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search instances..."
	searchInput.Prompt = "/ "
	searchInput.CharLimit = 100
	searchInput.Width = 30

	return model{
		contexts:         ctxList,
		vms:              vmTable,
		actions:          actionList,
		configList:       configList,
		columnConfigList: colList,
		sortList:         sortList,
		descView:         vp,
		searchInput:      searchInput,
		activePane:       paneSplash,
		breadcrumbs:      "Select a context to view instances",
		statusMsg:        "Ready.",
		showSidebar:      true,
		sortColumn:       "Name",
		sortAsc:          true,
		tableCols:        tableCols,
	}
}

func (m model) Init() tea.Cmd {
	m.statusMsg = "Loading contexts..."
	return fetchContextsCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.isSearching {
			switch msg.String() {
			case "esc", "enter":
				m.isSearching = false
				m.searchInput.Blur()
				m.statusMsg = "Search applied."
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				cmds = append(cmds, cmd)
				
				// Filter logic
				query := strings.ToLower(m.searchInput.Value())
				var filtered []VM
				for _, vm := range m.vmData {
					// basic filtering across core text fields
					match := strings.Contains(strings.ToLower(vm.Name), query) ||
						strings.Contains(strings.ToLower(vm.ID), query) ||
						strings.Contains(strings.ToLower(vm.PrivateIP), query) ||
						strings.Contains(strings.ToLower(vm.PublicIP), query) ||
						strings.Contains(strings.ToLower(vm.Labels), query)
					if match {
						filtered = append(filtered, vm)
					}
				}
				m.vms.SetRows(mapVMsToRows(filtered, m.tableCols))
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.activePane == paneContexts || m.activePane == paneVMs {
				return m, tea.Quit
			}
		
		case "b":
			m.showSidebar = !m.showSidebar
			if !m.showSidebar && m.activePane == paneContexts {
				m.activePane = paneVMs
				m.vms.Focus()
			}
			m = refreshVMTable(m)

		case "c":
			if m.activePane == paneContexts {
				m.activePane = paneConfig
				m.statusMsg = "Loading GCP projects for configuration..."
				cmds = append(cmds, fetchAllGCPProjectsCmd())
			}

		case "C":
			m.activePane = paneColumnConfig
			m.statusMsg = "Configure VM Columns"

		case "S":
			if m.activePane == paneVMs || m.activePane == paneContexts {
				m.activePane = paneSortConfig
				m.statusMsg = "Select a column to sort by"
			}

		case "/":
			if m.activePane == paneVMs || m.activePane == paneContexts {
				m.isSearching = true
				m.searchInput.Focus()
				m.statusMsg = "Typing search query (Enter or Esc to apply)"
			}

		case "r":
			if m.activePane == paneVMs || m.activePane == paneContexts {
				m.loading = true
				m.statusMsg = fmt.Sprintf("Force refreshing instances for %s...", m.activeCtx.accountName)
				cmds = append(cmds, fetchVMsCmd(m.activeCtx, true))
			}

		case " ":
			if m.activePane == paneConfig {
				selected := m.configList.SelectedItem()
				if selected != nil {
					idx := m.configList.Index()
					item := selected.(gcpProjectItem)
					item.selected = !item.selected
					m.configList.SetItem(idx, item)
				}
			} else if m.activePane == paneColumnConfig {
				selected := m.columnConfigList.SelectedItem()
				if selected != nil {
					idx := m.columnConfigList.Index()
					item := selected.(columnItem)
					item.selected = !item.selected
					m.columnConfigList.SetItem(idx, item)
				}
			}

		case "esc":
			if m.activePane == paneConfig {
				m.activePane = paneContexts
				m.statusMsg = "Canceled configuration."
			} else if m.activePane == paneColumnConfig {
				m.activePane = paneContexts
				m.statusMsg = "Canceled column configuration."
			} else if m.activePane == paneSortConfig {
				m.activePane = paneVMs
				m.statusMsg = "Canceled sort configuration."
			} else if m.activePane == paneDescribe {
				m.activePane = paneVMs
				m.statusMsg = "Closed details."
			} else if m.activePane == paneActions {
				m.activePane = paneVMs
				m.statusMsg = "Canceled action."
			} else if m.activePane == paneVMs {
				m.activePane = paneContexts
				m.vms.Blur()
			}

		case "tab":
			if m.activePane == paneContexts {
				m.activePane = paneVMs
				m.vms.Focus()
			} else if m.activePane == paneVMs {
				m.activePane = paneContexts
				m.vms.Blur()
			}

		case "enter":
			if m.activePane == paneContexts {
				// Select context and load VMs
				selected := m.contexts.SelectedItem()
				if selected != nil {
					node := selected.(*treeNode)
					if node.isLeaf {
						ctx := node.context
						m.activeCtx = ctx
						m.breadcrumbs = fmt.Sprintf("%s \u203A %s \u203A %s", ctx.provider, ctx.accountName, ctx.region)
						m.activePane = paneVMs
						m.vms.Focus()
						m.loading = true
						m.statusMsg = fmt.Sprintf("Fetching instances for %s...", ctx.accountName)
						cmds = append(cmds, fetchVMsCmd(ctx, false))
					} else {
						node.expanded = !node.expanded
						m.contexts.SetItems(buildFlatList(m.rootNodes))
					}
				}
			} else if m.activePane == paneVMs {
				// Select VM and show actions
				if m.vms.SelectedRow() != nil {
					m.activePane = paneActions
				}
			} else if m.activePane == paneConfig {
				var selectedProjects []string
				for _, item := range m.configList.Items() {
					p := item.(gcpProjectItem)
					if p.selected {
						selectedProjects = append(selectedProjects, p.projectId)
					}
				}
				cfg := loadAppConfig()
				cfg.GCPConfigured = true
				cfg.GCPProjects = selectedProjects
				saveAppConfig(cfg)

				m.activePane = paneContexts
				m.statusMsg = "GCP configuration saved. Reloading contexts..."
				cmds = append(cmds, fetchContextsCmd())
			} else if m.activePane == paneColumnConfig {
				var selectedColumns []string
				for _, item := range m.columnConfigList.Items() {
					c := item.(columnItem)
					if c.selected {
						selectedColumns = append(selectedColumns, c.name)
					}
				}
				if len(selectedColumns) == 0 {
					selectedColumns = defaultVMColumns
				}
				
				cfg := loadAppConfig()
				cfg.VMColumns = selectedColumns
				saveAppConfig(cfg)

				m = refreshVMTable(m)
				
				m.activePane = paneVMs
				m.statusMsg = "Columns configuration saved."
			} else if m.activePane == paneSortConfig {
				selected := m.sortList.SelectedItem()
				if selected != nil {
					sItem := selected.(sortItem)
					if m.sortColumn == sItem.name {
						m.sortAsc = !m.sortAsc
					} else {
						m.sortColumn = sItem.name
						m.sortAsc = true
					}
					
					sortVMs(m.vmData, m.sortColumn, m.sortAsc)
					m.vms.SetRows(mapVMsToRows(m.vmData, m.tableCols))
					
					m.activePane = paneVMs
					dir := "asc"
					if !m.sortAsc {
						dir = "desc"
					}
					m.statusMsg = fmt.Sprintf("Sorted by %s (%s)", m.sortColumn, dir)
				}
			} else if m.activePane == paneActions {
				// Execute Action
				cursor := m.vms.Cursor()
				if cursor >= 0 && cursor < len(m.vmData) {
					vm := m.vmData[cursor]
					action := m.actions.SelectedItem().(actionItem)

					if action.title == "SSH" {
						cmd := createSSHCmd(vm, m.activeCtx)
						if cmd != nil {
							m.activePane = paneVMs
							m.statusMsg = fmt.Sprintf("Starting SSH session with %s...", vm.Name)
							cmds = append(cmds, tea.ExecProcess(cmd, func(err error) tea.Msg {
								return sshCompleteMsg{err: err}
							}))
						} else {
							m.statusMsg = fmt.Sprintf("SSH not supported for %s", m.activeCtx.provider)
						}
					} else {
						m.statusMsg = fmt.Sprintf("Executing %s on %s...", action.title, vm.Name)
						m.activePane = paneVMs
						cmds = append(cmds, executeActionCmd(action.title, vm, m.activeCtx))
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate pane sizes
		sidebarWidth := m.width / 3

		h := m.height - 4 // footer & margins

		m.contexts.SetSize(sidebarWidth-2, h-2)
		m.configList.SetSize(m.width-4, m.height-4)
		m.columnConfigList.SetSize(m.width-4, m.height-4)
		m.sortList.SetSize(m.width-4, m.height-4)
		m.vms.SetHeight(h - 5) // leave room for breadcrumbs
		m.actions.SetSize(40, 15)
		
		m.descView.Width = m.width - 4
		m.descView.Height = m.height - 4

	case vmFetchMsg:
		m.loading = false
		m.vmData = msg.vms
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error fetching VMs: %v", msg.err)
			m.vms.SetRows([]table.Row{}) // Clear table on error
		} else {
			m.vms.SetRows(mapVMsToRows(m.vmData, m.tableCols))
			m.statusMsg = fmt.Sprintf("Loaded %d instances.", len(msg.vms))
		}

	case contextLoadMsg:
		m.rootNodes = msg.tree
		items := buildFlatList(m.rootNodes)
		m.contexts.SetItems(items)
		m.statusMsg = "Loaded context tree."
		if m.activePane == paneSplash {
			m.activePane = paneContexts
		}

	case gcpProjectFetchMsg:
		m.configList.SetItems(msg.items)
		m.statusMsg = "Select GCP projects to show (Space to toggle, Enter to save)."

	case sshCompleteMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("SSH session ended with error: %v", msg.err)
		} else {
			m.statusMsg = "SSH session closed."
		}

	case commandCompleteMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMsg = msg.output
		}

	case describeCompleteMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error fetching details: %v", msg.err)
		} else {
			m.descView.SetContent(msg.output)
			m.activePane = paneDescribe
			m.statusMsg = "Viewing instance details (Esc to close, Up/Down to scroll)."
		}
	}

	// Route update to active pane
	switch m.activePane {
	case paneContexts:
		m.contexts, cmd = m.contexts.Update(msg)
		cmds = append(cmds, cmd)
	case paneVMs:
		m.vms, cmd = m.vms.Update(msg)
		cmds = append(cmds, cmd)
	case paneActions:
		m.actions, cmd = m.actions.Update(msg)
		cmds = append(cmds, cmd)
	case paneConfig:
		m.configList, cmd = m.configList.Update(msg)
		cmds = append(cmds, cmd)
	case paneColumnConfig:
		m.columnConfigList, cmd = m.columnConfigList.Update(msg)
		cmds = append(cmds, cmd)
	case paneSortConfig:
		m.sortList, cmd = m.sortList.Update(msg)
		cmds = append(cmds, cmd)
	case paneDescribe:
		m.descView, cmd = m.descView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.activePane == paneSplash {
		splash := `
   ____ _                 _ __  __                                   
  / ___| | ___  _   _  __| |  \/  | __ _ _ __   __ _  __ _  ___ _ __ 
 | |   | |/ _ \| | | |/ _` + "`" + ` | |\/| |/ _` + "`" + ` | '_ \ / _` + "`" + ` |/ _` + "`" + ` |/ _ \ '__|
 | |___| | (_) | |_| | (_| | |  | | (_| | | | | (_| | (_| |  __/ |   
  \____|_|\___/ \__,_|\__,_|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|   
                                                     |___/           
`
		splashStyle := lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			Align(lipgloss.Center)
			
		statusStyle := lipgloss.NewStyle().
			Foreground(subtle).
			Align(lipgloss.Center).
			MarginTop(2)

		content := lipgloss.JoinVertical(lipgloss.Center,
			splashStyle.Render(splash),
			statusStyle.Render(m.statusMsg),
		)
		
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	if m.activePane == paneConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.configList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space: Toggle \u2022 Enter: Save \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneColumnConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.columnConfigList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Space: Toggle \u2022 Enter: Save \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneSortConfig {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.sortList.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Enter: Sort (Toggles Asc/Desc) \u2022 Esc: Cancel | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	if m.activePane == paneDescribe {
		vStyle := activeStyle.Copy().Width(m.width - 2).Height(m.height - 3)
		mainView := vStyle.Render(m.descView.View())
		footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Scroll \u2022 Esc: Close | Status: %s", m.statusMsg))
		return lipgloss.JoinVertical(lipgloss.Left, mainView, footer)
	}

	sidebarWidth := m.width / 3
	if !m.showSidebar {
		sidebarWidth = 0
	}

	// Contexts Pane
	sidebarView := ""
	if m.showSidebar {
		m.contexts.SetSize(sidebarWidth-2, m.height-5)
		cStyle := baseStyle.Copy().Width(sidebarWidth).Height(m.height - 3).MaxWidth(sidebarWidth)
		if m.activePane == paneContexts {
			cStyle = activeStyle.Copy().Width(sidebarWidth).Height(m.height - 3).MaxWidth(sidebarWidth)
		}
		sidebarView = cStyle.Render(m.contexts.View())
	}

	// VMs Pane
	mainWidth := m.width - sidebarWidth - 2
	if !m.showSidebar {
		mainWidth = m.width - 2
	}
	
	vStyle := baseStyle.Copy().Width(mainWidth).Height(m.height - 3).MaxWidth(mainWidth)
	if m.activePane == paneVMs || m.activePane == paneActions {
		vStyle = activeStyle.Copy().Width(mainWidth).Height(m.height - 3).MaxWidth(mainWidth)
	}

	header := breadcrumbStyle.Render(m.breadcrumbs)
	
	tableContent := m.vms.View()
	if m.loading {
		tableContent = lipgloss.NewStyle().Padding(2).Foreground(subtle).Render("Loading instances from CLI...")
	} else if m.isSearching || m.searchInput.Value() != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(m.searchInput.View()),
			tableContent,
		)
	}

	mainContentView := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"\n",
		tableContent,
	)

	// Check if we need to render the action overlay
	if m.activePane == paneActions {
		overlay := overlayStyle.Render(m.actions.View())
		mainContentView = lipgloss.Place(
			vStyle.GetWidth(), vStyle.GetHeight(),
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	mainView := vStyle.Render(mainContentView)

	// Combine horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainView)

	// Footer
	footer := statusLineStyle.Render(fmt.Sprintf("\u2191\u2193: Navigate \u2022 Enter: Select \u2022 Tab: Switch Pane \u2022 Esc/q: Back/Quit | Status: %s", m.statusMsg))

	return lipgloss.JoinVertical(lipgloss.Left, panes, footer)
}

// Background Task Stubs
func fetchContextsCmd() tea.Cmd {
	return func() tea.Msg {
		var allCtx []contextItem
		
		awsCtx := loadAWSContexts()
		allCtx = append(allCtx, awsCtx...)
		
		gcpCtx := loadGCPContexts()
		allCtx = append(allCtx, gcpCtx...)
		
		azCtx := loadAzureContexts()
		allCtx = append(allCtx, azCtx...)

		if len(allCtx) == 0 {
			allCtx = []contextItem{
				{"AWS", "123456789012", "production", "us-east-1"},
				{"AWS", "123456789012", "production", "us-west-2"},
				{"AWS", "987654321098", "staging", "eu-central-1"},
				{"GCP", "my-gcp-project-1", "backend-services", "us-central1"},
				{"GCP", "my-gcp-project-2", "data-pipeline", "europe-west1"},
				{"Azure", "sub-abc-123", "core-infra", "eastus"},
			}
		}

		tree := buildContextTree(allCtx)
		return contextLoadMsg{tree: tree}
	}
}

func fetchAllGCPProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("gcloud", "projects", "list", "--format=json(projectId)")
		output, err := cmd.Output()
		if err != nil {
			return gcpProjectFetchMsg{items: nil}
		}

		var projects []struct {
			ProjectId string `json:"projectId"`
		}
		if err := json.Unmarshal(output, &projects); err != nil {
			return gcpProjectFetchMsg{items: nil}
		}

		cfg := loadAppConfig()
		
		var items []list.Item
		for _, p := range projects {
			selected := false
			if !cfg.GCPConfigured {
				selected = true
			} else {
				selected = isGCPProjectSelected(p.ProjectId, cfg)
			}
			items = append(items, gcpProjectItem{
				projectId: p.ProjectId,
				selected:  selected,
			})
		}
		return gcpProjectFetchMsg{items: items}
	}
}

func fetchVMsCmd(ctx contextItem, force bool) tea.Cmd {
	return func() tea.Msg {
		cacheKey := fmt.Sprintf("%s-%s-%s", ctx.provider, ctx.accountID, ctx.region)
		if !force {
			if entry, ok := vmCache[cacheKey]; ok {
				if time.Since(entry.timestamp) < cacheTTL {
					return vmFetchMsg{vms: entry.vms, err: nil}
				}
			}
		}

		var rows []VM
		var err error

		switch ctx.provider {
		case "AWS":
			rows, err = fetchAWSVMs(ctx.accountName, ctx.region)
		case "GCP":
			rows, err = fetchGCPVMs(ctx.accountID)
		case "Azure":
			rows, err = fetchAzureVMs(ctx.accountID)
		}

		if err != nil {
			return vmFetchMsg{vms: nil, err: err}
		}

		vmCache[cacheKey] = cacheEntry{
			vms:       rows,
			timestamp: time.Now(),
		}

		return vmFetchMsg{vms: rows, err: nil}
	}
}

func executeCommandMock(action string, instanceID string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second) // simulate API call
		return commandCompleteMsg{
			output: fmt.Sprintf("Successfully executed '%s' on %s", action, instanceID),
			err:    nil,
		}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}