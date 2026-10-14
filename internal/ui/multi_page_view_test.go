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
	homeDirectory          string
	workingDirectory       string
	errorsByPath           map[string]error
	previewEntriesByPath   map[string][]fileMenuEntry
	previewTruncatedByPath map[string]bool
	fileContentsByPath     map[string][]byte
	fileTruncatedByPath    map[string]bool
	metadataByPath         map[string]fileMenuMetadata
	trashDirectory         string
	trashDirectoryErr      error
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
	batches  [][]fileMenuOperation
	result   fileMenuApplyResult
	progress []fileMenuOperationProgress
}

func (f *fakeFileMenuOperator) Apply(operations []fileMenuOperation, report func(fileMenuOperationProgress)) fileMenuApplyResult {
	batch := append([]fileMenuOperation(nil), operations...)
	f.batches = append(f.batches, batch)
	for _, progress := range f.progress {
		if report != nil {
			report(progress)
		}
	}
	result := f.result
	if result.err == nil && result.completed == 0 {
		result.completed = len(operations)
	}
	return result
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

func (f *fakeCurrentDirectoryReader) HomeDirectory() (string, error) {
	if f.homeDirectory == "" {
		return "/Users/test", nil
	}
	return f.homeDirectory, nil
}

func (f *fakeCurrentDirectoryReader) WorkingDirectory() (string, error) {
	if f.workingDirectory == "" {
		return "/workspace/project", nil
	}
	return f.workingDirectory, nil
}

func (f *fakeCurrentDirectoryReader) TrashDirectory() (string, error) {
	if f.trashDirectoryErr != nil {
		return "", f.trashDirectoryErr
	}
	if f.trashDirectory == "" {
		return "/Users/test/.Trash", nil
	}
	return f.trashDirectory, nil
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

func TestFileMenuShowsPlacesTreeAndPreviewPanels(t *testing.T) {
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
	}, homeDirectory: "/Users/test", workingDirectory: "/workspace/project"}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.terminalWidth = 100

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(MultiPageViewModel)
	view := model.View()

	threeBoxesOnOneLine := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╭") == 3 {
			threeBoxesOnOneLine = true
			break
		}
	}
	if !threeBoxesOnOneLine {
		t.Fatalf("file menu does not render places, tree, and preview side by side: %q", view)
	}
	for _, label := range []string{"Pinned", "project", "Home", "Downloads", "Desktop", "Documents", "Trash", "♻"} {
		if !strings.Contains(view, label) {
			t.Errorf("places panel does not contain %q: %q", label, view)
		}
	}
	if strings.Contains(view, "Disks") {
		t.Fatalf("places panel still contains the removed Disks section: %q", view)
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
		if label == "assets/" {
			coloredLabel = lipgloss.NewStyle().Foreground(color).Reverse(true).Render("a")
		}
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

func TestFileMenuTrashDeletionRequiresConfirmationAndBecomesPermanent(t *testing.T) {
	trashDirectory := "/Users/test/.Trash"
	reader := &fakeCurrentDirectoryReader{
		homeDirectory:  "/Users/test",
		trashDirectory: trashDirectory,
		entriesByPath: map[string][]fileMenuEntry{
			".":            {{name: "project", kind: fileMenuDirectory}},
			trashDirectory: {{name: "obsolete.bin", kind: fileMenuRegularFile}},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(
		model,
		runeKey('f'),
		runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft},
		tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyEnter},
	)

	if model.fileMenuPath != trashDirectory || model.fileMenuRoot != trashDirectory {
		t.Fatalf("path = %q, root = %q; want Trash rooted at %q", model.fileMenuPath, model.fileMenuRoot, trashDirectory)
	}
	if !model.isFileMenuProtectedPath(trashDirectory) {
		t.Fatalf("Trash shortcut %q is not protected", trashDirectory)
	}
	if len(model.fileMenuEntries) != 1 || model.fileMenuEntries[0].name != "obsolete.bin" {
		t.Fatalf("Trash entries = %#v, want obsolete.bin", model.fileMenuEntries)
	}

	model = updateFileMenuWithKeys(model, runeKey('d'), runeKey('d'))
	if model.fileMenuMode != fileMenuNormalMode || len(model.fileMenuOperations) != 1 || !model.fileMenuOperations[0].permanent {
		t.Fatalf("Trash deletion was not staged permanently: mode=%v operations=%#v", model.fileMenuMode, model.fileMenuOperations)
	}
	model = updateFileMenuWithKeys(model, runeKey(':'), runeKey('w'), tea.KeyMsg{Type: tea.KeyEnter})
	if model.fileMenuMode != fileMenuConfirmDeleteMode {
		t.Fatalf("Trash write did not open confirmation: mode=%v", model.fileMenuMode)
	}
	confirmation := model.View()
	for _, text := range []string{"Delete permanently?", "obsolete.bin", "yes", "no", "cannot be undone"} {
		if !strings.Contains(confirmation, text) {
			t.Errorf("confirmation popup does not contain %q: %q", text, confirmation)
		}
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.fileMenuMode != fileMenuNormalMode || len(model.fileMenuOperations) != 1 {
		t.Fatalf("cancelled Trash deletion changed state: mode=%v operations=%#v", model.fileMenuMode, model.fileMenuOperations)
	}

	model = updateFileMenuWithKeys(model, runeKey(':'), runeKey('w'))
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	if cmd != nil || model.fileMenuMode != fileMenuConfirmDeleteMode {
		t.Fatalf("confirmed write preparation mode=%v cmd=%v, want confirmation without filesystem command", model.fileMenuMode, cmd)
	}
	updated, cmd = model.Update(runeKey('y'))
	model = updated.(MultiPageViewModel)
	if cmd == nil || model.fileMenuMode != fileMenuProcessingMode {
		t.Fatalf("confirmed Trash write mode=%v cmd=%v, want processing command", model.fileMenuMode, cmd)
	}
	if len(model.fileMenuOperations) != 1 || !model.fileMenuOperations[0].permanent || model.fileMenuOperations[0].source != "/Users/test/.Trash/obsolete.bin" {
		t.Fatalf("Trash content operation = %#v, want permanent deletion", model.fileMenuOperations)
	}
}

func TestFileMenuTrashVisualDeletionStagesPermanentOperationsUntilWrite(t *testing.T) {
	trashDirectory := "/Users/test/.Trash"
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentMenu = FileMenu
	model.fileMenuPath = trashDirectory
	model.fileMenuRoot = trashDirectory
	model.fileMenuTrashPath = trashDirectory
	model.fileMenuEntries = []fileMenuEntry{
		{name: "one.txt", kind: fileMenuRegularFile},
		{name: "two.txt", kind: fileMenuRegularFile},
	}
	model.fileMenuMode = fileMenuVisualMode
	model.fileMenuVisualAnchor = 0
	model.fileMenuCursor = 1

	updated, _ := model.Update(runeKey('d'))
	model = updated.(MultiPageViewModel)
	if model.fileMenuMode != fileMenuNormalMode || len(model.fileMenuOperations) != 2 {
		t.Fatalf("visual Trash deletion mode=%v operations=%#v, want two staged operations", model.fileMenuMode, model.fileMenuOperations)
	}
	for _, operation := range model.fileMenuOperations {
		if !operation.permanent {
			t.Fatalf("visual Trash deletion operation = %#v, want permanent", operation)
		}
	}
}

func TestFileMenuLeaderArrowsFocusPlacesAndOpenHomeLocation(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		homeDirectory: "/Users/test",
		entriesByPath: map[string][]fileMenuEntry{
			".": {{name: "project", kind: fileMenuDirectory}},
			"/Users/test/Downloads": {
				{name: "archive.zip", kind: fileMenuRegularFile},
				{name: "notes.txt", kind: fileMenuRegularFile},
			},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft})
	if view := model.View(); !strings.Contains(view, "-- PLACES --") {
		t.Fatalf("space+left did not visibly focus the places panel: %q", view)
	}
	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter})

	if model.fileMenuPath != "/Users/test/Downloads" {
		t.Fatalf("file menu path = %q, want Downloads opened from places", model.fileMenuPath)
	}
	if view := model.View(); !strings.Contains(view, "archive.zip") {
		t.Fatalf("file tree does not show the selected Home location: %q", view)
	}
	model = updateFileMenuWithKeys(model, runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft}, runeKey(' '), tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyDown})
	if model.fileMenuCursor != 1 {
		t.Fatalf("space+right did not return focus to the file tree: cursor=%d", model.fileMenuCursor)
	}
}

