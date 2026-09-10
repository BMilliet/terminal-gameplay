package ui

import (
	"fmt"
	"strings"
	"testing"

	"terminal-gameplay/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestFuzzySearchListStartsInFilteringMode(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{{T: "source/main.go", D: "/workspace/source/main.go"}},
		20,
		&endValue,
		true,
	)

	if !model.list.SettingFilter() {
		t.Fatal("fuzzy search list should start ready for filter input")
	}
	if len(model.list.VisibleItems()) != 1 {
		t.Fatalf("visible items on open = %d, want all items visible", len(model.list.VisibleItems()))
	}
}

func TestFuzzySearchMatchUsesExistingHighlightColorsWithoutUnderline(t *testing.T) {
	styles := DefaultStyles()
	matchStyle := fuzzySearchMatchStyle(styles)
	matchedRuneStyle := fuzzySearchMatchedRuneStyle(
		lipgloss.NewStyle().Foreground(styles.MutedTitleColor),
		matchStyle,
	)

	if matchStyle.GetUnderline() {
		t.Fatal("fuzzy-search match should not be underlined")
	}
	if got, want := fmt.Sprint(matchStyle.GetForeground()), fmt.Sprint(styles.CoralColor); got != want {
		t.Fatalf("match foreground = %q, want %q", got, want)
	}
	if _, ok := matchStyle.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("match background = %v, want no background color", matchStyle.GetBackground())
	}
	if got, want := fmt.Sprint(matchedRuneStyle.GetForeground()), fmt.Sprint(styles.CoralColor); got != want {
		t.Fatalf("rendered match foreground = %q, want %q", got, want)
	}
}

func TestFuzzySearchNavigatesVisibleItemsBeforeTyping(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{
			{T: "first.txt", D: "/workspace/first.txt"},
			{T: "second.txt", D: "/workspace/second.txt"},
		},
		20,
		&endValue,
		true,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(ListViewModel)
	selected, ok := model.list.SelectedItem().(utils.ListItem)
	if !ok || selected.T != "second.txt" {
		t.Fatalf("selected item after down = %#v, want second.txt", model.list.SelectedItem())
	}
	if !model.list.SettingFilter() {
		t.Fatal("filter should remain active after navigating before typing")
	}
}

func TestFuzzySearchListAcceptsQAsFilterInput(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{{T: "query.txt", D: "/workspace/query.txt"}},
		20,
		&endValue,
		true,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(ListViewModel)
	if got.quitting {
		t.Fatal("q should be entered into an active fuzzy filter, not quit the list")
	}
	if got.list.FilterValue() != "q" {
		t.Fatalf("filter value = %q, want q", got.list.FilterValue())
	}
}

func TestFuzzySearchEnterUsesCurrentFilterBeforeAsyncMatchReturns(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{
			{T: "alpha.txt", D: "/workspace/alpha.txt"},
			{T: "query.txt", D: "/workspace/query.txt"},
		},
		20,
		&endValue,
		true,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = updated.(ListViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ListViewModel)

	if !got.quitting {
		t.Fatal("enter should select a fuzzy-search result")
	}
	if endValue.T != "query.txt" {
		t.Fatalf("selected item = %q, want query.txt", endValue.T)
	}
}

