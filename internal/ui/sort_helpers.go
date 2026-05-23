package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
)

type SortColumnItem struct {
	Name string
}

func (i SortColumnItem) Title() string       { return i.Name }
func (i SortColumnItem) Description() string { return "Sort by this column" }
func (i SortColumnItem) FilterValue() string { return i.Name }

func NewSortList(title string, columns []table.Column) list.Model {
	items := make([]list.Item, 0, len(columns))
	for _, col := range columns {
		items = append(items, SortColumnItem{Name: col.Title})
	}
	sortList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	sortList.Title = title
	sortList.SetShowStatusBar(false)
	sortList.SetFilteringEnabled(false)
	return sortList
}

func ToggleSortColumn(current string, asc bool, next string) (string, bool) {
	next = NormalizeSortColumnTitle(next)
	if current == next {
		return current, !asc
	}
	return next, true
}

type HeaderSortState struct {
	Active bool
	Index  int
}

func (s *HeaderSortState) Activate(columns []table.Column) {
	s.Active = true
	s.Clamp(columns)
}

func (s *HeaderSortState) Deactivate() {
	s.Active = false
}

func (s *HeaderSortState) Clamp(columns []table.Column) {
	if len(columns) == 0 {
		s.Index = 0
		return
	}
	if s.Index < 0 {
		s.Index = 0
	}
	if s.Index >= len(columns) {
		s.Index = len(columns) - 1
	}
}

func (s *HeaderSortState) Move(delta int, columns []table.Column) bool {
	if len(columns) == 0 {
		s.Index = 0
		return false
	}
	next := s.Index + delta
	if next < 0 || next >= len(columns) {
		return false
	}
	s.Index = next
	return true
}

func (s HeaderSortState) SelectedColumn(columns []table.Column) string {
	if len(columns) == 0 {
		return ""
	}
	if s.Index < 0 || s.Index >= len(columns) {
		return NormalizeSortColumnTitle(columns[0].Title)
	}
	return NormalizeSortColumnTitle(columns[s.Index].Title)
}

func DecorateSortColumns(columns []table.Column, state HeaderSortState, sortColumn string, asc bool) []table.Column {
	if len(columns) == 0 {
		return columns
	}
	decorated := make([]table.Column, len(columns))
	copy(decorated, columns)
	for i := range decorated {
		name := NormalizeSortColumnTitle(decorated[i].Title)
		prefix := ""
		if name == sortColumn {
			if asc {
				prefix = "↑ "
			} else {
				prefix = "↓ "
			}
		}
		if state.Active && i == state.Index {
			prefix = "▸ " + prefix
		}
		decorated[i].Title = prefix + name
	}
	return decorated
}

func NormalizeSortColumnTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimPrefix(title, "▸ ")
	title = strings.TrimPrefix(title, "↑ ")
	title = strings.TrimPrefix(title, "↓ ")
	return strings.TrimSpace(title)
}

func SortDirectionLabel(asc bool) string {
	if asc {
		return "asc"
	}
	return "desc"
}

func SortByColumn[T any](items []T, column string, asc bool, field func(T, string) string) {
	if strings.TrimSpace(column) == "" {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		cmp := CompareSortValues(field(items[i], column), field(items[j], column), column)
		if asc {
			return cmp < 0
		}
		return cmp > 0
	})
}

func CompareSortValues(left, right, column string) int {
	column = strings.ToLower(strings.TrimSpace(column))
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if matchesColumnTitle(column, "status", "state") {
		if cmp := StatusSortRank(left) - StatusSortRank(right); cmp != 0 {
			return cmp
		}
	}
	leftNum, leftOK := parseSortNumber(left)
	rightNum, rightOK := parseSortNumber(right)
	if leftOK && rightOK {
		switch {
		case leftNum < rightNum:
			return -1
		case leftNum > rightNum:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func parseSortNumber(value string) (float64, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.TrimPrefix(value, "$")
	value = strings.ReplaceAll(value, ",", "")
	if value == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil
}
