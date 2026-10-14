package ui

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"terminal-gameplay/internal/utils"

	"github.com/charmbracelet/bubbles/progress"
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
	fileMenuScrollOffset      = 5
	fileMenuPlacesOuterWidth  = 24

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

type fileMenuFocus int

const (
	fileMenuTreeFocus fileMenuFocus = iota
	fileMenuPlacesFocus
)

type fileMenuProcessState int

const (
	fileMenuProcessIdle fileMenuProcessState = iota
	fileMenuProcessRunning
	fileMenuProcessSuccessful
	fileMenuProcessFailed
)

type fileMenuPlace struct {
	label string
	path  string
	icon  string
}

type fileMenuEntry struct {
	name        string
	kind        fileMenuEntryKind
	pending     bool
	pendingKind fileMenuOperationKind
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
	HomeDirectory() (string, error)
	WorkingDirectory() (string, error)
	TrashDirectory() (string, error)
}

type osCurrentDirectoryReader struct{}

type fileMenuOperationFinishedMsg struct {
	result fileMenuApplyResult
}

type fileMenuOperationProgressMsg struct {
	progress fileMenuOperationProgress
	messages <-chan tea.Msg
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

func (osCurrentDirectoryReader) HomeDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return home, nil
}

func (osCurrentDirectoryReader) WorkingDirectory() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find working directory: %w", err)
	}
	return directory, nil
}

func (osCurrentDirectoryReader) TrashDirectory() (string, error) {
	return systemFileMenuTrash{}.Directory()
}

func (m *MultiPageViewModel) loadFileMenuPlaces() {
	m.fileMenuPlacesError = nil
	m.fileMenuProtectedPaths = nil
	home, err := m.currentDirectoryReader.HomeDirectory()
	if err != nil {
		m.fileMenuPlacesError = err
		m.fileMenuHomePlaces = nil
	} else {
		m.fileMenuHomePlaces = []fileMenuPlace{
			{label: "Downloads", path: filepath.Join(home, "Downloads"), icon: "⌂"},
			{label: "Desktop", path: filepath.Join(home, "Desktop"), icon: "⌂"},
			{label: "Documents", path: filepath.Join(home, "Documents"), icon: "⌂"},
		}
		if trashDirectory, trashErr := m.currentDirectoryReader.TrashDirectory(); trashErr == nil && trashDirectory != "" {
			m.fileMenuHomePlaces = append(m.fileMenuHomePlaces, fileMenuPlace{
				label: "Trash",
				path:  filepath.Clean(trashDirectory),
				icon:  "♻",
			})
		}
		m.fileMenuProtectedPaths = make([]string, 0, len(m.fileMenuHomePlaces))
		for _, place := range m.fileMenuHomePlaces {
			m.fileMenuProtectedPaths = append(m.fileMenuProtectedPaths, filepath.Clean(place.path))
		}
	}

	workingDirectory, workingDirectoryErr := m.currentDirectoryReader.WorkingDirectory()
	if workingDirectoryErr != nil {
		m.fileMenuPlacesError = workingDirectoryErr
		m.fileMenuPinnedPlaces = nil
	} else {
		workingDirectory = filepath.Clean(workingDirectory)
		label := filepath.Base(workingDirectory)
		if label == string(filepath.Separator) || label == "." {
			label = workingDirectory
		}
		m.fileMenuPinnedPlaces = []fileMenuPlace{{label: label, path: workingDirectory, icon: "◆"}}
	}
	m.clampFileMenuPlacesCursor()
}

func (m *MultiPageViewModel) clampFileMenuPlacesCursor() {
	count := len(m.fileMenuPinnedPlaces) + len(m.fileMenuHomePlaces)
	if count == 0 {
		m.fileMenuPlacesCursor = 0
		return
	}
	if m.fileMenuPlacesCursor < 0 {
		m.fileMenuPlacesCursor = 0
	}
	if m.fileMenuPlacesCursor >= count {
		m.fileMenuPlacesCursor = count - 1
	}
}

func (m MultiPageViewModel) selectedFileMenuPlace() (fileMenuPlace, bool) {
	if m.fileMenuPlacesCursor < 0 {
		return fileMenuPlace{}, false
	}
	if m.fileMenuPlacesCursor < len(m.fileMenuPinnedPlaces) {
		return m.fileMenuPinnedPlaces[m.fileMenuPlacesCursor], true
	}
	index := m.fileMenuPlacesCursor - len(m.fileMenuPinnedPlaces)
	if index >= len(m.fileMenuHomePlaces) {
		return fileMenuPlace{}, false
	}
	return m.fileMenuHomePlaces[index], true
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
	m.fileMenuNameCursor = 0
	m.fileMenuViewportStart = 0
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
	case fileMenuProcessingMode:
		return m, nil
	default:
		return m.updateFileMenuNormal(msg)
	}
}

