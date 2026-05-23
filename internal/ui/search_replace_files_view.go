package ui

import (
	"fmt"
	"os"
	"strings"

	"terminal-gameplay/internal/tools"
	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const minSearchReplaceVisibleItems = 10

type SearchReplaceFilesViewModel struct {
	title         string
	items         []utils.ListItem
	cursor        int
	viewportStart int
	maxVisible    int
	endValue      *utils.ListItem
	quitting      bool
	pendingAll    bool
	styles        *Styles
	terminalWidth int
}

func (m SearchReplaceFilesViewModel) Init() tea.Cmd {
	return nil
}

func (m SearchReplaceFilesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if len(m.items) == 0 || m.cursor >= len(m.items) {
				return m, nil
			}

			*m.endValue = m.items[m.cursor]
			m.quitting = true
			return m, tea.Quit

		case "a":
			if m.pendingAll {
				*m.endValue = utils.ListItem{T: tools.ReplaceAllAction}
				m.quitting = true
				return m, tea.Quit
			}

			m.pendingAll = true
			return m, nil

		case "up", "k":
			m.pendingAll = false
			m.moveCursor(-1)
			return m, nil

		case "down", "j":
			m.pendingAll = false
			m.moveCursor(1)
			return m, nil

		case "ctrl+c", "esc", "q":
			*m.endValue = utils.ListItem{T: utils.ExitSignal}
			m.quitting = true
			return m, tea.Quit

		default:
			m.pendingAll = false
		}
	}

	return m, nil
}

func (m *SearchReplaceFilesViewModel) moveCursor(delta int) {
	if len(m.items) == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(m.items) - 1
	} else if m.cursor >= len(m.items) {
		m.cursor = 0
	}

	if len(m.items) <= m.maxVisible {
		m.viewportStart = 0
		return
	}

	if m.cursor < m.viewportStart {
		m.viewportStart = m.cursor
	} else if m.cursor >= m.viewportStart+m.maxVisible {
		m.viewportStart = m.cursor - m.maxVisible + 1
	}
}

func (m SearchReplaceFilesViewModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.styles.TitleStyle.Render(m.title))
	b.WriteString("\n\n")

	visibleEnd := m.viewportStart + m.maxVisible
	if visibleEnd > len(m.items) {
		visibleEnd = len(m.items)
	}

	fileColumnWidth := m.fileColumnWidth(m.items[m.viewportStart:visibleEnd])
	for i := m.viewportStart; i < visibleEnd; i++ {
		b.WriteString(m.renderFileRow(m.items[i], i, fileColumnWidth))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderPosition())
	b.WriteString("\n")
	b.WriteString(m.renderHelp())
	return b.String()
}

func (m SearchReplaceFilesViewModel) renderFileRow(item utils.ListItem, index, fileColumnWidth int) string {
	selected := m.cursor == index
	width := m.contentWidth()
	statusText := ""
	if item.Status != "" {
		statusRaw := item.Status
		statusText = lipgloss.NewStyle().
			Foreground(m.styles.AquamarineColor).
			Bold(selected).
			Render(statusRaw)
	}

	prefix := "  "
	borderColor := m.styles.MutedBorderColor
	fileColor := m.styles.NyanzaColor
	if !selected {
		fileColor = m.styles.MutedTitleColor
	}

	if selected {
		prefix = "▌ "
		borderColor = m.styles.SelectedTitleColor
	}

	fileName := lipgloss.NewStyle().
		Foreground(fileColor).
		Bold(selected).
		Width(fileColumnWidth).
		Render(truncateSingleLine(item.T, fileColumnWidth))

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Foreground(borderColor).Render(prefix),
		fileName,
		"   ",
		statusText,
	)

	if selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#313244")).
			Width(width).
			Render(row)
	}

	return lipgloss.NewStyle().Width(width).Render(row)
}

func (m SearchReplaceFilesViewModel) fileColumnWidth(items []utils.ListItem) int {
	statusWidth := lipgloss.Width("   " + tools.ReplacedStatus)
	maxWidth := m.contentWidth() - rowChromeWidth - statusWidth
	if maxWidth < 24 {
		return 24
	}

	width := 24
	for _, item := range items {
		itemWidth := lipgloss.Width(item.T)
		if itemWidth > width {
			width = itemWidth
		}
	}

	if width > maxWidth {
		return maxWidth
	}
	return width
}

func (m SearchReplaceFilesViewModel) renderPosition() string {
	if len(m.items) == 0 {
		return m.styles.FooterStyle.Render("0 files")
	}

	start := m.viewportStart + 1
	end := m.viewportStart + m.maxVisible
	if end > len(m.items) {
		end = len(m.items)
	}

	return m.styles.FooterStyle.Render(fmt.Sprintf("%d-%d / %d files", start, end, len(m.items)))
}

func (m SearchReplaceFilesViewModel) renderHelp() string {
	helpText := "enter replace selected file  /  aa replace all  /  q esc back to tools"
	if m.pendingAll {
		helpText = "press a again to replace all  /  esc cancel"
	}
	return m.styles.FooterStyle.Render(helpText + "\n")
}

func (m SearchReplaceFilesViewModel) contentWidth() int {
	width := m.terminalWidth - screenGutterWidth
	if m.terminalWidth <= 0 {
		width = defaultContentWidth
	}
	if width < minContentWidth {
		return minContentWidth
	}
	return width
}

func SearchReplaceFilesView(title string, op []utils.ListItem, height int, endValue *utils.ListItem) {
	styles := DefaultStyles()
	maxVisible := height
	if maxVisible < minSearchReplaceVisibleItems {
		maxVisible = minSearchReplaceVisibleItems
	}

	m := SearchReplaceFilesViewModel{
		title:         title,
		items:         op,
		cursor:        firstPendingSearchReplaceItem(op),
		viewportStart: 0,
		maxVisible:    maxVisible,
		endValue:      endValue,
		styles:        styles,
		terminalWidth: defaultContentWidth + screenGutterWidth,
	}

	if m.cursor >= m.maxVisible {
		m.viewportStart = m.cursor - m.maxVisible + 1
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("SearchReplaceFilesView -> ", err)
		os.Exit(1)
	}
}

func firstPendingSearchReplaceItem(items []utils.ListItem) int {
	for i, item := range items {
		if item.Status == "" {
			return i
		}
	}
	return 0
}
