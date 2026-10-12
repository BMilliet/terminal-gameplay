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
	"time"
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
	fileMenuBottomBoxHeight   = 6

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
	name    string
	kind    fileMenuEntryKind
	pending bool
}

type fileMenuPreview struct {
	entry        fileMenuEntry
	directory    []fileMenuEntry
	content      string
	truncated    bool
	binary       bool
	pending      bool
	metadata     fileMenuMetadata
	metadataErr  error
	err          error
	hasSelection bool
}

type fileMenuMetadata struct {
	size     int64
	mode     fs.FileMode
	modified time.Time
}

type currentDirectoryReader interface {
	ReadDirectory(path string) ([]fileMenuEntry, error)
	ReadDirectoryPreview(path string, limit int) ([]fileMenuEntry, bool, error)
	ReadFilePreview(path string, maxBytes int) ([]byte, bool, error)
	ReadMetadata(path string) (fileMenuMetadata, error)
}

type osCurrentDirectoryReader struct{}

type fileMenuOperationFinishedMsg struct {
	err error
}

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

func (osCurrentDirectoryReader) ReadMetadata(path string) (fileMenuMetadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileMenuMetadata{}, fmt.Errorf("read metadata %q: %w", path, err)
	}
	return fileMenuMetadata{
		size:     info.Size(),
		mode:     info.Mode(),
		modified: info.ModTime(),
	}, nil
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
	m.fileMenuBaseEntries = append([]fileMenuEntry(nil), entries...)
	m.fileMenuError = nil
	m.fileMenuPath = filepath.Clean(path)
	m.fileMenuCursor = 0
	m.fileMenuVisualAnchor = 0
	m.fileMenuPendingKey = ""
	m.rebuildFileMenuEntries()
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
	preview := fileMenuPreview{entry: entry, hasSelection: true, pending: entry.pending}
	if entry.pending {
		m.fileMenuPreview = preview
		return
	}
	preview.metadata, preview.metadataErr = m.currentDirectoryReader.ReadMetadata(path)
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
	if msg.String() == "ctrl+c" {
		*m.selected = utils.ExitSignal
		m.quitting = true
		return m, tea.Quit
	}

	switch m.fileMenuMode {
	case fileMenuInsertMode:
		m.updateFileMenuInsert(msg)
		return m, nil
	case fileMenuVisualMode:
		m.updateFileMenuVisual(msg)
		return m, nil
	case fileMenuCommandMode:
		return m.updateFileMenuCommand(msg)
	case fileMenuConfirmMode:
		return m, m.updateFileMenuConfirmation(msg)
	case fileMenuProcessingMode:
		return m, nil
	default:
		return m.updateFileMenuNormal(msg)
	}
}

