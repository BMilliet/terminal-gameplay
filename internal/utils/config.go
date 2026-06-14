package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type ConfigDTO struct {
	GoTo      OrderedMap      `json:"goTo"`
	Scripts   OrderedMap      `json:"scripts"`
	Notes     OrderedMap      `json:"notes"`
	Clipboard OrderedMap      `json:"clipboard"`
	Env       OrderedEnvMap   `json:"env"`
	Aliases   OrderedAliasMap `json:"aliases"`

	migratedLegacyCommands bool
}

type ConfigItem struct {
	Label string
	Value string
}

func (c *ConfigDTO) UnmarshalJSON(data []byte) error {
	type configJSON struct {
		GoTo      OrderedMap      `json:"goTo"`
		Scripts   OrderedMap      `json:"scripts"`
		Commands  OrderedMap      `json:"commands"`
		Notes     OrderedMap      `json:"notes"`
		Clipboard OrderedMap      `json:"clipboard"`
		Env       OrderedEnvMap   `json:"env"`
		Aliases   OrderedAliasMap `json:"aliases"`
	}

	var parsed configJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	c.GoTo = parsed.GoTo
	c.Scripts = parsed.Scripts
	c.Notes = parsed.Notes
	c.Clipboard = parsed.Clipboard
	c.Env = parsed.Env
	c.Aliases = parsed.Aliases
	c.migratedLegacyCommands = false

	if c.Scripts.Len() == 0 && parsed.Commands.Len() > 0 {
		c.Scripts = parsed.Commands
		c.migratedLegacyCommands = true
	}

	return nil
}

func (c ConfigDTO) MigratedLegacyCommands() bool {
	return c.migratedLegacyCommands
}

type EnvValue struct {
	Value  string `json:"value"`
	Active bool   `json:"active"`
}

type AliasValue = EnvValue
type OrderedAliasMap = OrderedEnvMap

func (v *EnvValue) UnmarshalJSON(data []byte) error {
	var shorthand string
	if err := json.Unmarshal(data, &shorthand); err == nil {
		v.Value = shorthand
		v.Active = true
		return nil
	}

	type envValueJSON struct {
		Value  string `json:"value"`
		Active *bool  `json:"active"`
	}

	var parsed envValueJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	v.Value = parsed.Value
	v.Active = true
	if parsed.Active != nil {
		v.Active = *parsed.Active
	}

	return nil
}

// OrderedEnvMap preserves env key order while storing values and active state.
type OrderedEnvMap struct {
	Keys   []string
	Values map[string]EnvValue
}

func (om *OrderedEnvMap) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected {, got %v", t)
	}

	om.Keys = []string{}
	om.Values = make(map[string]EnvValue)
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}

		key := t.(string)
		var value EnvValue
		if err := dec.Decode(&value); err != nil {
			return err
		}

		om.Keys = append(om.Keys, key)
		om.Values[key] = value
	}

	return nil
}

func (om OrderedEnvMap) MarshalJSON() ([]byte, error) {
	if om.Values == nil {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteString("{")

	written := 0
	for _, key := range om.Keys {
		value, ok := om.Values[key]
		if !ok {
			continue
		}
		if written > 0 {
			buf.WriteString(",")
		}

		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		buf.Write(keyJSON)
		buf.WriteString(":")
		buf.Write(valueJSON)
		written++
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}

func (om OrderedEnvMap) Get(key string) (EnvValue, bool) {
	value, ok := om.Values[key]
	return value, ok
}

func (om *OrderedEnvMap) Set(key, value string, active bool) {
	if om.Values == nil {
		om.Values = make(map[string]EnvValue)
	}

	if _, exists := om.Values[key]; !exists {
		om.Keys = append(om.Keys, key)
	}

	om.Values[key] = EnvValue{Value: value, Active: active}
}

func (om *OrderedEnvMap) Delete(key string) {
	if om.Values != nil {
		delete(om.Values, key)
	}

	for i, existingKey := range om.Keys {
		if existingKey == key {
			om.Keys = append(om.Keys[:i], om.Keys[i+1:]...)
			return
		}
	}
}

func (om OrderedEnvMap) Len() int {
	return len(om.Keys)
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

func (om *OrderedMap) Delete(key string) {
	if om.Values != nil {
		delete(om.Values, key)
	}

	for i, existingKey := range om.Keys {
		if existingKey == key {
			om.Keys = append(om.Keys[:i], om.Keys[i+1:]...)
			return
		}
	}
}

func (om *OrderedMap) InsertInSection(rootSectionKey, sectionKey, key, value string) {
	if om.Values == nil {
		om.Values = make(map[string]string)
	}

	om.Delete(key)

	insertIndex := om.SectionInsertIndex(rootSectionKey, sectionKey)
	if insertIndex > len(om.Keys) {
		insertIndex = len(om.Keys)
	}

	om.Keys = append(om.Keys, "")
	copy(om.Keys[insertIndex+1:], om.Keys[insertIndex:])
	om.Keys[insertIndex] = key
	om.Values[key] = value
}

func (om *OrderedMap) AddDivider(label string) string {
	key := om.NextDividerKey()
	om.Set(key, label)
	return key
}

func (om OrderedMap) NextDividerKey() string {
	if _, exists := om.Values["div"]; !exists {
		return "div"
	}

	for i := 1; ; i++ {
		key := fmt.Sprintf("div%d", i)
		if _, exists := om.Values[key]; !exists {
			return key
		}
	}
}

func (om OrderedMap) SectionInsertIndex(rootSectionKey, sectionKey string) int {
	if sectionKey == rootSectionKey {
		for i, key := range om.Keys {
			if IsDividerKey(key) {
				return i
			}
		}
		return len(om.Keys)
	}

	for i, key := range om.Keys {
		if key != sectionKey {
			continue
		}

		insertIndex := i + 1
		for insertIndex < len(om.Keys) && !IsDividerKey(om.Keys[insertIndex]) {
			insertIndex++
		}
		return insertIndex
	}

	return len(om.Keys)
}

// Len returns the number of items
func (om OrderedMap) Len() int {
	return len(om.Keys)
}
