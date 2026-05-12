package storage

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

	"github.com/srivathsan-srinivasan/cloudmanager/internal/clipboard"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/providers"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/ui"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/views/tagging"
)

const (
	paneTable = iota
	paneActions
	paneDescribe
	paneTag
)

type storageFetchMsg struct {
	requestKey string
	buckets    []core.StorageBucket
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

type StorageView struct {
	table        table.Model
	actions      list.Model
	descView     viewport.Model
	activeCtx    core.CloudContext
	bucketData   []core.StorageBucket
	visibleRows  []core.StorageBucket
	tableCols    []table.Column
	tagInput     textinput.Model
	cfg          *config.AppConfig
	width        int
	height       int
	loading      bool
	statusMsg    string
	searchQuery  string
	requestKey   string
	activePane   int
	pending      core.StorageBucket
	copyableText string
	detailURL    string
}

func New(cfg *config.AppConfig) *StorageView {
	items := make([]list.Item, 0, len(core.StorageActions()))
	for _, action := range core.StorageActions() {
		items = append(items, actionItem{title: action.Title, desc: action.Description})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	actions := list.New(items, delegate, 42, 10)
	actions.Title = "Storage Actions"
	actions.SetShowStatusBar(false)
	actions.SetFilteringEnabled(false)

	descView := viewport.New(80, 20)
	descView.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.Highlight).PaddingRight(2)

	tagInput := textinput.New()
	tagInput.Placeholder = "comma separated tags..."
	tagInput.Prompt = "tags> "
	tagInput.CharLimit = 160
	tagInput.Width = 42
	return &StorageView{cfg: cfg, actions: actions, descView: descView, tagInput: tagInput}
}

func (v *StorageView) Title() string { return "Storage" }
func (v *StorageView) ShortHelp() string {
	return "↑↓: Navigate • Enter: Actions • t: Tag • / from Find • r: Refresh"
}
func (v *StorageView) IsInputActive() bool {
	return v.activePane == paneActions || v.activePane == paneDescribe || v.activePane == paneTag
}

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
	v.copyableText = ""
	v.detailURL = ""
	v.loading = true
	v.statusMsg = fmt.Sprintf("Fetching storage for %s...", ctx.DisplayName())
	v.refreshTable()
	return v.fetchCmd()
}

func (v *StorageView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.actions.SetSize(50, ui.ActionListHeight(len(v.actions.Items()), height))
	v.descView.Width = width - 4
	v.descView.Height = height - 4
	v.refreshTable()
}

func (v *StorageView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			switch v.activePane {
			case paneActions, paneDescribe, paneTag:
				v.activePane = paneTable
				v.tagInput.Blur()
				return v, nil
			}
		}
		if v.activePane == paneTag {
			return v.handleTagKeys(msg)
		}
		if v.activePane == paneActions {
			return v.handleActionKeys(msg)
		}
		if v.activePane == paneDescribe {
			if msg.String() == "c" || msg.String() == "C" {
				return v.copyText(v.copyableText)
			}
			if msg.String() == "o" || msg.String() == "O" {
				return v.openConsole(v.detailURL)
			}
			v.descView, cmd = v.descView.Update(msg)
			return v, cmd
		}
		if msg.String() == "r" {
			v.loading = true
			v.statusMsg = "Refreshing storage..."
			return v, v.fetchCmd()
		}
		if msg.String() == "enter" {
			bucket, ok := v.selectedBucket()
			if !ok {
				return v, nil
			}
			v.actions.Title = fmt.Sprintf("Actions: %s", bucket.Name)
			v.activePane = paneActions
			return v, nil
		}
		if msg.String() == "t" {
			return v.openTag()
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
	case clipboardCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Copy failed: %v", msg.err)
		} else {
			v.statusMsg = "Copied to clipboard."
		}
	case ui.BrowserOpenMsg:
		if msg.Err != nil {
			v.statusMsg = fmt.Sprintf("Open failed: %v", msg.Err)
		} else {
			v.statusMsg = "Opened provider console."
		}
	}
	return v, cmd
}

func (v *StorageView) Render() string {
	content := v.table.View()
	if v.loading {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("Loading storage...")
	}
	header := ui.BreadcrumbStyle.Render(fmt.Sprintf("%s › %s › Storage", v.activeCtx.Provider, v.activeCtx.DisplayName()))
	if v.activePane == paneDescribe {
		return ui.ClampToWindow(v.descView.View(), v.width, v.height)
	}
	if v.activePane == paneActions {
		overlay := ui.OverlayStyle.Render(v.actions.View())
		bodyWithOverlay := lipgloss.Place(v.width, v.height-6, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
		return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyWithOverlay), v.width, v.height)
	}
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
	cursor := v.table.Cursor()
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
	if len(rows) > 0 {
		if cursor >= len(rows) {
			cursor = len(rows) - 1
		}
		if cursor < 0 {
			cursor = 0
		}
		tbl.SetCursor(cursor)
	}
	tbl.Focus()
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

func (v *StorageView) handleActionKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	if msg.String() != "enter" {
		var cmd tea.Cmd
		v.actions, cmd = v.actions.Update(msg)
		return v, cmd
	}
	bucket, ok := v.selectedBucket()
	if !ok || v.actions.SelectedItem() == nil {
		return v, nil
	}
	action := v.actions.SelectedItem().(actionItem).title
	switch action {
	case "Describe":
		return v.showDetails(core.DescribeStorageBucket(v.activeCtx, bucket), core.StorageConsoleURL(v.activeCtx, bucket)), nil
	case "Copy URI":
		return v.copyText(core.StorageURI(v.activeCtx, bucket))
	case "Copy ID":
		return v.copyText(bucket.GetID())
	case "Copy Console URL":
		consoleURL := core.StorageConsoleURL(v.activeCtx, bucket)
		if strings.TrimSpace(consoleURL) == "" {
			v.statusMsg = "No console URL available for this storage resource."
			return v, nil
		}
		return v.copyText(consoleURL)
	case "Open Console":
		return v.openConsole(core.StorageConsoleURL(v.activeCtx, bucket))
	case "Tag":
		return v.openTag()
	}
	return v, nil
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

func (v *StorageView) openTag() (ui.View, tea.Cmd) {
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

func (v *StorageView) showDetails(content, consoleURL string) ui.View {
	v.detailURL = strings.TrimSpace(consoleURL)
	v.copyableText = content
	v.descView.SetContent(v.copyableText)
	v.descView.GotoTop()
	v.activePane = paneDescribe
	v.statusMsg = "Viewing details (c copy, o open, Esc close)."
	return v
}

func (v *StorageView) copyText(text string) (ui.View, tea.Cmd) {
	text = strings.TrimSpace(text)
	if text == "" {
		v.statusMsg = "Nothing to copy."
		return v, nil
	}
	v.statusMsg = "Copying to clipboard..."
	return v, func() tea.Msg {
		return clipboardCompleteMsg{err: clipboard.Write(text)}
	}
}

func (v *StorageView) openConsole(consoleURL string) (ui.View, tea.Cmd) {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		v.statusMsg = "No provider console URL available."
		return v, nil
	}
	v.statusMsg = "Opening provider console..."
	return v, ui.OpenURLCmd(consoleURL)
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
