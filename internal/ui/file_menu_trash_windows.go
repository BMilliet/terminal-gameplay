//go:build windows

package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

func moveFileMenuPathToTrash(path string) (string, error) {
	script := `Add-Type -AssemblyName Microsoft.VisualBasic; ` +
		`if (Test-Path -LiteralPath $args[0] -PathType Container) { ` +
		`[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory($args[0], 'OnlyErrorDialogs', 'SendToRecycleBin') ` +
		`} else { ` +
		`[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($args[0], 'OnlyErrorDialogs', 'SendToRecycleBin') }`
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script, path)
	output, err := command.CombinedOutput()
	if err == nil {
		return "", nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		return "", err
	}
	return "", fmt.Errorf("%w: %s", err, message)
}
