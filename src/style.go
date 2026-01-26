package src

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
}

func DefaultStyles() *Styles {
	s := new(Styles)

	s.PeachColor = lipgloss.Color("#F2B391")
	s.CoralColor = lipgloss.Color("#F39194")
	s.OrchidColor = lipgloss.Color("#E3B5BF")
	s.ThistleColor = lipgloss.Color("#DAC3E9")
	s.NyanzaColor = lipgloss.Color("#E9F2D0")
	s.ErrorColor = lipgloss.Color("#FF99B8")
	s.AquamarineColor = lipgloss.Color("#B4F8D5")
	s.DividerColor = lipgloss.Color("#6B6B6B")

	// Muted colors for unselected items
	s.MutedTitleColor = lipgloss.Color("#6B6B6B")  // Subtle gray
	s.MutedBorderColor = lipgloss.Color("#3A3A3A") // Very dark gray

	// Search and highlight colors
	s.SearchBoxColor = s.AquamarineColor
	s.SearchTextColor = s.ThistleColor
	s.HighlightBgColor = lipgloss.Color("#FFD700") // Gold/yellow
	s.HighlightFgColor = lipgloss.Color("#1A1A1A") // Dark text for readability
	
	// Settings colors - slightly desaturated/grayed versions of main palette
	s.SettingsTitleColor = lipgloss.Color("#9B8B9F")         // Muted purple-gray (less saturated Orchid/Thistle)
	s.SettingsSelectedTitleColor = lipgloss.Color("#C5B0C9") // Soft purple-gray (grayed Orchid)
	s.SettingsBorderColor = lipgloss.Color("#7A6B7E")        // Medium purple-gray
	s.SettingsValueColor = lipgloss.Color("#ADA0B0")         // Light purple-gray

	s.BorderColor = s.OrchidColor
	s.FooterColor = s.NyanzaColor
	s.TitleColor = s.ThistleColor
	s.SelectedTitleColor = s.OrchidColor

	s.InputField = lipgloss.NewStyle().
		BorderForeground(s.BorderColor).
		BorderStyle(lipgloss.NormalBorder()).
		Padding(1).
		Width(80)

	s.InputFieldWithError = lipgloss.NewStyle().
		BorderForeground(s.ErrorColor).
		BorderStyle(lipgloss.NormalBorder()).
		Padding(1).
		Width(80)

	s.FooterStyle = lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(s.FooterColor).
		Italic(true)

	s.TitleStyle = lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(s.TitleColor).
		Bold(true)

	s.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	s.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	s.SelectedItemStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(s.SelectedTitleColor).
		Foreground(s.SelectedTitleColor).
		Padding(0, 0, 0, 1)

	return s
}

func (s Styles) Text(t string, c lipgloss.Color) string {
	var style = lipgloss.NewStyle().Foreground(c)
	return style.Render(t)
}
