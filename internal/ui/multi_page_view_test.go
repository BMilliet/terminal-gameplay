package ui

import (
	"io/fs"
	"strings"
	"testing"

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
		if strings.Contains(line, "notes.txt") && strings.Contains(line, "▌") {
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
