package launcherapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
)

// multiplayerModDirectories maps tracked package names to the normalized
// directories created by the mod installer. The dependency packages are not
// included: Silent ASI Loader remains active, while Mod Loader is the component
// this compatibility path intentionally replaces.
var multiplayerModDirectories = []struct {
	name      string
	directory string
}{
	{"SilentPatch", "SilentPatch"},
	{"Widescreen Fixes", "WidescreenFix"},
	{"SkyGFX", "SkyGFX"},
	{"Frontend Mods", "FrontendMods"},
	{mods.Framerate, "FramerateVigilante"},
	{"GInput", "GInput"},
	{"Project 2DFX", "Project2DFX"},
	{mods.SALtdRussian, "SALtdRussian"},
}

type multiplayerOverlayEntry struct {
	Path           string `json:"path"`
	Source         string `json:"source"`
	StagedSHA256   string `json:"staged_sha256"`
	HadOriginal    bool   `json:"had_original"`
	OriginalSHA256 string `json:"original_sha256,omitempty"`
}

type multiplayerOverlayState struct {
	Schema  int                       `json:"schema_version"`
	Entries []multiplayerOverlayEntry `json:"entries"`
}

func (runtime *Runtime) multiplayerOverlayPaths() (state, backup string) {
	directory := filepath.Join(runtime.GameDir, ".downgrader", "multiplayer")
	return filepath.Join(directory, "mod-overlay.json"), filepath.Join(directory, "mod-overlay-backup")
}

// runWithMultiplayerMods avoids Mod Loader's SA-MP/Wine startup crash while
// retaining the selected modifications. The already-normalized contents of
// each tracked mod directory are staged directly into the game directory, where
// Silent ASI Loader and the game can load them without Mod Loader's hooks.
func (runtime *Runtime) runWithMultiplayerMods(ctx context.Context, run func() error) error {
	return runtime.runWithMultiplayerModsAndCleanup(ctx, run, nil)
}

func (runtime *Runtime) runWithMultiplayerModsAndCleanup(ctx context.Context, run func() error, cleanup func() error) error {
	before, err := runtime.gameProcessSnapshot()
	if err != nil {
		return fmt.Errorf("inspect running GTA processes: %w", err)
	}
	if _, err := runtime.disableModLoader(); err != nil {
		return err
	}
	if err := runtime.stageMultiplayerOverlay(ctx); err != nil {
		return errors.Join(err, runtime.restoreModLoader())
	}
	runDone := make(chan error, 1)
	launcherDone := make(chan struct{})
	go func() {
		runDone <- run()
		close(launcherDone)
	}()
	started, waitErr := runtime.waitForSpawnedGame(ctx, before, launcherDone)
	if waitErr != nil {
		// The game may still have these files mapped. Preserve the complete
		// transaction for restoreCompatibility on the next safe launcher run.
		var runErr error
		select {
		case runErr = <-runDone:
		default:
		}
		return errors.Join(runErr, fmt.Errorf("wait for GTA to exit: %w", waitErr))
	}
	var runErr error
	if started {
		// Bootstrap programs can outlive GTA or return a Wine status even though
		// they started it successfully. Observing GTA is authoritative.
		select {
		case <-runDone:
		default:
		}
	} else {
		runErr = <-runDone
	}
	var cleanupErr error
	if cleanup != nil {
		cleanupErr = cleanup()
	}
	restoreErr := runtime.restoreMultiplayerOverlay()
	loaderErr := runtime.restoreModLoader()
	return errors.Join(runErr, cleanupErr, restoreErr, loaderErr)
}

func (runtime *Runtime) gameProcessSnapshot() (gameProcessSet, error) {
	if runtime.snapshotGameProcesses == nil {
		return snapshotGameProcesses()
	}
	return runtime.snapshotGameProcesses()
}

func (runtime *Runtime) waitForSpawnedGame(ctx context.Context, before gameProcessSet, launcherDone <-chan struct{}) (bool, error) {
	if runtime.waitForGameProcess == nil {
		return waitForSpawnedGame(ctx, before, launcherDone)
	}
	return runtime.waitForGameProcess(ctx, before, launcherDone)
}

