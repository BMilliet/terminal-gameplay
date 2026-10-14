package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeFileMenuTrash struct {
	paths       []string
	destination string
	err         error
}

func (f *fakeFileMenuTrash) Move(path string) (string, error) {
	f.paths = append(f.paths, path)
	if f.err != nil {
		return "", f.err
	}
	return f.destination, os.RemoveAll(path)
}

func TestOSFileMenuOperatorAppliesStagedOperationsInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("copied content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "source-dir"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source-dir", "nested.txt"), []byte("nested content"), 0o600); err != nil {
		t.Fatal(err)
	}

	trasher := &fakeFileMenuTrash{}
	operator := osFileMenuOperator{root: root, trash: trasher}
	operations := []fileMenuOperation{
		{kind: fileMenuCreateFile, destination: "created.txt"},
		{kind: fileMenuCreateDirectory, destination: "created-dir"},
		{kind: fileMenuCopy, source: "source.txt", destination: "copied.txt"},
		{kind: fileMenuCopy, source: "source-dir", destination: "copied-dir"},
		{kind: fileMenuMove, source: "created.txt", destination: "moved.txt"},
		{kind: fileMenuDelete, source: "source-dir"},
	}

	result := operator.Apply(operations, nil)
	if result.err != nil {
		t.Fatalf("Apply() error = %v", result.err)
	}
	if result.completed != len(operations) {
		t.Fatalf("Apply() completed = %d, want %d", result.completed, len(operations))
	}
	if content, err := os.ReadFile(filepath.Join(root, "copied.txt")); err != nil || string(content) != "copied content" {
		t.Fatalf("copied file content = %q, err = %v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "copied-dir", "nested.txt")); err != nil || string(content) != "nested content" {
		t.Fatalf("copied nested content = %q, err = %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(root, "moved.txt")); err != nil {
		t.Fatalf("moved file is missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "source-dir")); !os.IsNotExist(err) {
		t.Fatalf("deleted directory still exists or returned unexpected error: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "created-dir")); err != nil || !info.IsDir() {
		t.Fatalf("created directory info = %#v, err = %v", info, err)
	}
	if len(trasher.paths) != 1 || trasher.paths[0] != filepath.Join(root, "source-dir") {
		t.Fatalf("trash paths = %#v, want source-dir", trasher.paths)
	}
}

func TestOSFileMenuOperatorRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	operator := osFileMenuOperator{root: root}

	err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: "../outside.txt"}}, nil).err

	if err == nil {
		t.Fatal("Apply() error = nil, want path escape rejection")
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(root), "outside.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file was created or returned unexpected error: %v", statErr)
	}
}

func TestOSFileMenuOperatorAcceptsAbsolutePathInsideRoot(t *testing.T) {
	root := t.TempDir()
	operator := osFileMenuOperator{root: root}
	destination := filepath.Join(root, "created.txt")

	if err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: destination}}, nil).err; err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("absolute destination inside root was not created: %v", err)
	}
}

func TestOSFileMenuOperatorCopiesAcrossAllowedPlaces(t *testing.T) {
	sourceRoot := t.TempDir()
	destinationRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "source.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	operator := osFileMenuOperator{root: sourceRoot}.allowRoot(destinationRoot)
	destination := filepath.Join(destinationRoot, "copied.txt")

	err := operator.Apply([]fileMenuOperation{{
		kind:        fileMenuCopy,
		source:      "source.txt",
		destination: destination,
	}}, nil).err
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "content" {
		t.Fatalf("copied content = %q, err = %v", content, err)
	}
}

func TestOSFileMenuOperatorRejectsDestinationThroughSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	operator := osFileMenuOperator{root: root}

	err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: "outside-link/escaped.txt"}}, nil).err

	if err == nil {
		t.Fatal("Apply() error = nil, want symlink escape rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "escaped.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file was created or returned unexpected error: %v", statErr)
	}
}

