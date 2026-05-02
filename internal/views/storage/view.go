package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
)

type storageFetchMsg struct {
	requestKey string
	buckets    []core.StorageBucket
	err        error
}

type StorageView struct {
	table       table.Model
	activeCtx   core.CloudContext
	bucketData  []core.StorageBucket
	tableCols   []table.Column
	cfg         *config.AppConfig
	width       int
	height      int
	loading     bool
	statusMsg   string
	searchQuery string
	requestKey  string
}

func New(cfg *config.AppConfig) *StorageView {
	return &StorageView{cfg: cfg}
}

func (v *StorageView) Title() string       { return "Storage" }
func (v *StorageView) ShortHelp() string   { return "↑↓: Navigate • / from Find • r: Refresh" }
func (v *StorageView) IsInputActive() bool { return false }

func (v *StorageView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *StorageView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.requestKey = ctx.CacheKey()
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching storage for %s...", ctx.DisplayName())
	v.refreshTable()
	return v.fetchCmd()
}

func (v *StorageView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.refreshTable()
}

func (v *StorageView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "r" {
			v.loading = true
			v.statusMsg = "Refreshing storage..."
			return v, v.fetchCmd()
		}
		v.table, cmd = v.table.Update(msg)
		return v, cmd

	case storageFetchMsg:
		if msg.requestKey != v.requestKey {
			return v, nil
		}
		v.loading = false
		v.bucketData = msg.buckets
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			v.statusMsg = fmt.Sprintf("Loaded %d storage resources.", len(v.bucketData))
			indexCtx := v.activeCtx
			indexBuckets := v.bucketData
			cmd = func() tea.Msg {
				return ui.StorageIndexUpdateMsg{Ctx: indexCtx, Buckets: indexBuckets}
			}
		}
		v.refreshTable()
	}
	return v, cmd
}

func (v *StorageView) Render() string {
	content := v.table.View()
	if v.loading {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading storage...")
	}
	header := ui.BreadcrumbStyle.Render(fmt.Sprintf("%s › %s › Storage", v.activeCtx.Provider, v.activeCtx.DisplayName()))
	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", content), v.width, v.height)
}

func (v *StorageView) refreshTable() {
	if v.width == 0 {
		return
	}
	columnNames := v.cfg.StorageColumns
	if len(columnNames) == 0 {
		columnNames = core.DefaultStorageColumns
	}
	cols := make([]table.Column, 0, len(columnNames))
	for _, c := range columnNames {
		cols = append(cols, table.Column{Title: c, Width: 16})
	}
	tbl, _, _, _, _ := ui.NewResourceTable(cols, v.width, 0, "Name")
	rows := make([]table.Row, 0, len(v.bucketData))
	for _, bucket := range v.bucketData {
		if !storageMatchesQuery(bucket, v.searchQuery) {
			continue
		}
		row := make([]string, 0, len(cols))
		for _, col := range cols {
			row = append(row, ui.TruncateText(bucket.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	tbl.SetRows(rows)
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	v.table = tbl
	v.tableCols = cols
}

func storageMatchesQuery(bucket core.StorageBucket, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	blob := strings.Join([]string{
		bucket.Name, bucket.ID, bucket.ProviderType, bucket.Region, bucket.StorageClass,
		bucket.Access, bucket.Encrypted, bucket.Versioning, bucket.CreatedAt, bucket.ResourceGroup, bucket.Labels,
	}, " ")
	return strings.Contains(strings.ToLower(blob), query)
}

func (v *StorageView) fetchCmd() tea.Cmd {
	activeCtx := v.activeCtx
	requestKey := v.requestKey
	cfg := *v.cfg
	return func() tea.Msg {
		provider := providers.GetProvider(cfg)
		storageProvider, ok := provider.(providers.StorageProvider)
		if !ok {
			return storageFetchMsg{requestKey: requestKey, err: fmt.Errorf("storage not supported by provider backend")}
		}
		buckets, err := storageProvider.FetchStorageBuckets(context.Background(), activeCtx)
		return storageFetchMsg{requestKey: requestKey, buckets: buckets, err: err}
	}
}
