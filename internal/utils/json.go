package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ListItem struct {
	T      string
	D      string
	IsDiv  bool
	Status string
}

func (i ListItem) Title() string       { return i.T }
func (i ListItem) Description() string { return i.D }
func (i ListItem) FilterValue() string { return i.T }

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

// OrderedMapToListItems converts ordered map items to list items preserving JSON order.
func OrderedMapToListItems(items OrderedMap) []ListItem {
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

func IsDividerKey(key string) bool {
	return strings.HasPrefix(key, "div")
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() *ConfigDTO {
	return &ConfigDTO{
		GoTo: OrderedMap{
			Keys:   []string{"home"},
			Values: map[string]string{"home": "~"},
		},
		Scripts: OrderedMap{
			Keys:   []string{"example"},
			Values: map[string]string{"example": "Example Lua script"},
		},
		Notes: OrderedMap{
			Keys:   []string{"example"},
			Values: map[string]string{"example": "Add your notes in config.json"},
		},
		Clipboard: OrderedMap{
			Values: make(map[string]string),
		},
		Env: OrderedEnvMap{
			Values: make(map[string]EnvValue),
		},
		Aliases: OrderedAliasMap{
			Values: make(map[string]AliasValue),
		},
	}
}
