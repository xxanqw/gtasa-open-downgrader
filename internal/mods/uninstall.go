package mods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

type UninstallResult struct {
	BackupDir string
	Moved     int
}

// Uninstall moves the known mod location into a dated backup before changing
// the tracker. This also handles installations made before file inventories
// existed, without downloading a potentially different release archive.
func Uninstall(ctx context.Context, root, name, appVersion string) (UninstallResult, error) {
	var result UninstallResult
	name = canonicalModName(name)
	tracker, err := LoadTracker(root)
	if err != nil {
		return result, err
	}
	if !tracker.Has(name) {
		return result, fmt.Errorf("%s is not installed by the downgrader", name)
	}
	if reason := uninstallDependency(name, tracker); reason != "" {
		return result, fmt.Errorf("%s: %s", name, reason)
	}
	if err = platform.RequireGameClosed(); err != nil {
		return result, err
	}
	paths := uninstallPaths(name)
	if len(paths) == 0 {
		return result, fmt.Errorf("%s has no known install location", name)
	}
	backupBase, err := fsutil.Join(root, "backups/mods")
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(backupBase, 0755); err != nil {
		return result, err
	}
	backupDir, err := os.MkdirTemp(backupBase, backupPrefix(name)+"-"+time.Now().UTC().Format("20060102T150405")+"-*")
	if err != nil {
		return result, err
	}
	type move struct{ source, backup string }
	var moved []move
	rollback := func(cause error) (UninstallResult, error) {
		var failures []error
		for index := len(moved) - 1; index >= 0; index-- {
			entry := moved[index]
			if restoreErr := os.Rename(entry.backup, entry.source); restoreErr != nil {
				failures = append(failures, restoreErr)
			}
		}
		if len(failures) == 0 {
			_ = os.RemoveAll(backupDir)
			return UninstallResult{}, cause
		}
		return UninstallResult{BackupDir: backupDir, Moved: len(moved)}, fmt.Errorf("%w; restore incomplete (%v); files retained at %s", cause, errors.Join(failures...), backupDir)
	}
	for _, rel := range paths {
		if err = ctx.Err(); err != nil {
			return rollback(err)
		}
		if err = platform.RequireGameClosed(); err != nil {
			return rollback(err)
		}
		source, joinErr := fsutil.Join(root, rel)
		if joinErr != nil {
			return rollback(joinErr)
		}
		if _, statErr := os.Lstat(source); errors.Is(statErr, os.ErrNotExist) {
			continue
		} else if statErr != nil {
			return rollback(statErr)
		}
		backup, joinErr := fsutil.Join(backupDir, rel)
		if joinErr != nil {
			return rollback(joinErr)
		}
		if err = os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
			return rollback(err)
		}
		if err = os.Rename(source, backup); err != nil {
			return rollback(err)
		}
		moved = append(moved, move{source: source, backup: backup})
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = platform.RequireGameClosed(); err != nil {
		return rollback(err)
	}
	names := make([]string, 0, len(tracker.Installed)-1)
	for _, installed := range tracker.Installed {
		if installed != name {
			names = append(names, installed)
		}
	}
	tracker.Installed = names
	delete(tracker.Versions, name)
	if err = tracker.Save(context.Background(), root, appVersion); err != nil {
		return rollback(fmt.Errorf("save mod tracker: %w", err))
	}
	result.Moved = len(moved)
	if result.Moved == 0 {
		_ = os.Remove(backupDir)
	} else {
		result.BackupDir = backupDir
	}
	return result, nil
}

func uninstallDependency(name string, tracker Tracker) string {
	for _, installed := range tracker.Installed {
		if installed == name {
			continue
		}
		for _, dep := range RequiredMods(installed) {
			if name == dep {
				return "remove " + installed + " first; it depends on this package"
			}
		}
		if name == "ASI Loader" {
			return "remove the other installed mods first; they need ASI Loader"
		}
		if name == "ModLoader" && installed != "ASI Loader" {
			return "remove the other installed mods first; they need ModLoader"
		}
	}
	return ""
}

func uninstallPaths(name string) []string {
	if spec, ok := interfacePacks[name]; ok {
		return []string{spec.folder}
	}
	switch name {
	case "ASI Loader":
		return []string{"vorbisFile.dll", "vorbisHooked.dll"}
	case "ModLoader":
		return []string{"modloader.asi"}
	case "SilentPatch":
		return []string{"modloader/SilentPatch"}
	case "Widescreen Fixes":
		return []string{"modloader/WidescreenFix"}
	case "SkyGFX":
		return []string{"modloader/SkyGFX"}
	case "Frontend Mods":
		return []string{"modloader/FrontendMods"}
	case HDFonts:
		return []string{"modloader/HDFonts", hdFontStore}
	case ExtendedFonts:
		return []string{extendedBundle}
	case Framerate:
		return []string{"modloader/FramerateVigilante"}
	case "GInput":
		return []string{"modloader/GInput"}
	case "Project 2DFX":
		return []string{"modloader/Project2DFX"}
	case SanLtdRussian:
		return []string{"modloader/SALtdRussian"}
	}
	return nil
}

func backupPrefix(name string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(name) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			result.WriteRune(char)
		} else if result.Len() > 0 {
			result.WriteByte('-')
		}
	}
	return strings.Trim(result.String(), "-")
}
