package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

func TestSortByColumnHandlesStatusAndNumbers(t *testing.T) {
	type row struct {
		Name   string
		Status string
		Count  string
	}
	rows := []row{
		{Name: "stopped", Status: "stopped", Count: "10"},
		{Name: "running", Status: "running", Count: "2"},
		{Name: "unknown", Status: "unknown", Count: "1"},
	}

	SortByColumn(rows, "Status", true, func(row row, column string) string {
		switch column {
		case "Status":
			return row.Status
		case "Count":
			return row.Count
		default:
			return row.Name
		}
	})
	if rows[0].Status != "running" || rows[1].Status != "stopped" || rows[2].Status != "unknown" {
		t.Fatalf("expected status sort rank, got %+v", rows)
	}

	SortByColumn(rows, "Count", true, func(row row, column string) string {
		return row.Count
	})
	if rows[0].Count != "1" || rows[1].Count != "2" || rows[2].Count != "10" {
		t.Fatalf("expected numeric sort, got %+v", rows)
	}
}

func TestHeaderSortStateDecoratesAndNormalizesColumns(t *testing.T) {
	columns := DecorateSortColumns(
		[]table.Column{{Title: "Name", Width: 10}, {Title: "Status", Width: 10}},
		HeaderSortState{Active: true, Index: 1},
		"Status",
		false,
	)
	if columns[1].Title != "▸ ↓ Status" {
		t.Fatalf("expected active sorted header marker, got %q", columns[1].Title)
	}
	if got := NormalizeSortColumnTitle(columns[1].Title); got != "Status" {
		t.Fatalf("expected normalized title, got %q", got)
	}
}