func TestFileMenuPlacesOnlyAllowsNavigation(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {{name: "project", kind: fileMenuDirectory}},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuClipboard = []fileMenuClipboardEntry{{path: "project", kind: fileMenuDirectory}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft})
	for _, key := range []rune{'d', 'd', 'o', 'i', 'a', 'p', 'v', 'V'} {
		model = updateFileMenuWithKeys(model, runeKey(key))
	}

	if model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("places mode = %v, want normal mode after mutation keys", model.fileMenuMode)
	}
	if len(model.fileMenuOperations) != 0 {
		t.Fatalf("places staged operations = %#v, want none", model.fileMenuOperations)
	}
}

func TestFileMenuHomePlacesProtectRootButAllowContentChanges(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		homeDirectory: "/Users/test",
		entriesByPath: map[string][]fileMenuEntry{
			".":                     {{name: "project", kind: fileMenuDirectory}},
			"/Users/test/Downloads": {{name: "folder", kind: fileMenuDirectory}, {name: "notes.txt", kind: fileMenuRegularFile}},
			"/Users/test/Downloads/folder": {
				{name: "nested.txt", kind: fileMenuRegularFile},
			},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuClipboard = []fileMenuClipboardEntry{{path: "project", kind: fileMenuDirectory}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter})
	if model.fileMenuPath != "/Users/test/Downloads" || model.fileMenuRoot != "/Users/test/Downloads" {
		t.Fatalf("path = %q, root = %q; want Downloads anchored as the tree root", model.fileMenuPath, model.fileMenuRoot)
	}
	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyLeft})
	if model.fileMenuPath != "/Users/test/Downloads" {
		t.Fatalf("left navigated above protected root to %q", model.fileMenuPath)
	}

	model = updateFileMenuWithKeys(model, runeKey('d'), runeKey('d'))
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].source != "/Users/test/Downloads/folder" {
		t.Fatalf("deleting Home content staged %#v, want child folder deletion", model.fileMenuOperations)
	}
	model = updateFileMenuWithKeys(model, runeKey('o'))
	if model.fileMenuMode != fileMenuInsertMode {
		t.Fatalf("creating Home content left mode %v, want insert", model.fileMenuMode)
	}
}

