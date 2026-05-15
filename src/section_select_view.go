package src

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SectionSelectViewModel struct {
	title    string
	sections []ListItem
	cursor   int
	selected *ListItem
	quitting bool
	styles   *Styles
}

func (m SectionSelectViewModel) Init() tea.Cmd {
	return nil
}

func (m SectionSelectViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			*m.selected = ListItem{T: ExitSignal}
			m.quitting = true
			return m, tea.Quit

		case "a":
			*m.selected = ListItem{T: AddGoToSectionAction, D: "create new section"}
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.sections)-1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			if len(m.sections) == 0 {
				return m, nil
			}
			*m.selected = m.sections[m.cursor]
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m SectionSelectViewModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(m.styles.TitleColor).
		Bold(true).
		Render(m.title))
	b.WriteString("\n\n")

	for i, section := range m.sections {
		selected := i == m.cursor
		prefix := "  "
		titleColor := m.styles.MutedTitleColor
		valueColor := m.styles.FooterColor

		if selected {
			prefix = "▌ "
			titleColor = m.styles.SelectedTitleColor
			valueColor = m.styles.NyanzaColor
		}

		label := section.D
		if label == "" {
			label = section.T
		}

		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Foreground(m.styles.SelectedTitleColor).Render(prefix),
			lipgloss.NewStyle().Foreground(titleColor).Bold(selected).Width(24).Render(label),
			lipgloss.NewStyle().Foreground(valueColor).Render(sectionHint(section)),
		)

		if selected {
			row = lipgloss.NewStyle().Background(lipgloss.Color("#313244")).Width(64).Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.FooterStyle.Render("a new section  /  up down navigate  /  enter select  /  esc cancel\n"))
	return b.String()
}

func SectionSelectView(title string, sections []ListItem, selected *ListItem) {
	m := SectionSelectViewModel{
		title:    title,
		sections: sections,
		selected: selected,
		styles:   DefaultStyles(),
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("SectionSelectView -> ", err)
		os.Exit(1)
	}
}

func sectionHint(section ListItem) string {
	if section.T == RootGoToSection {
		return "root"
	}
	return section.T
}