func (m MultiPageViewModel) updateFileMenuNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "d" || key == "y" || key == "m" {
		if m.fileMenuPendingKey != key {
			m.fileMenuPendingKey = key
			m.fileMenuStatus = key
			return m, nil
		}

		m.fileMenuPendingKey = ""
		switch key {
		case "d":
			m.stageFileMenuDelete(m.selectedFileMenuEntries())
		case "y":
			m.setFileMenuClipboard(m.selectedFileMenuEntries(), false)
		case "m":
			m.setFileMenuClipboard(m.selectedFileMenuEntries(), true)
		}
		return m, nil
	}
	m.fileMenuPendingKey = ""

	switch key {
	case "q":
		if len(m.fileMenuOperations) > 0 {
			m.fileMenuStatus = "pending changes; use :w to save or :q! to discard"
			return m, nil
		}
		*m.selected = utils.ExitSignal
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.fileMenuStatus = ""
		return m, nil

	case "up", "k":
		m.moveFileMenuCursor(-1)
		return m, nil

	case "down", "j":
		m.moveFileMenuCursor(1)
		return m, nil

	case "left":
		if m.fileMenuPath == "." {
			return m, nil
		}
		m.loadFileMenuDirectory(filepath.Dir(m.fileMenuPath))
		return m, nil

	case "right":
		if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
			return m, nil
		}
		entry := m.fileMenuEntries[m.fileMenuCursor]
		if entry.kind == fileMenuDirectory && !entry.pending {
			path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
			m.loadFileMenuDirectory(path)
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
		if len(m.fileMenuOperations) > 0 {
			m.fileMenuStatus = "save or discard pending changes before opening a file"
			return m, nil
		}

		*m.selected = fmt.Sprintf("%s|%s|%s", FileMenuPageName, OpenFileAction, path)
		m.quitting = true
		return m, tea.Quit

	case "v", "V":
		if len(m.fileMenuEntries) > 0 {
			m.fileMenuMode = fileMenuVisualMode
			m.fileMenuVisualAnchor = m.fileMenuCursor
			m.fileMenuStatus = "visual line selection"
		}
		return m, nil

	case "o", "a":
		m.fileMenuMode = fileMenuInsertMode
		m.fileMenuInputAction = fileMenuCreateInput
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		m.fileMenuInputSource = ""
		m.fileMenuStatus = "new entry; end with / for a directory"
		return m, nil

	case "i":
		if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
			return m, nil
		}
		entry := m.fileMenuEntries[m.fileMenuCursor]
		m.fileMenuMode = fileMenuInsertMode
		m.fileMenuInputAction = fileMenuRenameInput
		m.fileMenuInput = entry.name
		m.fileMenuInputCursor = len([]rune(entry.name))
		m.fileMenuInputSource = filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		m.fileMenuInputKind = entry.kind
		m.fileMenuStatus = "edit name or relative path"
		return m, nil

	case "p":
		m.stageFileMenuPaste()
		return m, nil

	case ":":
		m.fileMenuMode = fileMenuCommandMode
		m.fileMenuCommand = ""
		m.fileMenuStatus = ""
		return m, nil
	}

	return m, nil
}

func (m *MultiPageViewModel) updateFileMenuVisual(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "v", "V":
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuStatus = ""
	case "up", "k":
		m.moveFileMenuCursor(-1)
	case "down", "j":
		m.moveFileMenuCursor(1)
	case "y":
		m.setFileMenuClipboard(m.selectedFileMenuEntries(), false)
		m.fileMenuMode = fileMenuNormalMode
	case "m":
		m.setFileMenuClipboard(m.selectedFileMenuEntries(), true)
		m.fileMenuMode = fileMenuNormalMode
	case "d":
		m.stageFileMenuDelete(m.selectedFileMenuEntries())
		m.fileMenuMode = fileMenuNormalMode
	}
}

func (m *MultiPageViewModel) updateFileMenuInsert(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.fileMenuInput) == "" && m.fileMenuInputAction == fileMenuCreateInput {
			m.fileMenuMode = fileMenuNormalMode
			m.fileMenuInput = ""
			m.fileMenuInputCursor = 0
			m.fileMenuStatus = "empty edit discarded"
			return
		}
		if err := m.commitFileMenuInput(); err != nil {
			m.fileMenuStatus = err.Error()
			return
		}
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		return
	case "left":
		if m.fileMenuInputCursor > 0 {
			m.fileMenuInputCursor--
		}
		return
	case "right":
		if m.fileMenuInputCursor < len([]rune(m.fileMenuInput)) {
			m.fileMenuInputCursor++
		}
		return
	case "home":
		m.fileMenuInputCursor = 0
		return
	case "end":
		m.fileMenuInputCursor = len([]rune(m.fileMenuInput))
		return
	case "backspace":
		runes := []rune(m.fileMenuInput)
		if m.fileMenuInputCursor > 0 {
			position := m.fileMenuInputCursor
			m.fileMenuInput = string(append(runes[:position-1], runes[position:]...))
			m.fileMenuInputCursor--
		}
		return
	case "delete":
		runes := []rune(m.fileMenuInput)
		if m.fileMenuInputCursor < len(runes) {
			position := m.fileMenuInputCursor
			m.fileMenuInput = string(append(runes[:position], runes[position+1:]...))
		}
		return
	case "ctrl+u":
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		return
	case "enter":
		if err := m.commitFileMenuInput(); err != nil {
			m.fileMenuStatus = err.Error()
			return
		}
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		return
	}

	if msg.Type == tea.KeyRunes {
		runes := []rune(m.fileMenuInput)
		position := m.fileMenuInputCursor
		inserted := append([]rune(nil), msg.Runes...)
		runes = append(runes[:position], append(inserted, runes[position:]...)...)
		m.fileMenuInput = string(runes)
		m.fileMenuInputCursor += len(inserted)
	}
}

