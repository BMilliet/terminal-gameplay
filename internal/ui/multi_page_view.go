package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/frequent"
	gototab "terminal-gameplay/internal/goto"
	"terminal-gameplay/internal/notes"
	"terminal-gameplay/internal/scripts"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/tools"
	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultContentWidth = 112
	minContentWidth     = 72
	screenGutterWidth   = 4
	rowChromeWidth      = 4
	headerTickDuration  = 140 * time.Millisecond
	gradientSteps       = 14
)

var headerGradientPalette = []string{
	"#F5C2E7", // pink
	"#CBA6F7", // mauve
	"#F38BA8", // red
	"#FAB387", // peach
	"#F9E2AF", // yellow
	"#CBA6F7", // mauve
}

type headerTickMsg time.Time

type PageType int

const (
	FrequentPage PageType = iota
	GoToPage
	ScriptsPage
	NotesPage
	EnvPage
	ToolsPage
	SettingsPage
	FeaturesPage
)

type MultiPageViewModel struct {
	config        *utils.ConfigDTO
	features      *settings.FeaturesDTO
	currentPage   PageType
	frequentList  []utils.ListItem
	goToList      []utils.ListItem
	scriptList    []utils.ListItem
	notesList     []utils.ListItem
	envList       []utils.ListItem
	toolsList     []utils.ListItem
	settingsList  []utils.ListItem
	featuresList  []utils.ListItem
	availPages    []PageType
	pageIndex     int
	cursor        int
	viewportStart int // First visible item index for scrolling
	maxVisible    int // Maximum items to show at once
	selected      *string
	quitting      bool
	styles        *Styles
	terminalWidth int
	headerFrame   int
	pendingDelete bool
	// Fuzzy find state
	searchMode   bool
	searchQuery  string
	filteredList []utils.ListItem
}

func NewMultiPageViewModel(config *utils.ConfigDTO, features *settings.FeaturesDTO, initialPage ...string) MultiPageViewModel {
	frequentList := frequent.BuildList(config.GoTo, features.FrequentGoTo, features.Frequencies)
	toolsList := tools.BuildList()
	settingsList := settings.BuildSettingsList()
	featuresList := settings.BuildFeaturesList(features)

	// Build list of available pages (non-empty)
	availPages := []PageType{}

	// Add frequent page first if enabled and has items
	if len(frequentList) > 0 {
		availPages = append(availPages, FrequentPage)
	}

	availPages = append(availPages, GoToPage)
	if features.Scripts {
		availPages = append(availPages, ScriptsPage)
	}
	if features.Notes {
		availPages = append(availPages, NotesPage)
	}
	if features.Env {
		availPages = append(availPages, EnvPage)
	}

	// Always add tools before settings.
	availPages = append(availPages, ToolsPage)

	// Always add settings page at the end.
	availPages = append(availPages, SettingsPage)

	currentPage := GoToPage
	pageIndex := 0
	if len(availPages) > 0 {
		currentPage = availPages[0]
	}
	if len(initialPage) > 0 {
		for i, page := range availPages {
			if pageNameByType(page) == initialPage[0] {
				currentPage = page
				pageIndex = i
				break
			}
		}
	}

	m := MultiPageViewModel{
		config:        config,
		features:      features,
		currentPage:   currentPage,
		frequentList:  frequentList,
		goToList:      gototab.BuildList(config.GoTo),
		scriptList:    scripts.BuildList(config.Scripts),
		notesList:     notes.BuildList(config.Notes),
		envList:       envtab.BuildList(config.Env),
		toolsList:     toolsList,
		settingsList:  settingsList,
		featuresList:  featuresList,
		availPages:    availPages,
		pageIndex:     pageIndex,
		cursor:        0,
		viewportStart: 0,
		maxVisible:    10,
		quitting:      false,
		styles:        DefaultStyles(),
		terminalWidth: defaultContentWidth + screenGutterWidth,
		searchMode:    false,
		searchQuery:   "",
		filteredList:  []utils.ListItem{},
	}

	// Move cursor to first non-divider item
	items := m.getCurrentList()
	for m.cursor < len(items) && items[m.cursor].IsDiv {
		m.cursor++
	}

	return m
}

func (m MultiPageViewModel) Init() tea.Cmd {
	return tickHeader()
}

