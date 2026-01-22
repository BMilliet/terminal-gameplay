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
	config      *ConfigDTO
	currentPage PageType
	warpList    []ListItem
	commandList []ListItem
	notesList   []ListItem
	availPages  []PageType
	pageIndex   int
	cursor      int
	selected    *string
	quitting    bool
	styles      *Styles
}

func NewMultiPageViewModel(config *ConfigDTO) MultiPageViewModel {
	// Build list of available pages (non-empty)
	availPages := []PageType{}
	if len(config.Warp) > 0 {
		availPages = append(availPages, WarpPage)
	}
	if len(config.Commands) > 0 {
		availPages = append(availPages, CommandsPage)
	}
	if len(config.Notes) > 0 {
		availPages = append(availPages, NotesPage)
	}

	currentPage := WarpPage
	if len(availPages) > 0 {
		currentPage = availPages[0]
	}

	return MultiPageViewModel{
		config:      config,
		currentPage: currentPage,
		warpList:    ConfigItemsToListItems(config.Warp),
		commandList: ConfigItemsToListItems(config.Commands),
		notesList:   ConfigItemsToListItems(config.Notes),
		availPages:  availPages,
		pageIndex:   0,
		cursor:      0,
		quitting:    false,
		styles:      DefaultStyles(),
	}
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
			}

		case "right", "l":
			// Navigate to next page
			if m.pageIndex < len(m.availPages)-1 {
				m.pageIndex++
				m.currentPage = m.availPages[m.pageIndex]
				m.cursor = 0
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			items := m.getCurrentList()
			if m.cursor < len(items)-1 {
				m.cursor++
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
		for i, item := range items {
			// Style for item box
			var itemBox lipgloss.Style
			if m.cursor == i {
				// Selected item - with border
				itemBox = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(m.styles.SelectedTitleColor).
					Padding(0, 1).
					Width(70)
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
	return m.getPageNameByType(m.currentPage)
}

func (m MultiPageViewModel) getPageNameByType(page PageType) string {
	switch page {
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

func MultiPageView(config *ConfigDTO, selected *string) {
	m := NewMultiPageViewModel(config)
	m.selected = selected

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("MultiPageView -> ", err)
		os.Exit(1)
	}
}