func (m MultiPageViewModel) updateFileMenuCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuCommand = ""
		return m, nil
	case "backspace":
		runes := []rune(m.fileMenuCommand)
		if len(runes) > 0 {
			m.fileMenuCommand = string(runes[:len(runes)-1])
		}
		return m, nil
	case "enter":
		command := strings.TrimSpace(m.fileMenuCommand)
		m.fileMenuCommand = ""
		switch command {
		case "w":
			if len(m.fileMenuOperations) == 0 {
				m.fileMenuMode = fileMenuNormalMode
				m.fileMenuStatus = "no pending changes"
				return m, nil
			}
			m.fileMenuMode = fileMenuConfirmMode
			m.fileMenuStatus = fmt.Sprintf("apply %d pending operation(s)? y/n", len(m.fileMenuOperations))
			return m, nil
		case "q":
			if len(m.fileMenuOperations) > 0 {
				m.fileMenuMode = fileMenuNormalMode
				m.fileMenuStatus = "pending changes; use :w or :q!"
				return m, nil
			}
			*m.selected = utils.ExitSignal
			m.quitting = true
			return m, tea.Quit
		case "q!":
			m.fileMenuOperations = nil
			*m.selected = utils.ExitSignal
			m.quitting = true
			return m, tea.Quit
		default:
			m.fileMenuMode = fileMenuNormalMode
			m.fileMenuStatus = "unknown command: :" + command
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes {
		m.fileMenuCommand += string(msg.Runes)
	}
	return m, nil
}

func (m *MultiPageViewModel) updateFileMenuConfirmation(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "enter":
		operations := append([]fileMenuOperation(nil), m.fileMenuOperations...)
		operator := m.fileMenuOperator
		m.fileMenuMode = fileMenuProcessingMode
		m.fileMenuProcessFrame = 0
		m.fileMenuStatus = fmt.Sprintf("processing %d operation(s)", len(operations))
		return func() tea.Msg {
			return fileMenuOperationFinishedMsg{err: operator.Apply(operations)}
		}
	case "n", "esc":
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuStatus = "save cancelled; changes remain pending"
	}
	return nil
}

func (m *MultiPageViewModel) finishFileMenuOperations(err error) {
	if m.fileMenuMode != fileMenuProcessingMode {
		return
	}
	if err != nil {
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuStatus = "save failed: " + err.Error()
		return
	}
	count := len(m.fileMenuOperations)
	m.fileMenuOperations = nil
	if m.fileMenuClipboardCut {
		m.fileMenuClipboard = nil
		m.fileMenuClipboardMove = false
		m.fileMenuClipboardCut = false
	}
	m.fileMenuMode = fileMenuNormalMode
	m.loadFileMenuDirectory(m.fileMenuPath)
	m.fileMenuStatus = fmt.Sprintf("saved %d operation(s)", count)
}

func (m *MultiPageViewModel) moveFileMenuCursor(delta int) {
	if len(m.fileMenuEntries) == 0 {
		return
	}
	m.fileMenuCursor = (m.fileMenuCursor + delta + len(m.fileMenuEntries)) % len(m.fileMenuEntries)
	m.refreshFileMenuPreview()
}

func (m MultiPageViewModel) selectedFileMenuEntries() []fileMenuEntry {
	if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
		return nil
	}
	start, end := m.fileMenuCursor, m.fileMenuCursor
	if m.fileMenuMode == fileMenuVisualMode {
		start = m.fileMenuVisualAnchor
		if start > end {
			start, end = end, start
		}
	}
	return append([]fileMenuEntry(nil), m.fileMenuEntries[start:end+1]...)
}

func (m *MultiPageViewModel) setFileMenuClipboard(entries []fileMenuEntry, move bool) {
	m.fileMenuClipboard = make([]fileMenuClipboardEntry, 0, len(entries))
	for _, entry := range entries {
		m.fileMenuClipboard = append(m.fileMenuClipboard, fileMenuClipboardEntry{
			path: filepath.Clean(filepath.Join(m.fileMenuPath, entry.name)),
			kind: entry.kind,
		})
	}
	m.fileMenuClipboardMove = move
	m.fileMenuClipboardCut = false
	action := "yanked"
	if move {
		action = "marked to move"
	}
	m.fileMenuStatus = fmt.Sprintf("%s %d entrie(s)", action, len(entries))
}

