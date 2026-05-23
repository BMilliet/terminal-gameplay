package scripts

import (
	"reflect"
	"strings"
	"testing"

	"terminal-gameplay/internal/utils"
)

type fakeScriptsFileManager struct {
	files          map[string]string
	ensureDirCalls int
	deleted        []string
}

func newFakeScriptsFileManager() *fakeScriptsFileManager {
	return &fakeScriptsFileManager{files: make(map[string]string)}
}

func (f *fakeScriptsFileManager) CheckIfPathExists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

func (f *fakeScriptsFileManager) DeleteFileIfExists(path string) error {
	f.deleted = append(f.deleted, path)
	delete(f.files, path)
	return nil
}

func (f *fakeScriptsFileManager) EnsureScriptsDir() error {
	f.ensureDirCalls++
	return nil
}

func (f *fakeScriptsFileManager) ScriptsPath(fileName string) string {
	return "scripts/" + fileName
}

func (f *fakeScriptsFileManager) WriteFileContent(filePath, content string) error {
	f.files[filePath] = content
	return nil
}

func TestDefaultLuaContentIncludesNameDescriptionAndPrint(t *testing.T) {
	got := DefaultLuaContent(" deploy.lua ", " Ship prod ")

	for _, want := range []string{
		"-- deploy.lua",
		"-- Ship prod",
		`print("Hello from deploy.lua")`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("DefaultLuaContent() = %q, want to contain %q", got, want)
		}
	}
}

func TestSyncFilesCreatesScriptsThroughAbstractionAndSkipsDividers(t *testing.T) {
	fm := newFakeScriptsFileManager()
	items := utils.OrderedMap{
		Keys: []string{"deploy prod", "div"},
		Values: map[string]string{
			"deploy prod": "Deploy prod",
			"div":         "ignored divider",
		},
	}

	if err := SyncFiles(fm, &items); err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if fm.ensureDirCalls != 1 {
		t.Fatalf("EnsureScriptsDir calls = %d, want 1", fm.ensureDirCalls)
	}
	if _, ok := fm.files["scripts/deploy_prod.lua"]; !ok {
		t.Fatalf("expected scripts/deploy_prod.lua to be created")
	}
	if _, ok := fm.files["scripts/div.lua"]; ok {
		t.Fatalf("divider should not create a script file")
	}

	firstContent := fm.files["scripts/deploy_prod.lua"]
	if err := SyncFiles(fm, &items); err != nil {
		t.Fatalf("SyncFiles() second call error = %v", err)
	}
	if got := fm.files["scripts/deploy_prod.lua"]; got != firstContent {
		t.Fatalf("existing script content changed to %q, want %q", got, firstContent)
	}
}

func TestFileNameConvertsWhitespaceToUnderscores(t *testing.T) {
	if got := FileName("get current branch"); got != "get_current_branch.lua" {
		t.Fatalf("FileName() = %q, want get_current_branch.lua", got)
	}
}

func TestDeleteFileUsesSanitizedScriptPath(t *testing.T) {
	fm := newFakeScriptsFileManager()

	if err := DeleteFile(fm, "../deploy"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	want := []string{"scripts/deploy.lua"}
	if !reflect.DeepEqual(fm.deleted, want) {
		t.Fatalf("deleted paths = %#v, want %#v", fm.deleted, want)
	}
}
