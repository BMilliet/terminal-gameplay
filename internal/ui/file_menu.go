package ui

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	fileMenuBoxGap            = 2
	fileMenuBoxChrome         = 4
	fileMenuMinBoxHeight      = 12
	fileMenuPreviewMaxEntries = 16
	fileMenuPreviewMaxBytes   = 4 * 1024
	fileMenuPreviewMaxLines   = 16

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

type fileMenuPreview struct {
	entry        fileMenuEntry
	directory    []fileMenuEntry
	content      string
	truncated    bool
	binary       bool
	err          error
	hasSelection bool
}

type currentDirectoryReader interface {
	ReadDirectory(path string) ([]fileMenuEntry, error)
	ReadDirectoryPreview(path string, limit int) ([]fileMenuEntry, bool, error)
	ReadFilePreview(path string, maxBytes int) ([]byte, bool, error)
}

type osCurrentDirectoryReader struct{}

func (osCurrentDirectoryReader) ReadDirectory(path string) ([]fileMenuEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", path, err)
	}
	return fileMenuEntriesFromDirEntries(entries)
}

func (osCurrentDirectoryReader) ReadDirectoryPreview(path string, limit int) ([]fileMenuEntry, bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("open directory %q: %w", path, err)
	}
	defer directory.Close()

	entries, err := directory.ReadDir(limit + 1)
	if err != nil && err != io.EOF {
		return nil, false, fmt.Errorf("read directory preview %q: %w", path, err)
	}

	truncated := len(entries) > limit
	if truncated {
		entries = entries[:limit]
	}
	result, err := fileMenuEntriesFromDirEntries(entries)
	return result, truncated, err
}

func (osCurrentDirectoryReader) ReadFilePreview(path string, maxBytes int) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("open file preview %q: %w", path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, int64(maxBytes+1)))
	if err != nil {
		return nil, false, fmt.Errorf("read file preview %q: %w", path, err)
	}

	truncated := len(content) > maxBytes
	if truncated {
		content = content[:maxBytes]
	}
	return content, truncated, nil
}

func fileMenuEntriesFromDirEntries(entries []os.DirEntry) ([]fileMenuEntry, error) {
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
	m.refreshFileMenuPreview()
	return true
}

func (m *MultiPageViewModel) refreshFileMenuPreview() {
	m.fileMenuPreview = fileMenuPreview{}
	if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
		return
	}

	entry := m.fileMenuEntries[m.fileMenuCursor]
	path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
	preview := fileMenuPreview{entry: entry, hasSelection: true}
	if entry.kind == fileMenuDirectory {
		preview.directory, preview.truncated, preview.err = m.currentDirectoryReader.ReadDirectoryPreview(path, fileMenuPreviewMaxEntries)
		sortFileMenuEntries(preview.directory)
	} else {
		content, truncated, err := m.currentDirectoryReader.ReadFilePreview(path, fileMenuPreviewMaxBytes)
		preview.truncated = truncated
		preview.err = err
		if err == nil {
			var linesTruncated bool
			preview.content, preview.binary, linesTruncated = sanitizeFilePreview(content, fileMenuPreviewMaxLines)
			preview.truncated = preview.truncated || linesTruncated
		}
	}
	m.fileMenuPreview = preview
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
		m.refreshFileMenuPreview()
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
		m.refreshFileMenuPreview()
		return m, nil

	case "left":
		if m.fileMenuPath == "." {
			return m, nil
		}
		m.loadFileMenuDirectory(filepath.Dir(m.fileMenuPath))
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

func sanitizeFilePreview(content []byte, maxLines int) (string, bool, bool) {
	if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
		return "", true, false
	}

	var sanitized strings.Builder
	for _, value := range string(content) {
		switch {
		case value == '\n':
			sanitized.WriteRune(value)
		case value == '\t':
			sanitized.WriteString("    ")
		case unicode.IsControl(value):
			continue
		default:
			sanitized.WriteRune(value)
		}
	}

	lines := strings.Split(sanitized.String(), "\n")
	truncated := len(lines) > maxLines
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n"), false, truncated
}

func (m MultiPageViewModel) renderFileMenu() string {
	leftContent := m.renderFileTree()
	rightContent := m.renderFilePreview()
	contentHeight := lipgloss.Height(leftContent)
	if rightHeight := lipgloss.Height(rightContent); rightHeight > contentHeight {
		contentHeight = rightHeight
	}
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
	rightBox := boxStyle.Render(rightContent)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		strings.Repeat(" ", fileMenuBoxGap),
		rightBox,
	)
}

func (m MultiPageViewModel) renderFilePreview() string {
	preview := m.fileMenuPreview
	if !preview.hasSelection {
		return lipgloss.NewStyle().Foreground(m.styles.FooterColor).Render("no selection")
	}

	icon, label, color := m.fileMenuEntryPresentation(preview.entry)
	header := lipgloss.NewStyle().Foreground(color).Render(icon + " " + label)
	if preview.err != nil {
		return header + "\n\n" + lipgloss.NewStyle().
			Foreground(m.styles.ErrorColor).
			Render("unable to load preview")
	}

	if preview.entry.kind == fileMenuDirectory {
		lines := []string{header}
		for index, entry := range preview.directory {
			last := index == len(preview.directory)-1 && !preview.truncated
			connector := "├─"
			if last {
				connector = "└─"
			}
			entryIcon, entryLabel, entryColor := m.fileMenuEntryPresentation(entry)
			branch := lipgloss.NewStyle().Foreground(m.styles.DividerColor).Render("  " + connector + " ")
			item := lipgloss.NewStyle().Foreground(entryColor).Render(entryIcon + " " + entryLabel)
			lines = append(lines, branch+item)
		}
		if preview.truncated {
			lines = append(lines, lipgloss.NewStyle().Foreground(m.styles.FooterColor).Render("  └─ …"))
		} else if len(preview.directory) == 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(m.styles.FooterColor).Render("  empty directory"))
		}
		return strings.Join(lines, "\n")
	}

	if preview.binary {
		return header + "\n\n" + lipgloss.NewStyle().
			Foreground(m.styles.FooterColor).
			Render("binary file — no text preview")
	}

	content := preview.content
	if content == "" {
		content = "empty file"
	}
	lines := strings.Split(content, "\n")
	labelWidth := (m.contentWidth()-fileMenuBoxGap)/2 - fileMenuBoxChrome
	for index := range lines {
		lines[index] = truncateFilePreviewLine(lines[index], labelWidth)
	}
	if preview.truncated {
		lines = append(lines, "…")
	}
	return header + "\n\n" + strings.Join(lines, "\n")
}

func truncateFilePreviewLine(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	if maxWidth == 1 {
		return "…"
	}
	return truncateToWidth(text, maxWidth-1) + "…"
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