func (m *MultiPageViewModel) stageFileMenuDelete(entries []fileMenuEntry) {
	if len(entries) == 0 {
		return
	}
	m.setFileMenuClipboard(entries, true)
	m.fileMenuClipboardCut = true
	added := 0
	for _, entry := range entries {
		path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if m.hasFileMenuOperation(fileMenuDelete, path, "") {
			continue
		}
		m.fileMenuOperations = append(m.fileMenuOperations, fileMenuOperation{kind: fileMenuDelete, source: path})
		added++
	}
	m.fileMenuStatus = fmt.Sprintf("cut %d entrie(s); p moves, :w deletes", added)
	m.rebuildFileMenuEntries()
	m.clampFileMenuCursor()
	m.refreshFileMenuPreview()
}

func (m *MultiPageViewModel) stageFileMenuPaste() {
	if len(m.fileMenuClipboard) == 0 {
		m.fileMenuStatus = "clipboard is empty"
		return
	}

	if m.fileMenuClipboardCut {
		restoresOriginalDirectory := true
		for _, entry := range m.fileMenuClipboard {
			destination := filepath.Clean(filepath.Join(m.fileMenuPath, filepath.Base(entry.path)))
			if destination != entry.path {
				restoresOriginalDirectory = false
				break
			}
		}
		if restoresOriginalDirectory {
			firstPath := m.fileMenuClipboard[0].path
			m.removeFileMenuClipboardDeletes()
			count := len(m.fileMenuClipboard)
			m.fileMenuClipboard = nil
			m.fileMenuClipboardMove = false
			m.fileMenuClipboardCut = false
			m.rebuildFileMenuEntries()
			m.selectFileMenuPath(firstPath)
			m.fileMenuStatus = fmt.Sprintf("restored %d cut entrie(s)", count)
			return
		}
	}

	operations := make([]fileMenuOperation, 0, len(m.fileMenuClipboard))
	for _, entry := range m.fileMenuClipboard {
		destination := filepath.Clean(filepath.Join(m.fileMenuPath, filepath.Base(entry.path)))
		if entry.kind == fileMenuDirectory && fileMenuPathContains(entry.path, destination) {
			m.fileMenuStatus = "cannot copy or move a directory inside itself"
			return
		}
		kind := fileMenuCopy
		if m.fileMenuClipboardMove {
			kind = fileMenuMove
			if destination == entry.path {
				m.fileMenuStatus = "choose another directory before pasting a move"
				return
			}
			if m.fileMenuDestinationExists(destination) {
				m.fileMenuStatus = "destination already exists: " + destination
				return
			}
		} else {
			destination = m.availableFileMenuCopyDestination(destination, entry.kind)
		}
		operations = append(operations, fileMenuOperation{kind: kind, source: entry.path, destination: destination, entryKind: entry.kind})
	}

	if m.fileMenuClipboardCut {
		m.removeFileMenuClipboardDeletes()
	}
	m.fileMenuOperations = append(m.fileMenuOperations, operations...)
	m.rebuildFileMenuEntries()
	if len(operations) > 0 {
		m.selectFileMenuPath(operations[0].destination)
	}
	verb := "copy"
	if m.fileMenuClipboardMove {
		verb = "move"
		m.fileMenuClipboard = nil
		m.fileMenuClipboardMove = false
		m.fileMenuClipboardCut = false
	}
	m.fileMenuStatus = fmt.Sprintf("staged %s of %d entrie(s)", verb, len(operations))
}

func (m *MultiPageViewModel) removeFileMenuClipboardDeletes() {
	cutPaths := make(map[string]struct{}, len(m.fileMenuClipboard))
	for _, entry := range m.fileMenuClipboard {
		cutPaths[entry.path] = struct{}{}
	}
	operations := m.fileMenuOperations[:0]
	for _, operation := range m.fileMenuOperations {
		_, isCutPath := cutPaths[operation.source]
		if operation.kind == fileMenuDelete && isCutPath {
			continue
		}
		operations = append(operations, operation)
	}
	m.fileMenuOperations = operations
}