func (m MultiPageViewModel) updateFileMenuNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == " " || key == "space" {
		m.fileMenuLeaderPending = true
		m.fileMenuPendingKey = ""
		m.fileMenuStatus = "<leader>"
		return m, nil
	}
	if m.fileMenuLeaderPending {
		m.fileMenuLeaderPending = false
		switch key {
		case "left":
			m.loadFileMenuPlaces()
			m.fileMenuFocus = fileMenuPlacesFocus
			m.fileMenuStatus = "places"
			return m, nil
		case "right":
			m.fileMenuFocus = fileMenuTreeFocus
			m.fileMenuStatus = "files"
			return m, nil
		}
	}

	if m.fileMenuFocus == fileMenuPlacesFocus {
		placeCount := len(m.fileMenuPinnedPlaces) + len(m.fileMenuHomePlaces)
		switch key {
		case "up", "k":
			if placeCount > 0 {
				m.fileMenuPlacesCursor = (m.fileMenuPlacesCursor - 1 + placeCount) % placeCount
			}
			return m, nil
		case "down", "j":
			if placeCount > 0 {
				m.fileMenuPlacesCursor = (m.fileMenuPlacesCursor + 1) % placeCount
			}
			return m, nil
		case "enter":
			if len(m.fileMenuOperations) > 0 {
				m.fileMenuStatus = "save or discard pending changes before changing location"
				return m, nil
			}
			place, ok := m.selectedFileMenuPlace()
			if !ok {
				return m, nil
			}
			if m.loadFileMenuDirectory(place.path) {
				m.setFileMenuRoot(place.path)
				m.fileMenuFocus = fileMenuTreeFocus
				m.fileMenuStatus = "opened " + place.label
			}
			return m, nil
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
		case ":":
			m.fileMenuMode = fileMenuCommandMode
			m.fileMenuCommand = ""
			m.fileMenuStatus = ""
			return m, nil
		default:
			return m, nil
		}
	}

	if key == "g" {
		if m.fileMenuPendingKey == "g" {
			m.fileMenuPendingKey = ""
			m.jumpFileMenuCursor(0)
		} else {
			m.fileMenuPendingKey = "g"
		}
		return m, nil
	}
	if key == "d" || key == "y" || key == "m" {
		if m.fileMenuPendingKey != key {
			m.fileMenuPendingKey = key
			m.fileMenuStatus = key
			return m, nil
		}

		m.fileMenuPendingKey = ""
		switch key {
		case "d":
			m.stageFileMenuDelete(m.selectedFileMenuEntries(), false)
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

	case "G":
		m.jumpFileMenuCursor(len(m.fileMenuEntries) - 1)
		return m, nil

	case "h", "home", "0":
		m.moveFileMenuNameCursor(-m.fileMenuNameCursor)
		return m, nil

	case "l":
		m.moveFileMenuNameCursor(1)
		return m, nil

	case "end", "$":
		m.moveFileMenuNameCursor(len(m.currentFileMenuEntryName()))
		return m, nil

	case "left":
		if filepath.Clean(m.fileMenuPath) == filepath.Clean(m.fileMenuRoot) {
			return m, nil
		}
		parent := filepath.Dir(m.fileMenuPath)
		if !fileMenuPathContains(m.fileMenuRoot, parent) {
			return m, nil
		}
		m.loadFileMenuDirectory(parent)
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
		if m.fileMenuPreview.err != nil {
			m.fileMenuStatus = "warning: unable to verify that the selected file is text"
			return m, nil
		}
		if m.fileMenuPreview.binary {
			m.fileMenuStatus = "warning: only text files can be opened in nvim"
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

	case "o":
		m.beginFileMenuCreateInput()
		return m, nil

	case "i", "a", "I", "A":
		if len(m.fileMenuEntries) == 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
			m.beginFileMenuCreateInput()
			return m, nil
		}
		entry := m.fileMenuEntries[m.fileMenuCursor]
		entryPath := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if m.isFileMenuProtectedPath(entryPath) {
			m.fileMenuStatus = "protected shortcut: cannot rename or move " + entry.name
			return m, nil
		}
		m.fileMenuMode = fileMenuInsertMode
		m.fileMenuInputAction = fileMenuRenameInput
		m.fileMenuInput = entry.name
		switch key {
		case "a":
			m.fileMenuInputCursor = m.fileMenuNameCursor + 1
		case "I":
			m.fileMenuInputCursor = 0
		case "A":
			m.fileMenuInputCursor = len([]rune(entry.name))
		default:
			m.fileMenuInputCursor = m.fileMenuNameCursor
		}
		m.fileMenuInputSource = entryPath
		m.fileMenuInputKind = entry.kind
		m.fileMenuStatus = "edit name or relative path"
		return m, nil

	case "p":
		m.stageFileMenuPaste()
		return m, nil

	case "D":
		m.stageFileMenuDelete(m.selectedFileMenuEntries(), true)
		return m, nil

	case ":":
		m.fileMenuMode = fileMenuCommandMode
		m.fileMenuCommand = ""
		m.fileMenuStatus = ""
		return m, nil
	}

	return m, nil
}

