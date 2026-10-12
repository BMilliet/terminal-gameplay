package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	fileMenuBoxGap       = 2
	fileMenuBoxChrome    = 4
	fileMenuMinBoxHeight = 12
)

type fileMenuEntryKind int

const (
	fileMenuRegularFile fileMenuEntryKind = iota
	fileMenuDirectory
	fileMenuExecutable
	fileMenuImage
)

type fileMenuEntry struct {
	name string
	kind fileMenuEntryKind
}

type currentDirectoryReader interface {
	ReadCurrentDirectory() ([]fileMenuEntry, error)
}

type osCurrentDirectoryReader struct{}

func (osCurrentDirectoryReader) ReadCurrentDirectory() ([]fileMenuEntry, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read current directory: %w", err)
	}

	result := make([]fileMenuEntry, 0, len(entries))
	for _, entry := range entries {
		mode := fs.FileMode(0)
		if !entry.IsDir() {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return nil, fmt.Errorf("inspect %q: %w", entry.Name(), infoErr)
			}
			mode = info.Mode()
		}

		result = append(result, fileMenuEntry{
			name: entry.Name(),
			kind: classifyFileMenuEntry(entry.Name(), entry.IsDir(), mode),
		})
	}

	return result, nil
}

func classifyFileMenuEntry(name string, isDir bool, mode fs.FileMode) fileMenuEntryKind {
	if isDir {
		return fileMenuDirectory
	}
	if mode.Perm()&0111 != 0 {
		return fileMenuExecutable
	}
	if isImageFile(name) {
		return fileMenuImage
	}
	return fileMenuRegularFile
}

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".avif", ".bmp", ".gif", ".heic", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func sortFileMenuEntries(entries []fileMenuEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		leftDirectory := entries[i].kind == fileMenuDirectory
		rightDirectory := entries[j].kind == fileMenuDirectory
		if leftDirectory != rightDirectory {
			return leftDirectory
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})
}

func (m MultiPageViewModel) renderFileMenu() string {
	leftContent := m.renderFileTree()
	contentHeight := lipgloss.Height(leftContent)
	if contentHeight < fileMenuMinBoxHeight {
		contentHeight = fileMenuMinBoxHeight
	}

	boxWidth := (m.contentWidth() - fileMenuBoxGap) / 2
	contentWidth := boxWidth - fileMenuBoxChrome
	if contentWidth < 1 {
		contentWidth = 1
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.MutedBorderColor).
		Padding(0, 1).
		Width(contentWidth).
		Height(contentHeight)

	leftBox := boxStyle.Render(leftContent)
	rightBox := boxStyle.Render("")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		strings.Repeat(" ", fileMenuBoxGap),
		rightBox,
	)
}

func (m MultiPageViewModel) renderFileTree() string {
	lines := []string{
		lipgloss.NewStyle().
			Foreground(m.styles.FileMenuDirectoryColor).
			Render("▾ ."),
	}

	if m.fileMenuError != nil {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(m.styles.ErrorColor).
			Render("  ! unable to read current directory"))
		return strings.Join(lines, "\n")
	}

	labelWidth := (m.contentWidth()-fileMenuBoxGap)/2 - fileMenuBoxChrome - 7
	if labelWidth < 1 {
		labelWidth = 1
	}

	for index, entry := range m.fileMenuEntries {
		connector := "├─"
		if index == len(m.fileMenuEntries)-1 {
			connector = "└─"
		}

		icon, label, color := m.fileMenuEntryPresentation(entry)
		label = truncateSingleLine(label, labelWidth)
		branch := lipgloss.NewStyle().
			Foreground(m.styles.DividerColor).
			Render("  " + connector + " ")
		iconText := lipgloss.NewStyle().Foreground(color).Render(icon + " ")
		labelText := lipgloss.NewStyle().Foreground(color).Render(label)
		lines = append(lines, branch+iconText+labelText)
	}

	return strings.Join(lines, "\n")
}

func (m MultiPageViewModel) fileMenuEntryPresentation(entry fileMenuEntry) (icon, label string, color lipgloss.Color) {
	switch entry.kind {
	case fileMenuDirectory:
		return "▸", entry.name + "/", m.styles.FileMenuDirectoryColor
	case fileMenuExecutable:
		return "◆", entry.name, m.styles.FileMenuExecutableColor
	case fileMenuImage:
		return "◇", entry.name, m.styles.FileMenuImageColor
	default:
		return "•", entry.name, m.styles.FileMenuFileColor
	}
}
