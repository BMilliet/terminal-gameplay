package shellstate

import (
	"reflect"
	"testing"

	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"
)

type fakeFileManager struct {
	configContent   string
	featuresContent string
	setupCalls      int
}

func (f *fakeFileManager) BasicSetup() error {
	f.setupCalls++
	return nil
}

func (f *fakeFileManager) GetConfigContent() (string, error) {
	return f.configContent, nil
}

func (f *fakeFileManager) GetFeaturesContent() (string, error) {
	return f.featuresContent, nil
}

func TestLoadCommandsBuildsCurrentPosixShellState(t *testing.T) {
	fileManager := &fakeFileManager{
		configContent: `{
			"env": {
				"FOO": {"value": "123", "active": true},
				"OLD": {"value": "456", "active": false}
			},
			"aliases": {
				"cat": {"value": "bat", "active": true},
				"ll": {"value": "ls -la", "active": false}
			}
		}`,
		featuresContent: `{"env": true, "alias": true}`,
	}

	got, err := LoadCommands(fileManager, "")
	if err != nil {
		t.Fatalf("LoadCommands() error = %v", err)
	}

	want := []string{
		"export FOO='123';",
		"unset OLD;",
		"builtin alias cat='bat';",
		"builtin unalias ll 2>/dev/null || builtin true;",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCommands() = %#v, want %#v", got, want)
	}
	if fileManager.setupCalls != 1 {
		t.Fatalf("BasicSetup() calls = %d, want 1", fileManager.setupCalls)
	}
}

func TestLoadCommandsUsesDefaultsForEmptyFiles(t *testing.T) {
	fileManager := &fakeFileManager{}

	got, err := LoadCommands(fileManager, envtab.FishShell)
	if err != nil {
		t.Fatalf("LoadCommands() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadCommands() = %#v, want no commands", got)
	}
}

func TestCommandsDisablesManagedStateWhenFeaturesAreDisabled(t *testing.T) {
	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys:   []string{"FOO"},
			Values: map[string]utils.EnvValue{"FOO": {Value: "123", Active: true}},
		},
		Aliases: utils.OrderedAliasMap{
			Keys:   []string{"cat"},
			Values: map[string]utils.AliasValue{"cat": {Value: "bat", Active: true}},
		},
	}

	got, err := Commands(config, &settings.FeaturesDTO{}, envtab.FishShell)
	if err != nil {
		t.Fatalf("Commands() error = %v", err)
	}

	want := []string{"set -e FOO;", "builtin functions -e cat;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands() = %#v, want %#v", got, want)
	}
}
