package src

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type ConfigDTO struct {
	GoTo    OrderedMap `json:"goTo"`
	Scripts OrderedMap `json:"scripts"`
	Notes   OrderedMap `json:"notes"`

	migratedLegacyCommands bool
}

type ConfigItem struct {
	Label string
	Value string
}

func (c *ConfigDTO) UnmarshalJSON(data []byte) error {
	type configJSON struct {
		GoTo     OrderedMap `json:"goTo"`
		Scripts  OrderedMap `json:"scripts"`
		Commands OrderedMap `json:"commands"`
		Notes    OrderedMap `json:"notes"`
	}

	var parsed configJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	c.GoTo = parsed.GoTo
	c.Scripts = parsed.Scripts
	c.Notes = parsed.Notes
	c.migratedLegacyCommands = false

	if c.Scripts.Len() == 0 && parsed.Commands.Len() > 0 {
		c.Scripts = parsed.Commands
		c.migratedLegacyCommands = true
	}

	return nil
}

// OrderedMap preserves the order of keys as they appear in JSON
type OrderedMap struct {
	Keys   []string
	Values map[string]string
}

// UnmarshalJSON custom unmarshaler to preserve key order
func (om *OrderedMap) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a map to get values
	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	// Parse again to extract key order
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Unmarshal a third time using decoder to preserve order
	dec := json.NewDecoder(bytes.NewReader(data))

	// Read opening brace
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected {, got %v", t)
	}

	keys := []string{}
	for dec.More() {
		// Read key
		t, err := dec.Token()
		if err != nil {
			return err
		}
		key := t.(string)
		keys = append(keys, key)

		// Read value (skip it, we already have it in the map)
		var value string
		if err := dec.Decode(&value); err != nil {
			return err
		}
	}

	om.Keys = keys
	om.Values = values
	return nil
}

// MarshalJSON custom marshaler
func (om OrderedMap) MarshalJSON() ([]byte, error) {
	if om.Values == nil {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("{")

	for i, key := range om.Keys {
		if i > 0 {
			buf.WriteString(",")
		}

		keyJSON, _ := json.Marshal(key)
		valueJSON, _ := json.Marshal(om.Values[key])

		buf.Write(keyJSON)
		buf.WriteString(":")
		buf.Write(valueJSON)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}

// Get returns the value for a key
func (om OrderedMap) Get(key string) (string, bool) {
	val, ok := om.Values[key]
	return val, ok
}

func (om *OrderedMap) Set(key, value string) {
	if om.Values == nil {
		om.Values = make(map[string]string)
	}

	if _, exists := om.Values[key]; !exists {
		om.Keys = append(om.Keys, key)
	}

	om.Values[key] = value
}

// Len returns the number of items
func (om OrderedMap) Len() int {
	return len(om.Keys)
}
