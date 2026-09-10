package ui

import (
	"fmt"
	"io"
	"os"

	"terminal-gameplay/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListViewModel struct {
	list             list.Model
	selected         string
	endValue         *utils.ListItem
	quitting         bool
	continuousFilter bool
	styles           Styles
}

// continuousFilterDelegate keeps the selected row visible while Bubble's
// filter input has focus. The default delegate intentionally hides selection
// styling in that state.
type continuousFilterDelegate struct {
	list.DefaultDelegate
}

func (d continuousFilterDelegate) Render(w io.Writer, model list.Model, index int, item list.Item) {
	if model.FilterState() != list.Filtering {
		d.DefaultDelegate.Render(w, model, index, item)
		return
	}

	defaultItem, ok := item.(list.DefaultItem)
	if !ok || model.Width() <= 0 {
		return
	}

	styles := d.Styles
	textWidth := model.Width() - styles.NormalTitle.GetPaddingLeft() - styles.NormalTitle.GetPaddingRight()
	title := truncateSingleLine(defaultItem.Title(), textWidth)
	description := truncateSingleLine(defaultItem.Description(), textWidth)

	titleStyle := styles.NormalTitle
	descriptionStyle := styles.NormalDesc
	if index == model.Index() {
		titleStyle = styles.SelectedTitle
		descriptionStyle = styles.SelectedDesc
	}

	unmatched := titleStyle.Inline(true)
	matched := fuzzySearchMatchedRuneStyle(unmatched, styles.FilterMatch)
	title = lipgloss.StyleRunes(title, model.MatchesForItem(index), matched, unmatched)
	title = titleStyle.Render(title)
	description = descriptionStyle.Render(description)

	if d.ShowDescription {
		fmt.Fprintf(w, "%s\n%s", title, description) //nolint:errcheck
		return
	}
	fmt.Fprint(w, title) //nolint:errcheck
}

func (m ListViewModel) Init() tea.Cmd {
	return nil
}

func (m ListViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		if m.continuousFilter && m.list.SettingFilter() {
			switch msg.String() {
			case "up":
				m.list.CursorUp()
				return m, nil
			case "down":
				m.list.CursorDown()
				return m, nil
			case "left":
				m.changeFuzzySearchPage(-1)
				return m, nil
			case "right":
				m.changeFuzzySearchPage(1)
				return m, nil
			}
		}

		switch keypress := msg.String(); keypress {

		case "enter":
			i, ok := m.list.SelectedItem().(utils.ListItem)
			if !ok {
				return m, nil
			}
			m.quitting = true
			*m.endValue = i
			return m, tea.Quit

		case "ctrl+c", "esc":
			*m.endValue = utils.ListItem{T: utils.ExitSignal}
			m.quitting = true
			return m, tea.Quit

		case "q":
			if !m.list.SettingFilter() {
				*m.endValue = utils.ListItem{T: utils.ExitSignal}
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	i, ok := m.list.SelectedItem().(utils.ListItem)
	if ok {
		m.selected = string(i.D)
	}

	previousFilter := m.list.FilterValue()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.continuousFilter && m.list.FilterValue() != previousFilter {
		// Apply matches synchronously as text changes. This prevents Enter
		// from racing an outstanding filter command while allowing navigation
		// to preserve the highlighted cursor position.
		filter := m.list.FilterValue()
		m.list.SetFilterText(filter)
		m.list.SetFilterState(list.Filtering)
		return m, nil
	}
	return m, cmd
}

func (m *ListViewModel) changeFuzzySearchPage(direction int) {
	if direction < 0 {
		m.list.Paginator.PrevPage()
	} else {
		m.list.Paginator.NextPage()
	}

	itemsOnPage := m.list.Paginator.ItemsOnPage(len(m.list.VisibleItems()))
	if itemsOnPage == 0 || m.list.Cursor() < itemsOnPage {
		return
	}

	lastIndexOnPage := m.list.Paginator.Page*m.list.Paginator.PerPage + itemsOnPage - 1
	m.list.Select(lastIndexOnPage)
}

func (m ListViewModel) View() string {
	if m.quitting {
		return ""
	}

	return m.list.View()
}

func fuzzySearchMatchStyle(styles *Styles) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.CoralColor)
}

func fuzzySearchMatchedRuneStyle(base, match lipgloss.Style) lipgloss.Style {
	return base.Foreground(match.GetForeground())
}

func newListViewModel(title string, op []utils.ListItem, height int, endValue *utils.ListItem, startFiltering bool) ListViewModel {
	items := []list.Item{}
	for _, o := range op {
		items = append(items, o)
	}

	styles := DefaultStyles()

	const defaultWidth = 20

	defaultDelegate := list.NewDefaultDelegate()

	defaultDelegate.Styles.SelectedTitle = styles.SelectedItemStyle
	defaultDelegate.Styles.SelectedDesc = defaultDelegate.Styles.SelectedTitle.
		Foreground(styles.SelectedTitleColor)

	var delegate list.ItemDelegate = defaultDelegate
	if startFiltering {
		defaultDelegate.ShowDescription = false
		defaultDelegate.SetSpacing(0)
		defaultDelegate.Styles.FilterMatch = fuzzySearchMatchStyle(styles)
		delegate = continuousFilterDelegate{DefaultDelegate: defaultDelegate}
	}

	l := list.New(items, delegate, defaultWidth, height)
	l.Title = fmt.Sprintf("\n%s", title)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	if startFiltering {
		// Keep every printable character available to the fuzzy input. Only
		// arrow keys navigate matches while that input remains active.
		l.KeyMap.CursorUp.SetKeys("up")
		l.KeyMap.CursorUp.SetHelp("↑", "up")
		l.KeyMap.CursorDown.SetKeys("down")
		l.KeyMap.CursorDown.SetHelp("↓", "down")
		l.KeyMap.PrevPage.SetKeys("left")
		l.KeyMap.PrevPage.SetHelp("←", "prev page")
		l.KeyMap.NextPage.SetKeys("right")
		l.KeyMap.NextPage.SetHelp("→", "next page")
		l.KeyMap.AcceptWhileFiltering.SetKeys("enter")
		l.SetFilterText("")
		l.SetFilterState(list.Filtering)
	}
	l.Styles.Title = styles.TitleStyle
	l.Styles.Title.Align(lipgloss.Left)
	l.Styles.PaginationStyle = styles.PaginationStyle
	l.Styles.HelpStyle = styles.HelpStyle

	return ListViewModel{
		list:             l,
		endValue:         endValue,
		selected:         "",
		continuousFilter: startFiltering,
		styles:           *styles,
	}
}

func runListView(title string, op []utils.ListItem, height int, endValue *utils.ListItem, startFiltering bool) {
	m := newListViewModel(title, op, height, endValue, startFiltering)

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("ListView -> ", err)
		os.Exit(1)
	}
}

func ListView(title string, op []utils.ListItem, height int, endValue *utils.ListItem) {
	runListView(title, op, height, endValue, false)
}

func FuzzySearchView(title string, op []utils.ListItem, height int, endValue *utils.ListItem) {
	runListView(title, op, height, endValue, true)
}
