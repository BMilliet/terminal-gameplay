package env

import (
	"reflect"
	"testing"

	"terminal-gameplay/internal/utils"
)

type fakeRuntime struct {
	set   map[string]string
	unset []string
}

func (f *fakeRuntime) SetEnv(key, value string) error {
	f.set[key] = value
	return nil
}

func (f *fakeRuntime) UnsetEnv(key string) error {
	delete(f.set, key)
	f.unset = append(f.unset, key)
	return nil
}

func TestBuildListShowsKeysAndActiveStatesWithoutValues(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "SECRET"},
		Values: map[string]utils.EnvValue{
			"FOO":    {Value: "123", Active: true},
			"SECRET": {Value: "do-not-show", Active: false},
		},
	}

	got := BuildList(items)

	want := []utils.ListItem{
		{T: "FOO", D: "active ✓", Status: ActiveState},
		{T: "SECRET", D: "inactive ✗", Status: InactiveState},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildList() = %#v, want %#v", got, want)
	}
}

func TestValidateKeyRejectsInvalidAndReservedKeys(t *testing.T) {
	if err := ValidateKey("FOO"); err != nil {
		t.Fatalf("ValidateKey(FOO) error = %v", err)
	}
	if err := ValidateKey("INVALID-KEY"); err == nil {
		t.Fatal("ValidateKey() expected invalid key error")
	}
	if err := ValidateKey(AddAction); err == nil {
		t.Fatal("ValidateKey() expected reserved key error")
	}
	if err := ValidateKey(ShellIntegrationEnv); err == nil {
		t.Fatal("ValidateKey() expected internal shell marker to be reserved")
	}
}

func TestApplySetsOnlyActiveValuesAndUnsetsInactiveValues(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "OLD"},
		Values: map[string]utils.EnvValue{
			"FOO": {Value: "123", Active: true},
			"OLD": {Value: "456", Active: false},
		},
	}
	runtime := &fakeRuntime{set: map[string]string{"OLD": "previous"}}

	if err := Apply(items, runtime); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if got := runtime.set; !reflect.DeepEqual(got, map[string]string{"FOO": "123"}) {
		t.Fatalf("runtime env = %#v, want only active FOO", got)
	}
	if got := runtime.unset; !reflect.DeepEqual(got, []string{"OLD"}) {
		t.Fatalf("unset calls = %#v, want OLD", got)
	}
}

func TestApplyRejectsInvalidKeyBeforeChangingRuntime(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "INVALID-KEY"},
		Values: map[string]utils.EnvValue{
			"FOO":         {Value: "123", Active: true},
			"INVALID-KEY": {Value: "456", Active: true},
		},
	}
	runtime := &fakeRuntime{set: make(map[string]string)}

	if err := Apply(items, runtime); err == nil {
		t.Fatal("Apply() expected invalid key error")
	}
	if len(runtime.set) != 0 || len(runtime.unset) != 0 {
		t.Fatalf("runtime changed after validation error: set=%#v unset=%#v", runtime.set, runtime.unset)
	}
}

func TestDisableUnsetsEveryManagedKeyAndPreservesConfig(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "OLD"},
		Values: map[string]utils.EnvValue{
			"FOO": {Value: "123", Active: true},
			"OLD": {Value: "456", Active: false},
		},
	}
	runtime := &fakeRuntime{set: map[string]string{"FOO": "123", "OLD": "previous"}}

	if err := Disable(items, runtime); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	if len(runtime.set) != 0 {
		t.Fatalf("runtime env = %#v, want empty", runtime.set)
	}
	if got := runtime.unset; !reflect.DeepEqual(got, []string{"FOO", "OLD"}) {
		t.Fatalf("unset calls = %#v, want all managed keys", got)
	}
	if got := items.Values["FOO"]; got.Value != "123" || !got.Active {
		t.Fatalf("configured FOO changed = %#v", got)
	}
}

func TestShellCommandsUseShellSpecificSyntaxAndQuoteValues(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "OLD"},
		Values: map[string]utils.EnvValue{
			"FOO": {Value: "one'two", Active: true},
			"OLD": {Active: false},
		},
	}

	posix, err := ShellCommands(items, "")
	if err != nil {
		t.Fatalf("ShellCommands(posix) error = %v", err)
	}
	wantPosix := []string{`export FOO='one'"'"'two';`, "unset OLD;"}
	if !reflect.DeepEqual(posix, wantPosix) {
		t.Fatalf("posix commands = %#v, want %#v", posix, wantPosix)
	}

	fish, err := ShellCommands(items, FishShell)
	if err != nil {
		t.Fatalf("ShellCommands(fish) error = %v", err)
	}
	wantFish := []string{`set -gx FOO 'one\'two';`, "set -e OLD;"}
	if !reflect.DeepEqual(fish, wantFish) {
		t.Fatalf("fish commands = %#v, want %#v", fish, wantFish)
	}
}

func TestDisableShellCommandsUnsetAllKeys(t *testing.T) {
	items := utils.OrderedEnvMap{
		Keys: []string{"FOO", "OLD"},
		Values: map[string]utils.EnvValue{
			"FOO": {Value: "123", Active: true},
			"OLD": {Value: "456", Active: false},
		},
	}

	got, err := DisableShellCommands(items, FishShell)
	if err != nil {
		t.Fatalf("DisableShellCommands() error = %v", err)
	}
	want := []string{"set -e FOO;", "set -e OLD;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}
