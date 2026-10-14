//go:build darwin

package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileMenuPathToTrashDirectoryRenamesWithoutStartingAnApplication(t *testing.T) {
	sourceDirectory := t.TempDir()
	trashDirectory := filepath.Join(t.TempDir(), ".Trash")
	if err := os.Mkdir(trashDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDirectory, "discard.txt")
	if err := os.WriteFile(source, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trashDirectory, "discard.txt"), []byte("older"), 0o600); err != nil {
		t.Fatal(err)
	}

	destination, err := moveFileMenuPathToTrashDirectory(source, trashDirectory)
	if err != nil {
		t.Fatalf("moveFileMenuPathToTrashDirectory() error = %v", err)
	}
	if destination != filepath.Join(trashDirectory, "discard 2.txt") {
		t.Fatalf("destination = %q, want collision-safe trash path", destination)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists after trash move: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Fatalf("trashed content = %q, want content", content)
	}
}
