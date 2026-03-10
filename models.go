package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

func (i sortItem) Title() string       { return i.name }
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

	sortColumn  string
	sortAsc     bool
	tableCols   []table.Column
	isSearching bool
	cfg         AppConfig
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
		cfg:              cfg,
	}
}
