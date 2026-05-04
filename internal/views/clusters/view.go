package clusters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/iac"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/ui"
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

type k9sReadyMsg struct {
	cmd *exec.Cmd
	err error
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
	searchQuery    string
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
		cfg:     cfg,
	}
}

func (v *ClustersView) Title() string { return "Clusters" }
func (v *ClustersView) ShortHelp() string {
	return "↑↓: Navigate • Enter: Actions • r: Refresh"
}
func (v *ClustersView) IsInputActive() bool { return v.activePane == 1 }

func (v *ClustersView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

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
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
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
					v.activePane = 0
					v.statusMsg = fmt.Sprintf("Preparing k9s for %s...", v.selectedItem.Name)
					return v, v.prepareK9sCmd(v.selectedItem)
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
		v.clustersData = iac.ApplyTerraformToClusters(v.cfg.TerraformStatePaths, v.activeCtx, msg.clusters)
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = fmt.Sprintf("Loaded %d clusters.", len(v.clustersData))
			indexCtx := v.activeCtx
			indexClusters := v.clustersData
			cmd = func() tea.Msg {
				return ui.ClusterIndexUpdateMsg{Ctx: indexCtx, Clusters: indexClusters}
			}
		}
		v.refreshTable()

	case k9sReadyMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("k9s not supported: %v", msg.err)
			return v, nil
		}
		if msg.cmd == nil {
			v.statusMsg = "k9s is unavailable for this cluster"
			return v, nil
		}
		v.statusMsg = fmt.Sprintf("Launching k9s for %s...", v.selectedItem.Name)
		return v, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			return clustersFetchMsg{requestKey: v.requestKey, clusters: v.clustersData, err: err}
		})
	}
	return v, cmd
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
		if !clusterMatchesQuery(cluster, v.searchQuery) {
			continue
		}
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

func clusterMatchesQuery(cluster core.Cluster, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join([]string{
		cluster.Name, cluster.ID, cluster.Location, cluster.Version, cluster.Status, cluster.NodeCount, cluster.Labels,
	}, " ")), query)
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

func (v *ClustersView) prepareK9sCmd(cluster core.Cluster) tea.Cmd {
	activeCtx := v.activeCtx
	cfg := *v.cfg
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		clusterProvider, ok := provider.(providers.ClusterProvider)
		if !ok {
			return k9sReadyMsg{err: fmt.Errorf("clusters not supported by provider backend")}
		}
		cmd, err := clusterProvider.GetK9sCmd(context.Background(), cluster, activeCtx)
		return k9sReadyMsg{cmd: cmd, err: err}
	}
}
