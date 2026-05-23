package utils

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
	DeleteFileIfExists(path string) error
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
	NotesPath(fileName string) string
	ScriptsPath(fileName string) string
	CommandExecPath() string
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

func (m *FileManager) NotesPath(fileName string) string {
	return filepath.Join(m.NotesDir, fileName)
}

func (m *FileManager) ScriptsPath(fileName string) string {
	return filepath.Join(m.ScriptsDir, fileName)
}

func (m *FileManager) CommandExecPath() string {
	return filepath.Join(m.AppDir, CommandExecFileName)
}

func (m *FileManager) DeleteFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("DeleteFileIfExists -> %s %v", path, err)
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

func FileNameWithExtension(name, extension string) string {
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