func TestFuzzySearchEnterSelectsHighlightedItemAfterNavigation(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{
			{T: "alpha.txt", D: "/workspace/alpha.txt"},
			{T: "alpine.txt", D: "/workspace/alpine.txt"},
		},
		20,
		&endValue,
		true,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(ListViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(ListViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(ListViewModel)

	if !model.quitting {
		t.Fatal("enter should close the fuzzy search after selection")
	}
	if endValue.T != "alpine.txt" || endValue.D != "/workspace/alpine.txt" {
		t.Fatalf("selected item = %#v, want highlighted alpine.txt", endValue)
	}
}

func TestFuzzySearchKeepsFilterActiveWhileNavigatingResults(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{
			{T: "alpha.txt", D: "/workspace/alpha.txt"},
			{T: "alpine.txt", D: "/workspace/alpine.txt"},
		},
		20,
		&endValue,
		true,
	)
	model.list.SetFilterText("a")
	model.list.SetFilterState(list.Filtering)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(ListViewModel)
	if model.list.Index() != 1 {
		t.Fatalf("selected index after down = %d, want 1", model.list.Index())
	}
	selected, ok := model.list.SelectedItem().(utils.ListItem)
	if !ok || selected.T != "alpine.txt" {
		t.Fatalf("selected item after down = %#v, want alpine.txt", model.list.SelectedItem())
	}
	if !model.list.SettingFilter() {
		t.Fatal("filter should remain active after navigating results")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(ListViewModel)
	if model.list.FilterValue() != "al" {
		t.Fatalf("filter value after navigation and typing = %q, want al", model.list.FilterValue())
	}
	if !model.list.SettingFilter() {
		t.Fatal("filter should remain active after typing following navigation")
	}
}

func TestFuzzySearchShowsSelectedItemWhileFilterHasFocus(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{
			{T: "alpha.txt", D: "/workspace/alpha.txt"},
			{T: "alpine.txt", D: "/workspace/alpine.txt"},
		},
		20,
		&endValue,
		true,
	)
	model.list.SetFilterText("a")
	model.list.SetFilterState(list.Filtering)
	model.list.CursorDown()

	view := model.View()
	if !strings.Contains(view, "┃") || !strings.Contains(view, "alpine.txt") {
		t.Fatalf("fuzzy-search view does not visibly highlight selected item:\n%s", view)
	}
}

func TestFuzzySearchShowsAtLeastTenItems(t *testing.T) {
	endValue := utils.ListItem{}
	items := make([]utils.ListItem, 12)
	for index := range items {
		name := fmt.Sprintf("item-%02d.txt", index)
		items[index] = utils.ListItem{T: name, D: "/workspace/" + name}
	}
	model := newListViewModel("fuzzy search", items, 20, &endValue, true)

	view := model.View()
	visible := 0
	for _, item := range items {
		if strings.Contains(view, item.T) {
			visible++
		}
	}
	if visible < 10 {
		t.Fatalf("visible fuzzy-search items = %d, want at least 10:\n%s", visible, view)
	}
}

func TestFuzzySearchTreatsVimNavigationKeysAsFilterText(t *testing.T) {
	endValue := utils.ListItem{}
	model := newListViewModel(
		"fuzzy search",
		[]utils.ListItem{{T: "hjkl.txt", D: "/workspace/hjkl.txt"}},
		20,
		&endValue,
		true,
	)

	for _, character := range []rune{'h', 'j', 'k', 'l'} {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}})
		model = updated.(ListViewModel)
	}

	if model.list.FilterValue() != "hjkl" {
		t.Fatalf("filter value = %q, want hjkl", model.list.FilterValue())
	}
	if model.list.Index() != 0 {
		t.Fatalf("selected index = %d, want unchanged", model.list.Index())
	}
}

func TestFuzzySearchUsesLateralArrowsForPagination(t *testing.T) {
	endValue := utils.ListItem{}
	items := make([]utils.ListItem, 40)
	for index := range items {
		name := fmt.Sprintf("item-%02d.txt", index)
		items[index] = utils.ListItem{T: name, D: "/workspace/" + name}
	}
	model := newListViewModel("fuzzy search", items, 20, &endValue, true)
	if model.list.Paginator.TotalPages < 2 {
		t.Fatalf("total pages = %d, want at least 2", model.list.Paginator.TotalPages)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(ListViewModel)
	if model.list.Paginator.Page != 1 {
		t.Fatalf("page after right = %d, want 1", model.list.Paginator.Page)
	}
	if !model.list.SettingFilter() {
		t.Fatal("filter should remain active after changing page")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(ListViewModel)
	if model.list.Paginator.Page != 0 {
		t.Fatalf("page after left = %d, want 0", model.list.Paginator.Page)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	model = updated.(ListViewModel)
	if model.list.FilterValue() != "i" {
		t.Fatalf("filter value after pagination and typing = %q, want i", model.list.FilterValue())
	}
}

func TestFuzzySearchPaginationClampsCursorOnShortLastPage(t *testing.T) {
	endValue := utils.ListItem{}
	items := make([]utils.ListItem, 40)
	for index := range items {
		name := fmt.Sprintf("item-%02d.txt", index)
		items[index] = utils.ListItem{T: name, D: "/workspace/" + name}
	}
	model := newListViewModel("fuzzy search", items, 20, &endValue, true)
	lastPage := model.list.Paginator.TotalPages - 1
	model.list.Select(model.list.Paginator.PerPage - 1)
	for model.list.Paginator.Page < lastPage {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
		model = updated.(ListViewModel)
	}

	if model.list.SelectedItem() == nil {
		t.Fatal("last page should retain a valid selected item")
	}
}
