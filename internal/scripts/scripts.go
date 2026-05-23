package scripts

import (
	"fmt"
	"strings"

	"terminal-gameplay/internal/utils"
)

const (
	PageName      = "scripts"
	AddAction     = "__ADD_SCRIPT__"
	EditAction    = "__EDIT_SCRIPT__"
	DeleteAction  = "__DELETE_SCRIPT__"
	FileExtension = ".lua"
)

type FileManager interface {
	CheckIfPathExists(path string) (bool, error)
	DeleteFileIfExists(path string) error
	EnsureScriptsDir() error
	ScriptsPath(fileName string) string
	WriteFileContent(filePath, content string) error
}

func BuildList(items utils.OrderedMap) []utils.ListItem {
	return utils.OrderedMapToListItems(items)
}

func FileName(name string) string {
	return utils.FileNameWithExtension(name, FileExtension)
}

func EnsureFile(fm FileManager, name, description string) (string, error) {
	if err := fm.EnsureScriptsDir(); err != nil {
		return "", err
	}

	scriptPath := fm.ScriptsPath(FileName(name))
	exists, err := fm.CheckIfPathExists(scriptPath)
	if err != nil {
		return "", err
	}

	if !exists {
		content := DefaultLuaContent(name, description)
		if err := fm.WriteFileContent(scriptPath, content); err != nil {
			return "", fmt.Errorf("EnsureFile -> %s %v", scriptPath, err)
		}
	}

	return scriptPath, nil
}

func DeleteFile(fm FileManager, name string) error {
	return fm.DeleteFileIfExists(fm.ScriptsPath(FileName(name)))
}

func SyncFiles(fm FileManager, items *utils.OrderedMap) error {
	if items == nil {
		return nil
	}

	for _, key := range items.Keys {
		if utils.IsDividerKey(key) {
			continue
		}

		description, ok := items.Values[key]
		if !ok {
			continue
		}

		if _, err := EnsureFile(fm, key, description); err != nil {
			return fmt.Errorf("SyncFiles -> %s %v", key, err)
		}
	}

	return nil
}

func DefaultLuaContent(name, description string) string {
	lines := []string{
		fmt.Sprintf("-- %s", strings.TrimSpace(name)),
	}

	description = strings.TrimSpace(description)
	if description != "" {
		lines = append(lines, fmt.Sprintf("-- %s", description))
	}

	lines = append(lines, "", fmt.Sprintf("print(%q)", "Hello from "+strings.TrimSpace(name)), "")
	return strings.Join(lines, "\n")
}