func TestFileMenuPinnedWorkingDirectoryRestoresWritableTree(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		homeDirectory:    "/Users/test",
		workingDirectory: "/workspace/terminal-gameplay",
		entriesByPath: map[string][]fileMenuEntry{
			".":                            {{name: "project.txt", kind: fileMenuRegularFile}},
			"/Users/test/Downloads":        {{name: "download.txt", kind: fileMenuRegularFile}},
			"/workspace/terminal-gameplay": {{name: "main.go", kind: fileMenuRegularFile}},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(
		model,
		runeKey('f'),
		runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft},
		tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter},
		runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft},
		tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyEnter},
	)

	if model.fileMenuPath != "/workspace/terminal-gameplay" {
		t.Fatalf("file menu path = %q, want pinned working directory", model.fileMenuPath)
	}
	model = updateFileMenuWithKeys(model, runeKey('o'))
	if model.fileMenuMode != fileMenuInsertMode {
		t.Fatalf("pinned working directory mode after o = %v, want insert", model.fileMenuMode)
	}
}

func TestFileMenuPinnedWorkingDirectoryInsideHomeShortcutAllowsContentChanges(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		homeDirectory:    "/Users/test",
		workingDirectory: "/Users/test/Downloads/project",
		entriesByPath: map[string][]fileMenuEntry{
			".":                             {{name: "main.go", kind: fileMenuRegularFile}},
			"/Users/test/Downloads/project": {{name: "main.go", kind: fileMenuRegularFile}},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(
		model,
		runeKey('f'),
		runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyEnter},
		runeKey('o'),
	)

	if model.fileMenuMode != fileMenuInsertMode {
		t.Fatalf("create inside pinned Home directory left mode %v, want insert", model.fileMenuMode)
	}
}

func TestFileMenuHomeDirectoriesCannotBeChangedThroughPinnedParent(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		homeDirectory:    "/Users/test",
		workingDirectory: "/Users/test",
		entriesByPath: map[string][]fileMenuEntry{
			".": {},
			"/Users/test": {
				{name: "Desktop", kind: fileMenuDirectory},
				{name: "notes.txt", kind: fileMenuRegularFile},
			},
		},
	}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader

	model = updateFileMenuWithKeys(
		model,
		runeKey('f'),
		runeKey(' '), tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyEnter},
		runeKey('d'), runeKey('d'),
	)
	if len(model.fileMenuOperations) != 0 || !strings.Contains(model.fileMenuStatus, "protected") {
		t.Fatalf("deleting Desktop staged %#v with status %q", model.fileMenuOperations, model.fileMenuStatus)
	}
	model = updateFileMenuWithKeys(model, runeKey('m'), runeKey('m'))
	if len(model.fileMenuClipboard) != 0 || !strings.Contains(model.fileMenuStatus, "protected") {
		t.Fatalf("moving Desktop populated clipboard %#v with status %q", model.fileMenuClipboard, model.fileMenuStatus)
	}

	model = updateFileMenuWithKeys(model, runeKey('i'))
	if model.fileMenuMode != fileMenuNormalMode || !strings.Contains(model.fileMenuStatus, "protected") {
		t.Fatalf("renaming Desktop left mode %v with status %q", model.fileMenuMode, model.fileMenuStatus)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyDown}, runeKey('d'), runeKey('d'))
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].source != "/Users/test/notes.txt" {
		t.Fatalf("deleting sibling content staged %#v, want notes.txt deletion", model.fileMenuOperations)
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
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

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
	_, _, cursorColor := model.fileMenuEntryPresentation(model.fileMenuEntries[model.fileMenuCursor])
	wantCursor := lipgloss.NewStyle().
		Foreground(cursorColor).
		Reverse(true).
		Render("n")
	if view := model.View(); !strings.Contains(view, wantCursor) {
		t.Fatalf("selected file name does not contain a Vim-style block cursor: %q", view)
	}
}