func (m *MultiPageViewModel) commitFileMenuInput() error {
	value := strings.TrimSpace(m.fileMenuInput)
	if value == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("use a path relative to the current directory")
	}

	createDirectory := m.fileMenuInputAction == fileMenuCreateInput && strings.HasSuffix(value, string(filepath.Separator))
	if createDirectory {
		value = strings.TrimRight(value, string(filepath.Separator))
	}
	destination, err := resolveFileMenuPath(m.fileMenuPath, value)
	if err != nil {
		return err
	}
	if m.fileMenuInputAction == fileMenuRenameInput && destination == m.fileMenuInputSource {
		m.fileMenuStatus = "name unchanged"
		return nil
	}
	if m.fileMenuDestinationExists(destination) {
		return fmt.Errorf("destination already exists: %s", destination)
	}

	if m.fileMenuInputAction == fileMenuCreateInput {
		kind := fileMenuCreateFile
		if createDirectory {
			kind = fileMenuCreateDirectory
		}
		entryKind := fileMenuRegularFile
		if createDirectory {
			entryKind = fileMenuDirectory
		}
		m.fileMenuOperations = append(m.fileMenuOperations, fileMenuOperation{kind: kind, destination: destination, entryKind: entryKind})
		m.rebuildFileMenuEntries()
		m.selectFileMenuPath(destination)
		m.fileMenuStatus = "staged creation: " + destination
		return nil
	}

	m.fileMenuOperations = append(m.fileMenuOperations, fileMenuOperation{
		kind:        fileMenuMove,
		source:      m.fileMenuInputSource,
		destination: destination,
		entryKind:   m.fileMenuInputKind,
	})
	m.rebuildFileMenuEntries()
	m.selectFileMenuPath(destination)
	m.fileMenuStatus = "staged move: " + m.fileMenuInputSource + " → " + destination
	return nil
}

func resolveFileMenuPath(directory, value string) (string, error) {
	target := filepath.Clean(filepath.Join(directory, value))
	relative, err := filepath.Rel(".", target)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", value, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside the file menu root")
	}
	return target, nil
}

func fileMenuPathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func (m MultiPageViewModel) fileMenuDestinationExists(path string) bool {
	if filepath.Dir(path) == m.fileMenuPath {
		name := filepath.Base(path)
		for _, entry := range m.fileMenuEntries {
			if entry.name == name {
				return true
			}
		}
	}
	for _, operation := range m.fileMenuOperations {
		if operation.destination == path && operation.kind != fileMenuDelete {
			return true
		}
	}
	return false
}

func (m MultiPageViewModel) availableFileMenuCopyDestination(destination string, kind fileMenuEntryKind) string {
	if !m.fileMenuDestinationExists(destination) {
		return destination
	}
	directory := filepath.Dir(destination)
	base := filepath.Base(destination)
	extension := ""
	if kind != fileMenuDirectory {
		extension = filepath.Ext(base)
		base = strings.TrimSuffix(base, extension)
	}
	for number := 1; ; number++ {
		suffix := "-copy"
		if number > 1 {
			suffix = fmt.Sprintf("-copy-%d", number)
		}
		candidate := filepath.Join(directory, base+suffix+extension)
		if !m.fileMenuDestinationExists(candidate) {
			return candidate
		}
	}
}

func (m MultiPageViewModel) hasFileMenuOperation(kind fileMenuOperationKind, source, destination string) bool {
	for _, operation := range m.fileMenuOperations {
		if operation.kind == kind && operation.source == source && operation.destination == destination {
			return true
		}
	}
	return false
}

