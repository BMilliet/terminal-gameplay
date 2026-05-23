package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gototab "terminal-gameplay/internal/goto"
	"terminal-gameplay/internal/scripts"
	"terminal-gameplay/internal/settings"
	toolstab "terminal-gameplay/internal/tools"
	"terminal-gameplay/internal/utils"
)

type fakeFileManager struct {
	configContent         string
	featuresContent       string
	optionsContent        string
	goToFrequencyContent  string
	currentPath           string
	files                 map[string]string
	configWrites          []string
	featuresWrites        []string
	deleted               []string
	basicSetupCalls       int
	ensureNotesDirCalls   int
	ensureScriptsDirCalls int
}

func newFakeFileManager() *fakeFileManager {
	return &fakeFileManager{
		currentPath: "/home/test/work",
		files:       make(map[string]string),
	}
}

func (f *fakeFileManager) CheckIfPathExists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

func (f *fakeFileManager) ReadFileContent(filePath string) (string, error) {
	return f.files[filePath], nil
}

func (f *fakeFileManager) WriteFileContent(filePath, content string) error {
	f.files[filePath] = content
	return nil
}

func (f *fakeFileManager) DeleteFileIfExists(path string) error {
	f.deleted = append(f.deleted, path)
	delete(f.files, path)
	return nil
}

func (f *fakeFileManager) GetConfigContent() (string, error) {
	return f.configContent, nil
}

func (f *fakeFileManager) WriteConfigContent(content string) error {
	f.configWrites = append(f.configWrites, content)
	f.configContent = content
	return nil
}

func (f *fakeFileManager) GetFeaturesContent() (string, error) {
	return f.featuresContent, nil
}

func (f *fakeFileManager) WriteFeaturesContent(content string) error {
	f.featuresWrites = append(f.featuresWrites, content)
	f.featuresContent = content
	return nil
}

func (f *fakeFileManager) GetOptionsContent() (string, error) {
	return f.optionsContent, nil
}

func (f *fakeFileManager) WriteOptionsContent(content string) error {
	f.optionsContent = content
	return nil
}

func (f *fakeFileManager) GetGoToFrequencyContent() (string, error) {
	return f.goToFrequencyContent, nil
}

func (f *fakeFileManager) WriteGoToFrequencyContent(content string) error {
	f.goToFrequencyContent = content
	return nil
}

func (f *fakeFileManager) EnsureNotesDir() error {
	f.ensureNotesDirCalls++
	return nil
}

func (f *fakeFileManager) EnsureScriptsDir() error {
	f.ensureScriptsDirCalls++
	return nil
}

func (f *fakeFileManager) NotesPath(fileName string) string {
	return "notes/" + fileName
}

func (f *fakeFileManager) ScriptsPath(fileName string) string {
	return "scripts/" + fileName
}

func (f *fakeFileManager) CommandExecPath() string {
	return "cmd-exec"
}

func (f *fakeFileManager) BasicSetup() error {
	f.basicSetupCalls++
	return nil
}

func (f *fakeFileManager) GetCurrentPath() (string, error) {
	return f.currentPath, nil
}

func (f *fakeFileManager) GetCurrentDirectoryName() (string, error) {
	parts := strings.Split(strings.Trim(f.currentPath, "/"), "/")
	return parts[len(parts)-1], nil
}

type fakeRuntime struct {
	expanded        map[string]string
	contracted      map[string]string
	validatedInputs []string
	openedInNvim    []string
	ranLua          []string
	changedDirs     []string
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{
		expanded:   make(map[string]string),
		contracted: make(map[string]string),
	}
}

func (f *fakeRuntime) ValidateInput(input string) {
	f.validatedInputs = append(f.validatedInputs, input)
}

func (f *fakeRuntime) ExitWithError(message string) {
	panic(message)
}

func (f *fakeRuntime) HandleError(err error, message string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", message, err))
	}
}

func (f *fakeRuntime) ExpandPath(path string) string {
	if expanded, ok := f.expanded[path]; ok {
		return expanded
	}
	return path
}

func (f *fakeRuntime) ContractPath(path string) string {
	if contracted, ok := f.contracted[path]; ok {
		return contracted
	}
	return path
}

func (f *fakeRuntime) ExecuteCommand(command string) error {
	panic("ExecuteCommand should not be called by these unit tests")
}

func (f *fakeRuntime) CopyToClipboard(text string) error {
	panic("CopyToClipboard should not be called by these unit tests")
}

