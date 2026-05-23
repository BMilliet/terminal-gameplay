package gototab

import "terminal-gameplay/internal/utils"

const (
	PageName         = "goTo"
	AddAction        = "__ADD_GOTO__"
	DeleteAction     = "__DELETE_GOTO__"
	AddSectionAction = "__ADD_GOTO_SECTION__"
	RootSection      = "__ROOT_GOTO_SECTION__"
)

func BuildList(goTo utils.OrderedMap) []utils.ListItem {
	return utils.OrderedMapToListItems(goTo)
}

func BuildSectionOptions(goTo utils.OrderedMap) []utils.ListItem {
	sections := []utils.ListItem{
		{
			T: RootSection,
			D: "top / no section",
		},
	}

	for _, key := range goTo.Keys {
		if !utils.IsDividerKey(key) {
			continue
		}

		sections = append(sections, utils.ListItem{
			T: key,
			D: goTo.Values[key],
		})
	}

	return sections
}
