//go:build linux

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func moveFileMenuPathToTrash(path string) (string, error) {
	trashDirectory, directoryErr := systemFileMenuTrash{}.Directory()
	before := fileMenuTrashEntries(trashDirectory)
	var commandErr error
	if executable, err := exec.LookPath("gio"); err == nil {
		commandErr = runFileMenuTrashCommand(exec.Command(executable, "trash", path))
	} else if executable, err := exec.LookPath("trash-put"); err == nil {
		commandErr = runFileMenuTrashCommand(exec.Command(executable, path))
	} else {
		return "", fmt.Errorf("no supported trash command found (install gio or trash-cli)")
	}
	if commandErr != nil {
		return "", commandErr
	}
	if directoryErr != nil {
		return "", nil
	}
	return newFileMenuTrashDestination(trashDirectory, filepath.Base(path), before), nil
}

func fileMenuTrashEntries(directory string) map[string]struct{} {
	result := make(map[string]struct{})
	entries, err := os.ReadDir(directory)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		result[entry.Name()] = struct{}{}
	}
	return result
}

func newFileMenuTrashDestination(directory, base string, before map[string]struct{}) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if _, existed := before[entry.Name()]; existed {
			continue
		}
		if entry.Name() == base || strings.HasPrefix(entry.Name(), base+".") {
			return filepath.Join(directory, entry.Name())
		}
	}
	return ""
}

func runFileMenuTrashCommand(command *exec.Cmd) error {
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}