func TestFileMenuEscapeReturnsToNormalModeWithCursorOnLastInsertedCharacter(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('l'), runeKey('l'), runeKey('i'), runeKey('X'), tea.KeyMsg{Type: tea.KeyEsc},
	)

	if model.fileMenuMode != fileMenuNormalMode || model.fileMenuNameCursor != 2 {
		t.Fatalf("mode = %v cursor = %d, want normal mode with cursor 2", model.fileMenuMode, model.fileMenuNameCursor)
	}
	_, _, cursorColor := model.fileMenuEntryPresentation(model.fileMenuEntries[model.fileMenuCursor])
	wantCursor := lipgloss.NewStyle().
		Foreground(cursorColor).
		Reverse(true).
		Render("X")
	if tree := model.renderFileTree(); !strings.Contains(tree, wantCursor) {
		t.Fatalf("normal-mode cursor is not rendered on the last inserted character: %q", tree)
	}
}

func TestFileMenuScrollKeepsFiveRowsVisibleAheadOfCursor(t *testing.T) {
	entries := make([]fileMenuEntry, 40)
	for index := range entries {
		entries[index] = fileMenuEntry{name: fmt.Sprintf("file-%02d.txt", index), kind: fileMenuRegularFile}
	}
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": entries}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model = updateFileMenuWithKeys(model, runeKey('f'))

	visibleEntries := model.fileMenuTopContentHeight() - 1
	for range visibleEntries - fileMenuScrollOffset {
		model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyDown})
	}

	view := model.View()
	if strings.Contains(view, "file-00.txt") {
		t.Fatalf("file tree did not start scrolling with %d rows below the cursor: %q", fileMenuScrollOffset, view)
	}
	for index := model.fileMenuCursor + 1; index <= model.fileMenuCursor+fileMenuScrollOffset; index++ {
		if name := fmt.Sprintf("file-%02d.txt", index); !strings.Contains(view, name) {
			t.Fatalf("file tree does not keep %d rows visible below the cursor; missing %q: %q", fileMenuScrollOffset, name, view)
		}
	}
}

func TestFileMenuSupportsVimTopAndBottomNavigation(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "one.txt", kind: fileMenuRegularFile},
			{name: "two.txt", kind: fileMenuRegularFile},
			{name: "three.txt", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model = updateFileMenuWithKeys(model, runeKey('f'))

	model = updateFileMenuWithKeys(model, runeKey('G'))
	if model.fileMenuCursor != len(model.fileMenuEntries)-1 {
		t.Fatalf("file menu cursor after G = %d, want %d", model.fileMenuCursor, len(model.fileMenuEntries)-1)
	}

	model = updateFileMenuWithKeys(model, runeKey('g'))
	if model.fileMenuCursor != len(model.fileMenuEntries)-1 {
		t.Fatalf("file menu cursor after first g = %d, want it unchanged", model.fileMenuCursor)
	}
	model = updateFileMenuWithKeys(model, runeKey('g'))
	if model.fileMenuCursor != 0 {
		t.Fatalf("file menu cursor after gg = %d, want 0", model.fileMenuCursor)
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
	}, fileContentsByPath: map[string][]byte{"notes.txt": []byte("plain text\n")}}
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

