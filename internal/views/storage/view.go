package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/ui"
	"cloudmanager/internal/views/tagging"
)

const (
	paneTable = iota
	paneTag
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
	visibleRows []core.StorageBucket
	tableCols   []table.Column
	tagInput    textinput.Model
	cfg         *config.AppConfig
	width       int
	height      int
	loading     bool
	statusMsg   string
	searchQuery string
	requestKey  string
	activePane  int
	pending     core.StorageBucket
}

func New(cfg *config.AppConfig) *StorageView {
	tagInput := textinput.New()
	tagInput.Placeholder = "comma separated tags..."
	tagInput.Prompt = "tags> "
	tagInput.CharLimit = 160
	tagInput.Width = 42
	return &StorageView{cfg: cfg, tagInput: tagInput}
}

func (v *StorageView) Title() string { return "Storage" }
func (v *StorageView) ShortHelp() string {
	return "↑↓: Navigate • t: Tag • / from Find • r: Refresh"
}
func (v *StorageView) IsInputActive() bool { return v.activePane == paneTag }

func (v *StorageView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *StorageView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.requestKey = ctx.CacheKey()
	v.activePane = paneTable
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
		if v.activePane == paneTag {
			return v.handleTagKeys(msg)
		}
		if msg.String() == "r" {
			v.loading = true
			v.statusMsg = "Refreshing storage..."
			return v, v.fetchCmd()
		}
		if msg.String() == "t" {
			bucket, ok := v.selectedBucket()
			if !ok {
				return v, nil
			}
			v.pending = bucket
			v.tagInput.SetValue("")
			v.tagInput.Focus()
			v.activePane = paneTag
			v.statusMsg = fmt.Sprintf("Tag %s with CloudManager-only tags.", bucket.Name)
			return v, textinput.Blink
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
			v.bucketData = config.ApplyResourceTagsToStorageBuckets(*v.cfg, v.activeCtx, v.bucketData)
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
	if v.activePane == paneTag {
		tagStyle := ui.OverlayStyle.Copy().Padding(1, 2).Width(60)
		tagView := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("CloudManager Tags"),
			"",
			lipgloss.NewStyle().Render(fmt.Sprintf("Add local tags to %s:", v.pending.Name)),
			"",
			v.tagInput.View(),
			"",
			lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter: Save • Esc: Cancel"),
		)
		overlay := tagStyle.Render(tagView)
		bodyWithOverlay := lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyWithOverlay), v.width, v.height)
	}
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
		cols = append(cols, table.Column{Title: c, Width: storageColumnWidth(c)})
	}
	tbl, visibleCols, _, _, _ := ui.NewResourceTable(cols, v.width, 0, "Name")
	rows := make([]table.Row, 0, len(v.bucketData))
	v.visibleRows = v.visibleRows[:0]
	for _, bucket := range v.bucketData {
		if !storageMatchesQuery(bucket, v.searchQuery) {
			continue
		}
		v.visibleRows = append(v.visibleRows, bucket)
		row := make([]string, 0, len(visibleCols))
		for _, col := range visibleCols {
			row = append(row, ui.TruncateText(bucket.GetField(col.Title), col.Width))
		}
		rows = append(rows, table.Row(row))
	}
	tbl.SetRows(rows)
	tbl.SetHeight(ui.TableHeight(v.height))
	tbl.SetWidth(ui.TableViewportWidth(v.width))
	v.table = tbl
	v.tableCols = visibleCols
}

func (v *StorageView) selectedBucket() (core.StorageBucket, bool) {
	cursor := v.table.Cursor()
	if cursor < 0 || cursor >= len(v.visibleRows) {
		return core.StorageBucket{}, false
	}
	return v.visibleRows[cursor], true
}

func (v *StorageView) handleTagKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.tagInput.Blur()
		v.activePane = paneTable
		v.statusMsg = "Tag canceled."
		return v, nil
	case "enter":
		tags := tagging.SplitInput(v.tagInput.Value())
		if err := tagging.Save(v.cfg, v.activeCtx, v.pending, tags); err != nil {
			v.statusMsg = fmt.Sprintf("Tag save failed: %v", err)
			return v, nil
		}
		v.bucketData = config.ApplyResourceTagsToStorageBuckets(*v.cfg, v.activeCtx, v.bucketData)
		v.refreshTable()
		v.tagInput.Blur()
		v.activePane = paneTable
		v.statusMsg = fmt.Sprintf("Tagged %s with %s.", v.pending.Name, strings.Join(tags, ","))
		indexCtx := v.activeCtx
		indexBuckets := v.bucketData
		return v, func() tea.Msg {
			return ui.StorageIndexUpdateMsg{Ctx: indexCtx, Buckets: indexBuckets}
		}
	}
	var cmd tea.Cmd
	v.tagInput, cmd = v.tagInput.Update(msg)
	return v, cmd
}

func storageColumnWidth(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "name":
		return 28
	case "id":
		return 24
	case "provider type":
		return 22
	case "region":
		return 14
	case "storage class":
		return 16
	case "access":
		return 16
	case "encrypted":
		return 14
	case "versioning":
		return 12
	case "created at":
		return 22
	case "resource group":
		return 20
	case "labels":
		return 24
	default:
		return 16
	}
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
