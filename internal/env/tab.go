package env

import (
	"fmt"
	"regexp"
	"strings"

	"terminal-gameplay/internal/utils"
)

const (
	PageName            = "env"
	AddAction           = "__ADD_ENV__"
	DeleteAction        = "__DELETE_ENV__"
	ActiveState         = "active"
	InactiveState       = "inactive"
	ShellIntegrationEnv = "TG_SHELL_INTEGRATION"
	FishShell           = "fish"
)

var validKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Runtime interface {
	SetEnv(key, value string) error
	UnsetEnv(key string) error
}

func BuildList(items utils.OrderedEnvMap) []utils.ListItem {
	listItems := make([]utils.ListItem, 0, items.Len())
	for _, key := range items.Keys {
		value, ok := items.Values[key]
		if !ok {
			continue
		}

		state := InactiveState
		description := "inactive ✗"
		if value.Active {
			state = ActiveState
			description = "active ✓"
		}

		listItems = append(listItems, utils.ListItem{
			T:      key,
			D:      description,
			Status: state,
		})
	}

	return listItems
}

func ValidateKey(key string) error {
	if !validKey.MatchString(key) {
		return fmt.Errorf("invalid env key %q", key)
	}
	if key == AddAction || key == DeleteAction || key == ShellIntegrationEnv {
		return fmt.Errorf("reserved env key %q", key)
	}
	return nil
}

func Apply(items utils.OrderedEnvMap, runtime Runtime) error {
	if err := validateItems(items); err != nil {
		return err
	}

	for _, key := range items.Keys {
		value, ok := items.Values[key]
		if !ok {
			continue
		}

		if value.Active {
			if err := runtime.SetEnv(key, value.Value); err != nil {
				return err
			}
			continue
		}

		if err := runtime.UnsetEnv(key); err != nil {
			return err
		}
	}

	return nil
}

func Disable(items utils.OrderedEnvMap, runtime Runtime) error {
	if err := validateItems(items); err != nil {
		return err
	}

	for _, key := range items.Keys {
		if _, ok := items.Values[key]; !ok {
			continue
		}
		if err := runtime.UnsetEnv(key); err != nil {
			return err
		}
	}

	return nil
}

func ShellCommands(items utils.OrderedEnvMap, shell string) ([]string, error) {
	if err := validateItems(items); err != nil {
		return nil, err
	}

	commands := make([]string, 0, items.Len())
	for _, key := range items.Keys {
		value, ok := items.Values[key]
		if !ok {
			continue
		}

		if value.Active {
			commands = append(commands, setCommand(key, value.Value, shell))
			continue
		}

		command, err := UnsetCommand(key, shell)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}

	return commands, nil
}

func DisableShellCommands(items utils.OrderedEnvMap, shell string) ([]string, error) {
	if err := validateItems(items); err != nil {
		return nil, err
	}

	commands := make([]string, 0, items.Len())
	for _, key := range items.Keys {
		if _, ok := items.Values[key]; !ok {
			continue
		}

		command, err := UnsetCommand(key, shell)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}

	return commands, nil
}

func validateItems(items utils.OrderedEnvMap) error {
	for _, key := range items.Keys {
		if _, ok := items.Values[key]; !ok {
			continue
		}
		if err := ValidateKey(key); err != nil {
			return err
		}
	}
	return nil
}

func UnsetCommand(key, shell string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if shell == FishShell {
		return fmt.Sprintf("set -e %s;", key), nil
	}
	return fmt.Sprintf("unset %s;", key), nil
}

func setCommand(key, value, shell string) string {
	if shell == FishShell {
		return fmt.Sprintf("set -gx %s %s;", key, fishQuote(value))
	}
	return fmt.Sprintf("export %s=%s;", key, posixQuote(value))
}

func posixQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func fishQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(value) + "'"
}
