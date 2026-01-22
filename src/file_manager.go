package src

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileManagerInterface interface {
	CheckIfPathExists(path string) (bool, error)
	ReadFileContent(filePath string) (string, error)
	WriteFileContent(filePath, content string) error
	GetConfigContent() (string, error)
	WriteConfigContent(content string) error
	BasicSetup() error
	GetCurrentDirectoryName() (string, error)
}

type FileManager struct {
	HomeDir    string
	AppDir     string
	ConfigPath string
}

func NewFileManager() (*FileManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("NewFileManager -> %v", err)
	}

	appDir := filepath.Join(homeDir, AppDirName)
	configPath := filepath.Join(appDir, ConfigFileName)

	return &FileManager{
		HomeDir:    homeDir,
		AppDir:     appDir,
		ConfigPath: configPath,
	}, nil
}

func (m *FileManager) ensureAppDir() error {
	if _, err := os.Stat(m.AppDir); os.IsNotExist(err) {
		err := os.Mkdir(m.AppDir, 0755)
		if err != nil {
			return fmt.Errorf("ensureAppDir -> %v", err)
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
		_, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("checkAndCreateFile -> %s %v", filePath, err)
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

func (m *FileManager) BasicSetup() error {
	if err := m.ensureAppDir(); err != nil {
		return err
	}

	files := []string{
		m.ConfigPath,
	}

	for _, file := range files {
		if err := m.checkAndCreateFile(file); err != nil {
			return err
		}
	}

	return nil
}

func (m *FileManager) GetCurrentDirectoryName() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("GetCurrentDirectoryName -> %v", err)
	}

	return filepath.Base(dir), nil
}
