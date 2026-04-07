package clusters

import (
	"context"
	"fmt"
		"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
)



type actionItem struct {
	TitleStr string
	DescStr  string
}
func (i actionItem) Title() string       { return i.TitleStr }
func (i actionItem) Description() string { return i.DescStr }
func (i actionItem) FilterValue() string { return i.TitleStr }

type clustersFetchMsg struct {
	requestKey string
	clusters   []core.Cluster
	err        error
}

type ClustersView struct {
	table          table.Model
	actions        list.Model
	activePane     int
	selectedItem   core.Cluster
	activeCtx      core.CloudContext
	clustersData   []core.Cluster
	tableCols      []table.Column
	cfg            *config.AppConfig
	width, height  int
	loading        bool
	statusMsg      string
	requestKey     string
	canScrollLeft  bool
	canScrollRight bool
	columnOffset   int
}

func New(cfg *config.AppConfig) *ClustersView {
		// Actions list
	actionItems := []list.Item{
		actionItem{TitleStr: "Jump to k9s", DescStr: "Open k9s terminal for this cluster"},
	}
	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = true
	actionList := list.New(actionItems, actionDelegate, 30, 10)
	actionList.Title = "Cluster Actions"
	actionList.SetShowStatusBar(false)
	actionList.SetFilteringEnabled(false)

	return &ClustersView{
		actions: actionList,
		cfg: cfg,
	}
}

func (v *ClustersView) Title() string { return "Clusters" }
func (v *ClustersView) ShortHelp() string { return "↑↓: Navigate • Enter: Actions • r: Refresh" }
func (v *ClustersView) IsInputActive() bool { return v.activePane == 1 }

func (v *ClustersView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activePane = 0
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.requestKey = ctx.CacheKey()
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching clusters for %s...", ctx.DisplayName())
	v.refreshTable()
	return v.fetchClustersCmd()
}

func (v *ClustersView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.refreshTable()
}

func (v *ClustersView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" && v.activePane == 1 {
			v.activePane = 0
			return v, nil
		}

		if v.activePane == 1 {
			switch msg.String() {
			case "enter":
				action := v.actions.SelectedItem().(actionItem)
				if action.TitleStr == "Jump to k9s" {
					provider := providers.GetProvider(*v.cfg)
					clusterProvider, ok := provider.(providers.ClusterProvider)
					if ok {
						k9sCmd, err := clusterProvider.GetK9sCmd(context.Background(), v.selectedItem, v.activeCtx)
						if err == nil && k9sCmd != nil {
							v.activePane = 0
							v.statusMsg = fmt.Sprintf("Launching k9s for %s...", v.selectedItem.Name)
							return v, tea.ExecProcess(k9sCmd, func(err error) tea.Msg {
								return clustersFetchMsg{requestKey: v.requestKey, clusters: v.clustersData, err: err}
							})
						}
						v.statusMsg = fmt.Sprintf("k9s not supported: %v", err)
					}
					v.activePane = 0
				}
			}
			v.actions, cmd = v.actions.Update(msg)
			return v, cmd
		}

		switch msg.String() {
		case "r":
			v.loading = true
			v.statusMsg = "Refreshing clusters..."
			return v, v.fetchClustersCmd()
		case "enter":
			if v.table.SelectedRow() != nil {
				cursor := v.table.Cursor()
				if cursor >= 0 && cursor < len(v.clustersData) {
					v.selectedItem = v.clustersData[cursor]
					v.activePane = 1
				}
			}
		}
		v.table, cmd = v.table.Update(msg)
		return v, cmd

	case clustersFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.clustersData = msg.clusters
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = fmt.Sprintf("Loaded %d clusters.", len(v.clustersData))
		}
		v.refreshTable()
	}
	return v, nil
}

func (v *ClustersView) Render() string {
	content := v.table.View()
	if v.loading {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading clusters...")
	}
	header := ui.BreadcrumbStyle.Render(fmt.Sprintf("%s \u203A %s \u203A Clusters", v.activeCtx.Provider, v.activeCtx.DisplayName()))
	
	if v.activePane == 1 {
		overlay := ui.OverlayStyle.Render(v.actions.View())
		return ui.ClampToWindow(lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" ")), v.width, v.height)
	}

	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", content), v.width, v.height)
}

func (v *ClustersView) refreshTable() {
	if v.width == 0 {
		return
	}
	var cols []table.Column
	
	columnNames := v.cfg.ClusterColumns
	if len(columnNames) == 0 {
		columnNames = core.DefaultClusterColumns
	}
	
	for _, c := range columnNames {
		cols = append(cols, table.Column{Title: c, Width: 15})
	}
	tbl, _, _, _, _ := ui.NewResourceTable(cols, v.width, 0, "Name")
	
	var rows []table.Row
	for _, cluster := range v.clustersData {
		var row []string
		for _, col := range cols {
			row = append(row, ui.TruncateText(cluster.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	tbl.SetRows(rows)
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	v.table = tbl
	v.tableCols = cols
}

func (v *ClustersView) fetchClustersCmd() tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	cfg := *v.cfg
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		clusterProvider, ok := provider.(providers.ClusterProvider)
		if !ok {
			return clustersFetchMsg{requestKey: requestKey, err: fmt.Errorf("clusters not supported by provider backend")}
		}
		clusters, err := clusterProvider.FetchClusters(context.Background(), activeCtx)
		return clustersFetchMsg{requestKey: requestKey, clusters: clusters, err: err}
	}
}
