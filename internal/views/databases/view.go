package databases

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/iac"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
)

type dbsFetchMsg struct {
	requestKey string
	databases  []core.Database
	err        error
}

type DatabasesView struct {
	table          table.Model
	activeCtx      core.CloudContext
	dbData         []core.Database
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

func New(cfg *config.AppConfig) *DatabasesView {
	return &DatabasesView{
		cfg: cfg,
	}
}

func (v *DatabasesView) Title() string       { return "Databases" }
func (v *DatabasesView) ShortHelp() string   { return "↑↓: Navigate • r: Refresh" }
func (v *DatabasesView) IsInputActive() bool { return false }

func (v *DatabasesView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *DatabasesView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.requestKey = ctx.CacheKey()
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching databases for %s...", ctx.DisplayName())
	v.refreshTable()
	return v.fetchCmd()
}

func (v *DatabasesView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.refreshTable()
}

func (v *DatabasesView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			v.loading = true
			v.statusMsg = "Refreshing databases..."
			return v, v.fetchCmd()
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
		} else {
			v.statusMsg = fmt.Sprintf("Loaded %d databases.", len(v.dbData))
			indexCtx := v.activeCtx
			indexDatabases := v.dbData
			cmd = func() tea.Msg {
				return ui.DatabaseIndexUpdateMsg{Ctx: indexCtx, Databases: indexDatabases}
			}
		}
		v.refreshTable()
	}
	return v, cmd
}

func (v *DatabasesView) Render() string {
	content := v.table.View()
	if v.loading {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading databases...")
	}
	header := ui.BreadcrumbStyle.Render(fmt.Sprintf("%s \u203A %s \u203A Databases", v.activeCtx.Provider, v.activeCtx.DisplayName()))
	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", content), v.width, v.height)
}

func (v *DatabasesView) refreshTable() {
	if v.width == 0 {
		return
	}
	var cols []table.Column

	columnNames := v.cfg.DatabaseColumns
	if len(columnNames) == 0 {
		columnNames = core.DefaultDatabaseColumns
	}

	for _, c := range columnNames {
		cols = append(cols, table.Column{Title: c, Width: 15})
	}
	tbl, _, _, _, _ := ui.NewResourceTable(cols, v.width, 0, "Name")

	var rows []table.Row
	for _, db := range v.dbData {
		if !databaseMatchesQuery(db, v.searchQuery) {
			continue
		}
		var row []string
		for _, col := range cols {
			row = append(row, ui.TruncateText(db.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	tbl.SetRows(rows)
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	v.table = tbl
	v.tableCols = cols
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
		provider := providers.GetProvider(cfg)
		dbProvider, ok := provider.(providers.DatabaseProvider)
		if !ok {
			return dbsFetchMsg{requestKey: requestKey, err: fmt.Errorf("databases not supported by provider backend")}
		}
		dbs, err := dbProvider.FetchDatabases(context.Background(), activeCtx)
		return dbsFetchMsg{requestKey: requestKey, databases: dbs, err: err}
	}
}
