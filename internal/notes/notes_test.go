package notes

import (
	"reflect"
	"strings"
	"testing"

	"terminal-gameplay/internal/utils"
)

type fakeNotesFileManager struct {
	files          map[string]string
	ensureDirCalls int
	deleted        []string
}

func newFakeNotesFileManager() *fakeNotesFileManager {
	return &fakeNotesFileManager{files: make(map[string]string)}
}

func (f *fakeNotesFileManager) CheckIfPathExists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

func (f *fakeNotesFileManager) DeleteFileIfExists(path string) error {
	f.deleted = append(f.deleted, path)
	delete(f.files, path)
	return nil
}

func (f *fakeNotesFileManager) EnsureNotesDir() error {
	f.ensureDirCalls++
	return nil
}

func (f *fakeNotesFileManager) NotesPath(fileName string) string {
	return "notes/" + fileName
}

func (f *fakeNotesFileManager) ReadFileContent(filePath string) (string, error) {
	return f.files[filePath], nil
}

func (f *fakeNotesFileManager) WriteFileContent(filePath, content string) error {
	f.files[filePath] = content
	return nil
}

func TestPreviewCollapsesWhitespaceAndTruncates(t *testing.T) {
	if got := Preview("  hello\n\nworld\t "); got != "hello world" {
		t.Fatalf("Preview() = %q, want %q", got, "hello world")
	}

	long := strings.Repeat("a", previewMaxRunes+10)
	got := Preview(long)
	if len([]rune(got)) != previewMaxRunes {
		t.Fatalf("Preview() length = %d, want %d", len([]rune(got)), previewMaxRunes)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("Preview() = %q, want ellipsis suffix", got)
	}
}

func TestBuildListUsesNotePreviewAndPreservesDividers(t *testing.T) {
	items := utils.OrderedMap{
		Keys: []string{"daily.md", "div", "todo.md"},
		Values: map[string]string{
			"daily.md": "first line\nsecond line",
			"div":      "journal",
			"todo.md":  "",
		},
	}

	got := BuildList(items)

	want := []utils.ListItem{
		{T: "daily.md", D: "first line second line"},
		{T: "div", D: "journal", IsDiv: true},
		{T: "todo.md", D: "(empty note)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildList() = %#v, want %#v", got, want)
	}
}

func TestSyncContentUsesFileManagerAbstraction(t *testing.T) {
	fm := newFakeNotesFileManager()
	items := utils.OrderedMap{
		Keys: []string{"daily note", "div"},
		Values: map[string]string{
			"daily note": "seed content",
			"div":        "ignored divider",
		},
	}

	if err := SyncContent(fm, &items); err != nil {
		t.Fatalf("SyncContent() error = %v", err)
	}

	if fm.ensureDirCalls != 1 {
		t.Fatalf("EnsureNotesDir calls = %d, want 1", fm.ensureDirCalls)
	}
	if got := fm.files["notes/daily_note.md"]; got != "seed content" {
		t.Fatalf("created note content = %q, want %q", got, "seed content")
	}
	if got := fm.files["notes/div.md"]; got != "" {
		t.Fatalf("divider file content = %q, want no file", got)
	}

	fm.files["notes/daily_note.md"] = "edited content"
	if err := SyncContent(fm, &items); err != nil {
		t.Fatalf("SyncContent() second call error = %v", err)
	}
	if got := items.Values["daily note"]; got != "edited content" {
		t.Fatalf("synced config content = %q, want edited content", got)
	}
}

func TestFileNameConvertsWhitespaceToUnderscores(t *testing.T) {
	if got := FileName("test file name"); got != "test_file_name.md" {
		t.Fatalf("FileName() = %q, want test_file_name.md", got)
	}
}

func TestDeleteFileUsesSanitizedNotePath(t *testing.T) {
	fm := newFakeNotesFileManager()

	if err := DeleteFile(fm, "../daily"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	want := []string{"notes/daily.md"}
	if !reflect.DeepEqual(fm.deleted, want) {
		t.Fatalf("deleted paths = %#v, want %#v", fm.deleted, want)
	}
}
