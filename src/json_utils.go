package src

import (
	"encoding/json"
	"fmt"
	"strings"
)

const notePreviewMaxRunes = 64

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
			isDiv := IsDividerKey(key)
			listItems = append(listItems, ListItem{
				T:     key,
				D:     value,
				IsDiv: isDiv,
			})
		}
	}
	return listItems
}

func ConfigNotesToListItems(items OrderedMap) []ListItem {
	listItems := []ListItem{}
	for _, key := range items.Keys {
		if value, ok := items.Values[key]; ok {
			isDiv := IsDividerKey(key)
			description := value
			if !isDiv {
				description = NotePreview(value)
			}

			listItems = append(listItems, ListItem{
				T:     key,
				D:     description,
				IsDiv: isDiv,
			})
		}
	}
	return listItems
}

func IsDividerKey(key string) bool {
	return strings.HasPrefix(key, "div")
}

func NotePreview(text string) string {
	preview := strings.Join(strings.Fields(text), " ")
	if preview == "" {
		return "(empty note)"
	}

	runes := []rune(preview)
	if len(runes) <= notePreviewMaxRunes {
		return preview
	}

	return string(runes[:notePreviewMaxRunes-3]) + "..."
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
