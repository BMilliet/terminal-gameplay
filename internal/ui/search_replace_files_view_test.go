package ui

import (
	"strings"
	"testing"

	"terminal-gameplay/internal/tools"
	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFirstPendingSearchReplaceItemSkipsReplacedFiles(t *testing.T) {
	items := []utils.ListItem{
		{T: "one.txt", Status: tools.ReplacedStatus},
		{T: "two.txt", Status: tools.ReplacedStatus},
		{T: "three.txt"},
	}

	if got := firstPendingSearchReplaceItem(items); got != 2 {
		t.Fatalf("firstPendingSearchReplaceItem() = %d, want 2", got)
	}
}

func TestFirstPendingSearchReplaceItemReturnsZeroWhenAllReplaced(t *testing.T) {
	items := []utils.ListItem{
		{T: "one.txt", Status: tools.ReplacedStatus},
		{T: "two.txt", Status: tools.ReplacedStatus},
	}

	if got := firstPendingSearchReplaceItem(items); got != 0 {
		t.Fatalf("firstPendingSearchReplaceItem() = %d, want 0", got)
	}
}

func TestSearchReplaceFilesViewAATriggersReplaceAll(t *testing.T) {
	selected := utils.ListItem{}
	model := SearchReplaceFilesViewModel{
		items:    []utils.ListItem{{T: "one.txt"}},
		endValue: &selected,
		styles:   DefaultStyles(),
	}

	updated, cmd := model.Update(keyRunes("a"))
	if cmd != nil {
		t.Fatalf("first a returned command, want nil")
	}

	model = updated.(SearchReplaceFilesViewModel)
	if !model.pendingAll {
		t.Fatal("first a should set pendingAll")
	}

	updated, cmd = model.Update(keyRunes("a"))
	model = updated.(SearchReplaceFilesViewModel)
	if cmd == nil {
		t.Fatal("second a should return quit command")
	}
	if selected.T != tools.ReplaceAllAction {
		t.Fatalf("selected = %#v, want ReplaceAllAction", selected)
	}
	if !model.quitting {
		t.Fatal("second a should mark model quitting")
	}
}

func TestSearchReplaceFilesViewEnterReturnsSelectedItemWithStatus(t *testing.T) {
	selected := utils.ListItem{}
	model := SearchReplaceFilesViewModel{
		items: []utils.ListItem{
			{T: "one.txt"},
			{T: "two.txt", Status: tools.ReplacedStatus},
		},
		cursor:   1,
		endValue: &selected,
		styles:   DefaultStyles(),
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(SearchReplaceFilesViewModel)
	if cmd == nil {
		t.Fatal("enter should return quit command")
	}
	if selected.T != "two.txt" || selected.Status != tools.ReplacedStatus {
		t.Fatalf("selected = %#v, want replaced two.txt", selected)
	}
	if !model.quitting {
		t.Fatal("enter should mark model quitting")
	}
}

func TestSearchReplaceFilesViewMoveCursorWrapsAndScrolls(t *testing.T) {
	model := SearchReplaceFilesViewModel{
		items:      []utils.ListItem{{T: "0"}, {T: "1"}, {T: "2"}},
		maxVisible: 2,
		styles:     DefaultStyles(),
	}

	model.moveCursor(-1)
	if model.cursor != 2 || model.viewportStart != 1 {
		t.Fatalf("moveCursor(-1) cursor=%d viewport=%d, want cursor 2 viewport 1", model.cursor, model.viewportStart)
	}

	model.moveCursor(1)
	if model.cursor != 0 || model.viewportStart != 0 {
		t.Fatalf("moveCursor(1) cursor=%d viewport=%d, want cursor 0 viewport 0", model.cursor, model.viewportStart)
	}
}

func TestSearchReplaceFilesViewRendersReplacedAsAlignedColumn(t *testing.T) {
	selected := utils.ListItem{}
	model := SearchReplaceFilesViewModel{
		title:         "search/replace",
		items:         []utils.ListItem{{T: "a.go", Status: tools.ReplacedStatus}, {T: "cmd/projmap/main.go", Status: tools.ReplacedStatus}},
		cursor:        -1,
		maxVisible:    10,
		endValue:      &selected,
		styles:        DefaultStyles(),
		terminalWidth: defaultContentWidth + screenGutterWidth,
	}

	view := model.View()
	lines := strings.Split(view, "\n")
	replacedColumns := []int{}
	for _, line := range lines {
		if idx := strings.Index(line, tools.ReplacedStatus); idx >= 0 {
			replacedColumns = append(replacedColumns, idx)
		}
	}

	if len(replacedColumns) != 2 {
		t.Fatalf("replaced columns = %#v, want two rendered statuses in view:\n%s", replacedColumns, view)
	}
	if replacedColumns[0] != replacedColumns[1] {
		t.Fatalf("replaced columns = %#v, want aligned statuses", replacedColumns)
	}
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