func (f *fakeRuntime) OpenInNvim(filePath string) error {
	f.openedInNvim = append(f.openedInNvim, filePath)
	return nil
}

func (f *fakeRuntime) RunLuaScript(filePath string) error {
	f.ranLua = append(f.ranLua, filePath)
	return nil
}

func (f *fakeRuntime) ChangeDirectory(path string) error {
	f.changedDirs = append(f.changedDirs, path)
	return nil
}

type fakeViewBuilder struct {
	multiPageResults         []string
	textResults              []string
	confirmResults           []bool
	sectionResults           []utils.ListItem
	searchReplaceFileResults []utils.ListItem
}

func (f *fakeViewBuilder) NewListView(title string, op []utils.ListItem, height int) utils.ListItem {
	return utils.ListItem{}
}

func (f *fakeViewBuilder) NewConfirmView(title string) bool {
	if len(f.confirmResults) == 0 {
		return false
	}

	result := f.confirmResults[0]
	f.confirmResults = f.confirmResults[1:]
	return result
}

func (f *fakeViewBuilder) NewSectionSelectView(title string, sections []utils.ListItem) utils.ListItem {
	if len(f.sectionResults) == 0 {
		return utils.ListItem{T: utils.ExitSignal}
	}

	result := f.sectionResults[0]
	f.sectionResults = f.sectionResults[1:]
	return result
}

func (f *fakeViewBuilder) NewTextFieldView(title, placeHolder string) string {
	if len(f.textResults) == 0 {
		return utils.ExitSignal
	}

	result := f.textResults[0]
	f.textResults = f.textResults[1:]
	return result
}

func (f *fakeViewBuilder) NewSearchReplaceFilesView(title string, files []utils.ListItem, height int) utils.ListItem {
	if len(f.searchReplaceFileResults) == 0 {
		return utils.ListItem{T: utils.ExitSignal}
	}

	result := f.searchReplaceFileResults[0]
	f.searchReplaceFileResults = f.searchReplaceFileResults[1:]
	return result
}

func (f *fakeViewBuilder) NewMultiPageView(config *utils.ConfigDTO, features *settings.FeaturesDTO, initialPage ...string) string {
	if len(f.multiPageResults) == 0 {
		return "invalid"
	}

	result := f.multiPageResults[0]
	f.multiPageResults = f.multiPageResults[1:]
	return result
}

func TestStartGoToWritesCommandAndFrequencyWithMocks(t *testing.T) {
	fm := newFakeFileManager()
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		GoTo: utils.OrderedMap{
			Keys: []string{"work"},
			Values: map[string]string{
				"work": "~/work",
			},
		},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: true,
		Scripts:      false,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	runtime.expanded["~/work"] = "/home/test/work"
	views := &fakeViewBuilder{
		multiPageResults: []string{gototab.PageName + "|work|~/work"},
	}

	NewRunner(fm, runtime, views).Start()

	if got := fm.files[fm.CommandExecPath()]; got != "cd /home/test/work" {
		t.Fatalf("command file = %q, want %q", got, "cd /home/test/work")
	}
	if len(runtime.openedInNvim) != 0 {
		t.Fatalf("OpenInNvim calls = %#v, want none", runtime.openedInNvim)
	}
	if len(runtime.ranLua) != 0 {
		t.Fatalf("RunLuaScript calls = %#v, want none", runtime.ranLua)
	}

	var savedFeatures settings.FeaturesDTO
	mustUnmarshalJSON(t, last(t, fm.featuresWrites), &savedFeatures)
	if got := savedFeatures.Frequencies["work"]; got != 1 {
		t.Fatalf("saved frequency = %d, want 1", got)
	}
}

func TestStartAddGoToUsesTextAndSectionMocks(t *testing.T) {
	fm := newFakeFileManager()
	fm.currentPath = "/home/test/work/api"
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		GoTo: utils.OrderedMap{
			Keys: []string{"home", "div", "backend"},
			Values: map[string]string{
				"home":    "~",
				"div":     "work",
				"backend": "~/work/backend",
			},
		},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      false,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	runtime.contracted["/home/test/work/api"] = "~/work/api"
	views := &fakeViewBuilder{
		multiPageResults: []string{gototab.PageName + "|" + gototab.AddAction + "|", "invalid"},
		textResults:      []string{"api"},
		sectionResults:   []utils.ListItem{{T: gototab.RootSection}},
	}

	NewRunner(fm, runtime, views).Start()

	var savedConfig utils.ConfigDTO
	mustUnmarshalJSON(t, last(t, fm.configWrites), &savedConfig)

	if got, ok := savedConfig.GoTo.Get("api"); !ok || got != "~/work/api" {
		t.Fatalf("saved goTo api = %q, %v; want ~/work/api, true", got, ok)
	}

	wantKeys := []string{"home", "api", "div", "backend"}
	if !reflect.DeepEqual(savedConfig.GoTo.Keys, wantKeys) {
		t.Fatalf("goTo keys = %#v, want %#v", savedConfig.GoTo.Keys, wantKeys)
	}
}

