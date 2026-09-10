package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenInEditorUsesConfiguredEditorAndPreservesSelectedPath(t *testing.T) {
	tempDir := t.TempDir()
	editorPath := filepath.Join(tempDir, "fake-editor")
	outputPath := filepath.Join(tempDir, "editor-arguments")
	selectedPath := filepath.Join(tempDir, "file with spaces.txt")

	editorScript := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$EDITOR_TEST_OUTPUT\"\n"
	if err := os.WriteFile(editorPath, []byte(editorScript), 0755); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}
	t.Setenv("EDITOR", editorPath+" --wait")
	t.Setenv("EDITOR_TEST_OUTPUT", outputPath)

	if err := NewUtils().OpenInEditor(selectedPath); err != nil {
		t.Fatalf("OpenInEditor() error = %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read fake editor arguments: %v", err)
	}
	want := "--wait\n" + selectedPath + "\n"
	if string(content) != want {
		t.Fatalf("editor arguments = %q, want %q", content, want)
	}
}
