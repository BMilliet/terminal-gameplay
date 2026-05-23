package ui

import (
	"fmt"
	"os"

	"terminal-gameplay/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListViewModel struct {
	list     list.Model
	selected string
	endValue *utils.ListItem
	quitting bool
	styles   Styles
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
		switch keypress := msg.String(); keypress {

		case "enter":
			m.quitting = true
			i, ok := m.list.SelectedItem().(utils.ListItem)
			if ok {
				*m.endValue = i
			}
			return m, tea.Quit

		case "ctrl+c", "esc", "q":
			*m.endValue = utils.ListItem{T: utils.ExitSignal}
			return m, tea.Quit
		}
	}

	i, ok := m.list.SelectedItem().(utils.ListItem)
	if ok {
		m.selected = string(i.D)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListViewModel) View() string {
	if m.quitting {
		return ""
	}

	return m.list.View()
}

func ListView(title string, op []utils.ListItem, height int, endValue *utils.ListItem) {
	items := []list.Item{}
	for _, o := range op {
		items = append(items, o)
	}

	styles := DefaultStyles()

	const defaultWidth = 20

	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = styles.SelectedItemStyle
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.
		Foreground(styles.SelectedTitleColor)

	l := list.New(items, delegate, defaultWidth, height)
	l.Title = fmt.Sprintf("\n%s", title)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styles.TitleStyle
	l.Styles.Title.Align(lipgloss.Left)
	l.Styles.PaginationStyle = styles.PaginationStyle
	l.Styles.HelpStyle = styles.HelpStyle

	m := ListViewModel{list: l, endValue: endValue, selected: "", styles: *styles}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("ListView -> ", err)
		os.Exit(1)
	}
}
