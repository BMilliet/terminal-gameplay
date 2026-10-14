package ui

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type fileMenuMode int

const (
	fileMenuNormalMode fileMenuMode = iota
	fileMenuInsertMode
	fileMenuVisualMode
	fileMenuCommandMode
	fileMenuProcessingMode
	fileMenuConfirmDeleteMode
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
	permanent   bool
}

type fileMenuClipboardEntry struct {
	path string
	kind fileMenuEntryKind
}

type fileMenuOperator interface {
	Apply(operations []fileMenuOperation, report func(fileMenuOperationProgress)) fileMenuApplyResult
}

type fileMenuApplyResult struct {
	completed           int
	completedOperations []fileMenuOperation
	err                 error
	progress            fileMenuOperationProgress
}

type fileMenuOperationProgress struct {
	kind            fileMenuOperationKind
	currentFile     string
	doneItems       int64
	totalItems      int64
	doneBytes       int64
	totalBytes      int64
	operation       int
	totalOperations int
}

func (p fileMenuOperationProgress) ratio() float64 {
	if p.totalItems <= 0 {
		return 0
	}
	completed := float64(p.doneItems)
	if p.totalBytes > 0 && p.doneBytes < p.totalBytes {
		completed += float64(p.doneBytes) / float64(p.totalBytes)
	}
	ratio := completed / float64(p.totalItems)
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

type fileMenuTrash interface {
	Move(path string) (string, error)
}

type osFileMenuOperator struct {
	root         string
	allowedRoots []string
	trash        fileMenuTrash
	rename       func(string, string) error
	removeAll    func(string) error
}

func (o osFileMenuOperator) Apply(operations []fileMenuOperation, report func(fileMenuOperationProgress)) fileMenuApplyResult {
	progress := fileMenuOperationProgress{
		totalItems:      int64(len(operations)),
		totalOperations: len(operations),
	}
	completedOperations := make([]fileMenuOperation, 0, len(operations))
	for index, stagedOperation := range operations {
		operation := stagedOperation
		progress.kind = operation.kind
		progress.operation = index + 1
		progress.currentFile = fileMenuOperationName(operation)
		progress.doneBytes = 0
		progress.totalBytes = 0

		if operation.kind == fileMenuCopy {
			source, err := o.resolve(operation.source)
			if err != nil {
				return fileMenuApplyResult{completed: index, completedOperations: completedOperations, err: err, progress: progress}
			}
			items, countErr := countFileMenuCopyItems(source)
			if countErr != nil {
				return fileMenuApplyResult{completed: index, completedOperations: completedOperations, err: fmt.Errorf("inspect copy source %q: %w", operation.source, countErr), progress: progress}
			}
			progress.totalItems += items - 1
		}
		reportFileMenuProgress(report, progress)

		doneItemsBefore := progress.doneItems
		if err := o.apply(&operation, &progress, report); err != nil {
			return fileMenuApplyResult{completed: index, completedOperations: completedOperations, err: err, progress: progress}
		}
		completedOperations = append(completedOperations, operation)
		if progress.doneItems == doneItemsBefore {
			progress.doneItems++
		}
		progress.doneBytes = progress.totalBytes
		reportFileMenuProgress(report, progress)
	}
	return fileMenuApplyResult{completed: len(operations), completedOperations: completedOperations, progress: progress}
}

func (o osFileMenuOperator) apply(operation *fileMenuOperation, progress *fileMenuOperationProgress, report func(fileMenuOperationProgress)) error {
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
		if copyErr := o.copyFileMenuEntryAtomic(source, destination, progress, report); copyErr != nil {
			return fmt.Errorf("copy %q to %q: %w", operation.source, operation.destination, copyErr)
		}
		return nil
	case fileMenuMove:
		if _, statErr := os.Lstat(destination); statErr == nil {
			return fmt.Errorf("move %q: destination %q already exists", operation.source, operation.destination)
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect destination %q: %w", operation.destination, statErr)
		}
		if renameErr := o.renamePath(source, destination); renameErr != nil {
			if !errors.Is(renameErr, syscall.EXDEV) {
				return fmt.Errorf("move %q to %q: %w", operation.source, operation.destination, renameErr)
			}
			items, countErr := countFileMenuCopyItems(source)
			if countErr != nil {
				return fmt.Errorf("inspect cross-filesystem move source %q: %w", operation.source, countErr)
			}
			progress.totalItems += items - 1
			reportFileMenuProgress(report, *progress)
			if copyErr := o.copyFileMenuEntryAtomic(source, destination, progress, report); copyErr != nil {
				return fmt.Errorf("move %q to %q across filesystems: %w", operation.source, operation.destination, copyErr)
			}
			if removeErr := o.removePath(source); removeErr != nil {
				rollbackErr := o.removePath(destination)
				if rollbackErr != nil {
					return fmt.Errorf("remove move source %q: %w; rollback destination: %v", operation.source, removeErr, rollbackErr)
				}
				return fmt.Errorf("remove move source %q: %w", operation.source, removeErr)
			}
		}
		return nil
	case fileMenuDelete:
		if operation.permanent {
			if removeErr := o.removePath(source); removeErr != nil {
				return fmt.Errorf("permanently delete %q: %w", operation.source, removeErr)
			}
			return nil
		}
		trasher := o.trash
		if trasher == nil {
			trasher = systemFileMenuTrash{}
		}
		trashDestination, trashErr := trasher.Move(source)
		if trashErr != nil {
			return fmt.Errorf("move %q to trash: %w (use D for permanent deletion)", operation.source, trashErr)
		}
		operation.destination = trashDestination
		return nil
	default:
		return fmt.Errorf("unknown file operation %d", operation.kind)
	}
}

