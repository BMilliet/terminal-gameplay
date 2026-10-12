package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	fileMenuBoxGap       = 2
	fileMenuBoxChrome    = 4
	fileMenuMinBoxHeight = 12

	FileMenuPageName = "file-menu"
	OpenFileAction   = "open-file"
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
	ReadDirectory(path string) ([]fileMenuEntry, error)
}

type osCurrentDirectoryReader struct{}

func (osCurrentDirectoryReader) ReadDirectory(path string) ([]fileMenuEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", path, err)
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

func (m *MultiPageViewModel) loadFileMenuDirectory(path string) bool {
	entries, err := m.currentDirectoryReader.ReadDirectory(path)
	if err != nil {
		m.fileMenuError = err
		return false
	}

	sortFileMenuEntries(entries)
	m.fileMenuEntries = entries
	m.fileMenuError = nil
	m.fileMenuPath = filepath.Clean(path)
	m.fileMenuCursor = 0
	return true
}

func (m MultiPageViewModel) updateFileMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		*m.selected = utils.ExitSignal
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if len(m.fileMenuEntries) == 0 {
			return m, nil
		}
		if m.fileMenuCursor > 0 {
			m.fileMenuCursor--
		} else {
			m.fileMenuCursor = len(m.fileMenuEntries) - 1
		}
		return m, nil

	case "down", "j":
		if len(m.fileMenuEntries) == 0 {
			return m, nil
		}
		if m.fileMenuCursor < len(m.fileMenuEntries)-1 {
			m.fileMenuCursor++
		} else {
			m.fileMenuCursor = 0
		}
		return m, nil

	case "enter":
		if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
			return m, nil
		}

		entry := m.fileMenuEntries[m.fileMenuCursor]
		path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if entry.kind == fileMenuDirectory {
			m.loadFileMenuDirectory(path)
			return m, nil
		}

		*m.selected = fmt.Sprintf("%s|%s|%s", FileMenuPageName, OpenFileAction, path)
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
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
	rootName := filepath.Base(m.fileMenuPath)
	lines := []string{
		lipgloss.NewStyle().
			Foreground(m.styles.FileMenuDirectoryColor).
			Render("▾ " + rootName),
	}

	if m.fileMenuError != nil {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(m.styles.ErrorColor).
			Render("  ! unable to read current directory"))
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
		marker := "  "
		branchColor := m.styles.DividerColor
		if index == m.fileMenuCursor {
			marker = "▌ "
			branchColor = m.styles.SelectedTitleColor
		}
		branch := lipgloss.NewStyle().
			Foreground(branchColor).
			Render(marker + connector + " ")
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