func (m MultiPageViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		return m, nil

	case headerTickMsg:
		if m.quitting {
			return m, nil
		}
		m.headerFrame++
		return m, tickHeader()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			*m.selected = utils.ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.pendingDelete {
				m.pendingDelete = false
				return m, nil
			}
			// If in search mode, exit search mode
			if m.searchMode {
				m.searchMode = false
				m.searchQuery = ""
				m.filteredList = []utils.ListItem{}
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
			*m.selected = utils.ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.searchMode {
				m.searchMode = false
				m.searchQuery = ""
				m.filteredList = []utils.ListItem{}
				m.cursor = 0
				m.viewportStart = 0
				return m, nil
			}
			*m.selected = utils.ExitSignal
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.pendingDelete = false
			// Enter search mode
			if !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.filteredList = []utils.ListItem{}
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
			m.pendingDelete = false
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
			m.pendingDelete = false
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
			m.pendingDelete = false
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
			m.pendingDelete = false
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
			m.pendingDelete = false
			items := m.getActiveList()
			if len(items) > 0 && m.cursor < len(items) {
				selectedItem := items[m.cursor]
				if selectedItem.IsDiv {
					return m, nil
				}
				if m.currentPage == SettingsPage && selectedItem.T == settings.FeaturesAction {
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
			if !m.searchMode && msg.String() == "d" && m.canDeleteCurrentPage() {
				item, ok := m.selectedActionItem()
				if !ok {
					m.pendingDelete = false
					return m, nil
				}

				if !m.pendingDelete {
					m.pendingDelete = true
					return m, nil
				}

				*m.selected = fmt.Sprintf("%s|%s|%s", m.getPageName(), m.deleteActionForCurrentPage(), item.T)
				m.quitting = true
				return m, tea.Quit
			}

			m.pendingDelete = false

			if !m.searchMode && m.currentPage == GoToPage && msg.String() == "a" {
				*m.selected = fmt.Sprintf("%s|%s|", m.getPageName(), gototab.AddAction)
				m.quitting = true
				return m, tea.Quit
			}

			if !m.searchMode && m.currentPage == NotesPage && msg.String() == "a" {
				*m.selected = fmt.Sprintf("%s|%s|", m.getPageName(), notes.AddAction)
				m.quitting = true
				return m, tea.Quit
			}

			if !m.searchMode && m.currentPage == ScriptsPage && msg.String() == "a" {
				*m.selected = fmt.Sprintf("%s|%s|", m.getPageName(), scripts.AddAction)
				m.quitting = true
				return m, tea.Quit
			}

			if !m.searchMode && m.currentPage == EnvPage && msg.String() == "a" {
				*m.selected = fmt.Sprintf("%s|%s|", m.getPageName(), envtab.AddAction)
				m.quitting = true
				return m, tea.Quit
			}

			if !m.searchMode && m.currentPage == ScriptsPage && msg.String() == "e" {
				items := m.getActiveList()
				if len(items) == 0 || m.cursor >= len(items) || items[m.cursor].IsDiv {
					return m, nil
				}
				*m.selected = fmt.Sprintf("%s|%s|%s", m.getPageName(), scripts.EditAction, items[m.cursor].T)
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
	b.WriteString(m.renderHeader())

	if m.searchMode {
		b.WriteString(m.renderSearchBox())
		b.WriteString("\n\n")
	}

	items := m.getActiveList()
	if len(items) == 0 {
		b.WriteString(m.renderEmptyState())
	} else {
		visibleEnd := m.viewportStart + m.maxVisible
		if visibleEnd > len(items) {
			visibleEnd = len(items)
		}

		if m.viewportStart > 0 {
			b.WriteString("\n")
		}

		for i := m.viewportStart; i < visibleEnd; i++ {
			item := items[i]
			if item.IsDiv {
				b.WriteString(m.renderDivider(item))
				b.WriteString("\n")
				continue
			}

			b.WriteString(m.renderListItem(item, i))
			b.WriteString("\n")
		}

		if visibleEnd < len(items) {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m MultiPageViewModel) renderHeader() string {
	var b strings.Builder

	wordmark := m.renderGradientBadge(" tg ", 0)
	context := m.renderGradientText("terminal-gameplay", 10, true)
	page := m.renderGradientText(m.getPageName(), 44, false)

	meta := lipgloss.JoinHorizontal(
		lipgloss.Center,
		wordmark,
		" ",
		context,
		m.styles.Text(" / ", m.styles.DividerColor),
		page,
	)

	b.WriteString("\n")
	b.WriteString(meta)
	b.WriteString("\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	return b.String()
}

func (m MultiPageViewModel) renderTabs() string {
	tabs := []string{}
	for _, page := range m.availPages {
		pageName := m.getPageNameByType(page)
		active := page == m.currentPage || (m.currentPage == FeaturesPage && page == SettingsPage)

		tabStyle := lipgloss.NewStyle().
			Foreground(m.styles.MutedTitleColor).
			Padding(0, 1)
		prefix := " "

		if active {
			accent := m.gradientColor(12)
			tabStyle = tabStyle.
				Foreground(accent).
				Background(lipgloss.Color("#313244")).
				Bold(true)
			prefix = "▌"
		}

		tabs = append(tabs, tabStyle.Render(prefix+" "+pageName))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m MultiPageViewModel) renderGradientBadge(text string, offset int) string {
	var b strings.Builder
	for i, r := range []rune(text) {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(m.gradientColor(offset + i*5)).
			Bold(true).
			Render(string(r)))
	}
	return b.String()
}

func (m MultiPageViewModel) renderGradientText(text string, offset int, bold bool) string {
	var b strings.Builder
	for i, r := range []rune(text) {
		b.WriteString(lipgloss.NewStyle().
			Foreground(m.gradientColor(offset + i*4)).
			Bold(bold).
			Render(string(r)))
	}
	return b.String()
}

func (m MultiPageViewModel) gradientColor(offset int) lipgloss.Color {
	phase := positiveMod(m.headerFrame*2+offset, len(headerGradientPalette)*gradientSteps)
	current := phase / gradientSteps
	next := (current + 1) % len(headerGradientPalette)
	step := phase % gradientSteps

	from := mustParseHexColor(headerGradientPalette[current])
	to := mustParseHexColor(headerGradientPalette[next])
	mixed := mixRGB(from, to, step, gradientSteps)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", mixed.r, mixed.g, mixed.b))
}

func (m MultiPageViewModel) renderSearchBox() string {
	searchText := m.searchQuery
	if searchText == "" {
		searchText = "type to search"
	}

	width := m.contentWidth()
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.styles.SearchBoxColor).
		Foreground(m.styles.SearchTextColor).
		Padding(0, 1).
		Width(width).
		Render(truncateSingleLine("/ "+searchText, width-2))
}

func (m MultiPageViewModel) renderEmptyState() string {
	message := "no items"
	if m.searchMode {
		message = "no matches"
	} else if m.currentPage == GoToPage {
		message = "no goTo items yet. press a to add current directory"
	} else if m.currentPage == NotesPage {
		message = "no notes yet. press a to add one"
	} else if m.currentPage == ScriptsPage {
		message = "no scripts yet. press a to add one"
	} else if m.currentPage == EnvPage {
		message = "no env vars yet. press a to add one"
	}

	return lipgloss.NewStyle().
		Foreground(m.styles.FooterColor).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.styles.MutedBorderColor).
		Padding(0, 0, 0, 1).
		Render(message + "\n")
}

func (m MultiPageViewModel) renderDivider(item utils.ListItem) string {
	width := m.contentWidth()
	return lipgloss.NewStyle().
		Foreground(m.styles.DividerColor).
		Faint(true).
		Width(width).
		Render(truncateSingleLine("  ─ "+item.D, width))
}

func (m MultiPageViewModel) renderListItem(item utils.ListItem, index int) string {
	selected := m.cursor == index
	settingsLike := m.currentPage == SettingsPage || m.currentPage == FeaturesPage || m.currentPage == EnvPage
	width := m.contentWidth()
	titleWidth := m.titleColumnWidth()
	valueWidth := width - titleWidth - rowChromeWidth
	if valueWidth < 16 {
		valueWidth = 16
	}

	titleText := truncateSingleLine(item.T, titleWidth)
	valueText := truncateSingleLine(item.D, valueWidth)
	if m.searchMode && m.searchQuery != "" {
		titleText = m.highlightMatches(titleText, m.searchQuery)
		valueText = m.highlightMatches(valueText, m.searchQuery)
	}

	titleColor := m.styles.MutedTitleColor
	valueColor := m.styles.FooterColor
	borderColor := m.styles.MutedBorderColor
	prefix := "  "

	if settingsLike {
		titleColor = m.styles.SettingsTitleColor
		valueColor = m.styles.SettingsValueColor
	}

	if selected {
		titleColor = m.styles.SelectedTitleColor
		valueColor = m.styles.NyanzaColor
		borderColor = m.styles.SelectedTitleColor
		prefix = "▌ "
		if settingsLike {
			titleColor = m.styles.SettingsSelectedTitleColor
			borderColor = m.styles.SettingsSelectedTitleColor
		}
	}

	title := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(selected).
		Render(titleText)

	valueStyle := lipgloss.NewStyle().
		Foreground(valueColor).
		Width(valueWidth)

	value := valueText
	lowerValue := strings.ToLower(item.D)
	lowerStatus := strings.ToLower(item.Status)
	if settingsLike && (lowerStatus == envtab.InactiveState || strings.Contains(lowerValue, "disabled") || strings.Contains(lowerValue, "inactive")) {
		value = lipgloss.NewStyle().
			Foreground(m.styles.SettingsDisabledColor).
			Bold(selected).
			Render(valueText)
	} else if settingsLike && (lowerStatus == envtab.ActiveState || strings.Contains(lowerValue, "enabled") || strings.Contains(lowerValue, "active")) {
		value = lipgloss.NewStyle().
			Foreground(m.styles.SettingsEnabledColor).
			Bold(selected).
			Render(valueText)
	} else {
		value = valueStyle.Render(valueText)
	}

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Foreground(borderColor).Render(prefix),
		lipgloss.NewStyle().Width(titleWidth).Render(title),
		m.styles.Text("  ", m.styles.DividerColor),
		value,
	)

	if selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#313244")).
			Width(width).
			Render(row)
	}

	return lipgloss.NewStyle().Width(width).Render(row)
}

func (m MultiPageViewModel) renderFooter() string {
	var helpText string
	if m.pendingDelete {
		helpText = "press d again to delete  /  esc cancel"
	} else if m.searchMode {
		helpText = "type to search  /  up down navigate  /  enter select  /  esc cancel"
	} else if m.currentPage == GoToPage {
		helpText = "a add current path  /  dd delete  /  / search  /  left right switch  /  enter select  /  q esc quit"
	} else if m.currentPage == NotesPage {
		helpText = "a add  /  dd delete  /  / search  /  up down navigate  /  enter open  /  q esc quit"
	} else if m.currentPage == ScriptsPage {
		helpText = "a add  /  e edit  /  dd delete  /  enter run  /  / search  /  left right switch  /  q esc quit"
	} else if m.currentPage == EnvPage {
		helpText = "a add  /  dd delete  /  enter toggle  /  / search  /  left right switch  /  q esc quit"
	} else if m.currentPage == ToolsPage {
		helpText = "/ search  /  up down navigate  /  enter select  /  left right switch  /  q esc quit"
	} else if m.currentPage == FeaturesPage {
		helpText = "/ search  /  up down navigate  /  enter toggle  /  esc back  /  q quit"
	} else {
		helpText = "/ search  /  left right switch  /  up down navigate  /  enter select  /  q esc quit"
	}

	return m.styles.FooterStyle.Render(helpText + "\n")
}

func (m MultiPageViewModel) contentWidth() int {
	width := m.terminalWidth - screenGutterWidth
	if m.terminalWidth <= 0 {
		width = defaultContentWidth
	}
	if width < minContentWidth {
		return minContentWidth
	}
	return width
}

func (m MultiPageViewModel) titleColumnWidth() int {
	width := m.contentWidth()
	if width >= 112 {
		return 30
	}
	if width >= 92 {
		return 26
	}
	return 22
}

func truncateSingleLine(text string, maxWidth int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}

	if maxWidth <= 3 {
		return truncateToWidth(text, maxWidth)
	}

	suffix := "..."
	runes := []rune(text)
	for len(runes) > 0 {
		candidate := strings.TrimRight(string(runes), " ") + suffix
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}

	return suffix
}

func truncateToWidth(text string, maxWidth int) string {
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

type rgbColor struct {
	r int
	g int
	b int
}

func mustParseHexColor(hex string) rgbColor {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return rgbColor{}
	}

	r, errR := strconv.ParseInt(hex[0:2], 16, 0)
	g, errG := strconv.ParseInt(hex[2:4], 16, 0)
	b, errB := strconv.ParseInt(hex[4:6], 16, 0)
	if errR != nil || errG != nil || errB != nil {
		return rgbColor{}
	}

	return rgbColor{r: int(r), g: int(g), b: int(b)}
}

func mixRGB(from, to rgbColor, step, totalSteps int) rgbColor {
	if totalSteps <= 0 {
		return from
	}

	return rgbColor{
		r: from.r + ((to.r-from.r)*step)/totalSteps,
		g: from.g + ((to.g-from.g)*step)/totalSteps,
		b: from.b + ((to.b-from.b)*step)/totalSteps,
	}
}

func positiveMod(value, modulo int) int {
	if modulo == 0 {
		return 0
	}
	result := value % modulo
	if result < 0 {
		result += modulo
	}
	return result
}

func (m MultiPageViewModel) getCurrentList() []utils.ListItem {
	switch m.currentPage {
	case FrequentPage:
		return m.frequentList
	case GoToPage:
		return m.goToList
	case ScriptsPage:
		return m.scriptList
	case NotesPage:
		return m.notesList
	case EnvPage:
		return m.envList
	case ToolsPage:
		return m.toolsList
	case SettingsPage:
		return m.settingsList
	case FeaturesPage:
		return m.featuresList
	default:
		return []utils.ListItem{}
	}
}

func (m MultiPageViewModel) canDeleteCurrentPage() bool {
	return m.currentPage == GoToPage || m.currentPage == NotesPage || m.currentPage == ScriptsPage || m.currentPage == EnvPage
}

func (m MultiPageViewModel) selectedActionItem() (utils.ListItem, bool) {
	items := m.getActiveList()
	if len(items) == 0 || m.cursor >= len(items) {
		return utils.ListItem{}, false
	}

	item := items[m.cursor]
	if item.IsDiv {
		return utils.ListItem{}, false
	}

	return item, true
}

func (m MultiPageViewModel) deleteActionForCurrentPage() string {
	if m.currentPage == GoToPage {
		return gototab.DeleteAction
	}
	if m.currentPage == NotesPage {
		return notes.DeleteAction
	}
	if m.currentPage == EnvPage {
		return envtab.DeleteAction
	}
	return scripts.DeleteAction
}

func (m MultiPageViewModel) getPageName() string {
	switch m.currentPage {
	case FrequentPage:
		return frequent.PageName
	case GoToPage:
		return gototab.PageName
	case ScriptsPage:
		return scripts.PageName
	case NotesPage:
		return notes.PageName
	case EnvPage:
		return envtab.PageName
	case ToolsPage:
		return tools.PageName
	case SettingsPage:
		return settings.PageName
	case FeaturesPage:
		return settings.FeaturesPageName
	default:
		return ""
	}
}

func (m MultiPageViewModel) getPageNameByType(page PageType) string {
	return pageNameByType(page)
}

func pageNameByType(page PageType) string {
	switch page {
	case FrequentPage:
		return frequent.PageName
	case GoToPage:
		return gototab.PageName
	case ScriptsPage:
		return scripts.PageName
	case NotesPage:
		return notes.PageName
	case EnvPage:
		return envtab.PageName
	case ToolsPage:
		return tools.PageName
	case SettingsPage:
		return settings.PageName
	case FeaturesPage:
		return settings.FeaturesPageName
	default:
		return ""
	}
}

func tickHeader() tea.Cmd {
	return tea.Tick(headerTickDuration, func(t time.Time) tea.Msg {
		return headerTickMsg(t)
	})
}

func MultiPageView(config *utils.ConfigDTO, features *settings.FeaturesDTO, selected *string, initialPage ...string) {
	m := NewMultiPageViewModel(config, features, initialPage...)
	m.selected = selected

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("MultiPageView -> ", err)
		os.Exit(1)
	}
}

// getActiveList returns the current list or filtered list if in search mode
func (m MultiPageViewModel) getActiveList() []utils.ListItem {
	if m.searchMode && m.searchQuery != "" {
		return m.filteredList
	}
	return m.getCurrentList()
}

// updateFilteredList performs fuzzy matching and updates the filtered list
func (m *MultiPageViewModel) updateFilteredList() {
	if m.searchQuery == "" {
		m.filteredList = []utils.ListItem{}
		return
	}

	currentList := m.getCurrentList()
	m.filteredList = []utils.ListItem{}

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
