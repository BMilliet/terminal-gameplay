package ui

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	aliastab "terminal-gameplay/internal/alias"
	"terminal-gameplay/internal/clipboard"
	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"
	"terminal-gameplay/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type fakeCurrentDirectoryReader struct {
	entriesByPath          map[string][]fileMenuEntry
	errorsByPath           map[string]error
	previewEntriesByPath   map[string][]fileMenuEntry
	previewTruncatedByPath map[string]bool
	fileContentsByPath     map[string][]byte
	fileTruncatedByPath    map[string]bool
	metadataByPath         map[string]fileMenuMetadata
	calls                  []string
	directoryPreviewCalls  []directoryPreviewCall
	filePreviewCalls       []filePreviewCall
}

type directoryPreviewCall struct {
	path  string
	limit int
}

type filePreviewCall struct {
	path     string
	maxBytes int
}

type fakeFileMenuOperator struct {
	batches [][]fileMenuOperation
	err     error
}

func (f *fakeFileMenuOperator) Apply(operations []fileMenuOperation) error {
	batch := append([]fileMenuOperation(nil), operations...)
	f.batches = append(f.batches, batch)
	return f.err
}

func updateFileMenuWithKeys(model MultiPageViewModel, keys ...tea.KeyMsg) MultiPageViewModel {
	for _, key := range keys {
		updated, _ := model.Update(key)
		model = updated.(MultiPageViewModel)
	}
	return model
}

func runeKey(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}

func (f *fakeCurrentDirectoryReader) ReadDirectory(path string) ([]fileMenuEntry, error) {
	f.calls = append(f.calls, path)
	return f.entriesByPath[path], f.errorsByPath[path]
}

func (f *fakeCurrentDirectoryReader) ReadDirectoryPreview(path string, limit int) ([]fileMenuEntry, bool, error) {
	f.directoryPreviewCalls = append(f.directoryPreviewCalls, directoryPreviewCall{path: path, limit: limit})
	return f.previewEntriesByPath[path], f.previewTruncatedByPath[path], f.errorsByPath[path]
}

func (f *fakeCurrentDirectoryReader) ReadFilePreview(path string, maxBytes int) ([]byte, bool, error) {
	f.filePreviewCalls = append(f.filePreviewCalls, filePreviewCall{path: path, maxBytes: maxBytes})
	return f.fileContentsByPath[path], f.fileTruncatedByPath[path], f.errorsByPath[path]
}

func (f *fakeCurrentDirectoryReader) ReadMetadata(path string) (fileMenuMetadata, error) {
	return f.metadataByPath[path], f.errorsByPath[path]
}

func TestHeaderShowsAppVersion(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})

	if view := model.View(); !strings.Contains(view, "v"+version.AppVersion) {
		t.Fatalf("view does not show app version %q: %q", version.AppVersion, view)
	}
}

func TestFKeyOpensFileMenuWithHeaderWithoutQuickMenuPages(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	reader := &fakeCurrentDirectoryReader{}
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)

	if model.currentMenu != FileMenu {
		t.Fatalf("current menu = %v, want FileMenu", model.currentMenu)
	}
	if len(reader.calls) != 1 || reader.calls[0] != "." {
		t.Fatalf("directory read calls = %#v, want [.]", reader.calls)
	}
	view := model.View()
	if !strings.Contains(view, "terminal-gameplay") || !strings.Contains(view, "v"+version.AppVersion) {
		t.Fatalf("file menu does not show the standard app header: %q", view)
	}
	for _, page := range model.availPages {
		if pageName := model.getPageNameByType(page); strings.Contains(view, pageName) {
			t.Fatalf("file menu header contains quick-menu page %q: %q", pageName, view)
		}
	}
}

func TestFileMenuShowsTwoSideBySideBoxesAndColoredFileTree(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "notes.txt", kind: fileMenuRegularFile},
			{name: "preview.PNG", kind: fileMenuImage},
			{name: "run-tool", kind: fileMenuExecutable},
			{name: "assets", kind: fileMenuDirectory},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.terminalWidth = 100

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	view := model.View()

	twoBoxesOnOneLine := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╭") == 2 {
			twoBoxesOnOneLine = true
			break
		}
	}
	if !twoBoxesOnOneLine {
		t.Fatalf("file menu does not render two side-by-side boxes: %q", view)
	}

	wantLabels := map[string]lipgloss.Color{
		"assets/":     model.styles.FileMenuDirectoryColor,
		"notes.txt":   model.styles.FileMenuFileColor,
		"run-tool":    model.styles.FileMenuExecutableColor,
		"preview.PNG": model.styles.FileMenuImageColor,
	}
	seenColors := map[lipgloss.Color]bool{}
	for label, color := range wantLabels {
		seenColors[color] = true
		coloredLabel := lipgloss.NewStyle().Foreground(color).Render(label)
		if !strings.Contains(view, coloredLabel) {
			t.Errorf("file menu does not render %q with color %q: %q", label, color, view)
		}
	}
	if len(seenColors) != len(wantLabels) {
		t.Fatalf("file menu colors are not distinct: %#v", wantLabels)
	}

	if directoryIndex, fileIndex := strings.Index(view, "assets/"), strings.Index(view, "notes.txt"); directoryIndex < 0 || fileIndex < 0 || directoryIndex > fileIndex {
		t.Fatalf("directory should be shown before files: %q", view)
	}
}

