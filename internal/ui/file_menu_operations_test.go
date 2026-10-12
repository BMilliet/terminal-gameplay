package ui

import (
	"os"
	"path/filepath"
	"testing"
)

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

	operator := osFileMenuOperator{root: root}
	operations := []fileMenuOperation{
		{kind: fileMenuCreateFile, destination: "created.txt"},
		{kind: fileMenuCreateDirectory, destination: "created-dir"},
		{kind: fileMenuCopy, source: "source.txt", destination: "copied.txt"},
		{kind: fileMenuCopy, source: "source-dir", destination: "copied-dir"},
		{kind: fileMenuMove, source: "created.txt", destination: "moved.txt"},
		{kind: fileMenuDelete, source: "source-dir"},
	}

	if err := operator.Apply(operations); err != nil {
		t.Fatalf("Apply() error = %v", err)
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
}

func TestOSFileMenuOperatorRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	operator := osFileMenuOperator{root: root}

	err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: "../outside.txt"}})

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

	if err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: destination}}); err != nil {
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
	}})
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

	err := operator.Apply([]fileMenuOperation{{kind: fileMenuCreateFile, destination: "outside-link/escaped.txt"}})

	if err == nil {
		t.Fatal("Apply() error = nil, want symlink escape rejection")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "escaped.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file was created or returned unexpected error: %v", statErr)
	}
}