func TestFileMenuEnterOnBinaryFileShowsWarningWithoutOpeningNvim(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{
		entriesByPath: map[string][]fileMenuEntry{
			".": {{name: "document.pdf", kind: fileMenuRegularFile}},
		},
		fileContentsByPath: map[string][]byte{
			"document.pdf": []byte("%PDF-1.7\n1 0 obj\n"),
		},
	}
	selected := ""
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.selected = &selected

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyEnter})

	if selected != "" || model.quitting {
		t.Fatalf("binary file opened nvim: selected=%q quitting=%v", selected, model.quitting)
	}
	if !strings.Contains(model.fileMenuStatus, "only text files") {
		t.Fatalf("binary file warning = %q, want text-only explanation", model.fileMenuStatus)
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

func TestFileMenuWriteAppliesStagedChangesWithoutExtraConfirmation(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}
	operator := &fakeFileMenuOperator{}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuOperator = operator

	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('o'), runeKey('n'), runeKey('e'), runeKey('w'), runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter},
		runeKey(':'), runeKey('w'),
	)

	if len(operator.batches) != 0 {
		t.Fatalf("operations were applied before :w was submitted: %#v", operator.batches)
	}
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuCreateFile || model.fileMenuOperations[0].destination != "new.txt" {
		t.Fatalf("staged operations = %#v, want create new.txt", model.fileMenuOperations)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	if model.fileMenuMode != fileMenuProcessingMode || cmd == nil {
		t.Fatalf(":w did not start background processing directly: mode=%v cmd=%v", model.fileMenuMode, cmd)
	}
	if view := model.View(); !strings.Contains(view, "Creating new.txt") {
		t.Fatalf("process panel does not show active work: %q", view)
	}
	finished := cmd()
	reader.entriesByPath["."] = []fileMenuEntry{{name: "new.txt", kind: fileMenuRegularFile}}
	updated, _ = model.Update(finished)
	model = updated.(MultiPageViewModel)

	if len(operator.batches) != 1 || len(operator.batches[0]) != 1 {
		t.Fatalf("applied batches = %#v, want one operation", operator.batches)
	}
	if len(model.fileMenuOperations) != 0 || model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("successful save did not clear pending changes: %#v mode=%v", model.fileMenuOperations, model.fileMenuMode)
	}
	if len(model.fileMenuEntries) != 1 || model.fileMenuEntries[0].name != "new.txt" || model.fileMenuEntries[0].pending {
		t.Fatalf("saved entry did not return to its normal state: %#v", model.fileMenuEntries)
	}
	addedName := lipgloss.NewStyle().Foreground(model.styles.FileMenuAddedColor).Reverse(true).Render("n")
	if view := model.View(); !strings.Contains(view, addedName) || !strings.Contains(view, "A ") {
		t.Fatalf("saved entry does not show its current-session added state: %q", view)
	}
}

func TestFileMenuWriteRendersSPFStyleProgressUpdates(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	operator := &fakeFileMenuOperator{progress: []fileMenuOperationProgress{{
		kind:        fileMenuCopy,
		currentFile: "archive.tar",
		doneItems:   1,
		totalItems:  4,
	}}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}
	model.fileMenuOperator = operator
	model.currentMenu = FileMenu
	model.fileMenuOperations = []fileMenuOperation{{kind: fileMenuCopy, source: "archive.tar", destination: "archive-copy.tar"}}
	model.fileMenuMode = fileMenuCommandMode
	model.fileMenuCommand = "w"

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	if cmd == nil {
		t.Fatal(":w command = nil, want progress command")
	}
	updated, next := model.Update(cmd())
	model = updated.(MultiPageViewModel)
	if next == nil {
		t.Fatal("progress update did not schedule the next operation message")
	}
	view := model.renderFileMenuProcessPanel(34)
	for _, want := range []string{"Processes", "Copying archive.tar", "1/4", "25%", "█", "░", "󰥔"} {
		if !strings.Contains(view, want) {
			t.Errorf("process panel does not contain %q: %q", want, view)
		}
	}

	updated, _ = model.Update(next())
	model = updated.(MultiPageViewModel)
	if model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("file menu mode = %v, want normal after completion", model.fileMenuMode)
	}
}

func TestFileMenuFailureRemovesCompletedOperationsAndKeepsRetryableRemainder(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "created.txt", kind: fileMenuRegularFile}},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuMode = fileMenuProcessingMode
	model.fileMenuOperations = []fileMenuOperation{
		{kind: fileMenuCreateFile, destination: "created.txt"},
		{kind: fileMenuCopy, source: "source.txt", destination: "copy.txt"},
		{kind: fileMenuDelete, source: "old.txt"},
	}

	model.finishFileMenuOperations(fileMenuApplyResult{completed: 1, err: fmt.Errorf("copy failed")})

	if len(model.fileMenuOperations) != 2 || model.fileMenuOperations[0].destination != "copy.txt" {
		t.Fatalf("pending operations = %#v, want only failed and remaining operations", model.fileMenuOperations)
	}
	if !strings.Contains(model.fileMenuStatus, "saved 1") || !strings.Contains(model.fileMenuStatus, "2 pending") {
		t.Fatalf("status = %q, want partial completion details", model.fileMenuStatus)
	}
	if change, ok := model.fileMenuSessionChanges["created.txt"]; !ok || change != fileMenuSessionAdded {
		t.Fatalf("completed creation session change = %v, %v; want added", change, ok)
	}
	if _, ok := model.fileMenuSessionChanges["copy.txt"]; ok {
		t.Fatal("failed copy was marked as changed in the current session")
	}
}