func TestFileMenuFillsTerminalHeightWithoutResizingWhileNavigating(t *testing.T) {
	manyEntries := make([]fileMenuEntry, 30)
	for index := range manyEntries {
		manyEntries[index] = fileMenuEntry{name: fmt.Sprintf("file-%02d.txt", index), kind: fileMenuRegularFile}
	}
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":     {{name: "large", kind: fileMenuDirectory}},
		"large": manyEntries,
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(MultiPageViewModel)
	model = updateFileMenuWithKeys(model, runeKey('f'))
	rootHeight := lipgloss.Height(model.View())

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEnter})
	nestedHeight := lipgloss.Height(model.View())

	if rootHeight != 40 || nestedHeight != 40 {
		t.Fatalf("file menu heights = root %d, nested %d; want terminal height 40", rootHeight, nestedHeight)
	}
	for range 25 {
		model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyDown})
	}
	if view := model.View(); !strings.Contains(view, "file-25.txt") {
		t.Fatalf("cursor moved outside the fixed-height viewport: %q", view)
	}
}

func TestFileMenuPreviewIsClippedToTreePanelDimensions(t *testing.T) {
	longName := strings.Repeat("very-long-name-", 20) + ".txt"
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {{name: "directory", kind: fileMenuDirectory}},
		},
		previewEntriesByPath: map[string][]fileMenuEntry{
			"directory": {
				{name: longName, kind: fileMenuRegularFile},
				{name: longName + "-second", kind: fileMenuRegularFile},
			},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 34})
	model = updated.(MultiPageViewModel)
	model = updateFileMenuWithKeys(model, runeKey('f'))

	if height := lipgloss.Height(model.View()); height != 34 {
		t.Fatalf("long preview changed file menu height to %d, want 34: %q", height, model.View())
	}
}

func TestRenderFixedHeightFileMenuBoxPreservesExactHeightAndBottomBorder(t *testing.T) {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(12).
		Height(4)
	content := strings.Repeat("content that wraps ", 20)

	rendered := renderFixedHeightFileMenuBox(style, content, 6)
	lines := strings.Split(rendered, "\n")

	if height := lipgloss.Height(rendered); height != 6 {
		t.Fatalf("fixed box height = %d, want 6: %q", height, rendered)
	}
	if !strings.Contains(lines[len(lines)-1], "╰") || !strings.Contains(lines[len(lines)-1], "╯") {
		t.Fatalf("fixed box lost its bottom border: %q", rendered)
	}
}

