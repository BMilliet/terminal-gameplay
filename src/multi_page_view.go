package src

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PageType int

const (
	WarpPage PageType = iota
	CommandsPage
	NotesPage
)

type MultiPageViewModel struct {
	config        *ConfigDTO
	currentPage   PageType
	warpList      []ListItem
	commandList   []ListItem
	notesList     []ListItem
	availPages    []PageType
	pageIndex     int
	cursor        int
	viewportStart int // First visible item index for scrolling
	maxVisible    int // Maximum items to show at once
	selected      *string
	quitting      bool
	styles        *Styles
}

func NewMultiPageViewModel(config *ConfigDTO) MultiPageViewModel {
	// Build list of available pages (non-empty)
	availPages := []PageType{}
	if len(config.Warp.Keys) > 0 {
		availPages = append(availPages, WarpPage)
	}
	if len(config.Commands.Keys) > 0 {
		availPages = append(availPages, CommandsPage)
	}
	if len(config.Notes.Keys) > 0 {
		availPages = append(availPages, NotesPage)
	}

	currentPage := WarpPage
	if len(availPages) > 0 {
		currentPage = availPages[0]
	}

	m := MultiPageViewModel{
		config:        config,
		currentPage:   currentPage,
		warpList:      ConfigItemsToListItems(config.Warp),
		commandList:   ConfigItemsToListItems(config.Commands),
		notesList:     ConfigItemsToListItems(config.Notes),
		availPages:    availPages,
		pageIndex:     0,
		cursor:        0,
		viewportStart: 0,
		maxVisible:    10, // Show max 10 items at a time
		quitting:      false,
		styles:        DefaultStyles(),
	}

	// Move cursor to first non-divider item
	items := m.getCurrentList()
	for m.cursor < len(items) && items[m.cursor].IsDiv {
		m.cursor++
	}

	return m
}

func (m MultiPageViewModel) Init() tea.Cmd {
	return nil
}

