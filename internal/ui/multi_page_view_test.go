package ui

import (
	"strings"
	"testing"

	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEnvPageShowsOnlyKeyAndStatusAndTogglesOnEnter(t *testing.T) {
	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "secret-value", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Env: true}
	selected := ""

	model := NewMultiPageViewModel(config, features, envtab.PageName)
	model.selected = &selected

	if model.currentPage != EnvPage {
		t.Fatalf("current page = %v, want EnvPage", model.currentPage)
	}
	if got := model.envList[0]; got.T != "FOO" || got.D != "active ✓" {
		t.Fatalf("env item = %#v, want FOO active", got)
	}
	if view := model.View(); strings.Contains(view, "secret-value") {
		t.Fatalf("env view exposed value: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != envtab.PageName+"|FOO|active ✓" {
		t.Fatalf("selected = %q, want env toggle result", selected)
	}
}

func TestEnvPageAddShortcutReturnsAddAction(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	selected := ""
	model.selected = &selected

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if selected != envtab.PageName+"|"+envtab.AddAction+"|" {
		t.Fatalf("selected = %q, want env add action", selected)
	}
}

func TestEnvPageIsHiddenWhenFeatureIsDisabled(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Env: false})

	for _, page := range model.availPages {
		if page == EnvPage {
			t.Fatal("EnvPage should not be available when env feature is disabled")
		}
	}
}
