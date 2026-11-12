package launcherapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
)

// A reinstall can recreate modloader.asi while an interrupted multiplayer
// session still has its disabled copy. Keep the active file only when both
// copies are identical, and retain the redundant copy outside recovery state.
func (runtime *Runtime) preserveDuplicateModLoader(active, disabled string) error {
	activeInfo, err := os.Lstat(active)
	if err != nil {
		return fmt.Errorf("inspect active Mod Loader: %w", err)
	}
	disabledInfo, err := os.Lstat(disabled)
	if err != nil {
		return fmt.Errorf("inspect disabled Mod Loader: %w", err)
	}
	if !activeInfo.Mode().IsRegular() || !disabledInfo.Mode().IsRegular() || activeInfo.Size() == 0 || disabledInfo.Size() == 0 {
		return fmt.Errorf("cannot restore Mod Loader: active and disabled copies must be nonempty regular files; retained at %s and %s", active, disabled)
	}
	activeHash, err := entrypoint.Hash(context.Background(), active)
	if err != nil {
		return fmt.Errorf("compare active Mod Loader: %w", err)
	}
	disabledHash, err := entrypoint.Hash(context.Background(), disabled)
	if err != nil {
		return fmt.Errorf("compare disabled Mod Loader: %w", err)
	}
	if activeHash != disabledHash {
		return fmt.Errorf("cannot restore Mod Loader: active and disabled copies differ; both are preserved. Compare %s and %s before choosing which version to keep", active, disabled)
	}
	for _, file := range []struct {
		path string
		info os.FileInfo
	}{{active, activeInfo}, {disabled, disabledInfo}} {
		current, err := os.Lstat(file.path)
		if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, file.info) || current.Size() != file.info.Size() || !current.ModTime().Equal(file.info.ModTime()) {
			return fmt.Errorf("Mod Loader changed during recovery; both copies were left untouched")
		}
	}
	archive, err := os.MkdirTemp(filepath.Join(runtime.GameDir, ".downgrader"), "recovered-modloader-")
	if err != nil {
		return fmt.Errorf("prepare duplicate Mod Loader backup: %w", err)
	}
	if err := os.Rename(disabled, filepath.Join(archive, "modloader.asi.disabled")); err != nil {
		_ = os.Remove(archive)
		return fmt.Errorf("preserve duplicate Mod Loader: %w", err)
	}
	return nil
}
