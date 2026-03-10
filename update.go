package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

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

		case m.cfg.Keybindings["search"]:
			if m.activePane == paneVMs || m.activePane == paneContexts {
				m.isSearching = true
				m.searchInput.Focus()
				m.statusMsg = "Typing search query (Enter or Esc to apply)"
			}

		case m.cfg.Keybindings["refresh"]:
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