func TestFileMenuRightBoxPreviewsSelectedFileAndUpdatesWithCursor(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {
				{name: "notes.txt", kind: fileMenuRegularFile},
				{name: "assets", kind: fileMenuDirectory},
			},
		},
		previewEntriesByPath: map[string][]fileMenuEntry{
			"assets": {
				{name: "logo.png", kind: fileMenuImage},
			},
		},
		fileContentsByPath: map[string][]byte{
			"notes.txt": []byte("first preview line\nsecond preview line"),
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	if view := model.View(); !strings.Contains(view, "logo.png") {
		t.Fatalf("directory preview does not show child entry: %q", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(MultiPageViewModel)
	view := model.View()
	if !strings.Contains(view, "first preview line") || !strings.Contains(view, "second preview line") {
		t.Fatalf("file preview does not show selected file contents: %q", view)
	}
	if len(reader.filePreviewCalls) != 1 {
		t.Fatalf("file preview calls = %#v, want one call", reader.filePreviewCalls)
	}
	if call := reader.filePreviewCalls[0]; call.path != "notes.txt" || call.maxBytes != fileMenuPreviewMaxBytes {
		t.Fatalf("file preview call = %#v, want notes.txt limited to %d bytes", call, fileMenuPreviewMaxBytes)
	}
}

func TestFileMenuDirectoryPreviewUsesBoundedReadAndShowsTruncation(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {
				{name: "vendor", kind: fileMenuDirectory},
			},
		},
		previewEntriesByPath: map[string][]fileMenuEntry{
			"vendor": {
				{name: "package-a", kind: fileMenuDirectory},
				{name: "LICENSE", kind: fileMenuRegularFile},
			},
		},
		previewTruncatedByPath: map[string]bool{"vendor": true},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	view := model.View()

	if !strings.Contains(view, "package-a/") || !strings.Contains(view, "LICENSE") || !strings.Contains(view, "…") {
		t.Fatalf("bounded directory preview is incomplete: %q", view)
	}
	if len(reader.directoryPreviewCalls) != 1 {
		t.Fatalf("directory preview calls = %#v, want one call", reader.directoryPreviewCalls)
	}
	if call := reader.directoryPreviewCalls[0]; call.path != "vendor" || call.limit != fileMenuPreviewMaxEntries {
		t.Fatalf("directory preview call = %#v, want vendor limited to %d entries", call, fileMenuPreviewMaxEntries)
	}
	if len(reader.calls) != 1 || reader.calls[0] != "." {
		t.Fatalf("full directory reads = %#v, want only current directory", reader.calls)
	}
}

func TestSanitizeFilePreviewRemovesTerminalControlsAndLimitsLines(t *testing.T) {
	content := []byte("safe\x1b[31m red\nsecond\nthird")

	got, binary, truncated := sanitizeFilePreview(content, 2)

	if binary {
		t.Fatal("text preview was classified as binary")
	}
	if !truncated {
		t.Fatal("line-limited preview was not marked as truncated")
	}
	if strings.Contains(got, "\x1b") || got != "safe[31m red\nsecond" {
		t.Fatalf("sanitized preview = %q, want controls removed and two lines", got)
	}
}

func TestFileMenuSelectorMovesThroughEntries(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "notes.txt", kind: fileMenuRegularFile},
			{name: "assets", kind: fileMenuDirectory},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	if model.fileMenuCursor != 0 {
		t.Fatalf("initial file menu cursor = %d, want 0", model.fileMenuCursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(MultiPageViewModel)
	if model.fileMenuCursor != 1 {
		t.Fatalf("file menu cursor after down = %d, want 1", model.fileMenuCursor)
	}
	selectedRowFound := false
	for _, line := range strings.Split(model.View(), "\n") {
		if strings.Contains(line, "notes.txt") && strings.Contains(line, "█") {
			selectedRowFound = true
		}
	}
	if !selectedRowFound {
		t.Fatal("selected file row was not rendered")
	}
}

func TestFileMenuEnterOnDirectoryShowsItsContents(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "projects", kind: fileMenuDirectory},
		},
		"projects": {
			{name: "main.go", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)

	if model.fileMenuPath != "projects" {
		t.Fatalf("file menu path = %q, want projects", model.fileMenuPath)
	}
	if len(reader.calls) != 2 || reader.calls[0] != "." || reader.calls[1] != "projects" {
		t.Fatalf("directory read calls = %#v, want [., projects]", reader.calls)
	}
	if view := model.View(); !strings.Contains(view, "main.go") || !strings.Contains(view, "▾ projects") {
		t.Fatalf("file menu does not show selected directory contents: %q", view)
	}
}

func TestFileMenuRightArrowEntersSelectedDirectory(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "projects", kind: fileMenuDirectory},
		},
		"projects": {
			{name: "main.go", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyRight})

	if model.fileMenuPath != "projects" {
		t.Fatalf("file menu path after right = %q, want projects", model.fileMenuPath)
	}
	if view := model.View(); !strings.Contains(view, "main.go") {
		t.Fatalf("right arrow did not show directory contents: %q", view)
	}
}

func TestFileMenuRightArrowDoesNotOpenSelectedFile(t *testing.T) {
	selected := ""
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}
	model.selected = &selected

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyRight})

	if selected != "" || model.quitting {
		t.Fatalf("right arrow opened file: selected=%q quitting=%v", selected, model.quitting)
	}
}

