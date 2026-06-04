package shellstate

import (
	"fmt"
	"strings"

	aliastab "terminal-gameplay/internal/alias"
	envtab "terminal-gameplay/internal/env"
	"terminal-gameplay/internal/settings"
	"terminal-gameplay/internal/utils"
)

type FileManager interface {
	BasicSetup() error
	GetConfigContent() (string, error)
	GetFeaturesContent() (string, error)
}

func LoadCommands(fileManager FileManager, shell string) ([]string, error) {
	if err := fileManager.BasicSetup(); err != nil {
		return nil, fmt.Errorf("initialize application: %w", err)
	}

	features, err := loadFeatures(fileManager)
	if err != nil {
		return nil, err
	}

	config, err := loadConfig(fileManager)
	if err != nil {
		return nil, err
	}

	commands, err := Commands(config, features, shell)
	if err != nil {
		return nil, err
	}

	if notification := injectionNotification(config, features); notification != "" {
		commands = append(commands, notification)
	}
	return commands, nil
}

func Commands(config *utils.ConfigDTO, features *settings.FeaturesDTO, shell string) ([]string, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if features == nil {
		return nil, fmt.Errorf("features are required")
	}

	var commands []string
	var err error
	if features.Env {
		commands, err = envtab.ShellCommands(config.Env, shell)
	} else {
		commands, err = envtab.DisableShellCommands(config.Env, shell)
	}
	if err != nil {
		return nil, err
	}

	var aliasCommands []string
	if features.Alias {
		aliasCommands, err = aliastab.ShellCommands(config.Aliases, shell)
	} else {
		aliasCommands, err = aliastab.DisableShellCommands(config.Aliases, shell)
	}
	if err != nil {
		return nil, err
	}

	return append(commands, aliasCommands...), nil
}

func loadFeatures(fileManager FileManager) (*settings.FeaturesDTO, error) {
	content, err := fileManager.GetFeaturesContent()
	if err != nil {
		return nil, fmt.Errorf("read features: %w", err)
	}
	if content == "" {
		return settings.GetDefaultFeatures(), nil
	}

	features, err := utils.ParseJSONContent[settings.FeaturesDTO](content)
	if err != nil {
		return nil, fmt.Errorf("parse features.json: %w", err)
	}
	features.Normalize()
	return features, nil
}

func loadConfig(fileManager FileManager) (*utils.ConfigDTO, error) {
	content, err := fileManager.GetConfigContent()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if content == "" {
		return utils.GetDefaultConfig(), nil
	}

	config, err := utils.ParseJSONContent[utils.ConfigDTO](content)
	if err != nil {
		return nil, fmt.Errorf("parse config.json: %w", err)
	}
	return config, nil
}

func injectionNotification(config *utils.ConfigDTO, features *settings.FeaturesDTO) string {
	var injected []string
	if features.Env && hasActiveItems(config.Env) {
		injected = append(injected, "env")
	}
	if features.Alias && hasActiveItems(config.Aliases) {
		injected = append(injected, "aliases")
	}
	if len(injected) == 0 {
		return ""
	}

	return fmt.Sprintf("builtin printf '%%s\\n' '[tg] injected %s';", strings.Join(injected, " and "))
}

func hasActiveItems(items utils.OrderedEnvMap) bool {
	for _, key := range items.Keys {
		if value, ok := items.Values[key]; ok && value.Active {
			return true
		}
	}
	return false
}
