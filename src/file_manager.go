package src

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileManagerInterface interface {
	CheckIfPathExists(path string) (bool, error)
	ReadFileContent(filePath string) (string, error)
	WriteFileContent(filePath, content string) error
	GetConfigContent() (string, error)
	WriteConfigContent(content string) error
	GetFeaturesContent() (string, error)
	WriteFeaturesContent(content string) error
	GetOptionsContent() (string, error)
	WriteOptionsContent(content string) error
	GetGoToFrequencyContent() (string, error)
	WriteGoToFrequencyContent(content string) error
	EnsureNotesDir() error
	EnsureScriptsDir() error
	GetNotePath(title string) string
	GetScriptPath(name string) string
	EnsureNoteFile(title, content string) (string, error)
	EnsureScriptFile(name, description string) (string, error)
	DeleteNoteFile(title string) error
	DeleteScriptFile(name string) error
	SyncNotesContent(notes *OrderedMap) error
	SyncScriptsFiles(scripts *OrderedMap) error
	BasicSetup() error
	GetCurrentPath() (string, error)
	GetCurrentDirectoryName() (string, error)
}

type FileManager struct {
	HomeDir           string
	AppDir            string
	NotesDir          string
	ScriptsDir        string
	ConfigPath        string
	FeaturesPath      string
	OptionsPath       string
	GoToFrequencyPath string
}

func NewFileManager() (*FileManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("NewFileManager -> %v", err)
	}

	appDir := filepath.Join(homeDir, AppDirName)
	notesDir := filepath.Join(appDir, NotesDirName)
	scriptsDir := filepath.Join(appDir, ScriptsDirName)
	configPath := filepath.Join(appDir, ConfigFileName)
	featuresPath := filepath.Join(appDir, FeaturesFileName)
	optionsPath := filepath.Join(appDir, OptionsFileName)
	goToFrequencyPath := filepath.Join(appDir, GoToFrequencyFileName)

	return &FileManager{
		HomeDir:           homeDir,
		AppDir:            appDir,
		NotesDir:          notesDir,
		ScriptsDir:        scriptsDir,
		ConfigPath:        configPath,
		FeaturesPath:      featuresPath,
		OptionsPath:       optionsPath,
		GoToFrequencyPath: goToFrequencyPath,
	}, nil
}

func (m *FileManager) ensureAppDir() error {
	return m.ensureDir(m.AppDir)
}

func (m *FileManager) ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			return fmt.Errorf("ensureDir -> %s %v", path, err)
		}
	}
	return nil
}

func (m *FileManager) CheckIfPathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("CheckIfPathExists -> %v", err)
}

func (m *FileManager) checkAndCreateFile(filePath string) error {
	exists, err := m.CheckIfPathExists(filePath)
	if err != nil {
		return err
	}
	if !exists {
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("checkAndCreateFile -> %s %v", filePath, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("checkAndCreateFile -> close %s %v", filePath, err)
		}
	}
	return nil
}

func (m *FileManager) ReadFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("ReadFileContent -> %s %v", filePath, err)
	}
	return string(data), nil
}

func (m *FileManager) WriteFileContent(filePath, content string) error {
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("WriteFileContent -> %s %v", filePath, err)
	}
	return nil
}

func (m *FileManager) GetConfigContent() (string, error) {
	str, err := m.ReadFileContent(m.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("GetConfigContent -> %s %v", m.ConfigPath, err)
	}
	return str, nil
}

func (m *FileManager) WriteConfigContent(content string) error {
	err := m.WriteFileContent(m.ConfigPath, content)
	if err != nil {
		return fmt.Errorf("WriteConfigContent -> %s: %v", m.ConfigPath, err)
	}
	return nil
}

func (m *FileManager) GetFeaturesContent() (string, error) {
	str, err := m.ReadFileContent(m.FeaturesPath)
	if err != nil {
		return "", fmt.Errorf("GetFeaturesContent -> %s %v", m.FeaturesPath, err)
	}
	return str, nil
}

func (m *FileManager) WriteFeaturesContent(content string) error {
	err := m.WriteFileContent(m.FeaturesPath, content)
	if err != nil {
		return fmt.Errorf("WriteFeaturesContent -> %s: %v", m.FeaturesPath, err)
	}
	return nil
}

func (m *FileManager) GetOptionsContent() (string, error) {
	str, err := m.ReadFileContent(m.OptionsPath)
	if err != nil {
		return "", fmt.Errorf("GetOptionsContent -> %s %v", m.OptionsPath, err)
	}
	return str, nil
}

func (m *FileManager) WriteOptionsContent(content string) error {
	err := m.WriteFileContent(m.OptionsPath, content)
	if err != nil {
		return fmt.Errorf("WriteOptionsContent -> %s: %v", m.OptionsPath, err)
	}
	return nil
}

func (m *FileManager) GetGoToFrequencyContent() (string, error) {
	str, err := m.ReadFileContent(m.GoToFrequencyPath)
	if err != nil {
		return "", fmt.Errorf("GetGoToFrequencyContent -> %s %v", m.GoToFrequencyPath, err)
	}
	return str, nil
}