func TestFileMenuLeftArrowReturnsToPreviousDirectoryWithoutLeavingInitialRoot(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "projects", kind: fileMenuDirectory},
		},
		"projects": {
			{name: "main.go", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(MultiPageViewModel)

	if model.fileMenuPath != "." {
		t.Fatalf("file menu path after left = %q, want initial root", model.fileMenuPath)
	}
	if len(reader.calls) != 3 || reader.calls[0] != "." || reader.calls[1] != "projects" || reader.calls[2] != "." {
		t.Fatalf("directory read calls = %#v, want [., projects, .]", reader.calls)
	}
	if view := model.View(); !strings.Contains(view, "projects/") || !strings.Contains(view, "▾ .") {
		t.Fatalf("file menu does not show previous directory: %q", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(MultiPageViewModel)
	if model.fileMenuPath != "." || len(reader.calls) != 3 {
		t.Fatalf("left at initial root changed path or reloaded it: path=%q calls=%#v", model.fileMenuPath, reader.calls)
	}
}

func TestFileMenuEnterOnFileReturnsNvimAction(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "notes.txt", kind: fileMenuRegularFile},
		},
	}}
	selected := ""
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.selected = &selected

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)

	want := FileMenuPageName + "|" + OpenFileAction + "|notes.txt"
	if selected != want {
		t.Fatalf("selected action = %q, want %q", selected, want)
	}
	if !model.quitting {
		t.Fatal("file menu should quit before handing the file to the runner")
	}
}

func TestFileMenuVisualModeSelectsMultipleEntriesForCopy(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "one.txt", kind: fileMenuRegularFile},
			{name: "two.txt", kind: fileMenuRegularFile},
			{name: "three.txt", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('v'), tea.KeyMsg{Type: tea.KeyDown}, runeKey('y'))

	if model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("file menu mode = %v, want normal after yank", model.fileMenuMode)
	}
	if len(model.fileMenuClipboard) != 2 {
		t.Fatalf("clipboard = %#v, want two selected entries", model.fileMenuClipboard)
	}
	if model.fileMenuClipboard[0].path != "one.txt" || model.fileMenuClipboard[1].path != "three.txt" {
		t.Fatalf("clipboard paths = %#v, want one.txt and three.txt", model.fileMenuClipboard)
	}
	if model.fileMenuClipboardMove {
		t.Fatal("visual yank should copy, not move")
	}
}

func TestFileMenuShiftVEntersVisualLineMode(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "one.txt", kind: fileMenuRegularFile},
			{name: "two.txt", kind: fileMenuRegularFile},
		},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('V'), tea.KeyMsg{Type: tea.KeyDown})

	if model.fileMenuMode != fileMenuVisualMode {
		t.Fatalf("file menu mode = %v, want visual line mode", model.fileMenuMode)
	}
	if selected := model.selectedFileMenuEntries(); len(selected) != 2 {
		t.Fatalf("selected entries = %#v, want two lines", selected)
	}
	if view := model.View(); !strings.Contains(view, "-- VISUAL LINE --") {
		t.Fatalf("visual line mode is not visible: %q", view)
	}
}

func TestFileMenuCreateIsStagedAndAppliedOnlyAfterWriteConfirmation(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}
	operator := &fakeFileMenuOperator{}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuOperator = operator

	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('o'), runeKey('n'), runeKey('e'), runeKey('w'), runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter},
		runeKey(':'), runeKey('w'), tea.KeyMsg{Type: tea.KeyEnter},
	)

	if model.fileMenuMode != fileMenuConfirmMode {
		t.Fatalf("file menu mode = %v, want confirmation", model.fileMenuMode)
	}
	if len(operator.batches) != 0 {
		t.Fatalf("operations were applied before confirmation: %#v", operator.batches)
	}
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuCreateFile || model.fileMenuOperations[0].destination != "new.txt" {
		t.Fatalf("staged operations = %#v, want create new.txt", model.fileMenuOperations)
	}

	updated, cmd := model.Update(runeKey('y'))
	model = updated.(MultiPageViewModel)
	if model.fileMenuMode != fileMenuProcessingMode || cmd == nil {
		t.Fatalf("save did not start background processing: mode=%v cmd=%v", model.fileMenuMode, cmd)
	}
	if view := model.View(); !strings.Contains(view, "running") {
		t.Fatalf("process panel does not show active work: %q", view)
	}
	finished := cmd()
	updated, _ = model.Update(finished)
	model = updated.(MultiPageViewModel)

	if len(operator.batches) != 1 || len(operator.batches[0]) != 1 {
		t.Fatalf("applied batches = %#v, want one operation", operator.batches)
	}
	if len(model.fileMenuOperations) != 0 || model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("successful save did not clear pending changes: %#v mode=%v", model.fileMenuOperations, model.fileMenuMode)
	}
}