func (o osFileMenuOperator) renamePath(source, destination string) error {
	if o.rename != nil {
		return o.rename(source, destination)
	}
	return os.Rename(source, destination)
}

func (o osFileMenuOperator) removePath(path string) error {
	if o.removeAll != nil {
		return o.removeAll(path)
	}
	return os.RemoveAll(path)
}

func fileMenuOperationName(operation fileMenuOperation) string {
	path := operation.source
	if path == "" {
		path = operation.destination
	}
	return filepath.Base(path)
}

func reportFileMenuProgress(report func(fileMenuOperationProgress), progress fileMenuOperationProgress) {
	if report != nil {
		report(progress)
	}
}

func (o osFileMenuOperator) resolve(path string) (string, error) {
	root, err := filepath.Abs(o.root)
	if err != nil {
		return "", fmt.Errorf("resolve file menu root: %w", err)
	}
	target := filepath.Clean(path)
	if !filepath.IsAbs(target) {
		target, err = filepath.Abs(filepath.Join(root, target))
		if err != nil {
			return "", fmt.Errorf("resolve path %q: %w", path, err)
		}
	}
	allowedRoots := append([]string{root}, o.allowedRoots...)
	for _, allowedRoot := range allowedRoots {
		absoluteRoot, rootErr := filepath.Abs(allowedRoot)
		if rootErr != nil {
			return "", fmt.Errorf("resolve allowed file menu root: %w", rootErr)
		}
		relative, relativeErr := filepath.Rel(absoluteRoot, target)
		if relativeErr != nil {
			continue
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		return resolveFileMenuPathSymlinks(absoluteRoot, target, path)
	}
	return "", fmt.Errorf("path %q is outside the file menu root", path)
}

func resolveFileMenuPathSymlinks(root, target, originalPath string) (string, error) {
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
			return "", fmt.Errorf("cannot resolve parent for %q", originalPath)
		}
		existingParent = next
	}
	realParent, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return "", fmt.Errorf("resolve path parent %q: %w", originalPath, err)
	}
	realRelative, err := filepath.Rel(realRoot, realParent)
	if err != nil {
		return "", fmt.Errorf("check real path %q: %w", originalPath, err)
	}
	if realRelative == ".." || strings.HasPrefix(realRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q resolves outside the file menu root", originalPath)
	}
	return target, nil
}

func (o osFileMenuOperator) allowRoot(path string) osFileMenuOperator {
	cleanPath := filepath.Clean(path)
	for _, root := range o.allowedRoots {
		if filepath.Clean(root) == cleanPath {
			return o
		}
	}
	o.allowedRoots = append(o.allowedRoots, cleanPath)
	return o
}

func countFileMenuCopyItems(source string) (int64, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 1, nil
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 1, nil
	}
	var total int64
	for _, entry := range entries {
		count, countErr := countFileMenuCopyItems(filepath.Join(source, entry.Name()))
		if countErr != nil {
			return 0, countErr
		}
		total += count
	}
	return total, nil
}

func (o osFileMenuOperator) copyFileMenuEntryAtomic(source, destination string, progress *fileMenuOperationProgress, report func(fileMenuOperationProgress)) error {
	if _, err := os.Lstat(destination); err == nil {
		return fs.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(destination)
	temporaryDirectory, err := os.MkdirTemp(parent, ".tg-copy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryDirectory)
	temporaryDestination := filepath.Join(temporaryDirectory, filepath.Base(destination))
	if err := copyFileMenuEntry(source, temporaryDestination, progress, report); err != nil {
		return err
	}
	if err := o.renamePath(temporaryDestination, destination); err != nil {
		return err
	}
	return nil
}

func copyFileMenuEntry(source, destination string, progress *fileMenuOperationProgress, report func(fileMenuOperationProgress)) error {

	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(source)
		if readErr != nil {
			return readErr
		}
		if err := os.Symlink(target, destination); err != nil {
			return err
		}
		completeFileMenuProgressItem(progress, source, 0, report)
		return nil
	}
	if info.IsDir() {
		if err := os.Mkdir(destination, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			completeFileMenuProgressItem(progress, source, 0, report)
			return nil
		}
		for _, entry := range entries {
			if err := copyFileMenuEntry(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), progress, report); err != nil {
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
	progress.currentFile = filepath.Base(source)
	progress.doneBytes = 0
	progress.totalBytes = info.Size()
	reportFileMenuProgress(report, *progress)
	written, err := io.Copy(&fileMenuProgressWriter{writer: output, progress: progress, report: report}, input)
	if err != nil {
		_ = output.Close()
		return err
	}
	if closeErr := output.Close(); closeErr != nil {
		return closeErr
	}
	completeFileMenuProgressItem(progress, source, written, report)
	return nil
}

type fileMenuProgressWriter struct {
	writer   io.Writer
	progress *fileMenuOperationProgress
	report   func(fileMenuOperationProgress)
}

func (w *fileMenuProgressWriter) Write(data []byte) (int, error) {
	written, err := w.writer.Write(data)
	w.progress.doneBytes += int64(written)
	reportFileMenuProgress(w.report, *w.progress)
	return written, err
}

func completeFileMenuProgressItem(progress *fileMenuOperationProgress, source string, size int64, report func(fileMenuOperationProgress)) {
	progress.currentFile = filepath.Base(source)
	progress.doneBytes = size
	progress.totalBytes = size
	progress.doneItems++
	reportFileMenuProgress(report, *progress)
}
