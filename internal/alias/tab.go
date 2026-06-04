package alias

import (
	"fmt"
	"regexp"
	"strings"

	"terminal-gameplay/internal/utils"
)

const (
	PageName      = "alias"
	AddAction     = "__ADD_ALIAS__"
	DeleteAction  = "__DELETE_ALIAS__"
	ActiveState   = "active"
	InactiveState = "inactive"
	FishShell     = "fish"
)

var validName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var reservedNames = map[string]struct{}{
	"alias":     {},
	"and":       {},
	"argparse":  {},
	"begin":     {},
	"break":     {},
	"builtin":   {},
	"case":      {},
	"command":   {},
	"continue":  {},
	"else":      {},
	"end":       {},
	"eval":      {},
	"exec":      {},
	"for":       {},
	"function":  {},
	"functions": {},
	"if":        {},
	"not":       {},
	"or":        {},
	"read":      {},
	"return":    {},
	"set":       {},
	"status":    {},
	"string":    {},
	"switch":    {},
	"test":      {},
	"tg":        {},
	"time":      {},
	"unalias":   {},
	"while":     {},
}

func BuildList(items utils.OrderedAliasMap) []utils.ListItem {
	listItems := make([]utils.ListItem, 0, items.Len())
	for _, name := range items.Keys {
		value, ok := items.Values[name]
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
			T:      name,
			D:      description,
			Status: state,
		})
	}

	return listItems
}

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	if name == AddAction || name == DeleteAction {
		return fmt.Errorf("reserved alias name %q", name)
	}
	if _, reserved := reservedNames[name]; reserved {
		return fmt.Errorf("reserved alias name %q", name)
	}
	return nil
}

func ShellCommands(items utils.OrderedAliasMap, shell string) ([]string, error) {
	if err := validateItems(items); err != nil {
		return nil, err
	}

	commands := make([]string, 0, items.Len())
	for _, name := range items.Keys {
		value, ok := items.Values[name]
		if !ok {
			continue
		}

		if value.Active {
			commands = append(commands, setCommand(name, value.Value, shell))
			continue
		}

		command, err := RemoveCommand(name, shell)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}

	return commands, nil
}

func DisableShellCommands(items utils.OrderedAliasMap, shell string) ([]string, error) {
	if err := validateItems(items); err != nil {
		return nil, err
	}

	commands := make([]string, 0, items.Len())
	for _, name := range items.Keys {
		if _, ok := items.Values[name]; !ok {
			continue
		}

		command, err := RemoveCommand(name, shell)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}

	return commands, nil
}

func RemoveCommand(name, shell string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if shell == FishShell {
		return fmt.Sprintf("builtin functions -e %s;", name), nil
	}
	return fmt.Sprintf("builtin unalias %s 2>/dev/null || builtin true;", name), nil
}

func validateItems(items utils.OrderedAliasMap) error {
	for _, name := range items.Keys {
		if _, ok := items.Values[name]; !ok {
			continue
		}
		if err := ValidateName(name); err != nil {
			return err
		}
	}
	return nil
}

func setCommand(name, value, shell string) string {
	if shell == FishShell {
		return fmt.Sprintf("alias %s %s;", name, fishQuote(value))
	}
	return fmt.Sprintf("builtin alias %s=%s;", name, posixQuote(value))
}

func posixQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func fishQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(value) + "'"
}