func TestFileMenuUppercaseDStagesPermanentDelete(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "obsolete.bin", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('D'))

	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuDelete || !model.fileMenuOperations[0].permanent {
		t.Fatalf("operations = %#v, want one permanent delete", model.fileMenuOperations)
	}
	if !strings.Contains(model.fileMenuStatus, "permanent") {
		t.Fatalf("status = %q, want permanent deletion warning", model.fileMenuStatus)
	}
}

func TestFileMenuXWritesWithoutConfirmationAndQuitsAfterSuccess(t *testing.T) {
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}
	operator := &fakeFileMenuOperator{}
	selected := ""
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuOperator = operator
	model.selected = &selected
	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('o'), runeKey('n'), runeKey('e'), runeKey('w'), tea.KeyMsg{Type: tea.KeyEnter},
		runeKey(':'), runeKey('x'),
	)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	if model.fileMenuMode != fileMenuProcessingMode || cmd == nil {
		t.Fatalf(":x did not start processing directly: mode=%v cmd=%v", model.fileMenuMode, cmd)
	}

	updated, quitCmd := model.Update(cmd())
	model = updated.(MultiPageViewModel)
	if !model.quitting || selected != utils.ExitSignal || quitCmd == nil {
		t.Fatalf(":x did not quit after saving: quitting=%v selected=%q cmd=%v", model.quitting, selected, quitCmd)
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

func TestFileMenuMemoryPanelKeepsFixedHeightWithLargeClipboard(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.terminalWidth = 120
	for index := range 30 {
		model.fileMenuClipboard = append(model.fileMenuClipboard, fileMenuClipboardEntry{
			path: fmt.Sprintf("/a/very/long/path/to/file-%02d.txt", index),
		})
	}

	panels := model.renderFileMenuBottomPanels()
	wantHeight := fileMenuBottomBoxHeight + 2
	if got := lipgloss.Height(panels); got != wantHeight {
		t.Fatalf("bottom panels height = %d, want fixed height %d:\n%s", got, wantHeight, panels)
	}
	if !strings.Contains(panels, "…") {
		t.Fatalf("large Memory panel does not show truncation marker: %q", panels)
	}
}

func TestFileMenuMarksFilesChangedDuringCurrentSessionAfterSave(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {
			{name: "created.txt", kind: fileMenuRegularFile},
			{name: "renamed.txt", kind: fileMenuRegularFile},
		},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuMode = fileMenuProcessingMode
	model.fileMenuOperations = []fileMenuOperation{
		{kind: fileMenuCreateFile, destination: "created.txt", entryKind: fileMenuRegularFile},
		{kind: fileMenuMove, source: "original.txt", destination: "renamed.txt", entryKind: fileMenuRegularFile},
	}

	model.finishFileMenuOperations(fileMenuApplyResult{completed: 2})
	tree := model.renderFileTree()
	for _, expectation := range []struct {
		name   string
		marker string
	}{
		{name: "created.txt", marker: "A "},
		{name: "renamed.txt", marker: "M "},
	} {
		var matchingLine string
		for _, line := range strings.Split(tree, "\n") {
			if strings.Contains(line, expectation.name) {
				matchingLine = line
				break
			}
		}
		if !strings.Contains(matchingLine, expectation.marker) {
			t.Errorf("tree line for %s = %q, want session marker %q", expectation.name, matchingLine, expectation.marker)
		}
	}
}

func TestFileMenuMarksTrashedFileAtItsTrashDestination(t *testing.T) {
	trashDirectory := "/Users/test/.Trash"
	reader := &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":            {},
		trashDirectory: {{name: "removed.txt", kind: fileMenuRegularFile}},
	}}
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = reader
	model.fileMenuMode = fileMenuProcessingMode
	model.fileMenuOperations = []fileMenuOperation{{kind: fileMenuDelete, source: "removed.txt"}}

	model.finishFileMenuOperations(fileMenuApplyResult{
		completed: 1,
		completedOperations: []fileMenuOperation{{
			kind:        fileMenuDelete,
			source:      "removed.txt",
			destination: trashDirectory + "/removed.txt",
		}},
	})
	if !model.loadFileMenuDirectory(trashDirectory) {
		t.Fatal("failed to open fake Trash directory")
	}

	var removedLine string
	for _, line := range strings.Split(model.renderFileTree(), "\n") {
		if strings.Contains(line, "removed.txt") {
			removedLine = line
			break
		}
	}
	if !strings.Contains(removedLine, "M ") {
		t.Fatalf("trashed file line = %q, want current-session moved marker", removedLine)
	}
}