func TestFileMenuRendersProcessMetadataAndMemoryPanels(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	modified := time.Date(2026, time.October, 12, 14, 30, 0, 0, time.Local)
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {
				{name: "one.txt", kind: fileMenuRegularFile},
				{name: "two.txt", kind: fileMenuRegularFile},
			},
		},
		metadataByPath: map[string]fileMenuMetadata{
			"one.txt": {size: 12, mode: 0o644, modified: modified},
			"two.txt": {size: 24, mode: 0o600, modified: modified},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.terminalWidth = 120

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('V'), tea.KeyMsg{Type: tea.KeyDown}, runeKey('y'))
	view := model.View()

	threeBottomPanels := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╭") == 3 {
			threeBottomPanels = true
			break
		}
	}
	if !threeBottomPanels {
		t.Fatalf("file menu does not render three lower panels: %q", view)
	}
	for _, text := range []string{"Process", "Metadata", "Memory", "24 B", "-rw-------", "one.txt", "two.txt"} {
		if !strings.Contains(view, text) {
			t.Errorf("lower panels do not contain %q: %q", text, view)
		}
	}
}

func TestFileMenuCreateWithTrailingSlashStagesDirectory(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('o'), runeKey('d'), runeKey('o'), runeKey('c'), runeKey('s'), runeKey('/'), tea.KeyMsg{Type: tea.KeyEnter})

	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuCreateDirectory || model.fileMenuOperations[0].destination != "docs" {
		t.Fatalf("staged operations = %#v, want create directory docs", model.fileMenuOperations)
	}
}

func TestFileMenuDeleteStagesVisualSelection(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "one.txt", kind: fileMenuRegularFile},
			{name: "two.txt", kind: fileMenuRegularFile},
		},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('v'), tea.KeyMsg{Type: tea.KeyDown}, runeKey('d'))

	if len(model.fileMenuOperations) != 2 {
		t.Fatalf("staged operations = %#v, want two deletes", model.fileMenuOperations)
	}
	for _, operation := range model.fileMenuOperations {
		if operation.kind != fileMenuDelete {
			t.Fatalf("operation = %#v, want delete", operation)
		}
	}
}

func TestFileMenuDDKeepsEntryInRegisterAndPasteTurnsDeleteIntoMove(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "destination", kind: fileMenuDirectory},
			{name: "notes.txt", kind: fileMenuRegularFile},
		},
		"destination": {},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyDown}, runeKey('d'), runeKey('d'))

	if len(model.fileMenuClipboard) != 1 || model.fileMenuClipboard[0].path != "notes.txt" || !model.fileMenuClipboardMove || !model.fileMenuClipboardCut {
		t.Fatalf("register after dd = %#v move=%v cut=%v", model.fileMenuClipboard, model.fileMenuClipboardMove, model.fileMenuClipboardCut)
	}
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuDelete {
		t.Fatalf("operations after dd = %#v, want pending delete", model.fileMenuOperations)
	}
	if len(model.fileMenuEntries) != 1 || model.fileMenuEntries[0].name != "destination" {
		t.Fatalf("deleted line is still present in the file tree: %#v", model.fileMenuEntries)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEnter}, runeKey('p'))

	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("operations after paste = %#v, want one move", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuMove || operation.source != "notes.txt" || operation.destination != "destination/notes.txt" {
		t.Fatalf("operation after paste = %#v, want move into destination", operation)
	}
	if model.fileMenuClipboardCut || len(model.fileMenuClipboard) != 0 {
		t.Fatalf("cut register was not consumed after paste: %#v cut=%v", model.fileMenuClipboard, model.fileMenuClipboardCut)
	}
	if view := model.View(); !strings.Contains(view, "notes.txt") || !strings.Contains(view, "pending change") {
		t.Fatalf("moved entry is not visible at destination: %q", view)
	}
}

func TestFileMenuDDThenPasteInSameDirectoryRestoresLine(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('d'), runeKey('d'), runeKey('p'))

	if len(model.fileMenuOperations) != 0 {
		t.Fatalf("operations after restoring cut line = %#v, want none", model.fileMenuOperations)
	}
	if len(model.fileMenuEntries) != 1 || model.fileMenuEntries[0].name != "notes.txt" {
		t.Fatalf("restored entries = %#v, want notes.txt", model.fileMenuEntries)
	}
}

func TestFileMenuCopyAndMovePasteIntoCurrentDirectory(t *testing.T) {
	entries := []fileMenuEntry{
		{name: "dest", kind: fileMenuDirectory},
		{name: "one.txt", kind: fileMenuRegularFile},
		{name: "two.txt", kind: fileMenuRegularFile},
	}
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":    entries,
		"dest": {},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyDown}, runeKey('y'), runeKey('y'), tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyEnter}, runeKey('p'))

	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("copy operations = %#v, want one", model.fileMenuOperations)
	}
	copyOperation := model.fileMenuOperations[0]
	if copyOperation.kind != fileMenuCopy || copyOperation.source != "one.txt" || copyOperation.destination != "dest/one.txt" {
		t.Fatalf("copy operation = %#v", copyOperation)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown}, runeKey('m'), runeKey('m'), tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyEnter}, runeKey('p'))
	if len(model.fileMenuOperations) != 2 {
		t.Fatalf("copy/move operations = %#v, want two", model.fileMenuOperations)
	}
	moveOperation := model.fileMenuOperations[1]
	if moveOperation.kind != fileMenuMove || moveOperation.source != "two.txt" || moveOperation.destination != "dest/two.txt" {
		t.Fatalf("move operation = %#v", moveOperation)
	}
}

