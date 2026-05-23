package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	FooterColor        lipgloss.Color
	BorderColor        lipgloss.Color
	TitleColor         lipgloss.Color
	SelectedTitleColor lipgloss.Color

	FooterStyle         lipgloss.Style
	TitleStyle          lipgloss.Style
	InputField          lipgloss.Style
	InputFieldWithError lipgloss.Style

	PaginationStyle   lipgloss.Style
	HelpStyle         lipgloss.Style
	SelectedItemStyle lipgloss.Style

	PeachColor      lipgloss.Color
	CoralColor      lipgloss.Color
	OrchidColor     lipgloss.Color
	ThistleColor    lipgloss.Color
	NyanzaColor     lipgloss.Color
	AquamarineColor lipgloss.Color
	ErrorColor      lipgloss.Color
	DividerColor    lipgloss.Color

	// Muted colors for unselected items
	MutedTitleColor  lipgloss.Color
	MutedBorderColor lipgloss.Color

	// Search and highlight colors
	SearchBoxColor   lipgloss.Color
	SearchTextColor  lipgloss.Color
	HighlightBgColor lipgloss.Color
	HighlightFgColor lipgloss.Color

	// Settings colors
	SettingsTitleColor         lipgloss.Color
	SettingsSelectedTitleColor lipgloss.Color
	SettingsBorderColor        lipgloss.Color
	SettingsValueColor         lipgloss.Color
	SettingsEnabledColor       lipgloss.Color
	SettingsDisabledColor      lipgloss.Color
}

func DefaultStyles() *Styles {
	s := new(Styles)

	s.PeachColor = lipgloss.Color("#FAB387")
	s.CoralColor = lipgloss.Color("#F38BA8")
	s.OrchidColor = lipgloss.Color("#CBA6F7")
	s.ThistleColor = lipgloss.Color("#F5C2E7")
	s.NyanzaColor = lipgloss.Color("#CDD6F4")
	s.ErrorColor = lipgloss.Color("#F38BA8")
	s.AquamarineColor = lipgloss.Color("#A6E3A1")
	s.DividerColor = lipgloss.Color("#7F849C")

	// Muted colors for unselected items
	s.MutedTitleColor = lipgloss.Color("#9399B2")
	s.MutedBorderColor = lipgloss.Color("#45475A")

	// Search and highlight colors
	s.SearchBoxColor = lipgloss.Color("#F5C2E7")
	s.SearchTextColor = s.NyanzaColor
	s.HighlightBgColor = lipgloss.Color("#F5C2E7")
	s.HighlightFgColor = lipgloss.Color("#1E1E2E")

	// Settings colors
	s.SettingsTitleColor = lipgloss.Color("#F5C2E7")
	s.SettingsSelectedTitleColor = lipgloss.Color("#CBA6F7")
	s.SettingsBorderColor = lipgloss.Color("#585B70")
	s.SettingsValueColor = lipgloss.Color("#BAC2DE")
	s.SettingsEnabledColor = lipgloss.Color("#A6E3A1")
	s.SettingsDisabledColor = lipgloss.Color("#F38BA8")

	s.BorderColor = lipgloss.Color("#585B70")
	s.FooterColor = lipgloss.Color("#A6ADC8")
	s.TitleColor = s.NyanzaColor
	s.SelectedTitleColor = lipgloss.Color("#CBA6F7")

	s.InputField = lipgloss.NewStyle().
		BorderForeground(s.SearchBoxColor).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(72)

	s.InputFieldWithError = lipgloss.NewStyle().
		BorderForeground(s.ErrorColor).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(72)

	s.FooterStyle = lipgloss.NewStyle().
		Foreground(s.FooterColor).
		Faint(true)

	s.TitleStyle = lipgloss.NewStyle().
		Foreground(s.TitleColor).
		Bold(true)

	s.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	s.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	s.SelectedItemStyle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(s.SelectedTitleColor).
		Foreground(s.SelectedTitleColor).
		Padding(0, 0, 0, 1)

	return s
}

func (s Styles) Text(t string, c lipgloss.Color) string {
	var style = lipgloss.NewStyle().Foreground(c)
	return style.Render(t)
}
