package disks

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
)

func TestRenderWithManyDisksFitsViewWidth(t *testing.T) {
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"}}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.activeCtx = core.CloudContext{
		Provider:    "AWS",
		AccountID:   "1193",
		AccountName: "poc",
		Region:      "us-east-2",
	}
	view.requestKey = view.activeCtx.CacheKey()
	view.breadcrumbs = "AWS › 1193 (poc) › us-east-2"

	for i := 0; i < 100; i++ {
		view.diskData = append(view.diskData, core.Disk{
			Name:         "disk-with-a-long-name",
			ID:           "vol-0123456789abcdef0",
			State:        "in-use",
			SizeGB:       250,
			Type:         "gp3",
			AttachedToVM: "vm-with-a-long-name",
			Zone:         "us-east-2a",
			Encrypted:    true,
		})
	}

	view.refreshTable()

	rendered := strings.TrimRight(view.Render(), "\n")
	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		if lipgloss.Width(line) > view.width {
			t.Fatalf("rendered line exceeds view width: got %d want <= %d\n%s", lipgloss.Width(line), view.width, line)
		}
	}
	if len(lines) > view.height {
		t.Fatalf("rendered view exceeds height: got %d want <= %d", len(lines), view.height)
	}
}

func TestHorizontalPanChangesVisibleDiskColumns(t *testing.T) {
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"}}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.refreshTable()

	initialFirst := view.tableCols[0].Title
	if !view.canScrollRight {
		t.Fatal("expected horizontal scrolling to be available in a narrow view")
	}

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*DisksView)

	if next.columnOffset != 1 {
		t.Fatalf("expected column offset 1 after panning right, got %d", next.columnOffset)
	}
	if next.tableCols[0].Title == initialFirst {
		t.Fatalf("expected visible columns to change after panning, still starts with %q", next.tableCols[0].Title)
	}
}

func TestDiskTableArrowKeysMoveSelection(t *testing.T) {
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.diskData = []core.Disk{
		{Name: "alpha", ID: "disk-1"},
		{Name: "beta", ID: "disk-2"},
	}
	view.refreshTable()
	view.syncVisibleRows()
	view.disks.Focus()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(*DisksView)

	if got := next.disks.Cursor(); got != 1 {
		t.Fatalf("expected cursor to move to row 1 after down key, got %d", got)
	}
}

func TestSelectedDiskUsesVisibleRows(t *testing.T) {
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.diskData = []core.Disk{
		{Name: "alpha", ID: "disk-1"},
		{Name: "beta", ID: "disk-2"},
	}
	view.refreshTable()
	view.searchInput.SetValue("beta")
	view.syncVisibleRows()

	selected, ok := view.selectedDisk()
	if !ok {
		t.Fatal("expected a selected disk")
	}
	if selected.ID != "disk-2" {
		t.Fatalf("expected filtered selection to return disk-2, got %s", selected.ID)
	}
}

func TestSetSearchQueryFiltersDisks(t *testing.T) {
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.diskData = []core.Disk{
		{Name: "alpha", ID: "disk-1"},
		{Name: "beta", ID: "disk-2"},
	}
	view.refreshTable()

	view.SetSearchQuery("disk-2")

	if len(view.visibleDisks) != 1 || view.visibleDisks[0].ID != "disk-2" {
		t.Fatalf("expected search handoff to filter to disk-2, got %+v", view.visibleDisks)
	}
}

func TestTagSelectedDiskSavesCloudManagerTag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{DiskColumns: []string{"Name", "ID", "Labels"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	view.diskData = []core.Disk{{Name: "data", ID: "vol-1"}}
	view.refreshTable()
	view.syncVisibleRows()

	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view = updated.(*DisksView)
	if cmd == nil || view.activePane != paneTag {
		t.Fatalf("expected tag pane, pane=%d cmd nil=%t", view.activePane, cmd == nil)
	}
	view.tagInput.SetValue("VFWEB")
	updated, _ = view.handleTagKeys(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*DisksView)

	if len(cfg.ResourceTags) != 1 || cfg.ResourceTags[0].ResourceKind != "Disk" {
		t.Fatalf("expected disk resource tag, got %+v", cfg.ResourceTags)
	}
	if !strings.Contains(view.diskData[0].Labels, "cm:VFWEB") {
		t.Fatalf("expected disk labels to include CloudManager tag, got %q", view.diskData[0].Labels)
	}
}
