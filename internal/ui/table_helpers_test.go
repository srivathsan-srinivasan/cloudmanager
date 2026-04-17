package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

func TestVisibleColumnsForWidthExpandsToFillViewport(t *testing.T) {
	cols := []table.Column{
		{Title: "Name", Width: 10},
		{Title: "Status", Width: 8},
		{Title: "Zone", Width: 12},
	}

	visible, _, canScrollLeft, canScrollRight := VisibleColumnsForWidth(cols, 40, 0)
	if canScrollLeft || canScrollRight {
		t.Fatalf("expected all columns visible without scrolling, got left=%t right=%t", canScrollLeft, canScrollRight)
	}

	total := 0
	overhead := 2
	for _, col := range visible {
		total += col.Width + overhead
	}
	if total != 40 {
		t.Fatalf("expected expanded columns to fill viewport width 40, got %d", total)
	}
	if visible[0].Width <= cols[0].Width {
		t.Fatalf("expected name column to expand, got %#v", visible)
	}
	nameGrowth := visible[0].Width - cols[0].Width
	statusGrowth := visible[1].Width - cols[1].Width
	if nameGrowth <= statusGrowth {
		t.Fatalf("expected descriptive column to absorb more width than status column, got %#v", visible)
	}
}

func TestVisibleColumnsForWidthKeepsSingleColumnClamped(t *testing.T) {
	cols := []table.Column{
		{Title: "Name", Width: 18},
		{Title: "Description", Width: 24},
	}

	visible, _, _, canScrollRight := VisibleColumnsForWidth(cols, 12, 0)
	if !canScrollRight {
		t.Fatal("expected right scrolling when viewport is narrower than full column set")
	}
	if len(visible) != 1 {
		t.Fatalf("expected one visible column in narrow viewport, got %d", len(visible))
	}
	if visible[0].Width != 10 { // 12 - 2 overhead
		t.Fatalf("expected single visible column to clamp to viewport width 10, got %d", visible[0].Width)
	}
}

func TestVisibleColumnsForWidthPrefersLongTextColumns(t *testing.T) {
	cols := []table.Column{
		{Title: "Name", Width: 12},
		{Title: "Description", Width: 18},
		{Title: "Rules", Width: 6},
		{Title: "Count", Width: 6},
	}

	visible, _, _, _ := VisibleColumnsForWidth(cols, 64, 0)
	if len(visible) != len(cols) {
		t.Fatalf("expected all columns visible, got %d", len(visible))
	}

	nameGrowth := visible[0].Width - cols[0].Width
	descriptionGrowth := visible[1].Width - cols[1].Width
	rulesGrowth := visible[2].Width - cols[2].Width
	countGrowth := visible[3].Width - cols[3].Width

	if descriptionGrowth <= rulesGrowth || descriptionGrowth <= countGrowth {
		t.Fatalf("expected description column to absorb more width than compact columns, got %#v", visible)
	}
	if nameGrowth <= rulesGrowth {
		t.Fatalf("expected name column to absorb more width than compact columns, got %#v", visible)
	}
}
