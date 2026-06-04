package ui

import (
	"strings"
	"testing"

	aliastab "terminal-gameplay/internal/alias"
	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestEnvPageShowsKeyValueAndStatusAndTogglesOnEnter(t *testing.T) {
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
	if got := model.envList[0]; got.T != "FOO" || got.D != "secret-value" || got.Status != envtab.ActiveState {
		t.Fatalf("env item = %#v, want FOO value and active state", got)
	}
	if view := model.View(); !strings.Contains(view, "secret-value") || !strings.Contains(view, "active ✓") {
		t.Fatalf("env view does not show value and status: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != envtab.PageName+"|FOO|secret-value" {
		t.Fatalf("selected = %q, want env toggle result", selected)
	}
}

func TestEnvValueUsesGoToValueColorAndSeparateStatusColor(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "123", Active: true},
			},
		},
	}
	model := NewMultiPageViewModel(config, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	model.cursor = 1

	row := model.renderListItem(model.envList[0], 0)
	valueStyle := lipgloss.NewStyle().Foreground(model.styles.FooterColor).Render("123")
	statusStyle := lipgloss.NewStyle().Foreground(model.styles.SettingsEnabledColor).Render("active ✓")
	statusColoredValue := lipgloss.NewStyle().Foreground(model.styles.SettingsEnabledColor).Render("123")

	if !strings.Contains(row, valueStyle) {
		t.Fatalf("env value does not use goTo value color: %q", row)
	}
	if !strings.Contains(row, statusStyle) {
		t.Fatalf("env active status does not use enabled color: %q", row)
	}
	if strings.Contains(row, statusColoredValue) {
		t.Fatalf("env value incorrectly uses status color: %q", row)
	}
}

func TestEnvStatusesUseFixedColumn(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"SHORT", "LONG"},
			Values: map[string]utils.EnvValue{
				"SHORT": {Value: "1", Active: true},
				"LONG":  {Value: "a much longer value", Active: false},
			},
		},
	}
	model := NewMultiPageViewModel(config, &settings.FeaturesDTO{Env: true}, envtab.PageName)
	model.cursor = len(model.envList)

	activeRow := model.renderListItem(model.envList[0], 0)
	inactiveRow := model.renderListItem(model.envList[1], 1)
	activeColumn := strings.Index(activeRow, "active ✓")
	inactiveColumn := strings.Index(inactiveRow, "inactive ✗")

	if activeColumn < 0 || inactiveColumn < 0 {
		t.Fatalf("status missing from rows: active=%q inactive=%q", activeRow, inactiveRow)
	}
	if activeColumn != inactiveColumn {
		t.Fatalf("status columns differ: active=%d inactive=%d", activeColumn, inactiveColumn)
	}

	valueAreaWidth := model.contentWidth() - model.titleColumnWidth() - rowChromeWidth
	if got := model.toggleValueColumnWidth(valueAreaWidth); got != toggleValueWidth {
		t.Fatalf("value column width = %d, want standard width %d", got, toggleValueWidth)
	}
}

func TestEnvAndAliasStatusesUseSameColumn(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	config := &utils.ConfigDTO{
		Env: utils.OrderedEnvMap{
			Keys: []string{"FOO"},
			Values: map[string]utils.EnvValue{
				"FOO": {Value: "short", Active: true},
			},
		},
		Aliases: utils.OrderedAliasMap{
			Keys: []string{"long_alias_name"},
			Values: map[string]utils.AliasValue{
				"long_alias_name": {Value: "a much longer command", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Env: true, Alias: true}
	envModel := NewMultiPageViewModel(config, features, envtab.PageName)
	aliasModel := NewMultiPageViewModel(config, features, aliastab.PageName)
	envModel.cursor = len(envModel.envList)
	aliasModel.cursor = len(aliasModel.aliasList)

	envColumn := strings.Index(envModel.renderListItem(envModel.envList[0], 0), "active ✓")
	aliasColumn := strings.Index(aliasModel.renderListItem(aliasModel.aliasList[0], 0), "active ✓")

	if envColumn < 0 || aliasColumn < 0 {
		t.Fatalf("status missing from rows: env=%d alias=%d", envColumn, aliasColumn)
	}
	if envColumn != aliasColumn {
		t.Fatalf("status columns differ: env=%d alias=%d", envColumn, aliasColumn)
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

func TestAliasPageShowsNameCommandAndStatusAndTogglesOnEnter(t *testing.T) {
	config := &utils.ConfigDTO{
		Aliases: utils.OrderedAliasMap{
			Keys: []string{"cat"},
			Values: map[string]utils.AliasValue{
				"cat": {Value: "bat --plain", Active: true},
			},
		},
	}
	features := &settings.FeaturesDTO{Alias: true}
	selected := ""

	model := NewMultiPageViewModel(config, features, aliastab.PageName)
	model.selected = &selected

	if model.currentPage != AliasPage {
		t.Fatalf("current page = %v, want AliasPage", model.currentPage)
	}
	if got := model.aliasList[0]; got.T != "cat" || got.D != "bat --plain" || got.Status != aliastab.ActiveState {
		t.Fatalf("alias item = %#v, want cat command and active state", got)
	}
	if view := model.View(); !strings.Contains(view, "bat --plain") || !strings.Contains(view, "active ✓") {
		t.Fatalf("alias view does not show command and status: %q", view)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != aliastab.PageName+"|cat|bat --plain" {
		t.Fatalf("selected = %q, want alias toggle result", selected)
	}
}

func TestAliasPageAddShortcutReturnsAddAction(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Alias: true}, aliastab.PageName)
	selected := ""
	model.selected = &selected

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if selected != aliastab.PageName+"|"+aliastab.AddAction+"|" {
		t.Fatalf("selected = %q, want alias add action", selected)
	}
}

func TestAliasPageIsHiddenWhenFeatureIsDisabled(t *testing.T) {
	model := NewMultiPageViewModel(&utils.ConfigDTO{}, &settings.FeaturesDTO{Alias: false})

	for _, page := range model.availPages {
		if page == AliasPage {
			t.Fatal("AliasPage should not be available when alias feature is disabled")
		}
	}
}