func TestFileMenuPasteAppearsImmediatelyAsPendingEntry(t *testing.T) {
	operator := &fakeFileMenuOperator{}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}
	model.fileMenuOperator = operator

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('y'), runeKey('y'), runeKey('p'))

	if len(operator.batches) != 0 {
		t.Fatalf("paste touched the filesystem before :w: %#v", operator.batches)
	}
	if len(model.fileMenuEntries) != 2 {
		t.Fatalf("visible entries = %#v, want source and pending copy", model.fileMenuEntries)
	}
	pasted := model.fileMenuEntries[model.fileMenuCursor]
	if pasted.name != "notes-copy.txt" || !pasted.pending {
		t.Fatalf("selected pasted entry = %#v, want pending notes-copy.txt", pasted)
	}
	if view := model.View(); !strings.Contains(view, "notes-copy.txt") || !strings.Contains(view, "pending change") {
		t.Fatalf("pending paste is not visible in file menu: %q", view)
	}
}

func TestFileMenuAEditsNewEntryInlineInsideEmptyDirectory(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":     {{name: "empty", kind: fileMenuDirectory}},
		"empty": {},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyEnter}, runeKey('a'))
	for _, value := range "notes.txt" {
		model = updateFileMenuWithKeys(model, runeKey(value))
	}

	if model.fileMenuMode != fileMenuInsertMode {
		t.Fatalf("file menu mode = %v, want insert", model.fileMenuMode)
	}
	if view := model.View(); !strings.Contains(view, "notes.txt█") {
		t.Fatalf("new entry is not being edited inline: %q", view)
	}
	if len(model.fileMenuOperations) != 0 {
		t.Fatalf("typing applied an operation before leaving insert mode: %#v", model.fileMenuOperations)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEsc})

	if model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("file menu mode after escape = %v, want normal", model.fileMenuMode)
	}
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuCreateFile || model.fileMenuOperations[0].destination != "empty/notes.txt" {
		t.Fatalf("pending operations = %#v, want create empty/notes.txt", model.fileMenuOperations)
	}
	if view := model.View(); !strings.Contains(view, "notes.txt") || !strings.Contains(view, "pending change") {
		t.Fatalf("new buffered entry disappeared after escape: %q", view)
	}
}

func TestFileMenuInsertModeHasMovableTextCursor(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('a'))
	for _, value := range "not.txt" {
		model = updateFileMenuWithKeys(model, runeKey(value))
	}
	model = updateFileMenuWithKeys(model,
		tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyLeft},
		runeKey('e'),
	)

	if model.fileMenuInput != "note.txt" || model.fileMenuInputCursor != 4 {
		t.Fatalf("input = %q cursor = %d, want note.txt cursor 4", model.fileMenuInput, model.fileMenuInputCursor)
	}
	if view := model.View(); !strings.Contains(view, "note█.txt") {
		t.Fatalf("movable text cursor is not rendered inline: %q", view)
	}
}

func TestFileMenuDoesNotCopyDirectoryInsideItself(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":      {{name: "assets", kind: fileMenuDirectory}},
		"assets": {},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('y'), runeKey('y'), tea.KeyMsg{Type: tea.KeyEnter}, runeKey('p'))

	if len(model.fileMenuOperations) != 0 {
		t.Fatalf("staged operations = %#v, want no recursive self-copy", model.fileMenuOperations)
	}
	if !strings.Contains(model.fileMenuStatus, "inside itself") {
		t.Fatalf("status = %q, want self-copy explanation", model.fileMenuStatus)
	}
}

func TestFileMenuInsertModeRenamesSelectedEntry(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "note", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('i'), runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter})

	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("rename operations = %#v, want one", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuMove || operation.source != "note" || operation.destination != "note.txt" {
		t.Fatalf("rename operation = %#v", operation)
	}
}

