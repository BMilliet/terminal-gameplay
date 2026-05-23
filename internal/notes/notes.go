package notes

import (
	"fmt"
	"strings"

	"terminal-gameplay/internal/utils"
)

const (
	PageName      = "notes"
	AddAction     = "__ADD_NOTE__"
	DeleteAction  = "__DELETE_NOTE__"
	FileExtension = ".md"
)

const previewMaxRunes = 64

type FileManager interface {
	CheckIfPathExists(path string) (bool, error)
	DeleteFileIfExists(path string) error
	EnsureNotesDir() error
	NotesPath(fileName string) string
	ReadFileContent(filePath string) (string, error)
	WriteFileContent(filePath, content string) error
}

func BuildList(items utils.OrderedMap) []utils.ListItem {
	listItems := []utils.ListItem{}
	for _, key := range items.Keys {
		if value, ok := items.Values[key]; ok {
			isDiv := utils.IsDividerKey(key)
			description := value
			if !isDiv {
				description = Preview(value)
			}

			listItems = append(listItems, utils.ListItem{
				T:     key,
				D:     description,
				IsDiv: isDiv,
			})
		}
	}
	return listItems
}

func FileName(title string) string {
	return utils.FileNameWithExtension(title, FileExtension)
}

func EnsureFile(fm FileManager, title, content string) (string, error) {
	if err := fm.EnsureNotesDir(); err != nil {
		return "", err
	}

	notePath := fm.NotesPath(FileName(title))
	exists, err := fm.CheckIfPathExists(notePath)
	if err != nil {
		return "", err
	}

	if !exists {
		if err := fm.WriteFileContent(notePath, content); err != nil {
			return "", fmt.Errorf("EnsureFile -> %s %v", notePath, err)
		}
	}

	return notePath, nil
}

func DeleteFile(fm FileManager, title string) error {
	return fm.DeleteFileIfExists(fm.NotesPath(FileName(title)))
}

func SyncContent(fm FileManager, items *utils.OrderedMap) error {
	if items == nil {
		return nil
	}

	for _, key := range items.Keys {
		if utils.IsDividerKey(key) {
			continue
		}

		content, ok := items.Values[key]
		if !ok {
			continue
		}

		notePath, err := EnsureFile(fm, key, content)
		if err != nil {
			return fmt.Errorf("SyncContent -> %s %v", key, err)
		}

		fileContent, err := fm.ReadFileContent(notePath)
		if err != nil {
			return fmt.Errorf("SyncContent -> read %s %v", notePath, err)
		}
		items.Values[key] = fileContent
	}

	return nil
}

func Preview(text string) string {
	preview := strings.Join(strings.Fields(text), " ")
	if preview == "" {
		return "(empty note)"
	}

	runes := []rune(preview)
	if len(runes) <= previewMaxRunes {
		return preview
	}

	return string(runes[:previewMaxRunes-3]) + "..."
}
