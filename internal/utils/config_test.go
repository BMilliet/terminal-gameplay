package utils

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOrderedMapPreservesJSONOrder(t *testing.T) {
	var items OrderedMap
	if err := json.Unmarshal([]byte(`{"first":"1","second":"2","third":"3"}`), &items); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	wantKeys := []string{"first", "second", "third"}
	if !reflect.DeepEqual(items.Keys, wantKeys) {
		t.Fatalf("keys = %#v, want %#v", items.Keys, wantKeys)
	}

	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	wantJSON := `{"first":"1","second":"2","third":"3"}`
	if string(encoded) != wantJSON {
		t.Fatalf("json = %s, want %s", encoded, wantJSON)
	}
}

func TestInsertInSectionPlacesItemBeforeNextDivider(t *testing.T) {
	items := OrderedMap{
		Keys: []string{"home", "div", "api", "div2", "notes"},
		Values: map[string]string{
			"home":  "~",
			"div":   "work",
			"api":   "~/api",
			"div2":  "personal",
			"notes": "~/notes",
		},
	}

	items.InsertInSection("__ROOT__", "div", "web", "~/web")

	wantKeys := []string{"home", "div", "api", "web", "div2", "notes"}
	if !reflect.DeepEqual(items.Keys, wantKeys) {
		t.Fatalf("keys = %#v, want %#v", items.Keys, wantKeys)
	}
	if got := items.Values["web"]; got != "~/web" {
		t.Fatalf("web value = %q, want ~/web", got)
	}
}

func TestFileNameWithExtensionSanitizesPathLikeInput(t *testing.T) {
	got := FileNameWithExtension("../deploy", ".lua")

	if got != "deploy.lua" {
		t.Fatalf("FileNameWithExtension() = %q, want deploy.lua", got)
	}

	got = FileNameWithExtension("report.md", ".md")
	if got != "report.md" {
		t.Fatalf("FileNameWithExtension() = %q, want report.md", got)
	}

	got = FileNameWithExtension("test file name", ".md")
	if got != "test_file_name.md" {
		t.Fatalf("FileNameWithExtension() = %q, want test_file_name.md", got)
	}

	got = FileNameWithExtension("get  current\tbranch.lua", ".lua")
	if got != "get_current_branch.lua" {
		t.Fatalf("FileNameWithExtension() = %q, want get_current_branch.lua", got)
	}
}

func TestOrderedEnvMapSupportsShorthandAndPreservesStateAndOrder(t *testing.T) {
	var items OrderedEnvMap
	if err := json.Unmarshal([]byte(`{"FOO":"123","BAR":{"value":"456","active":false}}`), &items); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if want := []string{"FOO", "BAR"}; !reflect.DeepEqual(items.Keys, want) {
		t.Fatalf("keys = %#v, want %#v", items.Keys, want)
	}
	if got, ok := items.Get("FOO"); !ok || got.Value != "123" || !got.Active {
		t.Fatalf("FOO = %#v, %v; want active shorthand value", got, ok)
	}
	if got, ok := items.Get("BAR"); !ok || got.Value != "456" || got.Active {
		t.Fatalf("BAR = %#v, %v; want inactive structured value", got, ok)
	}

	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	wantJSON := `{"FOO":{"value":"123","active":true},"BAR":{"value":"456","active":false}}`
	if string(encoded) != wantJSON {
		t.Fatalf("json = %s, want %s", encoded, wantJSON)
	}
}

func TestOrderedAliasMapSupportsShorthandAndPreservesStateAndOrder(t *testing.T) {
	var items OrderedAliasMap
	if err := json.Unmarshal([]byte(`{"cat":"bat","ll":{"value":"ls -la","active":false}}`), &items); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if want := []string{"cat", "ll"}; !reflect.DeepEqual(items.Keys, want) {
		t.Fatalf("keys = %#v, want %#v", items.Keys, want)
	}
	if got, ok := items.Get("cat"); !ok || got.Value != "bat" || !got.Active {
		t.Fatalf("cat = %#v, %v; want active shorthand value", got, ok)
	}
	if got, ok := items.Get("ll"); !ok || got.Value != "ls -la" || got.Active {
		t.Fatalf("ll = %#v, %v; want inactive structured value", got, ok)
	}
}