func TestClassifyFileMenuEntry(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		isDir    bool
		mode     fs.FileMode
		wantKind fileMenuEntryKind
	}{
		{name: "directory", fileName: "assets", isDir: true, mode: 0755, wantKind: fileMenuDirectory},
		{name: "executable", fileName: "run-tool", mode: 0755, wantKind: fileMenuExecutable},
		{name: "image case insensitive", fileName: "preview.PNG", mode: 0644, wantKind: fileMenuImage},
		{name: "regular file", fileName: "notes.txt", mode: 0644, wantKind: fileMenuRegularFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyFileMenuEntry(tt.fileName, tt.isDir, tt.mode); got != tt.wantKind {
				t.Fatalf("classifyFileMenuEntry(%q) = %v, want %v", tt.fileName, got, tt.wantKind)
			}
		})
	}
}

func TestEnvPageShowsKeyValueAndStatusAndTogglesOnEnter(t *testing.T) {
	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "secret-value", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Env: true}
	selected := ""

	model := NewMultiPageViewModel(config, features, envtab.PageName)
	model.selected = &selected

	if model.currentPage != EnvPage {
		t.Fatalf("current page = %v, want EnvPage", model.currentPage)
	}
	if got := model.envList[0]; got.T != "FOO" || got.D != "secret-value" || got.Status != envtab.ActiveState {
		t.Fatalf("env item = %#v, want FOO value and active state", got)
	}
	if view := model.View(); !strings.Contains(view, "secret-value") || !strings.Contains(view, "active ✓") {
		t.Fatalf("env view does not show value and status: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != envtab.PageName+"|FOO|secret-value" {
		t.Fatalf("selected = %q, want env toggle result", selected)
	}
}

func TestEnvValueUsesGoToValueColorAndSeparateStatusColor(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "123", Active: true},
			},
		},
	}
	model := NewMultiPageViewModel(config, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	model.cursor = 1

	row := model.renderListItem(model.envList[0], 0)
	valueStyle := lipgloss.NewStyle().Foreground(model.styles.FooterColor).Render("123")
	statusStyle := lipgloss.NewStyle().Foreground(model.styles.SettingsEnabledColor).Render("active ✓")
	statusColoredValue := lipgloss.NewStyle().Foreground(model.styles.SettingsEnabledColor).Render("123")

	if !strings.Contains(row, valueStyle) {
		t.Fatalf("env value does not use goTo value color: %q", row)
	}
	if !strings.Contains(row, statusStyle) {
		t.Fatalf("env active status does not use enabled color: %q", row)
	}
	if strings.Contains(row, statusColoredValue) {
		t.Fatalf("env value incorrectly uses status color: %q", row)
	}
}

func TestEnvStatusesUseFixedColumn(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"SHORT", "LONG"},
			Values: map[string]utils.EnvValue{
				"SHORT": {Value: "1", Active: true},
				"LONG":  {Value: "a much longer value", Active: false},
			},
		},
	}
	model := NewMultiPageViewModel(config, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	model.cursor = len(model.envList)

	activeRow := model.renderListItem(model.envList[0], 0)
	inactiveRow := model.renderListItem(model.envList[1], 1)
	activeColumn := strings.Index(activeRow, "active ✓")
	inactiveColumn := strings.Index(inactiveRow, "inactive ✗")

	if activeColumn < 0 || inactiveColumn < 0 {
		t.Fatalf("status missing from rows: active=%q inactive=%q", activeRow, inactiveRow)
	}
	if activeColumn != inactiveColumn {
		t.Fatalf("status columns differ: active=%d inactive=%d", activeColumn, inactiveColumn)
	}

	valueAreaWidth := model.contentWidth() - model.titleColumnWidth() - rowChromeWidth
	if got := model.toggleValueColumnWidth(valueAreaWidth); got != toggleValueWidth {
		t.Fatalf("value column width = %d, want standard width %d", got, toggleValueWidth)
	}
}

func TestEnvAndAliasStatusesUseSameColumn(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "short", Active: true},
			},
		},
		Aliases: utils.OrderedAliasMap{
			Keys: []string{"long_alias_name"},
			Values: map[string]utils.AliasValue{
				"long_alias_name": {Value: "a much longer command", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Env: true, Alias: true}
	envModel := NewMultiPageViewModel(config, features, envtab.PageName)
	aliasModel := NewMultiPageViewModel(config, features, aliastab.PageName)
	envModel.cursor = len(envModel.envList)
	aliasModel.cursor = len(aliasModel.aliasList)

	envColumn := strings.Index(envModel.renderListItem(envModel.envList[0], 0), "active ✓")
	aliasColumn := strings.Index(aliasModel.renderListItem(aliasModel.aliasList[0], 0), "active ✓")

	if envColumn < 0 || aliasColumn < 0 {
		t.Fatalf("status missing from rows: env=%d alias=%d", envColumn, aliasColumn)
	}
	if envColumn != aliasColumn {
		t.Fatalf("status columns differ: env=%d alias=%d", envColumn, aliasColumn)
	}
}

func TestEnvPageAddShortcutReturnsAddAction(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	selected := ""
	model.selected = &selected

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if selected != envtab.PageName+"|"+envtab.AddAction+"|" {
		t.Fatalf("selected = %q, want env add action", selected)
	}
}

func TestEnvPageIsHiddenWhenFeatureIsDisabled(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Env: false})

	for _, page := range model.availPages {
		if page == EnvPage {
			t.Fatal("EnvPage should not be available when env feature is disabled")
		}
	}
}

