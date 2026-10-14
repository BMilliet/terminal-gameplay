//go:build darwin

package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func moveFileMenuPathToTrash(path string) (string, error) {
	trashDirectory, err := darwinFileMenuTrashDirectory(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(trashDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create trash directory %q: %w", trashDirectory, err)
	}
	destination, err := moveFileMenuPathToTrashDirectory(path, trashDirectory)
	if err != nil {
		return "", fmt.Errorf("move to trash directory %q: %w", trashDirectory, err)
	}
	return destination, nil
}

func darwinFileMenuTrashDirectory(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home trash directory: %w", err)
	}
	homeTrash := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(homeTrash, 0o700); err != nil {
		return "", fmt.Errorf("create home trash directory: %w", err)
	}

	var sourceStat syscall.Stat_t
	if err := syscall.Lstat(path, &sourceStat); err != nil {
		return "", err
	}
	var trashStat syscall.Stat_t
	if err := syscall.Stat(homeTrash, &trashStat); err != nil {
		return "", err
	}
	if sourceStat.Dev == trashStat.Dev {
		return homeTrash, nil
	}

	var filesystem unix.Statfs_t
	if err := unix.Statfs(path, &filesystem); err != nil {
		return "", fmt.Errorf("find source volume: %w", err)
	}
	mountBytes := filesystem.Mntonname[:]
	if end := bytes.IndexByte(mountBytes, 0); end >= 0 {
		mountBytes = mountBytes[:end]
	}
	mountPoint := string(mountBytes)
	if mountPoint == "" {
		return "", errors.New("source volume has no mount point")
	}
	return filepath.Join(mountPoint, ".Trashes", strconv.Itoa(os.Getuid())), nil
}

func moveFileMenuPathToTrashDirectory(path, trashDirectory string) (string, error) {
	name := filepath.Base(path)
	extension := filepath.Ext(name)
	stem := name[:len(name)-len(extension)]
	for suffix := 1; ; suffix++ {
		candidateName := name
		if suffix > 1 {
			candidateName = fmt.Sprintf("%s %d%s", stem, suffix, extension)
		}
		destination := filepath.Join(trashDirectory, candidateName)
		err := unix.RenameatxNp(unix.AT_FDCWD, path, unix.AT_FDCWD, destination, unix.RENAME_EXCL)
		if err == nil {
			return destination, nil
		}
		if errors.Is(err, syscall.EEXIST) {
			continue
		}
		return "", err
	}
}
