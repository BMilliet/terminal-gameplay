package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindPathItemsWalksRootAndPrunesMetadataAndBuildDirectories(t *testing.T) {
	root := t.TempDir()
	mustCreateSearchFile(t, filepath.Join(root, "README.md"))
	mustCreateSearchFile(t, filepath.Join(root, "source", "main.go"))
	for _, directory := range []string{
		".git",
		".build",
		"node_modules",
		".next",
		"dist",
		"build",
		"Pods",
		".dart_tool",
		"vendor",
		"target",
	} {
		mustCreateSearchFile(t, filepath.Join(root, "source", directory, "ignored.txt"))
	}

	got, err := FindPathItems(root)
	if err != nil {
		t.Fatalf("FindPathItems() error = %v", err)
	}

	gotTitles := make([]string, 0, len(got))
	for _, item := range got {
		gotTitles = append(gotTitles, item.T)
		if !filepath.IsAbs(item.D) {
			t.Fatalf("item path = %q, want absolute path", item.D)
		}
	}
	wantTitles := []string{"README.md", "source/", "source/main.go"}
	if !reflect.DeepEqual(gotTitles, wantTitles) {
		t.Fatalf("FindPathItems() titles = %#v, want %#v", gotTitles, wantTitles)
	}
}

func TestFindPathItemsRejectsFileRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file.txt")
	mustCreateSearchFile(t, root)

	if _, err := FindPathItems(root); err == nil {
		t.Fatal("FindPathItems() error = nil, want non-directory error")
	}
}

func mustCreateSearchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
		t.Fatalf("create fixture file: %v", err)
	}
}
