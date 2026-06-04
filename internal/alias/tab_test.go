package alias

import (
	"reflect"
	"testing"

	"terminal-gameplay/internal/utils"
)

func TestBuildListShowsNamesAndActiveStatesWithoutCommands(t *testing.T) {
	items := utils.OrderedAliasMap{
		Keys: []string{"cat", "ll"},
		Values: map[string]utils.AliasValue{
			"cat": {Value: "bat", Active: true},
			"ll":  {Value: "ls -la", Active: false},
		},
	}

	got := BuildList(items)

	want := []utils.ListItem{
		{T: "cat", D: "active ✓", Status: ActiveState},
		{T: "ll", D: "inactive ✗", Status: InactiveState},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildList() = %#v, want %#v", got, want)
	}
}

func TestValidateNameRejectsInvalidAndReservedNames(t *testing.T) {
	if err := ValidateName("cat"); err != nil {
		t.Fatalf("ValidateName(cat) error = %v", err)
	}
	if err := ValidateName("invalid-name"); err == nil {
		t.Fatal("ValidateName() expected invalid name error")
	}
	for _, name := range []string{AddAction, "alias", "command", "set", "test", "tg"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("ValidateName(%q) expected reserved name error", name)
		}
	}
}

func TestShellCommandsUseShellSpecificSyntaxAndQuoteCommands(t *testing.T) {
	items := utils.OrderedAliasMap{
		Keys: []string{"cat", "ll"},
		Values: map[string]utils.AliasValue{
			"cat": {Value: "bat --theme='dark'", Active: true},
			"ll":  {Active: false},
		},
	}

	posix, err := ShellCommands(items, "")
	if err != nil {
		t.Fatalf("ShellCommands(posix) error = %v", err)
	}
	wantPosix := []string{
		`builtin alias cat='bat --theme='"'"'dark'"'"'';`,
		"builtin unalias ll 2>/dev/null || builtin true;",
	}
	if !reflect.DeepEqual(posix, wantPosix) {
		t.Fatalf("posix commands = %#v, want %#v", posix, wantPosix)
	}

	fish, err := ShellCommands(items, FishShell)
	if err != nil {
		t.Fatalf("ShellCommands(fish) error = %v", err)
	}
	wantFish := []string{
		`alias cat 'bat --theme=\'dark\'';`,
		"builtin functions -e ll;",
	}
	if !reflect.DeepEqual(fish, wantFish) {
		t.Fatalf("fish commands = %#v, want %#v", fish, wantFish)
	}
}

func TestDisableShellCommandsRemovesEveryManagedAlias(t *testing.T) {
	items := utils.OrderedAliasMap{
		Keys: []string{"cat", "ll"},
		Values: map[string]utils.AliasValue{
			"cat": {Value: "bat", Active: true},
			"ll":  {Value: "ls -la", Active: false},
		},
	}

	got, err := DisableShellCommands(items, FishShell)
	if err != nil {
		t.Fatalf("DisableShellCommands() error = %v", err)
	}
	want := []string{"builtin functions -e cat;", "builtin functions -e ll;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}