func (m *MultiPageViewModel) rebuildFileMenuEntries() {
	entries := append([]fileMenuEntry(nil), m.fileMenuBaseEntries...)
	for index := range entries {
		entries[index].pending = false
	}

	removePath := func(path string) {
		if filepath.Dir(path) != m.fileMenuPath {
			return
		}
		name := filepath.Base(path)
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.name != name {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	addDestination := func(operation fileMenuOperation) {
		if filepath.Dir(operation.destination) != m.fileMenuPath {
			return
		}
		entries = append(entries, fileMenuEntry{
			name:    filepath.Base(operation.destination),
			kind:    operation.entryKind,
			pending: true,
		})
	}

	for _, operation := range m.fileMenuOperations {
		switch operation.kind {
		case fileMenuCreateFile, fileMenuCreateDirectory, fileMenuCopy:
			addDestination(operation)
		case fileMenuMove:
			removePath(operation.source)
			addDestination(operation)
		case fileMenuDelete:
			removePath(operation.source)
		}
	}
	sortFileMenuEntries(entries)
	m.fileMenuEntries = entries
}

func (m *MultiPageViewModel) selectFileMenuPath(path string) {
	if filepath.Dir(path) != m.fileMenuPath {
		m.clampFileMenuCursor()
		m.refreshFileMenuPreview()
		return
	}
	name := filepath.Base(path)
	for index, entry := range m.fileMenuEntries {
		if entry.name == name {
			m.fileMenuCursor = index
			m.refreshFileMenuPreview()
			return
		}
	}
	m.clampFileMenuCursor()
	m.refreshFileMenuPreview()
}

func (m *MultiPageViewModel) clampFileMenuCursor() {
	if len(m.fileMenuEntries) == 0 {
		m.fileMenuCursor = 0
		return
	}
	if m.fileMenuCursor >= len(m.fileMenuEntries) {
		m.fileMenuCursor = len(m.fileMenuEntries) - 1
	}
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
	contentWidth := m.fileMenuTopContentWidth()
	leftContent := m.renderFileTree()
	rightContent := m.renderFilePreview()
	contentHeight := m.fileMenuTopContentHeight()
	leftContent = fitFileMenuTreeHeight(leftContent, contentHeight)
	rightContent = fitFileMenuPanelHeight(rightContent, contentHeight)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.MutedBorderColor).
		Padding(0, 1).
		Width(contentWidth).
		Height(contentHeight)

	outerHeight := contentHeight + 2
	leftBox := renderFixedHeightFileMenuBox(boxStyle, leftContent, outerHeight)
	rightBox := renderFixedHeightFileMenuBox(boxStyle, rightContent, outerHeight)

	boxes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBox,
		strings.Repeat(" ", fileMenuBoxGap),
		rightBox,
	)
	return boxes + "\n" + m.renderFileMenuBottomPanels() + "\n" + m.renderFileMenuStatus()
}

func renderFixedHeightFileMenuBox(style lipgloss.Style, content string, height int) string {
	rendered := style.Render(content)
	lines := strings.Split(rendered, "\n")
	if height <= 0 || len(lines) <= height {
		return rendered
	}
	bottomBorder := lines[len(lines)-1]
	lines = append(lines[:height-1], bottomBorder)
	return strings.Join(lines, "\n")
}

func (m MultiPageViewModel) fileMenuTopContentWidth() int {
	boxWidth := (m.contentWidth() - fileMenuBoxGap) / 2
	width := boxWidth - fileMenuBoxChrome
	if width < 1 {
		return 1
	}
	return width
}

func (m MultiPageViewModel) fileMenuTopContentHeight() int {
	headerHeight := lipgloss.Height(m.renderHeader())
	height := m.terminalHeight - headerHeight - (fileMenuBottomBoxHeight + 2) - 2
	if height < fileMenuMinBoxHeight {
		return fileMenuMinBoxHeight
	}
	return height
}

func fitFileMenuPanelHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	return strings.Join(lines[:height], "\n")
}

func fitFileMenuTreeHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= height || height < 2 {
		return fitFileMenuPanelHeight(content, height)
	}

	body := lines[1:]
	capacity := height - 1
	cursor := 0
	for index, line := range body {
		if strings.Contains(line, "█") {
			cursor = index
			break
		}
	}
	start := 0
	if cursor >= capacity {
		start = cursor - capacity + 1
	}
	end := start + capacity
	if end > len(body) {
		end = len(body)
		start = end - capacity
		if start < 0 {
			start = 0
		}
	}
	visible := append([]string{lines[0]}, body[start:end]...)
	return strings.Join(visible, "\n")
}