func (m *MultiPageViewModel) beginFileMenuCreateInput() {
	m.fileMenuMode = fileMenuInsertMode
	m.fileMenuInputAction = fileMenuCreateInput
	m.fileMenuInput = ""
	m.fileMenuInputCursor = 0
	m.fileMenuInputSource = ""
	m.fileMenuStatus = "new entry; end with / for a directory"
}

func (m *MultiPageViewModel) setFileMenuRoot(path string) {
	m.fileMenuRoot = filepath.Clean(path)
	switch operator := m.fileMenuOperator.(type) {
	case osFileMenuOperator:
		m.fileMenuOperator = operator.allowRoot(m.fileMenuRoot)
	case *osFileMenuOperator:
		*operator = operator.allowRoot(m.fileMenuRoot)
	}
}

func (m *MultiPageViewModel) updateFileMenuVisual(msg tea.KeyMsg) {
	key := msg.String()
	if key == "g" {
		if m.fileMenuPendingKey == "g" {
			m.fileMenuPendingKey = ""
			m.jumpFileMenuCursor(0)
		} else {
			m.fileMenuPendingKey = "g"
		}
		return
	}
	m.fileMenuPendingKey = ""

	switch key {
	case "esc", "v", "V":
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuStatus = ""
	case "up", "k":
		m.moveFileMenuCursor(-1)
	case "down", "j":
		m.moveFileMenuCursor(1)
	case "G":
		m.jumpFileMenuCursor(len(m.fileMenuEntries) - 1)
	case "y":
		m.setFileMenuClipboard(m.selectedFileMenuEntries(), false)
		m.fileMenuMode = fileMenuNormalMode
	case "m":
		m.setFileMenuClipboard(m.selectedFileMenuEntries(), true)
		m.fileMenuMode = fileMenuNormalMode
	case "d":
		m.stageFileMenuDelete(m.selectedFileMenuEntries(), false)
		m.fileMenuMode = fileMenuNormalMode
	case "D":
		m.stageFileMenuDelete(m.selectedFileMenuEntries(), true)
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
		normalCursor := fileMenuNormalCursorAfterInsert(m.fileMenuInput, m.fileMenuInputCursor)
		if err := m.commitFileMenuInput(); err != nil {
			m.fileMenuStatus = err.Error()
			return
		}
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		m.fileMenuNameCursor = normalCursor
		m.clampFileMenuNameCursor()
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
		normalCursor := fileMenuNormalCursorAfterInsert(m.fileMenuInput, m.fileMenuInputCursor)
		if err := m.commitFileMenuInput(); err != nil {
			m.fileMenuStatus = err.Error()
			return
		}
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuInput = ""
		m.fileMenuInputCursor = 0
		m.fileMenuNameCursor = normalCursor
		m.clampFileMenuNameCursor()
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

func fileMenuNormalCursorAfterInsert(value string, insertCursor int) int {
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, string(filepath.Separator))
	base := filepath.Base(value)
	prefixLength := len([]rune(value)) - len([]rune(base))
	position := insertCursor - prefixLength - 1
	if position < 0 {
		return 0
	}
	if nameLength := len([]rune(base)); nameLength > 0 && position >= nameLength {
		return nameLength - 1
	}
	return position
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
		case "w", "x", "wq":
			if len(m.fileMenuOperations) == 0 {
				if command == "w" {
					m.fileMenuMode = fileMenuNormalMode
					m.fileMenuStatus = "no pending changes"
					return m, nil
				}
				*m.selected = utils.ExitSignal
				m.quitting = true
				return m, tea.Quit
			}
			operations := append([]fileMenuOperation(nil), m.fileMenuOperations...)
			operator := m.fileMenuOperator
			m.fileMenuMode = fileMenuProcessingMode
			m.fileMenuProcessState = fileMenuProcessRunning
			m.fileMenuProgress = fileMenuOperationProgress{
				kind:            operations[0].kind,
				currentFile:     fileMenuOperationName(operations[0]),
				totalItems:      int64(len(operations)),
				operation:       1,
				totalOperations: len(operations),
			}
			m.fileMenuQuitAfterWrite = command != "w"
			m.fileMenuStatus = fmt.Sprintf("processing %d operation(s)", len(operations))
			return m, startFileMenuOperations(operator, operations)
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

func startFileMenuOperations(operator fileMenuOperator, operations []fileMenuOperation) tea.Cmd {
	messages := make(chan tea.Msg, 1)
	return func() tea.Msg {
		go func() {
			result := operator.Apply(operations, func(update fileMenuOperationProgress) {
				message := fileMenuOperationProgressMsg{progress: update, messages: messages}
				select {
				case messages <- message:
				default:
				}
			})
			messages <- fileMenuOperationFinishedMsg{result: result}
			close(messages)
		}()
		return <-messages
	}
}

func waitForFileMenuOperationMessage(messages <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-messages
	}
}

func (m *MultiPageViewModel) finishFileMenuOperations(result fileMenuApplyResult) tea.Cmd {
	if m.fileMenuMode != fileMenuProcessingMode {
		return nil
	}
	completed := result.completed
	if completed < 0 {
		completed = 0
	}
	if completed > len(m.fileMenuOperations) {
		completed = len(m.fileMenuOperations)
	}
	if result.progress.totalItems > 0 {
		m.fileMenuProgress = result.progress
	}
	completedOperations := append([]fileMenuOperation(nil), m.fileMenuOperations[:completed]...)
	if result.err != nil {
		m.fileMenuOperations = append([]fileMenuOperation(nil), m.fileMenuOperations[completed:]...)
		m.removeCompletedFileMenuClipboardEntries(completedOperations)
		m.fileMenuMode = fileMenuNormalMode
		m.fileMenuProcessState = fileMenuProcessFailed
		m.fileMenuQuitAfterWrite = false
		m.loadFileMenuDirectory(m.fileMenuPath)
		m.fileMenuStatus = fmt.Sprintf("saved %d; %d pending; failed: %v", completed, len(m.fileMenuOperations), result.err)
		return nil
	}
	count := len(m.fileMenuOperations)
	quitAfterWrite := m.fileMenuQuitAfterWrite
	m.fileMenuOperations = nil
	m.fileMenuQuitAfterWrite = false
	m.fileMenuProcessState = fileMenuProcessSuccessful
	m.fileMenuProgress.doneItems = m.fileMenuProgress.totalItems
	m.fileMenuProgress.doneBytes = m.fileMenuProgress.totalBytes
	if m.fileMenuClipboardCut {
		m.fileMenuClipboard = nil
		m.fileMenuClipboardMove = false
		m.fileMenuClipboardCut = false
	}
	m.fileMenuMode = fileMenuNormalMode
	m.loadFileMenuDirectory(m.fileMenuPath)
	m.fileMenuStatus = fmt.Sprintf("saved %d operation(s)", count)
	if quitAfterWrite {
		*m.selected = utils.ExitSignal
		m.quitting = true
		return tea.Quit
	}
	return nil
}

func (m *MultiPageViewModel) removeCompletedFileMenuClipboardEntries(completed []fileMenuOperation) {
	if !m.fileMenuClipboardCut || len(completed) == 0 {
		return
	}
	completedSources := make(map[string]struct{}, len(completed))
	for _, operation := range completed {
		if operation.source != "" {
			completedSources[operation.source] = struct{}{}
		}
	}
	clipboard := m.fileMenuClipboard[:0]
	for _, entry := range m.fileMenuClipboard {
		if _, wasCompleted := completedSources[entry.path]; !wasCompleted {
			clipboard = append(clipboard, entry)
		}
	}
	m.fileMenuClipboard = clipboard
	if len(m.fileMenuClipboard) == 0 {
		m.fileMenuClipboardMove = false
		m.fileMenuClipboardCut = false
	}
}

func (m *MultiPageViewModel) moveFileMenuCursor(delta int) {
	if len(m.fileMenuEntries) == 0 {
		return
	}
	m.fileMenuCursor = (m.fileMenuCursor + delta + len(m.fileMenuEntries)) % len(m.fileMenuEntries)
	m.clampFileMenuNameCursor()
	m.adjustFileMenuViewport()
	m.refreshFileMenuPreview()
}

func (m *MultiPageViewModel) jumpFileMenuCursor(index int) {
	if len(m.fileMenuEntries) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.fileMenuEntries) {
		index = len(m.fileMenuEntries) - 1
	}
	m.fileMenuCursor = index
	m.clampFileMenuNameCursor()
	m.adjustFileMenuViewport()
	m.refreshFileMenuPreview()
}

