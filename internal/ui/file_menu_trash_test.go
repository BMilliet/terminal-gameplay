package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemFileMenuTrashUsesNativeMoverWithoutFinderAutomation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "discard.txt")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	var movedPath string
	trash := systemFileMenuTrash{move: func(path string) error {
		movedPath = path
		return nil
	}}

	if err := trash.Move(path); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if movedPath != path {
		t.Fatalf("native mover path = %q, want %q", movedPath, path)
	}
}

func TestFileMenuTrashDirectoryForSupportedPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		home     string
		dataHome string
		want     string
	}{
		{name: "macOS", goos: "darwin", home: "/Users/test", want: "/Users/test/.Trash"},
		{name: "Linux XDG", goos: "linux", home: "/home/test", dataHome: "/data/test", want: "/data/test/Trash/files"},
		{name: "Linux default", goos: "linux", home: "/home/test", want: "/home/test/.local/share/Trash/files"},
		{name: "Linux ignores relative XDG", goos: "linux", home: "/home/test", dataHome: "relative", want: "/home/test/.local/share/Trash/files"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := fileMenuTrashDirectory(test.goos, test.home, test.dataHome)
			if err != nil {
				t.Fatalf("fileMenuTrashDirectory() error = %v", err)
			}
			if filepath.Clean(got) != filepath.Clean(test.want) {
				t.Fatalf("fileMenuTrashDirectory() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFileMenuTrashDirectoryRejectsVirtualOrUnsupportedPlatforms(t *testing.T) {
	for _, goos := range []string{"windows", "plan9"} {
		t.Run(goos, func(t *testing.T) {
			if _, err := fileMenuTrashDirectory(goos, `C:\Users\test`, ""); err == nil {
				t.Fatalf("fileMenuTrashDirectory(%q) error = nil, want unsupported error", goos)
			}
		})
	}
}
