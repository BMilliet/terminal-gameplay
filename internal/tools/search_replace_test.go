package tools

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindFilesUsesRipgrepAndReturnsSortedAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	installFakeRipgrep(t, "printf 'b.txt\\000dir/a.txt\\000'\n")

	got, err := FindFiles(root, "needle")
	if err != nil {
		t.Fatalf("FindFiles() error = %v", err)
	}

	want := []string{
		filepath.Join(root, "b.txt"),
		filepath.Join(root, "dir/a.txt"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindFiles() = %#v, want %#v", got, want)
	}
}

func TestFindFilesTreatsRipgrepNoMatchesAsEmptyList(t *testing.T) {
	root := t.TempDir()
	installFakeRipgrep(t, "exit 1\n")

	got, err := FindFiles(root, "needle")
	if err != nil {
		t.Fatalf("FindFiles() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FindFiles() = %#v, want empty", got)
	}
}

func TestEnsureRipgrepReportsMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if err := EnsureRipgrep(); !errors.Is(err, ErrRipgrepNotFound) {
		t.Fatalf("EnsureRipgrep() error = %v, want ErrRipgrepNotFound", err)
	}
}

func TestFileListItemsUsesRelativeTitleAndAbsoluteDescription(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "dir", "a.txt")

	got := FileListItems(root, []string{file})
	want := []string{"dir/a.txt", file}
	if len(got) != 1 || got[0].T != want[0] || got[0].D != want[1] {
		t.Fatalf("FileListItems() = %#v, want title %q desc %q", got, want[0], want[1])
	}
}

func TestReplaceInFileReplacesAllOccurrencesAndPreservesPermissions(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("foo one foo"), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	count, err := ReplaceInFile(file, "foo", "bar")
	if err != nil {
		t.Fatalf("ReplaceInFile() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("ReplaceInFile() count = %d, want 2", count)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(got) != "bar one bar" {
		t.Fatalf("file content = %q, want %q", got, "bar one bar")
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat result: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestReplaceInFileNoMatchLeavesFileUnchanged(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("alpha"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	count, err := ReplaceInFile(file, "foo", "bar")
	if err != nil {
		t.Fatalf("ReplaceInFile() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ReplaceInFile() count = %d, want 0", count)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(got) != "alpha" {
		t.Fatalf("file content = %q, want unchanged", got)
	}
}

func TestReadAndWriteFileContent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("before"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := WriteFileContent(file, "after"); err != nil {
		t.Fatalf("WriteFileContent() error = %v", err)
	}

	got, err := ReadFileContent(file)
	if err != nil {
		t.Fatalf("ReadFileContent() error = %v", err)
	}
	if got != "after" {
		t.Fatalf("ReadFileContent() = %q, want %q", got, "after")
	}
}

func installFakeRipgrep(t *testing.T, body string) {
	t.Helper()

	binDir := t.TempDir()
	rgPath := filepath.Join(binDir, "rg")
	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(rgPath, []byte(content), 0755); err != nil {
		t.Fatalf("write fake rg: %v", err)
	}

	t.Setenv("PATH", binDir)
}