func (m MultiPageViewModel) renderFileMenuBottomPanels() string {
	availableWidth := m.contentWidth() - 2*fileMenuBoxGap
	boxWidth := availableWidth / 3
	contentWidth := boxWidth - fileMenuBoxChrome
	if contentWidth < 1 {
		contentWidth = 1
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.MutedBorderColor).
		Padding(0, 1).
		Width(contentWidth).
		Height(fileMenuBottomBoxHeight)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		boxStyle.Render(m.renderFileMenuProcessPanel(contentWidth)),
		strings.Repeat(" ", fileMenuBoxGap),
		boxStyle.Render(m.renderFileMenuMetadataPanel(contentWidth)),
		strings.Repeat(" ", fileMenuBoxGap),
		boxStyle.Render(m.renderFileMenuMemoryPanel(contentWidth)),
	)
}

func (m MultiPageViewModel) renderFileMenuProcessPanel(width int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor).Render("Process")
	barWidth := width - 2
	if barWidth < 4 {
		barWidth = 4
	}
	bar := []rune(strings.Repeat("░", barWidth))
	state := "idle"
	if m.fileMenuMode == fileMenuProcessingMode {
		state = "running"
		position := m.fileMenuProcessFrame % len(bar)
		for offset := 0; offset < 3 && offset < len(bar); offset++ {
			bar[(position+offset)%len(bar)] = '█'
		}
	}
	return title + "\n\n" + string(bar) + "\n" + state
}

func (m MultiPageViewModel) renderFileMenuMetadataPanel(width int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor).Render("Metadata")
	preview := m.fileMenuPreview
	if !preview.hasSelection {
		return title + "\n\nno selection"
	}
	if preview.pending {
		return title + "\n\n" + truncateSingleLine(preview.entry.name, width) + "\npending change"
	}
	if preview.metadataErr != nil {
		return title + "\n\nunable to read metadata"
	}
	return strings.Join([]string{
		title,
		"",
		truncateSingleLine(preview.entry.name, width),
		formatFileMenuSize(preview.metadata.size) + "  " + preview.metadata.mode.String(),
		preview.metadata.modified.Format("2006-01-02 15:04"),
	}, "\n")
}

func (m MultiPageViewModel) renderFileMenuMemoryPanel(width int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor).Render("Memory")
	if len(m.fileMenuClipboard) == 0 {
		return title + "\n\nempty"
	}
	lines := []string{title, ""}
	for index, entry := range m.fileMenuClipboard {
		if index >= fileMenuBottomBoxHeight-2 {
			lines = append(lines, "…")
			break
		}
		lines = append(lines, truncateSingleLine(entry.path, width))
	}
	return strings.Join(lines, "\n")
}

