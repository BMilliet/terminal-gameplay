package clipboard

import (
	"fmt"

	"terminal-gameplay/internal/utils"
)

const (
	PageName     = "clipboard"
	AddAction    = "__ADD_CLIPBOARD__"
	DeleteAction = "__DELETE_CLIPBOARD__"
	keyPrefix    = "clip"
)

func BuildList(items utils.OrderedMap) []utils.ListItem {
	listItems := make([]utils.ListItem, 0, items.Len())
	for _, key := range items.Keys {
		value, ok := items.Values[key]
		if !ok {
			continue
		}

		listItems = append(listItems, utils.ListItem{
			T: key,
			D: value,
		})
	}
	return listItems
}

func NextKey(items utils.OrderedMap) string {
	if _, exists := items.Values[keyPrefix]; !exists {
		return keyPrefix
	}

	for i := 1; ; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		if _, exists := items.Values[key]; !exists {
			return key
		}
	}
}
