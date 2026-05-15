package src

type ViewBuilderInterface interface {
	NewListView(title string, op []ListItem, height int) ListItem
	NewConfirmView(title string) bool
	NewTextFieldView(title, placeHolder string) string
	NewMultiPageView(config *ConfigDTO, features *FeaturesDTO) string
}

type ViewBuilder struct{}

func NewViewBuilder() *ViewBuilder {
	return &ViewBuilder{}
}

func (b *ViewBuilder) NewListView(title string, op []ListItem, height int) ListItem {
	endValue := ListItem{}
	ListView(title, op, height, &endValue)
	return endValue
}

func (b *ViewBuilder) NewConfirmView(title string) bool {
	confirmed := false
	ConfirmView(title, &confirmed)
	return confirmed
}

func (b *ViewBuilder) NewTextFieldView(title, placeHolder string) string {
	endValue := ""
	TextFieldView(title, placeHolder, &endValue)
	return endValue
}

func (b *ViewBuilder) NewMultiPageView(config *ConfigDTO, features *FeaturesDTO) string {
	selected := ""
	MultiPageView(config, features, &selected)
	return selected
}
