package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type UtilsInterface interface {
	ValidateInput(input string)
	ExitWithError(message string)
	HandleError(err error, message string)
	ExpandPath(path string) string
	ContractPath(path string) string
	ExecuteCommand(command string) error
	CopyToClipboard(text string) error
	OpenInNvim(filePath string) error
	RunLuaScript(filePath string) error
	ChangeDirectory(path string) error
}

type Utils struct{}

func NewUtils() *Utils {
	return &Utils{}
}

func (u *Utils) ValidateInput(input string) {
	if input == ExitSignal {
		os.Exit(0)
	}
}

func (u *Utils) ExitWithError(message string) {
	fmt.Println(errorText(message))
	os.Exit(1)
}

func (u *Utils) HandleError(err error, message string) {
	if err != nil {
		fullMessage := fmt.Sprintf("%s: %v", message, err)
		fmt.Println(errorText(fullMessage))
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

func (u *Utils) ContractPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == home {
		return "~"
	}

	homePrefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, homePrefix) {
		return "~/" + strings.TrimPrefix(path, homePrefix)
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

func (u *Utils) OpenInNvim(filePath string) error {
	editor, err := exec.LookPath("nvim")
	if err != nil {
		return fmt.Errorf("nvim not found in PATH")
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (u *Utils) RunLuaScript(filePath string) error {
	runner, err := findLuaRunner()
	if err != nil {
		return err
	}

	cmd := exec.Command(runner, filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func findLuaRunner() (string, error) {
	for _, candidate := range []string{"lua", "lua5.4", "lua5.3", "luajit"} {
		runner, err := exec.LookPath(candidate)
		if err == nil {
			return runner, nil
		}
	}

	return "", fmt.Errorf("lua not found in PATH")
}

func (u *Utils) ChangeDirectory(path string) error {
	expandedPath := u.ExpandPath(path)

	// Output the cd command so the shell can execute it
	fmt.Printf("cd %s\n", expandedPath)

	return nil
}

func errorText(message string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F38BA8")).
		Render(message)
}
