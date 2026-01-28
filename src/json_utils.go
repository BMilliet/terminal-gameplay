package src

import (
	"encoding/json"
	"fmt"
)

// ParseJSONContent parses JSON string into a struct
func ParseJSONContent[T any](content string) (*T, error) {
	var result T
	err := json.Unmarshal([]byte(content), &result)
	if err != nil {
		return nil, fmt.Errorf("ParseJSONContent -> %v", err)
	}
	return &result, nil
}

// ToJSON converts a struct to JSON string
func ToJSON[T any](data T) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("ToJSON -> %v", err)
	}
	return string(bytes), nil
}

// ConfigItemsToListItems converts config items to list items maintaining JSON order
func ConfigItemsToListItems(items OrderedMap) []ListItem {
	listItems := []ListItem{}
	for _, key := range items.Keys {
		if value, ok := items.Values[key]; ok {
			// Check if it's a divider (key starts with "div")
			isDiv := len(key) >= 3 && key[:3] == "div"
			listItems = append(listItems, ListItem{
				T:     key,
				D:     value,
				IsDiv: isDiv,
			})
		}
	}
	return listItems
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() *ConfigDTO {
	return &ConfigDTO{
		GoTo: OrderedMap{
			Keys:   []string{"home"},
			Values: map[string]string{"home": "~"},
		},
		Commands: OrderedMap{
			Keys:   []string{"example"},
			Values: map[string]string{"example": "echo 'Add your commands in config.json'"},
		},
		Notes: OrderedMap{
			Keys:   []string{"example"},
			Values: map[string]string{"example": "Add your notes in config.json"},
		},
	}
}
