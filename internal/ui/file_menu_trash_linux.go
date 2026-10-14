//go:build linux

package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

func moveFileMenuPathToTrash(path string) error {
	if executable, err := exec.LookPath("gio"); err == nil {
		return runFileMenuTrashCommand(exec.Command(executable, "trash", path))
	}
	if executable, err := exec.LookPath("trash-put"); err == nil {
		return runFileMenuTrashCommand(exec.Command(executable, path))
	}
	return fmt.Errorf("no supported trash command found (install gio or trash-cli)")
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
