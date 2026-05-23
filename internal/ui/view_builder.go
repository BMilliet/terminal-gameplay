package ui

import (
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"
)

type ViewBuilderInterface interface {
	NewListView(title string, op []utils.ListItem, height int) utils.ListItem
	NewConfirmView(title string) bool
	NewSectionSelectView(title string, sections []utils.ListItem) utils.ListItem
	NewTextFieldView(title, placeHolder string) string
	NewSearchReplaceFilesView(title string, files []utils.ListItem, height int) utils.ListItem
	NewMultiPageView(config *utils.ConfigDTO, features *settings.FeaturesDTO, initialPage ...string) string
}

type ViewBuilder struct{}

func NewViewBuilder() *ViewBuilder {
	return &ViewBuilder{}
}

func (b *ViewBuilder) NewListView(title string, op []utils.ListItem, height int) utils.ListItem {
	endValue := utils.ListItem{}
	ListView(title, op, height, &endValue)
	return endValue
}

func (b *ViewBuilder) NewConfirmView(title string) bool {
	confirmed := false
	ConfirmView(title, &confirmed)
	return confirmed
}

func (b *ViewBuilder) NewSectionSelectView(title string, sections []utils.ListItem) utils.ListItem {
	selected := utils.ListItem{}
	SectionSelectView(title, sections, &selected)
	return selected
}

func (b *ViewBuilder) NewTextFieldView(title, placeHolder string) string {
	endValue := ""
	TextFieldView(title, placeHolder, &endValue)
	return endValue
}

func (b *ViewBuilder) NewSearchReplaceFilesView(title string, files []utils.ListItem, height int) utils.ListItem {
	endValue := utils.ListItem{}
	SearchReplaceFilesView(title, files, height, &endValue)
	return endValue
}

func (b *ViewBuilder) NewMultiPageView(config *utils.ConfigDTO, features *settings.FeaturesDTO, initialPage ...string) string {
	selected := ""
	MultiPageView(config, features, &selected, initialPage...)
	return selected
}
