package src

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type UtilsInterface interface {
	ValidateInput(input string)
	ExitWithError(message string)
	HandleError(err error, message string)
	ExpandPath(path string) string
	ExecuteCommand(command string) error
	CopyToClipboard(text string) error
	ChangeDirectory(path string) error
}

type Utils struct{}

func NewUtils() *Utils {
	return &Utils{}
}

func (u *Utils) ValidateInput(input string) {
	if input == ExitSignal {
		fmt.Println("\nExiting...")
		os.Exit(0)
	}
}

func (u *Utils) ExitWithError(message string) {
	styles := DefaultStyles()
	fmt.Println(styles.Text(message, styles.ErrorColor))
	os.Exit(1)
}

func (u *Utils) HandleError(err error, message string) {
	if err != nil {
		styles := DefaultStyles()
		fullMessage := fmt.Sprintf("%s: %v", message, err)
		fmt.Println(styles.Text(fullMessage, styles.ErrorColor))
		os.Exit(1)
	}
}

func (u *Utils) ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func (u *Utils) ExecuteCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (u *Utils) CopyToClipboard(text string) error {
	// Try pbcopy (macOS)
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try xclip (Linux)
	cmd = exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try xsel (Linux alternative)
	cmd = exec.Command("xsel", "--clipboard", "--input")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (u *Utils) ChangeDirectory(path string) error {
	expandedPath := u.ExpandPath(path)

	// Output the cd command so the shell can execute it
	fmt.Printf("cd %s\n", expandedPath)

	return nil
}