func TestFileMenuDirectoryPasteEditsDestinationAndRejectsOriginalName(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "docs", kind: fileMenuDirectory}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('y'), runeKey('y'), runeKey('p'))
	if model.fileMenuMode != fileMenuInsertMode || model.fileMenuInput != "docs" {
		t.Fatalf("paste mode = %v input = %q, want directory name editing", model.fileMenuMode, model.fileMenuInput)
	}
	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("operations = %#v, want one directory copy", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuCopy || operation.source != "docs" || operation.destination != "docs" || operation.entryKind != fileMenuDirectory {
		t.Fatalf("copy operation = %#v, want recursive docs destination draft", operation)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.fileMenuMode != fileMenuNormalMode || !strings.Contains(model.fileMenuStatus, "new name") {
		t.Fatalf("unchanged duplicate draft did not leave a write-blocking warning: mode=%v status=%q", model.fileMenuMode, model.fileMenuStatus)
	}

	model = updateFileMenuWithKeys(model,
		runeKey('i'),
		tea.KeyMsg{Type: tea.KeyCtrlU},
		runeKey('d'), runeKey('o'), runeKey('c'), runeKey('s'), runeKey('-'), runeKey('c'), runeKey('o'), runeKey('p'), runeKey('y'),
		tea.KeyMsg{Type: tea.KeyEnter},
	)
	if model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("renamed paste mode = %v, want normal", model.fileMenuMode)
	}
	if got := model.fileMenuOperations[0].destination; got != "docs-copy" {
		t.Fatalf("copy destination = %q, want edited docs-copy", got)
	}
}

func TestFileMenuWriteRejectsDirectoryCopyWithUnchangedDestination(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentMenu = FileMenu
	model.fileMenuMode = fileMenuCommandMode
	model.fileMenuCommand = "w"
	model.fileMenuOperations = []fileMenuOperation{{
		kind:        fileMenuCopy,
		source:      "docs",
		destination: "docs",
		entryKind:   fileMenuDirectory,
	}}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(MultiPageViewModel)
	if cmd != nil || model.fileMenuMode != fileMenuNormalMode {
		t.Fatalf("invalid copy started writing: mode=%v cmd=%v", model.fileMenuMode, cmd)
	}
	if !strings.Contains(model.fileMenuStatus, "rename copied directory") {
		t.Fatalf("status = %q, want rename requirement", model.fileMenuStatus)
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

func TestFileMenuEditingPendingFileWithTrailingSlashConvertsItToDirectory(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}

	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('o'), runeKey('d'), runeKey('i'), runeKey('r'), tea.KeyMsg{Type: tea.KeyEsc},
		runeKey('A'), runeKey('/'), tea.KeyMsg{Type: tea.KeyEsc},
	)

	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("staged operations = %#v, want one converted creation", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuCreateDirectory || operation.destination != "dir" || operation.entryKind != fileMenuDirectory {
		t.Fatalf("converted operation = %#v, want create directory dir", operation)
	}
	if len(model.fileMenuEntries) != 1 || model.fileMenuEntries[0].kind != fileMenuDirectory {
		t.Fatalf("visible entries = %#v, want pending directory", model.fileMenuEntries)
	}
	if tree := model.renderFileTree(); !strings.Contains(tree, "dir/") {
		t.Fatalf("converted directory does not keep its trailing slash in the tree: %q", tree)
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

func TestFileMenuDDKeepsRedGhostInTreeAndPasteTurnsDeleteIntoMove(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

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
	if len(model.fileMenuEntries) != 2 || model.fileMenuEntries[1].name != "notes.txt" || !model.fileMenuEntries[1].pending {
		t.Fatalf("pending delete is not retained as a ghost entry: %#v", model.fileMenuEntries)
	}
	redName := lipgloss.NewStyle().Foreground(model.styles.FileMenuDeletedColor).Reverse(true).Render("n")
	if view := model.View(); !strings.Contains(view, redName) {
		t.Fatalf("pending delete is not rendered in red: %q", view)
	}

	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyEnter}, runeKey('p'))

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

func TestFileMenuPendingCopiesAreYellowAndNewEntriesAreGreen(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('y'), runeKey('y'), runeKey('p'))
	view := model.View()
	yellowName := lipgloss.NewStyle().Foreground(model.styles.FileMenuCopiedColor).Reverse(true).Render("n")
	if !strings.Contains(view, yellowName) {
		t.Fatalf("pending copy is not rendered in yellow: %q", view)
	}

	model = updateFileMenuWithKeys(model,
		runeKey('o'), runeKey('n'), runeKey('e'), runeKey('w'), runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter},
	)
	view = model.View()
	greenName := lipgloss.NewStyle().Foreground(model.styles.FileMenuAddedColor).Reverse(true).Render("t")
	if !strings.Contains(view, greenName) {
		t.Fatalf("pending new entry is not rendered in green: %q", view)
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

func TestFileMenuOEditsNewEntryInlineInsideEmptyDirectory(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":     {{name: "empty", kind: fileMenuDirectory}},
		"empty": {},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyEnter}, runeKey('o'))
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

func TestFileMenuEmptyDirectoryKeepsNormalCursorAndIStartsCreatingAFile(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".":     {{name: "empty", kind: fileMenuDirectory}},
		"empty": {},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), tea.KeyMsg{Type: tea.KeyEnter})
	wantCursor := lipgloss.NewStyle().
		Foreground(model.styles.FileMenuFileColor).
		Reverse(true).
		Render(" ")
	if tree := model.renderFileTree(); !strings.Contains(tree, wantCursor) {
		t.Fatalf("empty directory has no normal-mode cursor: %q", tree)
	}

	model = updateFileMenuWithKeys(model, runeKey('i'))
	if model.fileMenuMode != fileMenuInsertMode || model.fileMenuInputAction != fileMenuCreateInput {
		t.Fatalf("i in empty directory entered mode %v action %v, want create insert mode", model.fileMenuMode, model.fileMenuInputAction)
	}
	for _, value := range "notes.txt" {
		model = updateFileMenuWithKeys(model, runeKey(value))
	}
	model = updateFileMenuWithKeys(model, tea.KeyMsg{Type: tea.KeyEsc})

	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuCreateFile || model.fileMenuOperations[0].destination != "empty/notes.txt" {
		t.Fatalf("pending operations = %#v, want create empty/notes.txt", model.fileMenuOperations)
	}
}

