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
	FrequentPage PageType = iota
	GoToPage
	CommandsPage
	NotesPage
	SettingsPage
	FeaturesPage
)

type MultiPageViewModel struct {
	config        *ConfigDTO
	features      *FeaturesDTO
	currentPage   PageType
	frequentList  []ListItem
	goToList      []ListItem
	commandList   []ListItem
	notesList     []ListItem
	settingsList  []ListItem
	featuresList  []ListItem
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

func NewMultiPageViewModel(config *ConfigDTO, features *FeaturesDTO) MultiPageViewModel {
	// Build frequent list if enabled and has data
	var frequentList []ListItem
	if features.FrequentGoTo && !features.FrequencyIsEmpty() {
		topKeys := features.GetTopGoToKeys()
		for _, key := range topKeys {
			if value, exists := config.GoTo.Values[key]; exists {
				frequentList = append(frequentList, ListItem{
					T:     key,
					D:     value,
					IsDiv: false,
				})
			}
		}
	}

	// Build settings list
	settingsList := buildSettingsList()
	featuresList := buildFeaturesList(features)

	// Build list of available pages (non-empty)
	availPages := []PageType{}

	// Add frequent page first if enabled and has items
	if len(frequentList) > 0 {
		availPages = append(availPages, FrequentPage)
	}

	if len(config.GoTo.Keys) > 0 {
		availPages = append(availPages, GoToPage)
	}
	if len(config.Commands.Keys) > 0 {
		availPages = append(availPages, CommandsPage)
	}
	if features.Notes {
		availPages = append(availPages, NotesPage)
	}

	// Always add settings page at the end
	availPages = append(availPages, SettingsPage)

	currentPage := GoToPage
	if len(availPages) > 0 {
		currentPage = availPages[0]
	}

	m := MultiPageViewModel{
		config:        config,
		features:      features,
		currentPage:   currentPage,
		frequentList:  frequentList,
		goToList:      ConfigItemsToListItems(config.GoTo),
		commandList:   ConfigItemsToListItems(config.Commands),
		notesList:     ConfigNotesToListItems(config.Notes),
		settingsList:  settingsList,
		featuresList:  featuresList,
		availPages:    availPages,
		pageIndex:     0,
		cursor:        0,
		viewportStart: 0,
		maxVisible:    8, // Show max 8 items at a time
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

		case "esc":
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
			if m.currentPage == FeaturesPage {
				m.currentPage = SettingsPage
				m.cursor = 0
				m.viewportStart = 0
				return m, nil
			}
			// Otherwise quit
			*m.selected = ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.searchMode {
				m.searchMode = false
				m.searchQuery = ""
				m.filteredList = []ListItem{}
				m.cursor = 0
				m.viewportStart = 0
				return m, nil
			}
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

		case "left":
			// Don't allow page navigation in search mode
			if m.searchMode {
				return m, nil
			}
			// Navigate to previous page (circular)
			if m.pageIndex > 0 {
				m.pageIndex--
			} else {
				// Wrap to last page
				m.pageIndex = len(m.availPages) - 1
			}
			m.currentPage = m.availPages[m.pageIndex]
			m.cursor = 0
			m.viewportStart = 0
			// Skip dividers at start of page
			items := m.getActiveList()
			for m.cursor < len(items) && items[m.cursor].IsDiv {
				m.cursor++
			}

		case "right":
			// Don't allow page navigation in search mode
			if m.searchMode {
				return m, nil
			}
			// Navigate to next page (circular)
			if m.pageIndex < len(m.availPages)-1 {
				m.pageIndex++
			} else {
				// Wrap to first page
				m.pageIndex = 0
			}
			m.currentPage = m.availPages[m.pageIndex]
			m.cursor = 0
			m.viewportStart = 0
			// Skip dividers at start of page
			items := m.getActiveList()
			for m.cursor < len(items) && items[m.cursor].IsDiv {
				m.cursor++
			}

		case "up":
			items := m.getActiveList()
			if len(items) == 0 {
				return m, nil
			}
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

		case "down":
			items := m.getActiveList()
			if len(items) == 0 {
				return m, nil
			}
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
				if m.currentPage == SettingsPage && selectedItem.T == "features" {
					m.currentPage = FeaturesPage
					m.cursor = 0
					m.viewportStart = 0
					return m, nil
				}
				result := fmt.Sprintf("%s|%s|%s", m.getPageName(), selectedItem.T, selectedItem.D)
				*m.selected = result
				m.quitting = true
				return m, tea.Quit
			}

		default:
			if !m.searchMode && m.currentPage == NotesPage && msg.String() == "a" {
				*m.selected = fmt.Sprintf("%s|%s|", m.getPageName(), AddNoteAction)
				m.quitting = true
				return m, tea.Quit
			}

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
		if page == m.currentPage || (m.currentPage == FeaturesPage && page == SettingsPage) {
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
			var isSettingsPage = m.currentPage == SettingsPage || m.currentPage == FeaturesPage

			if m.cursor == i {
				// Selected item - with bright border and indented
				borderColor := m.styles.SelectedTitleColor
				if isSettingsPage {
					borderColor = m.styles.SettingsSelectedTitleColor
				}
				itemBox = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(borderColor).
					Padding(0, 1).
					Width(70).
					MarginLeft(2) // Indent selected item
			} else {
				// Unselected item - subtle border
				borderColor := m.styles.MutedBorderColor
				if isSettingsPage {
					borderColor = m.styles.SettingsBorderColor
				}
				itemBox = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(borderColor).
					Padding(0, 1).
					Width(70)
			}

			// Title (label) - bold and prominent
			titleStyle := lipgloss.NewStyle().Bold(true)
			var valueColor lipgloss.Color
			if m.cursor == i {
				if isSettingsPage {
					titleStyle = titleStyle.Foreground(m.styles.SettingsSelectedTitleColor)
					valueColor = m.styles.SettingsValueColor
				} else {
					titleStyle = titleStyle.Foreground(m.styles.SelectedTitleColor)
					valueColor = m.styles.FooterColor
				}
			} else {
				if isSettingsPage {
					titleStyle = titleStyle.Foreground(m.styles.SettingsTitleColor)
					valueColor = m.styles.SettingsValueColor
				} else {
					titleStyle = titleStyle.Foreground(m.styles.MutedTitleColor)
					valueColor = m.styles.MutedTitleColor
				}
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

			// Apply special colors for settings toggles (enabled/disabled)
			var renderedValue string
			if isSettingsPage {
				if strings.Contains(strings.ToLower(item.D), "enabled") {
					// Use green color for enabled
					enabledStyle := lipgloss.NewStyle().
						Foreground(m.styles.SettingsEnabledColor).
						Width(66).
						Italic(true)
					renderedValue = enabledStyle.Render(valueText)
				} else if strings.Contains(strings.ToLower(item.D), "disabled") {
					// Use red color for disabled
					disabledStyle := lipgloss.NewStyle().
						Foreground(m.styles.SettingsDisabledColor).
						Width(66).
						Italic(true)
					renderedValue = disabledStyle.Render(valueText)
				} else {
					// Default value style for non-toggle settings
					renderedValue = valueStyle.Render(valueText)
				}
			} else {
				renderedValue = valueStyle.Render(valueText)
			}

			content := fmt.Sprintf("%s\n%s",
				titleStyle.Render(titleText),
				renderedValue,
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
		helpText = "  type to search • ↑↓ navigate • enter select • esc cancel"
	} else if m.currentPage == NotesPage {
		helpText = "  a add • / search • ↑↓ navigate • enter select • q/esc quit"
	} else if m.currentPage == FeaturesPage {
		helpText = "  / search • ↑↓ navigate • enter toggle • esc back • q quit"
	} else {
		helpText = "  / search • ↑↓ navigate • enter select • q/esc quit"
		if len(m.availPages) > 1 {
			helpText = "  / search • ← → switch • ↑↓ navigate • enter select • q/esc quit"
		}
	}
	b.WriteString(m.styles.FooterStyle.Render(helpText + "\n"))

	return b.String()
}

func (m MultiPageViewModel) getCurrentList() []ListItem {
	switch m.currentPage {
	case FrequentPage:
		return m.frequentList
	case GoToPage:
		return m.goToList
	case CommandsPage:
		return m.commandList
	case NotesPage:
		return m.notesList
	case SettingsPage:
		return m.settingsList
	case FeaturesPage:
		return m.featuresList
	default:
		return []ListItem{}
	}
}

func (m MultiPageViewModel) getPageName() string {
	switch m.currentPage {
	case FrequentPage:
		return "frequent"
	case GoToPage:
		return "goTo"
	case CommandsPage:
		return "commands"
	case NotesPage:
		return "notes"
	case SettingsPage:
		return "settings"
	case FeaturesPage:
		return "features"
	default:
		return ""
	}
}

func (m MultiPageViewModel) getPageNameByType(page PageType) string {
	switch page {
	case FrequentPage:
		return "frequent ⭐"
	case GoToPage:
		return "goTo ⚡️"
	case CommandsPage:
		return "commands 🎮"
	case NotesPage:
		return "notes ✏️"
	case SettingsPage:
		return "settings ⚙️"
	case FeaturesPage:
		return "features"
	default:
		return ""
	}
}

func MultiPageView(config *ConfigDTO, features *FeaturesDTO, selected *string) {
	m := NewMultiPageViewModel(config, features)
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

// buildSettingsList creates the settings items list.
func buildSettingsList() []ListItem {
	items := []ListItem{}

	items = append(items, ListItem{
		T:     "features",
		D:     "configure feature flags",
		IsDiv: false,
	})

	// Clear Frequency History
	items = append(items, ListItem{
		T:     "clear_frequency",
		D:     "clear all frequency history",
		IsDiv: false,
	})

	return items
}

func buildFeaturesList(features *FeaturesDTO) []ListItem {
	items := []ListItem{
		{
			T:     "frequent_goTo",
			D:     enabledStatus(features.FrequentGoTo),
			IsDiv: false,
		},
		{
			T:     "notes",
			D:     enabledStatus(features.Notes),
			IsDiv: false,
		},
	}

	return items
}

func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled ✓"
	}
	return "disabled ✗"
}
