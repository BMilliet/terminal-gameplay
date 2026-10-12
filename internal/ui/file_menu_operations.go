package ui

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type fileMenuMode int

const (
	fileMenuNormalMode fileMenuMode = iota
	fileMenuInsertMode
	fileMenuVisualMode
	fileMenuCommandMode
	fileMenuProcessingMode
)

type fileMenuInputAction int

const (
	fileMenuCreateInput fileMenuInputAction = iota
	fileMenuRenameInput
)

type fileMenuOperationKind int

const (
	fileMenuCreateFile fileMenuOperationKind = iota
	fileMenuCreateDirectory
	fileMenuCopy
	fileMenuMove
	fileMenuDelete
)

type fileMenuOperation struct {
	kind        fileMenuOperationKind
	source      string
	destination string
	entryKind   fileMenuEntryKind
}

type fileMenuClipboardEntry struct {
	path string
	kind fileMenuEntryKind
}

type fileMenuOperator interface {
	Apply(operations []fileMenuOperation) error
}

type osFileMenuOperator struct {
	root string
}

func (o osFileMenuOperator) Apply(operations []fileMenuOperation) error {
	for _, operation := range operations {
		if err := o.apply(operation); err != nil {
			return err
		}
	}
	return nil
}

func (o osFileMenuOperator) apply(operation fileMenuOperation) error {
	source, err := o.resolve(operation.source)
	if err != nil && operation.source != "" {
		return err
	}
	destination, err := o.resolve(operation.destination)
	if err != nil && operation.destination != "" {
		return err
	}

	switch operation.kind {
	case fileMenuCreateFile:
		file, createErr := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if createErr != nil {
			return fmt.Errorf("create file %q: %w", operation.destination, createErr)
		}
		return file.Close()
	case fileMenuCreateDirectory:
		if mkdirErr := os.Mkdir(destination, 0o755); mkdirErr != nil {
			return fmt.Errorf("create directory %q: %w", operation.destination, mkdirErr)
		}
		return nil
	case fileMenuCopy:
		if copyErr := copyFileMenuEntry(source, destination); copyErr != nil {
			return fmt.Errorf("copy %q to %q: %w", operation.source, operation.destination, copyErr)
		}
		return nil
	case fileMenuMove:
		if _, statErr := os.Lstat(destination); statErr == nil {
			return fmt.Errorf("move %q: destination %q already exists", operation.source, operation.destination)
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect destination %q: %w", operation.destination, statErr)
		}
		if renameErr := os.Rename(source, destination); renameErr != nil {
			return fmt.Errorf("move %q to %q: %w", operation.source, operation.destination, renameErr)
		}
		return nil
	case fileMenuDelete:
		if removeErr := os.RemoveAll(source); removeErr != nil {
			return fmt.Errorf("delete %q: %w", operation.source, removeErr)
		}
		return nil
	default:
		return fmt.Errorf("unknown file operation %d", operation.kind)
	}
}

func (o osFileMenuOperator) resolve(path string) (string, error) {
	root, err := filepath.Abs(o.root)
	if err != nil {
		return "", fmt.Errorf("resolve file menu root: %w", err)
	}
	target, err := filepath.Abs(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("check path %q: %w", path, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the file menu root", path)
	}

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve file menu root symlinks: %w", err)
	}
	existingParent := filepath.Dir(target)
	for {
		if _, statErr := os.Lstat(existingParent); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("inspect path parent %q: %w", existingParent, statErr)
		}
		next := filepath.Dir(existingParent)
		if next == existingParent {
			return "", fmt.Errorf("cannot resolve parent for %q", path)
		}
		existingParent = next
	}
	realParent, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return "", fmt.Errorf("resolve path parent %q: %w", path, err)
	}
	realRelative, err := filepath.Rel(realRoot, realParent)
	if err != nil {
		return "", fmt.Errorf("check real path %q: %w", path, err)
	}
	if realRelative == ".." || strings.HasPrefix(realRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q resolves outside the file menu root", path)
	}
	return target, nil
}

func copyFileMenuEntry(source, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return fs.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}

	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(source)
		if readErr != nil {
			return readErr
		}
		return os.Symlink(target, destination)
	}
	if info.IsDir() {
		if err := os.Mkdir(destination, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyFileMenuEntry(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type %q", info.Mode().Type())
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
