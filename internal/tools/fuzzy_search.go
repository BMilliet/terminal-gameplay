package tools

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"terminal-gameplay/internal/utils"
)

var prunedDirectoryNames = map[string]struct{}{
	".git":         {},
	".build":       {},
	"node_modules": {},
	".next":        {},
	"dist":         {},
	"build":        {},
	"Pods":         {},
	".dart_tool":   {},
	"vendor":       {},
	"target":       {},
}

// FindPathItems walks root using only the Go standard library. Git metadata and
// Swift build output are omitted to keep the fuzzy-search list useful.
func FindPathItems(root string) ([]utils.ListItem, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve search root %q: %w", root, err)
	}

	rootInfo, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("open search root %q: %w", absRoot, err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("search root is not a directory: %s", absRoot)
	}

	items := make([]utils.ListItem, 0)
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absRoot {
			return nil
		}

		if entry.IsDir() {
			if _, prune := prunedDirectoryNames[entry.Name()]; prune {
				return filepath.SkipDir
			}
		}

		relativePath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		displayPath := filepath.ToSlash(relativePath)
		if entry.IsDir() {
			displayPath += "/"
		}

		items = append(items, utils.ListItem{T: displayPath, D: path})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk search root %q: %w", absRoot, err)
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].T)
		right := strings.ToLower(items[j].T)
		if left == right {
			return items[i].T < items[j].T
		}
		return left < right
	})

	return items, nil
}