func TestStartAddScriptCreatesFileAndOpensEditorThroughMocks(t *testing.T) {
	fm := newFakeFileManager()
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		Scripts: utils.OrderedMap{Values: make(map[string]string)},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      true,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{scripts.PageName + "|" + scripts.AddAction + "|", "invalid"},
		textResults:      []string{"get current branch", "Print current branch"},
	}

	NewRunner(fm, runtime, views).Start()

	if got := runtime.openedInNvim; !reflect.DeepEqual(got, []string{"scripts/get_current_branch.lua"}) {
		t.Fatalf("OpenInNvim calls = %#v, want script path", got)
	}
	if len(runtime.ranLua) != 0 {
		t.Fatalf("RunLuaScript calls = %#v, want none", runtime.ranLua)
	}

	scriptContent := fm.files["scripts/get_current_branch.lua"]
	for _, want := range []string{"-- get current branch", "-- Print current branch"} {
		if !strings.Contains(scriptContent, want) {
			t.Fatalf("script content = %q, want to contain %q", scriptContent, want)
		}
	}

	var savedConfig utils.ConfigDTO
	mustUnmarshalJSON(t, last(t, fm.configWrites), &savedConfig)
	if got, ok := savedConfig.Scripts.Get("get current branch"); !ok || got != "Print current branch" {
		t.Fatalf("saved script description = %q, %v; want Print current branch, true", got, ok)
	}
}

func TestStartAddNoteCreatesUnderscoreFileFromName(t *testing.T) {
	fm := newFakeFileManager()
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		Notes: utils.OrderedMap{Values: make(map[string]string)},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      false,
		Notes:        true,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{"notes|__ADD_NOTE__|", "invalid"},
		textResults:      []string{"test file name"},
	}

	NewRunner(fm, runtime, views).Start()

	if got := runtime.openedInNvim; !reflect.DeepEqual(got, []string{"notes/test_file_name.md"}) {
		t.Fatalf("OpenInNvim calls = %#v, want note path", got)
	}

	if _, ok := fm.files["notes/test_file_name.md"]; !ok {
		t.Fatalf("expected notes/test_file_name.md to be created")
	}
}

func TestStartScriptRunCancelledDoesNotRunLua(t *testing.T) {
	fm := newFakeFileManager()
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		Scripts: utils.OrderedMap{
			Keys: []string{"cleanup"},
			Values: map[string]string{
				"cleanup": "Clean build artifacts",
			},
		},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      true,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{scripts.PageName + "|cleanup|Clean build artifacts", "invalid"},
		confirmResults:   []bool{false},
	}

	NewRunner(fm, runtime, views).Start()

	if len(runtime.ranLua) != 0 {
		t.Fatalf("RunLuaScript calls = %#v, want none after cancelled confirmation", runtime.ranLua)
	}
}

func TestStartScriptRunUsesLuaRunnerMockAfterConfirmation(t *testing.T) {
	fm := newFakeFileManager()
	fm.configContent = mustJSON(t, &utils.ConfigDTO{
		Scripts: utils.OrderedMap{
			Keys: []string{"cleanup"},
			Values: map[string]string{
				"cleanup": "Clean build artifacts",
			},
		},
	})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      true,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{scripts.PageName + "|cleanup|Clean build artifacts"},
		confirmResults:   []bool{true},
	}

	NewRunner(fm, runtime, views).Start()

	want := []string{"scripts/cleanup.lua"}
	if !reflect.DeepEqual(runtime.ranLua, want) {
		t.Fatalf("RunLuaScript calls = %#v, want %#v", runtime.ranLua, want)
	}
	if len(runtime.openedInNvim) != 0 {
		t.Fatalf("OpenInNvim calls = %#v, want none", runtime.openedInNvim)
	}
	if _, ok := fm.files["scripts/cleanup.lua"]; !ok {
		t.Fatalf("expected script file to be prepared through fake FileManager")
	}
}

