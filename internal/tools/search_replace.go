package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"terminal-gameplay/internal/utils"
)

var ErrRipgrepNotFound = errors.New("ripgrep not found in PATH")

func FindFiles(root, search string) ([]string, error) {
	rgPath, err := ripgrepPath()
	if err != nil {
		return nil, err
	}

	if search == "" {
		return nil, fmt.Errorf("FindFiles -> search cannot be empty")
	}

	cmd := exec.Command(rgPath, "--files-with-matches", "--fixed-strings", "--null", "--", search, ".")
	cmd.Dir = root

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("FindFiles -> %v", err)
	}

	files := parseNullSeparatedPaths(root, output)
	sort.Strings(files)
	return files, nil
}

func EnsureRipgrep() error {
	_, err := ripgrepPath()
	return err
}

func FileListItems(root string, files []string) []utils.ListItem {
	items := make([]utils.ListItem, 0, len(files))
	for _, file := range files {
		display := file
		if rel, err := filepath.Rel(root, file); err == nil {
			display = rel
		}

		items = append(items, utils.ListItem{
			T: display,
			D: file,
		})
	}
	return items
}

func ReplaceInFile(filePath, search, replace string) (int, error) {
	if search == "" {
		return 0, fmt.Errorf("ReplaceInFile -> search cannot be empty")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("ReplaceInFile -> stat %s: %v", filePath, err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("ReplaceInFile -> read %s: %v", filePath, err)
	}

	content := string(data)
	count := strings.Count(content, search)
	if count == 0 {
		return 0, nil
	}

	updated := strings.ReplaceAll(content, search, replace)
	if err := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); err != nil {
		return 0, fmt.Errorf("ReplaceInFile -> write %s: %v", filePath, err)
	}

	return count, nil
}

func ReadFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("ReadFileContent -> read %s: %v", filePath, err)
	}
	return string(data), nil
}

func WriteFileContent(filePath, content string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("WriteFileContent -> stat %s: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, []byte(content), info.Mode().Perm()); err != nil {
		return fmt.Errorf("WriteFileContent -> write %s: %v", filePath, err)
	}

	return nil
}

func parseNullSeparatedPaths(root string, output []byte) []string {
	parts := strings.Split(string(output), "\x00")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		files = append(files, filepath.Join(root, part))
	}
	return files
}

func ripgrepPath() (string, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "", ErrRipgrepNotFound
	}
	return rgPath, nil
}
