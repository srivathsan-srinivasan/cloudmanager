package snapshots

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func TestRenderWithManySnapshotsFitsViewWidth(t *testing.T) {
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"}}
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
		view.snapData = append(view.snapData, core.Snapshot{
			Name:           "snapshot-with-a-long-name",
			ID:             "snap-0123456789abcdef0",
			State:          "completed",
			SizeGB:         250,
			SourceDiskName: "disk-with-a-long-name",
			CreatedAt:      "2026-03-26T12:00:00Z",
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

func TestHorizontalPanChangesVisibleSnapshotColumns(t *testing.T) {
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"}}
	view := New(&cfg)
	view.width = 90
	view.height = 20
	view.refreshTable()

	initialFirst := view.tableCols[0].Title
	if !view.canScrollRight {
		t.Fatal("expected horizontal scrolling to be available in a narrow view")
	}

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*SnapshotsView)

	if next.columnOffset != 1 {
		t.Fatalf("expected column offset 1 after panning right, got %d", next.columnOffset)
	}
	if next.tableCols[0].Title == initialFirst {
		t.Fatalf("expected visible columns to change after panning, still starts with %q", next.tableCols[0].Title)
	}
}

func TestSnapshotTableArrowKeysMoveSelection(t *testing.T) {
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.snapData = []core.Snapshot{
		{Name: "alpha", ID: "snap-1"},
		{Name: "beta", ID: "snap-2"},
	}
	view.refreshTable()
	view.syncVisibleRows()
	view.snaps.Focus()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(*SnapshotsView)

	if got := next.snaps.Cursor(); got != 1 {
		t.Fatalf("expected cursor to move to row 1 after down key, got %d", got)
	}
}

func TestSelectedSnapshotUsesVisibleRows(t *testing.T) {
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.snapData = []core.Snapshot{
		{Name: "alpha", ID: "snap-1"},
		{Name: "beta", ID: "snap-2"},
	}
	view.refreshTable()
	view.searchInput.SetValue("beta")
	view.syncVisibleRows()

	selected, ok := view.selectedSnapshot()
	if !ok {
		t.Fatal("expected a selected snapshot")
	}
	if selected.ID != "snap-2" {
		t.Fatalf("expected filtered selection to return snap-2, got %s", selected.ID)
	}
}

func TestSetSearchQueryFiltersSnapshots(t *testing.T) {
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "State"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.snapData = []core.Snapshot{
		{Name: "alpha", ID: "snap-1"},
		{Name: "beta", ID: "snap-2"},
	}
	view.refreshTable()

	view.SetSearchQuery("snap-2")

	if len(view.visibleSnaps) != 1 || view.visibleSnaps[0].ID != "snap-2" {
		t.Fatalf("expected search handoff to filter to snap-2, got %+v", view.visibleSnaps)
	}
}

func TestTagSelectedSnapshotSavesCloudManagerTag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{SnapshotColumns: []string{"Name", "ID", "Labels"}}
	view := New(&cfg)
	view.width = 100
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	view.snapData = []core.Snapshot{{Name: "backup", ID: "snap-1"}}
	view.refreshTable()
	view.syncVisibleRows()

	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view = updated.(*SnapshotsView)
	if cmd == nil || view.activePane != paneTag {
		t.Fatalf("expected tag pane, pane=%d cmd nil=%t", view.activePane, cmd == nil)
	}
	view.tagInput.SetValue("VFWEB")
	updated, _ = view.handleTagKeys(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*SnapshotsView)

	if len(cfg.ResourceTags) != 1 || cfg.ResourceTags[0].ResourceKind != "Snapshot" {
		t.Fatalf("expected snapshot resource tag, got %+v", cfg.ResourceTags)
	}
	if !strings.Contains(view.snapData[0].Labels, "cm:VFWEB") {
		t.Fatalf("expected snapshot labels to include CloudManager tag, got %q", view.snapData[0].Labels)
	}
}