func formatFileMenuSize(size int64) string {
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, label := range units {
		value /= float64(unit)
		if value < float64(unit) || label == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, label)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func (m MultiPageViewModel) renderFileMenuStatus() string {
	mode := "-- NORMAL --"
	prompt := m.fileMenuStatus
	switch m.fileMenuMode {
	case fileMenuInsertMode:
		mode = "-- INSERT --"
		label := "new"
		if m.fileMenuInputAction == fileMenuRenameInput {
			label = "edit"
		}
		prompt = label + ": " + m.renderFileMenuInputCursor()
	case fileMenuVisualMode:
		mode = "-- VISUAL LINE --"
	case fileMenuCommandMode:
		mode = "-- COMMAND --"
		prompt = ":" + m.fileMenuCommand + "█"
	case fileMenuConfirmMode:
		mode = "-- CONFIRM --"
	case fileMenuProcessingMode:
		mode = "-- PROCESSING --"
	}

	if prompt == "" {
		prompt = "a/o new · i edit · V select · yy copy · dd cut · p paste · :w save"
	}
	if pending := len(m.fileMenuOperations); pending > 0 {
		prompt = fmt.Sprintf("%s · %d pending", prompt, pending)
	}
	return lipgloss.NewStyle().
		Foreground(m.styles.FooterColor).
		Render(mode + "  " + prompt)
}

func (m MultiPageViewModel) renderFileMenuInputCursor() string {
	runes := []rune(m.fileMenuInput)
	position := m.fileMenuInputCursor
	if position < 0 {
		position = 0
	}
	if position > len(runes) {
		position = len(runes)
	}
	return string(runes[:position]) + "█" + string(runes[position:])
}

func (m MultiPageViewModel) renderFilePreview() string {
	contentWidth := m.fileMenuTopContentWidth()
	if m.fileMenuMode == fileMenuConfirmMode {
		lines := []string{"Pending filesystem changes", ""}
		for _, operation := range m.fileMenuOperations {
			lines = append(lines, truncateFilePreviewLine(operation.description(), contentWidth))
		}
		return strings.Join(lines, "\n")
	}
	preview := m.fileMenuPreview
	if !preview.hasSelection {
		return lipgloss.NewStyle().Foreground(m.styles.FooterColor).Render("no selection")
	}

	icon, label, color := m.fileMenuEntryPresentation(preview.entry)
	label = truncateSingleLine(label, contentWidth-lipgloss.Width(icon)-1)
	header := lipgloss.NewStyle().Foreground(color).Render(icon + " " + label)
	if preview.pending {
		return header + "\n\n" + lipgloss.NewStyle().
			Foreground(m.styles.FooterColor).
			Render("pending change — save with :w")
	}
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
			entryLabelWidth := contentWidth - lipgloss.Width("  "+connector+" ") - lipgloss.Width(entryIcon+" ")
			entryLabel = truncateSingleLine(entryLabel, entryLabelWidth)
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
	labelWidth := contentWidth
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
	rootName := truncateSingleLine(filepath.Base(m.fileMenuPath), m.fileMenuTopContentWidth()-2)
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

	hasDraftEntry := m.fileMenuMode == fileMenuInsertMode && m.fileMenuInputAction == fileMenuCreateInput
	for index, entry := range m.fileMenuEntries {
		connector := "├─"
		if index == len(m.fileMenuEntries)-1 && !hasDraftEntry {
			connector = "└─"
		}

		icon, label, color := m.fileMenuEntryPresentation(entry)
		entryPath := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if m.fileMenuMode == fileMenuInsertMode && m.fileMenuInputAction == fileMenuRenameInput && entryPath == m.fileMenuInputSource {
			label = m.renderFileMenuInputCursor()
		}
		label = truncateSingleLine(label, labelWidth)
		marker := "  "
		branchColor := m.styles.DividerColor
		if entry.pending {
			marker = "+ "
			branchColor = m.styles.SelectedTitleColor
		} else if m.fileMenuEntryIsPendingDelete(entry) {
			marker = "D "
			branchColor = m.styles.ErrorColor
		}
		if m.fileMenuEntryIsVisuallySelected(index) {
			marker = "◆ "
			branchColor = m.styles.SelectedTitleColor
		}
		if index == m.fileMenuCursor {
			marker = "█ "
			branchColor = m.styles.SelectedTitleColor
		}
		branch := lipgloss.NewStyle().
			Foreground(branchColor).
			Render(marker + connector + " ")
		iconText := lipgloss.NewStyle().Foreground(color).Render(icon + " ")
		labelText := lipgloss.NewStyle().Foreground(color).Render(label)
		lines = append(lines, branch+iconText+labelText)
	}

	if hasDraftEntry {
		draftKind := fileMenuRegularFile
		if strings.HasSuffix(m.fileMenuInput, string(filepath.Separator)) {
			draftKind = fileMenuDirectory
		}
		icon, _, color := m.fileMenuEntryPresentation(fileMenuEntry{kind: draftKind})
		label := truncateSingleLine(m.renderFileMenuInputCursor(), labelWidth)
		branch := lipgloss.NewStyle().
			Foreground(m.styles.SelectedTitleColor).
			Render("█ └─ ")
		iconText := lipgloss.NewStyle().Foreground(color).Render(icon + " ")
		labelText := lipgloss.NewStyle().Foreground(color).Render(label)
		lines = append(lines, branch+iconText+labelText)
	}

	return strings.Join(lines, "\n")
}

func (m MultiPageViewModel) fileMenuEntryIsVisuallySelected(index int) bool {
	if m.fileMenuMode != fileMenuVisualMode {
		return false
	}
	start, end := m.fileMenuVisualAnchor, m.fileMenuCursor
	if start > end {
		start, end = end, start
	}
	return index >= start && index <= end
}

func (m MultiPageViewModel) fileMenuEntryIsPendingDelete(entry fileMenuEntry) bool {
	path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
	return m.hasFileMenuOperation(fileMenuDelete, path, "")
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
