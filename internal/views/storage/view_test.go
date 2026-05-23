package storage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

func TestStorageTableUsesResponsiveVisibleColumns(t *testing.T) {
	cfg := config.AppConfig{
		StorageColumns: []string{"Name", "Provider Type", "Region", "Access", "Encrypted", "Versioning"},
	}
	view := New(&cfg)
	view.width = 180
	view.height = 40
	view.bucketData = []core.StorageBucket{
		{
			Name:         "full-storage-bucket-name-that-should-use-extra-width",
			ProviderType: "Cloud Storage bucket",
			Region:       "US-CENTRAL1",
			Access:       "public blocked",
			Encrypted:    "default",
			Versioning:   "off",
		},
	}

	view.refreshTable()

	cols := view.table.Columns()
	if len(cols) != len(cfg.StorageColumns) {
		t.Fatalf("expected all storage columns visible, got %#v", cols)
	}
	providerWidth := columnWidth(cols, "Provider Type")
	if providerWidth < lipgloss.Width("Cloud Storage bucket") {
		t.Fatalf("expected Provider Type to fit full text, width=%d cols=%#v", providerWidth, cols)
	}
	nameWidth := columnWidth(cols, "Name")
	if nameWidth <= storageColumnWidth("Name") {
		t.Fatalf("expected Name to expand with spare terminal width, got %d", nameWidth)
	}
	row := view.table.Rows()[0]
	if strings.Contains(row[1], "…") {
		t.Fatalf("expected provider type to render without truncation, got row=%#v cols=%#v", row, cols)
	}
}

func columnWidth(cols []table.Column, title string) int {
	for _, col := range cols {
		if col.Title == title {
			return col.Width
		}
	}
	return 0
}

func TestTagSelectedStorageSavesCloudManagerTag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{StorageColumns: []string{"Name", "ID", "Labels"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "global"}
	view.bucketData = []core.StorageBucket{{Name: "logs", ID: "bucket-1"}}
	view.refreshTable()

	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view = updated.(*StorageView)
	if cmd == nil || view.activePane != paneTag {
		t.Fatalf("expected tag pane, pane=%d cmd nil=%t", view.activePane, cmd == nil)
	}
	view.tagInput.SetValue("archive")
	updated, _ = view.handleTagKeys(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*StorageView)

	if len(cfg.ResourceTags) != 1 || cfg.ResourceTags[0].ResourceKind != "Storage" {
		t.Fatalf("expected storage resource tag, got %+v", cfg.ResourceTags)
	}
	if !strings.Contains(view.bucketData[0].Labels, "cm:archive") {
		t.Fatalf("expected storage labels to include CloudManager tag, got %q", view.bucketData[0].Labels)
	}
}

func TestStorageEnterOpensActionsAndDescribe(t *testing.T) {
	cfg := config.AppConfig{StorageColumns: []string{"Name", "ID", "Provider Type"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.activeCtx = core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "global"}
	view.bucketData = []core.StorageBucket{{Name: "logs", ID: "bucket-1", ProviderType: "Cloud Storage bucket"}}
	view.refreshTable()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*StorageView)
	if view.activePane != paneActions {
		t.Fatalf("expected actions pane, got %d", view.activePane)
	}

	updated, _ = view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*StorageView)
	if view.activePane != paneDescribe {
		t.Fatalf("expected describe pane, got %d", view.activePane)
	}
	if !strings.Contains(view.copyableText, "gs://logs") || !strings.Contains(view.copyableText, "Console:") {
		t.Fatalf("expected storage details with URI and console URL, got %q", view.copyableText)
	}
}

func TestStorageArrowKeysMoveSelection(t *testing.T) {
	cfg := config.AppConfig{StorageColumns: []string{"Name", "ID", "Provider Type"}}
	view := New(&cfg)
	view.width = 120
	view.height = 30
	view.bucketData = []core.StorageBucket{
		{Name: "logs", ID: "bucket-1", ProviderType: "Cloud Storage bucket"},
		{Name: "archive", ID: "bucket-2", ProviderType: "Cloud Storage bucket"},
	}
	view.refreshTable()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	view = updated.(*StorageView)
	selected, ok := view.selectedBucket()
	if !ok || selected.Name != "archive" {
		t.Fatalf("expected arrow down to select archive, ok=%t selected=%+v cursor=%d", ok, selected, view.table.Cursor())
	}
}
