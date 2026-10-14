//go:build !darwin && !linux && !windows

package ui

import (
	"fmt"
	"runtime"
)

func moveFileMenuPathToTrash(_ string) error {
	return fmt.Errorf("trash is not supported on %s", runtime.GOOS)
}