func TestFileMenuInsertModeHasMovableTextCursor(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{".": {}}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('o'))
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

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('A'), runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter})

	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("rename operations = %#v, want one", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuMove || operation.source != "note" || operation.destination != "note.txt" {
		t.Fatalf("rename operation = %#v", operation)
	}
}

func TestFileMenuVimCursorControlsWhereInsertModeEditsName(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "notes.txt", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model,
		runeKey('f'), runeKey('l'), runeKey('l'), runeKey('l'), runeKey('l'), runeKey('i'),
	)
	if model.fileMenuMode != fileMenuInsertMode || model.fileMenuInputCursor != 4 {
		t.Fatalf("insert mode started at position %d in mode %v, want position 4", model.fileMenuInputCursor, model.fileMenuMode)
	}
	wantCursor := lipgloss.NewStyle().Foreground(model.styles.FileMenuFileColor).Reverse(true).Render("s")
	if tree := model.renderFileTree(); !strings.Contains(tree, wantCursor) {
		t.Fatalf("insert cursor is not rendered over the edited name: %q", tree)
	}

	model = updateFileMenuWithKeys(model, runeKey('X'), tea.KeyMsg{Type: tea.KeyEnter})
	if len(model.fileMenuOperations) != 1 {
		t.Fatalf("rename operations = %#v, want one", model.fileMenuOperations)
	}
	operation := model.fileMenuOperations[0]
	if operation.kind != fileMenuMove || operation.source != "notes.txt" || operation.destination != "noteXs.txt" {
		t.Fatalf("rename operation = %#v, want notes.txt → noteXs.txt", operation)
	}
}

func TestFileMenuVimAppendEditsSelectedNameAfterCursor(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{})
	model.currentDirectoryReader = &fakeCurrentDirectoryReader{entriesByPath: map[string][]fileMenuEntry{
		".": {{name: "note", kind: fileMenuRegularFile}},
	}}

	model = updateFileMenuWithKeys(model, runeKey('f'), runeKey('$'), runeKey('a'))
	if model.fileMenuMode != fileMenuInsertMode || model.fileMenuInputAction != fileMenuRenameInput || model.fileMenuInputCursor != 4 {
		t.Fatalf("append mode = %v action=%v cursor=%d, want rename at end", model.fileMenuMode, model.fileMenuInputAction, model.fileMenuInputCursor)
	}

	model = updateFileMenuWithKeys(model, runeKey('.'), runeKey('t'), runeKey('x'), runeKey('t'), tea.KeyMsg{Type: tea.KeyEnter})
	if len(model.fileMenuOperations) != 1 || model.fileMenuOperations[0].kind != fileMenuMove || model.fileMenuOperations[0].destination != "note.txt" {
		t.Fatalf("append rename operations = %#v, want note → note.txt", model.fileMenuOperations)
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
