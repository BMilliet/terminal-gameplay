package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmViewModel struct {
	title     string
	cursor    int
	confirmed *bool
	quitting  bool
	styles    *Styles
}

func (m ConfirmViewModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			*m.confirmed = false
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			*m.confirmed = m.cursor == 0
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ConfirmViewModel) View() string {
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

	for i, label := range []string{"yes", "no"} {
		selected := i == m.cursor
		prefix := "  "
		optionStyle := lipgloss.NewStyle().
			Foreground(m.styles.MutedTitleColor).
			Width(28)

		if selected {
			prefix = "▌ "
			optionStyle = optionStyle.
				Foreground(m.styles.SelectedTitleColor).
				Background(lipgloss.Color("#313244")).
				Bold(true)
		}

		b.WriteString(lipgloss.NewStyle().
			Foreground(m.styles.SelectedTitleColor).
			Render(prefix))
		b.WriteString(optionStyle.Render(label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.FooterStyle.Render("up down navigate  /  enter select  /  esc cancel\n"))
	return b.String()
}

func ConfirmView(title string, confirmed *bool) {
	m := ConfirmViewModel{
		title:     title,
		confirmed: confirmed,
		styles:    DefaultStyles(),
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("ConfirmView -> ", err)
		os.Exit(1)
	}
}
