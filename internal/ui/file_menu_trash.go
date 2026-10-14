package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type systemFileMenuTrash struct {
	move func(string) error
}

func (systemFileMenuTrash) Directory() (string, error) {
	home := ""
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find %s Trash directory: %w", runtime.GOOS, err)
		}
	}
	return fileMenuTrashDirectory(runtime.GOOS, home, os.Getenv("XDG_DATA_HOME"))
}

func fileMenuTrashDirectory(goos, home, dataHome string) (string, error) {
	switch goos {
	case "darwin":
		return filepath.Join(home, ".Trash"), nil
	case "linux":
		if !filepath.IsAbs(dataHome) {
			dataHome = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(dataHome, "Trash", "files"), nil
	case "windows":
		return "", fmt.Errorf("the Windows Recycle Bin is not a filesystem directory")
	default:
		return "", fmt.Errorf("trash directory is not supported on %s", goos)
	}
}

func (t systemFileMenuTrash) Move(path string) error {
	return t.movePath(path)
}

func (t systemFileMenuTrash) movePath(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absolutePath); err != nil {
		return err
	}
	mover := t.move
	if mover == nil {
		mover = moveFileMenuPathToTrash
	}
	return mover(absolutePath)
}
