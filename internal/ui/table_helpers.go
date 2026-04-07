package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func DefaultTableStyles() table.Styles {
	return tableStyles(lipgloss.Color("229"), lipgloss.Color("57"))
}

func AlertSelectedTableStyles() table.Styles {
	return tableStyles(lipgloss.Color("255"), Alert)
}

func tableStyles(selectedForeground, selectedBackground lipgloss.TerminalColor) table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(true)
	styles.Selected = styles.Selected.Foreground(selectedForeground).
		Background(selectedBackground).Bold(false)
	return styles
}

func TableViewportWidth(availableWidth int) int {
	width := availableWidth - 6
	if width < 20 {
		return 20
	}
	return width
}

func TableHeight(totalHeight int) int {
	height := totalHeight - 12
	if height < 3 {
		return 3
	}
	return height
}

func VisibleColumnsForWidth(columns []table.Column, width, offset int) ([]table.Column, int, bool, bool) {
	if len(columns) == 0 {
		return nil, 0, false, false
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(columns) {
		offset = len(columns) - 1
	}

	var visible []table.Column
	used := 0
	for i := offset; i < len(columns); i++ {
		col := columns[i]
		if used+col.Width > width {
			if len(visible) == 0 {
				col.Width = width
				visible = append(visible, col)
			}
			break
		}
		visible = append(visible, col)
		used += col.Width
	}

	canScrollLeft := offset > 0
	canScrollRight := offset+len(visible) < len(columns)
	return expandColumnsToWidth(visible, width), offset, canScrollLeft, canScrollRight
}

func NewResourceTable(columns []table.Column, availableWidth, offset int, fallbackTitle string) (table.Model, []table.Column, int, bool, bool) {
	width := TableViewportWidth(availableWidth)
	visibleColumns, nextOffset, canScrollLeft, canScrollRight := VisibleColumnsForWidth(columns, width, offset)
	if len(visibleColumns) == 0 {
		visibleColumns = []table.Column{{Title: fallbackTitle, Width: width}}
	}

	tbl := table.New(
		table.WithColumns(visibleColumns),
		table.WithFocused(false),
		table.WithHeight(10),
		table.WithWidth(width),
	)
	tbl.SetStyles(DefaultTableStyles())
	return tbl, visibleColumns, nextOffset, canScrollLeft, canScrollRight
}

func TruncateText(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func AppendScrollHint(header string, canScrollLeft, canScrollRight bool, width int) string {
	if (!canScrollLeft && !canScrollRight) || width < 100 {
		return header
	}

	scrollHint := ""
	switch {
	case canScrollLeft && canScrollRight:
		scrollHint = "← more • → more"
	case canScrollLeft:
		scrollHint = "← more"
	case canScrollRight:
		scrollHint = "→ more"
	}

	hint := lipgloss.NewStyle().Foreground(Subtle).PaddingLeft(1).Render(scrollHint)
	candidate := lipgloss.JoinHorizontal(lipgloss.Left, header, hint)
	if lipgloss.Width(candidate) <= width-4 {
		return candidate
	}
	return header
}

func ClampToWindow(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

func expandColumnsToWidth(columns []table.Column, width int) []table.Column {
	if len(columns) == 0 || width <= 0 {
		return columns
	}
	used := 0
	for _, col := range columns {
		used += col.Width
	}
	extra := width - used
	if extra <= 0 {
		return columns
	}

	expanded := make([]table.Column, len(columns))
	copy(expanded, columns)

	weights := make([]int, len(expanded))
	totalWeight := 0
	for i, col := range expanded {
		weight := columnExpansionWeight(col)
		weights[i] = weight
		totalWeight += weight
	}
	if totalWeight <= 0 {
		totalWeight = len(expanded)
		for i := range weights {
			weights[i] = 1
		}
	}

	remainders := make([]int, len(expanded))
	assigned := 0
	for i := range expanded {
		share := extra * weights[i]
		growth := share / totalWeight
		remainders[i] = share % totalWeight
		expanded[i].Width += growth
		assigned += growth
	}

	for remaining := extra - assigned; remaining > 0; remaining-- {
		best := 0
		for i := 1; i < len(expanded); i++ {
			if remainders[i] > remainders[best] || (remainders[i] == remainders[best] && weights[i] > weights[best]) {
				best = i
			}
		}
		expanded[best].Width++
		remainders[best] = -1
	}
	return expanded
}

func columnExpansionWeight(col table.Column) int {
	title := strings.ToLower(strings.TrimSpace(col.Title))
	base := col.Width
	if base < 4 {
		base = 4
	}

	switch {
	case matchesColumnTitle(title, "name", "description", "details", "id", "image", "network", "subnet", "vpc", "project", "context", "group", "tags", "targets", "sources"):
		return base * 4
	case matchesColumnTitle(title, "status", "state", "type", "zone", "region", "count", "rules", "attached", "ports", "port", "proto", "protocol", "action", "direction", "age", "cpu", "ram", "size", "cost"):
		weight := base / 2
		if weight < 1 {
			return 1
		}
		return weight
	default:
		return base * 2
	}
}

func matchesColumnTitle(title string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(title, keyword) {
			return true
		}
	}
	return false
}
