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
	// Fuzzy find state
	searchMode   bool
	searchQuery  string
	filteredList []ListItem
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
		searchMode:    false,
		searchQuery:   "",
		filteredList:  []ListItem{},
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
		case "ctrl+c":
			*m.selected = ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "esc", "q":
			// If in search mode, exit search mode
			if m.searchMode {
				m.searchMode = false
				m.searchQuery = ""
				m.filteredList = []ListItem{}
				m.cursor = 0
				m.viewportStart = 0
				// Skip dividers at start
				items := m.getCurrentList()
				for m.cursor < len(items) && items[m.cursor].IsDiv {
					m.cursor++
				}
				return m, nil
			}
			// Otherwise quit
			*m.selected = ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "/":
			// Enter search mode
			if !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.filteredList = []ListItem{}
				m.cursor = 0
				m.viewportStart = 0
				return m, nil
			}

		case "backspace":
			// Handle backspace in search mode
			if m.searchMode && len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.updateFilteredList()
				m.cursor = 0
				m.viewportStart = 0
				return m, nil
			}

		case "left", "h":
			// Don't allow page navigation in search mode
			if m.searchMode {
				return m, nil
			}
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
			// Don't allow page navigation in search mode
			if m.searchMode {
				return m, nil
			}
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
			items := m.getCurrentList()
			if m.cursor > 0 {
				m.cursor--
				// Skip dividers when navigating up
				for m.cursor > 0 && items[m.cursor].IsDiv {
					m.cursor--
				}
				// Scroll up if cursor moves above viewport with offset
				if m.cursor < m.viewportStart+2 && m.viewportStart > 0 {
					m.viewportStart--
				}
			} else {
				// Wrap to last item
				m.cursor = len(items) - 1
				// Skip dividers from the end
				for m.cursor > 0 && items[m.cursor].IsDiv {
					m.cursor--
				}
				// Adjust viewport to show the last item
				if len(items) > m.maxVisible {
					m.viewportStart = len(items) - m.maxVisible
				} else {
					m.viewportStart = 0
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
			} else {
				// Wrap to first item
				m.cursor = 0
				// Skip dividers from the start
				for m.cursor < len(items)-1 && items[m.cursor].IsDiv {
					m.cursor++
				}
				// Reset viewport to top
				m.viewportStart = 0
			}

		case "enter":
			items := m.getActiveList()
			if len(items) > 0 && m.cursor < len(items) {
				selectedItem := items[m.cursor]
				result := fmt.Sprintf("%s|%s|%s", m.getPageName(), selectedItem.T, selectedItem.D)
				*m.selected = result
				m.quitting = true
				return m, tea.Quit
			}

		default:
			// Handle text input for search
			if m.searchMode {
				// Only accept single characters (letters, numbers, spaces, etc)
				key := msg.String()
				if len(key) == 1 {
					m.searchQuery += key
					m.updateFilteredList()
					m.cursor = 0
					m.viewportStart = 0
					return m, nil
				}
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
			tabViews = append(tabViews, m.styles.Text(fmt.Sprintf("  %s  ", pageName), m.styles.MutedTitleColor))
		}
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabViews...))
	b.WriteString("\n\n")

	// Show search box if in search mode
	if m.searchMode {
		searchBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.styles.SearchBoxColor).
			Padding(0, 1).
			Width(70).
			Foreground(m.styles.SearchTextColor)

		searchText := fmt.Sprintf("🔍 Search: %s", m.searchQuery)
		if m.searchQuery == "" {
			searchText = "🔍 Search: (type to search...)"
		}
		b.WriteString(searchBox.Render(searchText))
		b.WriteString("\n\n")
	}

	// Current page items with borders
	items := m.getActiveList()
	if len(items) == 0 {
		if m.searchMode {
			b.WriteString(m.styles.FooterStyle.Render("  No matches found\n"))
		} else {
			b.WriteString(m.styles.FooterStyle.Render("  No items configured\n"))
		}
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
					BorderForeground(m.styles.MutedBorderColor).
					Padding(0, 1).
					Width(70)
			}

			// Title (label) - bold and prominent
			titleStyle := lipgloss.NewStyle().Bold(true)
			var valueColor lipgloss.Color
			if m.cursor == i {
				titleStyle = titleStyle.Foreground(m.styles.SelectedTitleColor)
				valueColor = m.styles.FooterColor
			} else {
				titleStyle = titleStyle.Foreground(m.styles.MutedTitleColor)
				valueColor = m.styles.MutedTitleColor
			}

			// Value - smaller and wrapped
			valueStyle := lipgloss.NewStyle().
				Foreground(valueColor).
				Width(66). // Slightly less than box width for padding
				Italic(true)

			// Build content with highlighting if in search mode
			var titleText, valueText string
			if m.searchMode && m.searchQuery != "" {
				titleText = m.highlightMatches(item.T, m.searchQuery)
				valueText = m.highlightMatches(item.D, m.searchQuery)
			} else {
				titleText = item.T
				valueText = item.D
			}

			content := fmt.Sprintf("%s\n%s",
				titleStyle.Render(titleText),
				valueStyle.Render(valueText),
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
	var helpText string
	if m.searchMode {
		helpText = "  type to search • ↑↓/jk navigate • enter select • esc cancel"
	} else {
		helpText = "  / search • ↑↓/jk navigate • enter select • q/esc quit"
		if len(m.availPages) > 1 {
			helpText = "  / search • ← →/hl switch • ↑↓/jk navigate • enter select • q/esc quit"
		}
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

// getActiveList returns the current list or filtered list if in search mode
func (m MultiPageViewModel) getActiveList() []ListItem {
	if m.searchMode && m.searchQuery != "" {
		return m.filteredList
	}
	return m.getCurrentList()
}

// updateFilteredList performs fuzzy matching and updates the filtered list
func (m *MultiPageViewModel) updateFilteredList() {
	if m.searchQuery == "" {
		m.filteredList = []ListItem{}
		return
	}

	currentList := m.getCurrentList()
	m.filteredList = []ListItem{}

	query := strings.ToLower(m.searchQuery)

	for _, item := range currentList {
		// Skip dividers
		if item.IsDiv {
			continue
		}

		// Check if query matches in title or description
		titleLower := strings.ToLower(item.T)
		descLower := strings.ToLower(item.D)

		if fuzzyMatch(titleLower, query) || fuzzyMatch(descLower, query) {
			m.filteredList = append(m.filteredList, item)
		}
	}
}

// fuzzyMatch checks if query fuzzy matches the text
func fuzzyMatch(text, query string) bool {
	if query == "" {
		return true
	}

	textIdx := 0
	queryIdx := 0

	for textIdx < len(text) && queryIdx < len(query) {
		if text[textIdx] == query[queryIdx] {
			queryIdx++
		}
		textIdx++
	}

	return queryIdx == len(query)
}

// highlightMatches adds ANSI color codes to highlight matching characters
func (m MultiPageViewModel) highlightMatches(text, query string) string {
	if query == "" {
		return text
	}

	// Yellow background with dark text for highlighting
	highlightStyle := lipgloss.NewStyle().
		Background(m.styles.HighlightBgColor).
		Foreground(m.styles.HighlightFgColor)

	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)

	var result strings.Builder
	textIdx := 0
	queryIdx := 0

	for textIdx < len(text) {
		if queryIdx < len(queryLower) && textLower[textIdx] == queryLower[queryIdx] {
			// This character matches - highlight it
			result.WriteString(highlightStyle.Render(string(text[textIdx])))
			queryIdx++
		} else {
			// No match - write character as-is
			result.WriteByte(text[textIdx])
		}
		textIdx++
	}

	return result.String()
}