func TestStartSearchReplaceSelectedFileUsesRipgrepResults(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one.txt")
	two := filepath.Join(root, "two.txt")
	mustWriteFile(t, one, "foo one foo")
	mustWriteFile(t, two, "foo two")
	installFakeRipgrep(t, []string{"one.txt", "two.txt"})

	fm := newFakeFileManager()
	fm.currentPath = root
	fm.configContent = mustJSON(t, &utils.ConfigDTO{})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      false,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{
			toolstab.PageName + "|" + toolstab.SearchReplaceAction + "|find and replace",
			"invalid",
		},
		textResults: []string{"foo", "bar"},
		searchReplaceFileResults: []utils.ListItem{
			{T: "one.txt", D: one},
			{T: utils.ExitSignal},
		},
	}

	NewRunner(fm, runtime, views).Start()

	if got := mustReadFile(t, one); got != "bar one bar" {
		t.Fatalf("one.txt = %q, want selected file replaced", got)
	}
	if got := mustReadFile(t, two); got != "foo two" {
		t.Fatalf("two.txt = %q, want unselected file unchanged", got)
	}
}

func TestStartSearchReplaceAllUsesRemainingRipgrepResults(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one.txt")
	two := filepath.Join(root, "two.txt")
	mustWriteFile(t, one, "foo one foo")
	mustWriteFile(t, two, "foo two")
	installFakeRipgrep(t, []string{"one.txt", "two.txt"})

	fm := newFakeFileManager()
	fm.currentPath = root
	fm.configContent = mustJSON(t, &utils.ConfigDTO{})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      false,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{
			toolstab.PageName + "|" + toolstab.SearchReplaceAction + "|find and replace",
			"invalid",
		},
		textResults: []string{"foo", "bar"},
		searchReplaceFileResults: []utils.ListItem{
			{T: toolstab.ReplaceAllAction},
		},
	}

	NewRunner(fm, runtime, views).Start()

	if got := mustReadFile(t, one); got != "bar one bar" {
		t.Fatalf("one.txt = %q, want all occurrences replaced", got)
	}
	if got := mustReadFile(t, two); got != "bar two" {
		t.Fatalf("two.txt = %q, want all occurrences replaced", got)
	}
}

func TestStartSearchReplaceEnterOnReplacedFileRestoresOriginalContent(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one.txt")
	mustWriteFile(t, one, "foo one bar")
	installFakeRipgrep(t, []string{"one.txt"})

	fm := newFakeFileManager()
	fm.currentPath = root
	fm.configContent = mustJSON(t, &utils.ConfigDTO{})
	fm.featuresContent = mustJSON(t, &settings.FeaturesDTO{
		FrequentGoTo: false,
		Scripts:      false,
		Notes:        false,
		Frequencies:  make(map[string]int),
	})

	runtime := newFakeRuntime()
	views := &fakeViewBuilder{
		multiPageResults: []string{
			toolstab.PageName + "|" + toolstab.SearchReplaceAction + "|find and replace",
			"invalid",
		},
		textResults: []string{"foo", "bar"},
		searchReplaceFileResults: []utils.ListItem{
			{T: "one.txt", D: one},
			{T: "one.txt", D: one, Status: toolstab.ReplacedStatus},
			{T: utils.ExitSignal},
		},
	}

	NewRunner(fm, runtime, views).Start()

	if got := mustReadFile(t, one); got != "foo one bar" {
		t.Fatalf("one.txt = %q, want original content restored", got)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()

	content, err := utils.ToJSON(value)
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}
	return content
}

func mustUnmarshalJSON(t *testing.T, content string, value any) {
	t.Helper()

	if content == "" {
		t.Fatal("expected JSON content, got empty string")
	}
	if err := json.Unmarshal([]byte(content), value); err != nil {
		t.Fatalf("unmarshal %q error = %v", content, err)
	}
}

func last(t *testing.T, values []string) string {
	t.Helper()

	if len(values) == 0 {
		t.Fatal("expected at least one value")
	}
	return values[len(values)-1]
}

func installFakeRipgrep(t *testing.T, files []string) {
	t.Helper()

	binDir := t.TempDir()
	rgPath := filepath.Join(binDir, "rg")

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("printf '")
	for _, file := range files {
		script.WriteString(file)
		script.WriteString("\\0")
	}
	script.WriteString("'\n")

	if err := os.WriteFile(rgPath, []byte(script.String()), 0755); err != nil {
		t.Fatalf("write fake rg: %v", err)
	}
	t.Setenv("PATH", binDir)
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