func (m *FileManager) WriteGoToFrequencyContent(content string) error {
	err := m.WriteFileContent(m.GoToFrequencyPath, content)
	if err != nil {
		return fmt.Errorf("WriteGoToFrequencyContent -> %s: %v", m.GoToFrequencyPath, err)
	}
	return nil
}

func (m *FileManager) EnsureNotesDir() error {
	return m.ensureDir(m.NotesDir)
}

func (m *FileManager) EnsureScriptsDir() error {
	return m.ensureDir(m.ScriptsDir)
}

func (m *FileManager) GetNotePath(title string) string {
	return filepath.Join(m.NotesDir, noteFileName(title))
}

func (m *FileManager) GetScriptPath(name string) string {
	return filepath.Join(m.ScriptsDir, scriptFileName(name))
}

func (m *FileManager) EnsureNoteFile(title, content string) (string, error) {
	if err := m.EnsureNotesDir(); err != nil {
		return "", err
	}

	notePath := m.GetNotePath(title)
	exists, err := m.CheckIfPathExists(notePath)
	if err != nil {
		return "", err
	}

	if !exists {
		if err := m.WriteFileContent(notePath, content); err != nil {
			return "", fmt.Errorf("EnsureNoteFile -> %s %v", notePath, err)
		}
	}

	return notePath, nil
}

func (m *FileManager) EnsureScriptFile(name, description string) (string, error) {
	if err := m.EnsureScriptsDir(); err != nil {
		return "", err
	}

	scriptPath := m.GetScriptPath(name)
	exists, err := m.CheckIfPathExists(scriptPath)
	if err != nil {
		return "", err
	}

	if !exists {
		content := defaultLuaScriptContent(name, description)
		if err := m.WriteFileContent(scriptPath, content); err != nil {
			return "", fmt.Errorf("EnsureScriptFile -> %s %v", scriptPath, err)
		}
	}

	return scriptPath, nil
}

func (m *FileManager) DeleteNoteFile(title string) error {
	return m.deleteFileIfExists(m.GetNotePath(title))
}

func (m *FileManager) DeleteScriptFile(name string) error {
	return m.deleteFileIfExists(m.GetScriptPath(name))
}

func (m *FileManager) deleteFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("deleteFileIfExists -> %s %v", path, err)
}

func (m *FileManager) SyncNotesContent(notes *OrderedMap) error {
	if notes == nil {
		return nil
	}

	for _, key := range notes.Keys {
		if IsDividerKey(key) {
			continue
		}

		content, ok := notes.Values[key]
		if !ok {
			continue
		}

		notePath, err := m.EnsureNoteFile(key, content)
		if err != nil {
			return fmt.Errorf("SyncNotesContent -> %s %v", key, err)
		}

		fileContent, err := m.ReadFileContent(notePath)
		if err != nil {
			return fmt.Errorf("SyncNotesContent -> read %s %v", notePath, err)
		}
		notes.Values[key] = fileContent
	}

	return nil
}

func (m *FileManager) SyncScriptsFiles(scripts *OrderedMap) error {
	if scripts == nil {
		return nil
	}

	for _, key := range scripts.Keys {
		if IsDividerKey(key) {
			continue
		}

		description, ok := scripts.Values[key]
		if !ok {
			continue
		}

		if _, err := m.EnsureScriptFile(key, description); err != nil {
			return fmt.Errorf("SyncScriptsFiles -> %s %v", key, err)
		}
	}

	return nil
}

func (m *FileManager) BasicSetup() error {
	if err := m.ensureAppDir(); err != nil {
		return err
	}

	if err := m.EnsureNotesDir(); err != nil {
		return err
	}

	if err := m.EnsureScriptsDir(); err != nil {
		return err
	}

	files := []string{
		m.ConfigPath,
		m.FeaturesPath,
	}

	for _, file := range files {
		if err := m.checkAndCreateFile(file); err != nil {
			return err
		}
	}

	return nil
}

func (m *FileManager) GetCurrentPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("GetCurrentPath -> %v", err)
	}

	return dir, nil
}

func (m *FileManager) GetCurrentDirectoryName() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("GetCurrentDirectoryName -> %v", err)
	}

	return filepath.Base(dir), nil
}

func noteFileName(title string) string {
	return fileNameWithExtension(title, NoteFileExtension)
}

func scriptFileName(name string) string {
	return fileNameWithExtension(name, ScriptFileExtension)
}

func fileNameWithExtension(name, extension string) string {
	fileName := strings.TrimSpace(name)
	fileName = strings.ReplaceAll(fileName, "\\", "-")
	fileName = filepath.Base(fileName)
	fileName = strings.TrimSpace(fileName)

	if fileName == "" || fileName == "." {
		fileName = "untitled"
	}

	if filepath.Ext(fileName) == "" {
		fileName += extension
	}

	return fileName
}

func defaultLuaScriptContent(name, description string) string {
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