func (m MultiPageViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			*m.selected = ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "left", "h":
			// Navigate to previous page
			if m.pageIndex > 0 {
				m.pageIndex--
				m.currentPage = m.availPages[m.pageIndex]
				m.cursor = 0
				m.viewportStart = 0
				// Skip dividers at start of page
				items := m.getCurrentList()
				for m.cursor < len(items) && items[m.cursor].IsDiv {
					m.cursor++
				}
			}

		case "right", "l":
			// Navigate to next page
			if m.pageIndex < len(m.availPages)-1 {
				m.pageIndex++
				m.currentPage = m.availPages[m.pageIndex]
				m.cursor = 0
				m.viewportStart = 0
				// Skip dividers at start of page
				items := m.getCurrentList()
				for m.cursor < len(items) && items[m.cursor].IsDiv {
					m.cursor++
				}
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				// Skip dividers when navigating up
				items := m.getCurrentList()
				for m.cursor > 0 && items[m.cursor].IsDiv {
					m.cursor--
				}
				// Scroll up if cursor moves above viewport with offset
				if m.cursor < m.viewportStart+2 && m.viewportStart > 0 {
					m.viewportStart--
				}
			}

		case "down", "j":
			items := m.getCurrentList()
			if m.cursor < len(items)-1 {
				m.cursor++
				// Skip dividers when navigating down
				for m.cursor < len(items)-1 && items[m.cursor].IsDiv {
					m.cursor++
				}
				// Scroll down if cursor moves below viewport with offset
				if m.cursor >= m.viewportStart+m.maxVisible-2 {
					m.viewportStart++
				}
			}

		case "enter":
			items := m.getCurrentList()
			if len(items) > 0 && m.cursor < len(items) {
				selectedItem := items[m.cursor]
				result := fmt.Sprintf("%s|%s|%s", m.getPageName(), selectedItem.T, selectedItem.D)
				*m.selected = result
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m MultiPageViewModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header with tabs (only show non-empty pages)
	var tabViews []string
	for _, page := range m.availPages {
		pageName := m.getPageNameByType(page)
		if page == m.currentPage {
			tabViews = append(tabViews, m.styles.Text(fmt.Sprintf("[ %s ]", pageName), m.styles.SelectedTitleColor))
		} else {
			tabViews = append(tabViews, m.styles.Text(fmt.Sprintf("  %s  ", pageName), m.styles.TitleColor))
		}
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabViews...))
	b.WriteString("\n\n")

	// Current page items with borders
	items := m.getCurrentList()
	if len(items) == 0 {
		b.WriteString(m.styles.FooterStyle.Render("  No items configured\n"))
	} else {
		// Calculate visible range
		visibleEnd := m.viewportStart + m.maxVisible
		if visibleEnd > len(items) {
			visibleEnd = len(items)
		}

		// Show scroll indicator if there are more items above
		if m.viewportStart > 0 {
			b.WriteString(m.styles.FooterStyle.Render("  ⬆ More items above..."))
			b.WriteString("\n\n")
		}

		// Render only visible items
		for i := m.viewportStart; i < visibleEnd; i++ {
			item := items[i]
			
			// Check if this is a divider
			if item.IsDiv {
				// Render divider with subtle styling
				dividerText := fmt.Sprintf("─── %s ───", item.D)
				dividerStyle := lipgloss.NewStyle().
					Foreground(m.styles.DividerColor).
					Italic(true).
					Width(70).
					Align(lipgloss.Center)
				b.WriteString(dividerStyle.Render(dividerText))
				b.WriteString("\n")
				continue
			}
			
			// Style for item box
			var itemBox lipgloss.Style
			if m.cursor == i {
				// Selected item - with bright border and indented
				itemBox = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(m.styles.SelectedTitleColor).
					Padding(0, 1).
					Width(70).
					MarginLeft(2) // Indent selected item
			} else {
				// Unselected item - subtle border
				itemBox = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(m.styles.BorderColor).
					Padding(0, 1).
					Width(70)
			}

			// Title (label) - bold and prominent
			titleStyle := lipgloss.NewStyle().Bold(true)
			if m.cursor == i {
				titleStyle = titleStyle.Foreground(m.styles.SelectedTitleColor)
			} else {
				titleStyle = titleStyle.Foreground(m.styles.TitleColor)
			}

			// Value - smaller and wrapped
			valueStyle := lipgloss.NewStyle().
				Foreground(m.styles.FooterColor).
				Width(66). // Slightly less than box width for padding
				Italic(true)

			// Build content
			content := fmt.Sprintf("%s\n%s",
				titleStyle.Render(item.T),
				valueStyle.Render(item.D),
			)

			b.WriteString(itemBox.Render(content))
			b.WriteString("\n")
		}

		// Show scroll indicator if there are more items below
		if visibleEnd < len(items) {
			b.WriteString("\n")
			b.WriteString(m.styles.FooterStyle.Render("  ⬇ More items below..."))
		}
	}

	// Footer
	b.WriteString("\n")
	helpText := "  ↑↓/jk navigate • enter select • q/esc quit"
	if len(m.availPages) > 1 {
		helpText = "  ← →/hl switch • ↑↓/jk navigate • enter select • q/esc quit"
	}
	b.WriteString(m.styles.FooterStyle.Render(helpText + "\n"))

	return b.String()
}

func (m MultiPageViewModel) getCurrentList() []ListItem {
	switch m.currentPage {
	case WarpPage:
		return m.warpList
	case CommandsPage:
		return m.commandList
	case NotesPage:
		return m.notesList
	default:
		return []ListItem{}
	}
}

func (m MultiPageViewModel) getPageName() string {
	switch m.currentPage {
	case WarpPage:
		return "warp"
	case CommandsPage:
		return "commands"
	case NotesPage:
		return "notes"
	default:
		return ""
	}
}

func (m MultiPageViewModel) getPageNameByType(page PageType) string {
	switch page {
	case WarpPage:
		return "warp ⚡️"
	case CommandsPage:
		return "commands 🎮"
	case NotesPage:
		return "notes ✏️"
	default:
		return ""
	}
}

func MultiPageView(config *ConfigDTO, selected *string) {
	m := NewMultiPageViewModel(config)
	m.selected = selected

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("MultiPageView -> ", err)
		os.Exit(1)
	}
}