func TestOSFileMenuOperatorMovesAcrossFilesystemsByCopyingThenDeleting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "large.bin")
	destination := filepath.Join(root, "archive", "large.bin")
	if err := os.WriteFile(source, []byte("large content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}

	operator := osFileMenuOperator{
		root: root,
		rename: func(oldPath, newPath string) error {
			if oldPath == source && newPath == destination {
				return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: syscall.EXDEV}
			}
			return os.Rename(oldPath, newPath)
		},
	}
	result := operator.Apply([]fileMenuOperation{{kind: fileMenuMove, source: source, destination: destination}}, nil)

	if result.err != nil {
		t.Fatalf("Apply() error = %v", result.err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "large content" {
		t.Fatalf("destination content = %q, err = %v", content, err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists or returned unexpected error: %v", err)
	}
}

func TestOSFileMenuOperatorUsesTrashUnlessDeleteIsPermanent(t *testing.T) {
	root := t.TempDir()
	trashDestination := filepath.Join(root, ".Trash", "trashed.txt")
	trasher := &fakeFileMenuTrash{destination: trashDestination}
	operator := osFileMenuOperator{root: root, trash: trasher}
	trashed := filepath.Join(root, "trashed.txt")
	permanent := filepath.Join(root, "permanent.txt")
	for _, path := range []string{trashed, permanent} {
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result := operator.Apply([]fileMenuOperation{
		{kind: fileMenuDelete, source: trashed},
		{kind: fileMenuDelete, source: permanent, permanent: true},
	}, nil)

	if result.err != nil {
		t.Fatalf("Apply() error = %v", result.err)
	}
	if len(trasher.paths) != 1 || trasher.paths[0] != trashed {
		t.Fatalf("trash paths = %#v, want only %q", trasher.paths, trashed)
	}
	if len(result.completedOperations) != 2 || result.completedOperations[0].destination != trashDestination {
		t.Fatalf("completed operations = %#v, want reported trash destination %q", result.completedOperations, trashDestination)
	}
	if _, err := os.Stat(permanent); !os.IsNotExist(err) {
		t.Fatalf("permanent deletion left file behind or returned unexpected error: %v", err)
	}
}

func TestOSFileMenuOperatorReportsCopyProgressForLargeFiles(t *testing.T) {
	root := t.TempDir()
	content := make([]byte, 256*1024)
	source := filepath.Join(root, "large.bin")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	var reports []fileMenuOperationProgress
	operator := osFileMenuOperator{root: root}

	result := operator.Apply([]fileMenuOperation{{
		kind:        fileMenuCopy,
		source:      source,
		destination: filepath.Join(root, "large-copy.bin"),
	}}, func(progress fileMenuOperationProgress) {
		reports = append(reports, progress)
	})

	if result.err != nil {
		t.Fatalf("Apply() error = %v", result.err)
	}
	if len(reports) < 2 {
		t.Fatalf("progress reports = %#v, want streaming updates", reports)
	}
	last := reports[len(reports)-1]
	if last.doneItems != 1 || last.totalItems != 1 || last.doneBytes != int64(len(content)) || last.totalBytes != int64(len(content)) {
		t.Fatalf("last progress = %#v, want completed file and byte totals", last)
	}
	if last.currentFile != "large.bin" {
		t.Fatalf("current file = %q, want large.bin", last.currentFile)
	}
}

func TestOSFileMenuOperatorReturnsCompletedCountAndLeavesFailedCopyRetryable(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	operator := osFileMenuOperator{root: root}
	operations := []fileMenuOperation{
		{kind: fileMenuCreateFile, destination: filepath.Join(root, "created.txt")},
		{kind: fileMenuCopy, source: source, destination: filepath.Join(root, "missing", "copy.txt")},
	}

	result := operator.Apply(operations, nil)

	if result.err == nil {
		t.Fatal("Apply() error = nil, want second operation failure")
	}
	if result.completed != 1 {
		t.Fatalf("Apply() completed = %d, want 1", result.completed)
	}
	if _, err := os.Stat(filepath.Join(root, "missing", "copy.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed copy published a destination or returned unexpected error: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tg-copy-") {
			t.Fatalf("failed copy left temporary entry %q", entry.Name())
		}
	}
}

func TestOSFileMenuOperatorDoesNotPermanentlyDeleteWhenTrashFails(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	trashErr := errors.New("trash unavailable")
	operator := osFileMenuOperator{root: root, trash: &fakeFileMenuTrash{err: trashErr}}

	result := operator.Apply([]fileMenuOperation{{kind: fileMenuDelete, source: path}}, nil)

	if !errors.Is(result.err, trashErr) {
		t.Fatalf("Apply() error = %v, want trash error", result.err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("trash failure removed source: %v", err)
	}
}
