package databases

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func TestDatabaseEnterOpensActionsAndDescribe(t *testing.T) {
	cfg := config.AppConfig{DatabaseColumns: []string{"Name", "ID", "Engine", "Labels"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "AWS", AccountID: "acct", AccountName: "prod", Region: "us-east-1"}
	view.dbData = []core.Database{{Name: "orders", ID: "orders-db", Engine: "postgres", Status: "available"}}
	view.refreshTable()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*DatabasesView)
	if view.activePane != paneActions {
		t.Fatalf("expected actions pane, got %d", view.activePane)
	}

	updated, _ = view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*DatabasesView)
	if view.activePane != paneDescribe {
		t.Fatalf("expected describe pane, got %d", view.activePane)
	}
	if !strings.Contains(view.copyableText, "orders-db") || !strings.Contains(view.copyableText, "Console:") {
		t.Fatalf("expected database details with ID and console URL, got %q", view.copyableText)
	}
}

func TestDatabaseTagShortcutUsesSelectedRow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{DatabaseColumns: []string{"Name", "ID", "Labels"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "global"}
	view.dbData = []core.Database{{Name: "orders", ID: "db-1"}}
	view.refreshTable()

	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view = updated.(*DatabasesView)
	if cmd == nil || view.activePane != paneTag {
		t.Fatalf("expected tag pane, pane=%d cmd nil=%t", view.activePane, cmd == nil)
	}
	view.tagInput.SetValue("critical")
	updated, _ = view.handleTagKeys(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*DatabasesView)

	if len(cfg.ResourceTags) != 1 || cfg.ResourceTags[0].ResourceKind != "Database" {
		t.Fatalf("expected database resource tag, got %+v", cfg.ResourceTags)
	}
	if !strings.Contains(view.dbData[0].Labels, "cm:critical") {
		t.Fatalf("expected database labels to include CloudManager tag, got %q", view.dbData[0].Labels)
	}
}

func TestDatabaseArrowKeysMoveSelection(t *testing.T) {
	cfg := config.AppConfig{DatabaseColumns: []string{"Name", "ID", "Engine"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.dbData = []core.Database{
		{Name: "orders", ID: "db-1", Engine: "postgres"},
		{Name: "billing", ID: "db-2", Engine: "mysql"},
	}
	view.refreshTable()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	view = updated.(*DatabasesView)
	selected, ok := view.selectedDatabase()
	if !ok || selected.Name != "billing" {
		t.Fatalf("expected arrow down to select billing, ok=%t selected=%+v cursor=%d", ok, selected, view.table.Cursor())
	}
}