func TestAliasPageShowsNameCommandAndStatusAndTogglesOnEnter(t *testing.T) {
	config := &utils.ConfigDTO{
		Aliases: utils.OrderedAliasMap{
			Keys: []string{"cat"},
			Values: map[string]utils.AliasValue{
				"cat": {Value: "bat --plain", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Alias: true}
	selected := ""

	model := NewMultiPageViewModel(config, features, aliastab.PageName)
	model.selected = &selected

	if model.currentPage != AliasPage {
		t.Fatalf("current page = %v, want AliasPage", model.currentPage)
	}
	if got := model.aliasList[0]; got.T != "cat" || got.D != "bat --plain" || got.Status != aliastab.ActiveState {
		t.Fatalf("alias item = %#v, want cat command and active state", got)
	}
	if view := model.View(); !strings.Contains(view, "bat --plain") || !strings.Contains(view, "active ✓") {
		t.Fatalf("alias view does not show command and status: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != aliastab.PageName+"|cat|bat --plain" {
		t.Fatalf("selected = %q, want alias toggle result", selected)
	}
}

func TestAliasPageAddShortcutReturnsAddAction(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Alias: true}, aliastab.PageName)
	selected := ""
	model.selected = &selected

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if selected != aliastab.PageName+"|"+aliastab.AddAction+"|" {
		t.Fatalf("selected = %q, want alias add action", selected)
	}
}

func TestAliasPageIsHiddenWhenFeatureIsDisabled(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Alias: false})

	for _, page := range model.availPages {
		if page == AliasPage {
			t.Fatal("AliasPage should not be available when alias feature is disabled")
		}
	}
}

func TestClipboardPageShowsValueOnlyAndCopiesOnEnter(t *testing.T) {
	config := &utils.ConfigDTO{
		Clipboard: utils.OrderedMap{
			Keys: []string{"clip42"},
			Values: map[string]string{
				"clip42": "secret-token",
			},
		},
	}
	features := &settings.FeaturesDTO{Clipboard: true}
	selected := ""

	model := NewMultiPageViewModel(config, features, clipboard.PageName)
	model.selected = &selected

	if model.currentPage != ClipboardPage {
		t.Fatalf("current page = %v, want ClipboardPage", model.currentPage)
	}
	if got := model.clipboardList[0]; got.T != "clip42" || got.D != "secret-token" {
		t.Fatalf("clipboard item = %#v, want hidden key and value", got)
	}
	view := model.View()
	if !strings.Contains(view, "secret-token") {
		t.Fatalf("clipboard view does not show value: %q", view)
	}
	if strings.Contains(view, "clip42") {
		t.Fatalf("clipboard view should not expose internal key: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != clipboard.PageName+"|clip42|secret-token" {
		t.Fatalf("selected = %q, want clipboard copy result", selected)
	}
}

func TestClipboardPageAddShortcutReturnsAddAction(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Clipboard: true}, clipboard.PageName)
	selected := ""
	model.selected = &selected

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if selected != clipboard.PageName+"|"+clipboard.AddAction+"|" {
		t.Fatalf("selected = %q, want clipboard add action", selected)
	}
}

func TestClipboardPageDeleteShortcutReturnsDeleteActionWithHiddenKey(t *testing.T) {
	config := &utils.ConfigDTO{
		Clipboard: utils.OrderedMap{
			Keys:   []string{"clip42"},
			Values: map[string]string{"clip42": "secret-token"},
		},
	}
	model := NewMultiPageViewModel(config, &settings.FeaturesDTO{Clipboard: true}, clipboard.PageName)
	selected := ""
	model.selected = &selected

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = updated.(MultiPageViewModel)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if selected != clipboard.PageName+"|"+clipboard.DeleteAction+"|clip42" {
		t.Fatalf("selected = %q, want clipboard delete action", selected)
	}
}

func TestClipboardPageIsHiddenWhenFeatureIsDisabled(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Clipboard: false})

	for _, page := range model.availPages {
		if page == ClipboardPage {
			t.Fatal("ClipboardPage should not be available when clipboard feature is disabled")
		}
	}
}