func (runtime *Runtime) stageMultiplayerOverlay(ctx context.Context) error {
	statePath, backupRoot := runtime.multiplayerOverlayPaths()
	if _, err := os.Stat(statePath); err == nil {
		return fmt.Errorf("a multiplayer mod overlay is already active; start the launcher again to recover it")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect multiplayer mod overlay: %w", err)
	}

	tracker, err := mods.LoadTracker(runtime.GameDir)
	if err != nil {
		return fmt.Errorf("load installed mods for multiplayer: %w", err)
	}
	sources := make(map[string]multiplayerOverlayEntry)
	for _, candidate := range multiplayerModDirectories {
		if !tracker.Has(candidate.name) {
			continue
		}
		// Current Widescreen Fix builds overflow GTA's stack under Proton when
		// SA-MP is injected alongside either GInput or Framerate Vigilante.
		// Keep it in Mod Loader for single-player and retain the other selected
		// modifications in SA-MP/open.mp. Native Windows does not use this filter.
		if candidate.name == "Widescreen Fixes" && RunningUnderProton() {
			continue
		}
		root := filepath.Join(runtime.GameDir, "modloader", candidate.directory)
		if err := runtime.collectMultiplayerMod(ctx, root, sources); err != nil {
			return fmt.Errorf("prepare %s for multiplayer: %w", candidate.name, err)
		}
	}
	if len(sources) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	state := multiplayerOverlayState{Schema: 1, Entries: make([]multiplayerOverlayEntry, 0, len(keys))}
	for _, key := range keys {
		entry := sources[key]
		target, joinErr := fsutil.Join(runtime.GameDir, entry.Path)
		if joinErr != nil {
			return joinErr
		}
		if info, statErr := os.Stat(target); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("cannot stage multiplayer mod over non-file %s", entry.Path)
			}
			entry.HadOriginal = true
			entry.OriginalSHA256, err = entrypoint.Hash(ctx, target)
			if err == nil {
				err = fsutil.CopyFile(ctx, target, filepath.Join(backupRoot, filepath.FromSlash(entry.Path)))
			}
			if err != nil {
				return fmt.Errorf("preserve %s for multiplayer: %w", entry.Path, err)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect %s for multiplayer: %w", entry.Path, statErr)
		}
		state.Entries = append(state.Entries, entry)
	}

	if err := writeJSON(ctx, statePath, state); err != nil {
		return fmt.Errorf("record multiplayer mod overlay: %w", err)
	}
	for _, entry := range state.Entries {
		target, _ := fsutil.Join(runtime.GameDir, entry.Path)
		if err := fsutil.CopyFile(ctx, entry.Source, target); err != nil {
			return errors.Join(fmt.Errorf("stage %s for multiplayer: %w", entry.Path, err), runtime.restoreMultiplayerOverlay())
		}
	}
	return nil
}

func (runtime *Runtime) collectMultiplayerMod(ctx context.Context, root string, sources map[string]multiplayerOverlayEntry) error {
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("normalized mod path is not a directory: %s", root)
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		lower := strings.ToLower(relative)
		if entry.IsDir() {
			if relative != "." && (lower == "docs" || lower == "documentation" || strings.HasPrefix(lower, "docs/") || strings.HasPrefix(lower, "documentation/")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is not allowed: %s", relative)
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", relative)
		}
		base := strings.ToLower(filepath.Base(relative))
		if strings.HasPrefix(base, "readme") || strings.HasPrefix(base, "license") {
			return nil
		}
		digest, err := entrypoint.Hash(ctx, path)
		if err != nil {
			return err
		}
		// Later catalog entries take precedence, matching their installation
		// order when two curated packages intentionally target the same file.
		sources[strings.ToLower(relative)] = multiplayerOverlayEntry{
			Path:         relative,
			Source:       path,
			StagedSHA256: digest,
		}
		return nil
	})
}

func (runtime *Runtime) restoreMultiplayerOverlay() error {
	statePath, backupRoot := runtime.multiplayerOverlayPaths()
	data, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read multiplayer mod overlay: %w", err)
	}
	var state multiplayerOverlayState
	if err := json.Unmarshal(data, &state); err != nil || state.Schema != 1 || len(state.Entries) == 0 {
		return fmt.Errorf("multiplayer mod overlay state is invalid")
	}

	for index := len(state.Entries) - 1; index >= 0; index-- {
		entry := state.Entries[index]
		target, joinErr := fsutil.Join(runtime.GameDir, entry.Path)
		if joinErr != nil {
			return joinErr
		}
		needsRestore := false
		if info, statErr := os.Stat(target); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("multiplayer overlay target is no longer a file: %s", entry.Path)
			}
			digest, hashErr := entrypoint.Hash(context.Background(), target)
			if hashErr != nil {
				return hashErr
			}
			switch {
			case digest == entry.StagedSHA256:
				needsRestore = true
			case entry.HadOriginal && digest == entry.OriginalSHA256:
				continue
			default:
				return fmt.Errorf("%s changed while multiplayer mods were active; preserved original left at %s", entry.Path, backupRoot)
			}
		} else if errors.Is(statErr, os.ErrNotExist) {
			needsRestore = entry.HadOriginal
		} else {
			return fmt.Errorf("inspect multiplayer overlay target %s: %w", entry.Path, statErr)
		}
		if !needsRestore {
			continue
		}
		if entry.HadOriginal {
			backup := filepath.Join(backupRoot, filepath.FromSlash(entry.Path))
			if restoreErr := fsutil.CopyFile(context.Background(), backup, target); restoreErr != nil {
				return fmt.Errorf("restore %s after multiplayer: %w", entry.Path, restoreErr)
			}
		} else if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("remove temporary multiplayer mod %s: %w", entry.Path, removeErr)
		}
	}
	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear multiplayer mod overlay state: %w", err)
	}
	if err := os.RemoveAll(backupRoot); err != nil {
		return fmt.Errorf("clear multiplayer mod backups: %w", err)
	}
	return nil
}

func writeJSON(ctx context.Context, path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.Write(ctx, path, 0600, func(writer io.Writer) error {
		_, err := writer.Write(data)
		return err
	})
}