func (m *MultiPageViewModel) moveFileMenuNameCursor(delta int) {
	nameLength := len([]rune(m.currentFileMenuEntryName()))
	if nameLength == 0 {
		m.fileMenuNameCursor = 0
		return
	}
	m.fileMenuNameCursor += delta
	if m.fileMenuNameCursor < 0 {
		m.fileMenuNameCursor = 0
	}
	if m.fileMenuNameCursor >= nameLength {
		m.fileMenuNameCursor = nameLength - 1
	}
}

func (m MultiPageViewModel) currentFileMenuEntryName() string {
	if len(m.fileMenuEntries) == 0 || m.fileMenuCursor < 0 || m.fileMenuCursor >= len(m.fileMenuEntries) {
		return ""
	}
	return m.fileMenuEntries[m.fileMenuCursor].name
}

func (m *MultiPageViewModel) clampFileMenuNameCursor() {
	m.moveFileMenuNameCursor(0)
}

func (m *MultiPageViewModel) adjustFileMenuViewport() {
	if len(m.fileMenuEntries) == 0 {
		m.fileMenuViewportStart = 0
		return
	}

	capacity := m.fileMenuTopContentHeight() - 1
	if capacity < 1 {
		capacity = 1
	}
	offset := fileMenuScrollOffset
	if maxOffset := (capacity - 1) / 2; offset > maxOffset {
		offset = maxOffset
	}

	if m.fileMenuCursor < m.fileMenuViewportStart+offset {
		m.fileMenuViewportStart = m.fileMenuCursor - offset
	} else if m.fileMenuCursor >= m.fileMenuViewportStart+capacity-offset {
		m.fileMenuViewportStart = m.fileMenuCursor - capacity + offset + 1
	}

	maxStart := len(m.fileMenuEntries) - capacity
	if maxStart < 0 {
		maxStart = 0
	}
	if m.fileMenuViewportStart < 0 {
		m.fileMenuViewportStart = 0
	}
	if m.fileMenuViewportStart > maxStart {
		m.fileMenuViewportStart = maxStart
	}
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
	if move {
		for _, entry := range entries {
			path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
			if m.isFileMenuProtectedPath(path) {
				m.fileMenuStatus = "protected shortcut: cannot move " + entry.name
				return
			}
		}
	}
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

func (m *MultiPageViewModel) stageFileMenuDelete(entries []fileMenuEntry, permanent bool) {
	if len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if m.isFileMenuProtectedPath(path) {
			m.fileMenuStatus = "protected shortcut: cannot delete " + entry.name
			return
		}
	}
	if !permanent {
		m.setFileMenuClipboard(entries, true)
		m.fileMenuClipboardCut = true
	}
	added := 0
	for _, entry := range entries {
		path := filepath.Clean(filepath.Join(m.fileMenuPath, entry.name))
		if m.hasFileMenuOperation(fileMenuDelete, path, "") {
			continue
		}
		m.fileMenuOperations = append(m.fileMenuOperations, fileMenuOperation{kind: fileMenuDelete, source: path, permanent: permanent})
		added++
	}
	if permanent {
		m.fileMenuStatus = fmt.Sprintf("staged permanent deletion of %d entrie(s); :w applies", added)
	} else {
		m.fileMenuStatus = fmt.Sprintf("cut %d entrie(s); p moves, :w moves to trash", added)
	}
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

	directoryIntent := strings.HasSuffix(value, string(filepath.Separator))
	if directoryIntent {
		value = strings.TrimRight(value, string(filepath.Separator))
	}
	destination, err := resolveFileMenuPath(m.fileMenuRoot, m.fileMenuPath, value)
	if err != nil {
		return err
	}
	if m.fileMenuInputAction == fileMenuRenameInput && m.isFileMenuProtectedPath(m.fileMenuInputSource) {
		return fmt.Errorf("protected shortcut: cannot rename or move %s", filepath.Base(m.fileMenuInputSource))
	}
	if m.fileMenuInputAction == fileMenuRenameInput && directoryIntent && m.fileMenuInputKind != fileMenuDirectory {
		if destination != m.fileMenuInputSource && m.fileMenuDestinationExists(destination) {
			return fmt.Errorf("destination already exists: %s", destination)
		}
		if m.convertPendingFileMenuCreationToDirectory(m.fileMenuInputSource, destination) {
			m.rebuildFileMenuEntries()
			m.selectFileMenuPath(destination)
			m.fileMenuStatus = "staged directory creation: " + destination
			return nil
		}
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
		if directoryIntent {
			kind = fileMenuCreateDirectory
		}
		entryKind := fileMenuRegularFile
		if directoryIntent {
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

func (m *MultiPageViewModel) convertPendingFileMenuCreationToDirectory(source, destination string) bool {
	for index := range m.fileMenuOperations {
		operation := &m.fileMenuOperations[index]
		if operation.kind != fileMenuCreateFile || operation.destination != source {
			continue
		}
		operation.kind = fileMenuCreateDirectory
		operation.destination = destination
		operation.entryKind = fileMenuDirectory
		return true
	}
	return false
}

func resolveFileMenuPath(root, directory, value string) (string, error) {
	target := filepath.Clean(filepath.Join(directory, value))
	relative, err := filepath.Rel(root, target)
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

func (m MultiPageViewModel) isFileMenuProtectedPath(path string) bool {
	path = filepath.Clean(path)
	for _, protectedPath := range m.fileMenuProtectedPaths {
		if path == filepath.Clean(protectedPath) {
			return true
		}
	}
	return false
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
		entries[index].pendingKind = 0
	}

	addDestination := func(operation fileMenuOperation) {
		if filepath.Dir(operation.destination) != m.fileMenuPath {
			return
		}
		entries = append(entries, fileMenuEntry{
			name:        filepath.Base(operation.destination),
			kind:        operation.entryKind,
			pending:     true,
			pendingKind: operation.kind,
		})
	}
	markSourceDeleted := func(operation fileMenuOperation) {
		if filepath.Dir(operation.source) != m.fileMenuPath {
			return
		}
		name := filepath.Base(operation.source)
		for index := range entries {
			if entries[index].name == name {
				entries[index].pending = true
				entries[index].pendingKind = fileMenuDelete
				return
			}
		}
	}

	for _, operation := range m.fileMenuOperations {
		switch operation.kind {
		case fileMenuCreateFile, fileMenuCreateDirectory, fileMenuCopy:
			addDestination(operation)
		case fileMenuMove:
			markSourceDeleted(operation)
			addDestination(operation)
		case fileMenuDelete:
			markSourceDeleted(operation)
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
			m.fileMenuNameCursor = 0
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
	m.clampFileMenuNameCursor()
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
	contentType := http.DetectContentType(content)
	if !strings.HasPrefix(contentType, "text/") &&
		contentType != "application/json" &&
		contentType != "application/xml" &&
		contentType != "application/javascript" {
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
	placesContent := m.renderFileMenuPlaces()
	treeContent := m.renderFileTree()
	previewContent := m.renderFilePreview()
	contentHeight := m.fileMenuTopContentHeight()
	placesContent = fitFileMenuPanelHeight(placesContent, contentHeight)
	treeContent = fitFileMenuTreeHeight(treeContent, contentHeight, m.fileMenuViewportStart)
	previewContent = fitFileMenuPanelHeight(previewContent, contentHeight)

	boxStyle := func(width int, focused bool) lipgloss.Style {
		borderColor := m.styles.MutedBorderColor
		if focused {
			borderColor = m.styles.SelectedTitleColor
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			Width(width).
			Height(contentHeight)
	}

	outerHeight := contentHeight + 2
	placesBox := renderFixedHeightFileMenuBox(boxStyle(m.fileMenuPlacesContentWidth(), m.fileMenuFocus == fileMenuPlacesFocus), placesContent, outerHeight)
	treeBox := renderFixedHeightFileMenuBox(boxStyle(contentWidth, m.fileMenuFocus == fileMenuTreeFocus), treeContent, outerHeight)
	previewBox := renderFixedHeightFileMenuBox(boxStyle(contentWidth, false), previewContent, outerHeight)

	boxes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		placesBox,
		strings.Repeat(" ", fileMenuBoxGap),
		treeBox,
		strings.Repeat(" ", fileMenuBoxGap),
		previewBox,
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
	placesOuterWidth := m.fileMenuPlacesContentWidth() + fileMenuBoxChrome
	boxWidth := (m.contentWidth() - 2*fileMenuBoxGap - placesOuterWidth) / 2
	width := boxWidth - fileMenuBoxChrome
	if width < 1 {
		return 1
	}
	return width
}

func (m MultiPageViewModel) fileMenuPlacesContentWidth() int {
	outerWidth := fileMenuPlacesOuterWidth
	if maximum := m.contentWidth() / 3; outerWidth > maximum {
		outerWidth = maximum
	}
	width := outerWidth - fileMenuBoxChrome
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

func fitFileMenuTreeHeight(content string, height, viewportStart int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= height || height < 2 {
		return fitFileMenuPanelHeight(content, height)
	}

	body := lines[1:]
	capacity := height - 1
	start := viewportStart
	if start < 0 {
		start = 0
	}
	if maxStart := len(body) - capacity; start > maxStart {
		start = maxStart
	}
	end := start + capacity
	if end > len(body) {
		end = len(body)
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
	titleText := "Processes"
	if m.fileMenuProgress.totalItems > 0 {
		done := m.fileMenuProgress.doneItems
		if done > m.fileMenuProgress.totalItems {
			done = m.fileMenuProgress.totalItems
		}
		titleText += fmt.Sprintf("  %d/%d", done, m.fileMenuProgress.totalItems)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor).Render(titleText)
	if m.fileMenuProcessState == fileMenuProcessIdle {
		return title + "\n\n   No processes running"
	}

	verb := fileMenuProgressVerb(m.fileMenuProgress.kind, m.fileMenuProcessState)
	name := m.fileMenuProgress.currentFile
	if name == "" {
		name = "files"
	}
	stateIcon := "󰥔"
	stateColor := m.styles.PeachColor
	switch m.fileMenuProcessState {
	case fileMenuProcessSuccessful:
		stateIcon = ""
		stateColor = m.styles.AquamarineColor
	case fileMenuProcessFailed:
		stateIcon = ""
		stateColor = m.styles.ErrorColor
	}
	operationLine := fmt.Sprintf("  %s %s %s %s",
		fileMenuProgressOperationIcon(m.fileMenuProgress.kind),
		verb,
		truncateSingleLine(name, max(1, width-15)),
		lipgloss.NewStyle().Foreground(stateColor).Render(stateIcon),
	)

	barWidth := max(4, width-2)
	bar := progress.New(
		progress.WithScaledGradient(string(m.styles.OrchidColor), string(m.styles.ThistleColor)),
		progress.WithWidth(barWidth),
		progress.WithFillCharacters('█', '░'),
	)
	bar.PercentageStyle = lipgloss.NewStyle().Foreground(m.styles.FooterColor)
	return title + "\n" + operationLine + "\n  " + bar.ViewAs(m.fileMenuProgress.ratio())
}

func fileMenuProgressOperationIcon(kind fileMenuOperationKind) string {
	switch kind {
	case fileMenuCopy:
		return "󰆏"
	case fileMenuMove:
		return "󰆐"
	case fileMenuDelete:
		return "󰆴"
	default:
		return "󰥔"
	}
}

func fileMenuProgressVerb(kind fileMenuOperationKind, state fileMenuProcessState) string {
	if state == fileMenuProcessSuccessful {
		switch kind {
		case fileMenuCopy:
			return "Copied"
		case fileMenuMove:
			return "Moved"
		case fileMenuDelete:
			return "Deleted"
		default:
			return "Created"
		}
	}
	if state == fileMenuProcessFailed {
		return "Failed"
	}
	switch kind {
	case fileMenuCopy:
		return "Copying"
	case fileMenuMove:
		return "Moving"
	case fileMenuDelete:
		return "Deleting"
	default:
		return "Creating"
	}
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
	if m.fileMenuMode == fileMenuNormalMode && m.fileMenuFocus == fileMenuPlacesFocus {
		mode = "-- PLACES --"
	}
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
	case fileMenuProcessingMode:
		mode = "-- PROCESSING --"
	}

	if prompt == "" {
		if m.fileMenuFocus == fileMenuPlacesFocus {
			prompt = "shortcuts · j/k select · enter open · space+→ files"
		} else {
			prompt = "o new · h/l move · i/a edit · V select · yy copy · dd trash · D permanent · p paste · :w save"
		}
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

func renderFileMenuTextCursor(text string, position int, color lipgloss.Color) string {
	runes := []rune(text)
	if position < 0 {
		position = 0
	}
	if position > len(runes) {
		position = len(runes)
	}

	textStyle := lipgloss.NewStyle().Foreground(color)
	cursorStyle := textStyle.Reverse(true)
	before := textStyle.Render(string(runes[:position]))
	if position == len(runes) {
		return before + cursorStyle.Render(" ")
	}
	return before + cursorStyle.Render(string(runes[position])) + textStyle.Render(string(runes[position+1:]))
}

func (m MultiPageViewModel) renderFilePreview() string {
	contentWidth := m.fileMenuTopContentWidth()
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

func (m MultiPageViewModel) renderFileMenuPlaces() string {
	width := m.fileMenuPlacesContentWidth()
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor)
	itemStyle := lipgloss.NewStyle().Foreground(m.styles.FileMenuFileColor)
	mutedStyle := lipgloss.NewStyle().Foreground(m.styles.FooterColor)
	lines := []string{headingStyle.Render("Pinned")}

	placeIndex := 0
	appendPlace := func(place fileMenuPlace, icon string) {
		prefix := "  "
		style := itemStyle
		if m.fileMenuFocus == fileMenuPlacesFocus && placeIndex == m.fileMenuPlacesCursor {
			prefix = "› "
			style = lipgloss.NewStyle().Bold(true).Foreground(m.styles.SelectedTitleColor)
		}
		labelWidth := width - lipgloss.Width(prefix+icon+" ")
		label := truncateSingleLine(place.label, labelWidth)
		lines = append(lines, mutedStyle.Render(prefix)+style.Render(icon+" "+label))
		placeIndex++
	}

	for _, place := range m.fileMenuPinnedPlaces {
		appendPlace(place, place.icon)
	}
	lines = append(lines, "", headingStyle.Render("Home"))
	for _, place := range m.fileMenuHomePlaces {
		appendPlace(place, place.icon)
	}
	if len(m.fileMenuPinnedPlaces)+len(m.fileMenuHomePlaces) == 0 && m.fileMenuPlacesError != nil {
		lines = append(lines, mutedStyle.Render("  "+truncateSingleLine("unable to read home", width-2)))
	}
	return strings.Join(lines, "\n")
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
		editingName := m.fileMenuMode == fileMenuInsertMode && m.fileMenuInputAction == fileMenuRenameInput && entryPath == m.fileMenuInputSource
		if editingName {
			label = m.fileMenuInput
		}
		label = truncateSingleLine(label, labelWidth)
		marker := "  "
		branchColor := m.styles.DividerColor
		if m.fileMenuEntryIsPendingDelete(entry) {
			marker = "D "
			branchColor = m.styles.FileMenuDeletedColor
		} else if entry.pending {
			marker = "+ "
			branchColor = color
		}
		if m.fileMenuEntryIsVisuallySelected(index) {
			marker = "◆ "
			branchColor = m.styles.SelectedTitleColor
		}
		branch := lipgloss.NewStyle().
			Foreground(branchColor).
			Render(marker + connector + " ")
		iconText := lipgloss.NewStyle().Foreground(color).Render(icon + " ")
		labelText := lipgloss.NewStyle().Foreground(color).Render(label)
		if editingName {
			labelText = renderFileMenuTextCursor(label, m.fileMenuInputCursor, color)
		} else if m.fileMenuFocus == fileMenuTreeFocus && index == m.fileMenuCursor && (m.fileMenuMode == fileMenuNormalMode || m.fileMenuMode == fileMenuVisualMode) {
			labelText = renderFileMenuTextCursor(label, m.fileMenuNameCursor, color)
		}
		lines = append(lines, branch+iconText+labelText)
	}

	if hasDraftEntry {
		draftKind := fileMenuRegularFile
		if strings.HasSuffix(m.fileMenuInput, string(filepath.Separator)) {
			draftKind = fileMenuDirectory
		}
		icon, _, color := m.fileMenuEntryPresentation(fileMenuEntry{kind: draftKind})
		label := truncateSingleLine(m.fileMenuInput, labelWidth)
		branch := lipgloss.NewStyle().
			Foreground(m.styles.SelectedTitleColor).
			Render("  └─ ")
		iconText := lipgloss.NewStyle().Foreground(color).Render(icon + " ")
		labelText := renderFileMenuTextCursor(label, m.fileMenuInputCursor, color)
		lines = append(lines, branch+iconText+labelText)
	} else if len(m.fileMenuEntries) == 0 && m.fileMenuError == nil && m.fileMenuFocus == fileMenuTreeFocus && m.fileMenuMode == fileMenuNormalMode {
		_, _, color := m.fileMenuEntryPresentation(fileMenuEntry{kind: fileMenuRegularFile})
		branch := lipgloss.NewStyle().Foreground(m.styles.DividerColor).Render("  └─ ")
		icon := lipgloss.NewStyle().Foreground(color).Render("• ")
		lines = append(lines, branch+icon+renderFileMenuTextCursor("", 0, color))
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
	return entry.pending && entry.pendingKind == fileMenuDelete
}

func (m MultiPageViewModel) fileMenuEntryPresentation(entry fileMenuEntry) (icon, label string, color lipgloss.Color) {
	switch entry.kind {
	case fileMenuDirectory:
		icon, label, color = "▸", entry.name+"/", m.styles.FileMenuDirectoryColor
	case fileMenuExecutable:
		icon, label, color = "◆", entry.name, m.styles.FileMenuExecutableColor
	case fileMenuImage:
		icon, label, color = "◇", entry.name, m.styles.FileMenuImageColor
	default:
		icon, label, color = "•", entry.name, m.styles.FileMenuFileColor
	}
	if !entry.pending {
		return icon, label, color
	}
	switch entry.pendingKind {
	case fileMenuDelete:
		color = m.styles.FileMenuDeletedColor
	case fileMenuCopy, fileMenuMove:
		color = m.styles.FileMenuCopiedColor
	case fileMenuCreateFile, fileMenuCreateDirectory:
		color = m.styles.FileMenuAddedColor
	}
	return icon, label, color
}
