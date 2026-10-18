package launcherapp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

type widescreenEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type widescreenState struct {
	Schema  int               `json:"schema_version"`
	Entries []widescreenEntry `json:"entries"`
}

func (runtime *Runtime) widescreenPaths() (string, string) {
	base := filepath.Join(runtime.GameDir, ".downgrader", "multiplayer")
	return filepath.Join(base, "widescreen-disabled.json"), filepath.Join(base, "widescreen-backup")
}

func (runtime *Runtime) requireClosedForCompatibility() error {
	processes, err := runtime.gameProcessSnapshot()
	if err != nil {
		return fmt.Errorf("inspect running GTA processes: %w", err)
	}
	if len(processes) != 0 {
		return fmt.Errorf("close GTA (gta_sa.exe/gta-sa.exe) before changing multiplayer compatibility files")
	}
	return nil
}

// No symlink traversal, including state-controlled restoration paths.
func (runtime *Runtime) compatibilityPath(relative string) (string, error) {
	path, err := fsutil.Join(runtime.GameDir, relative)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(runtime.GameDir)
	for _, component := range strings.Split(filepath.FromSlash(relative), string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symbolic link in compatibility path: %s", relative)
		}
	}
	return path, nil
}

func widescreenName(name string) bool {
	return strings.EqualFold(name, "GTASA.WidescreenFix.asi") || strings.EqualFold(name, "GTASA.WidescreenFixLegacy.asi")
}

func (runtime *Runtime) findWidescreen(ctx context.Context) ([]widescreenEntry, error) {
	files := make(map[string]string)
	known := make(map[string]bool)
	inspect := func(path string, info os.FileInfo) error {
		if !strings.EqualFold(filepath.Ext(path), ".asi") {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot verify ASI exclusion for non-regular file: %s", path)
		}
		digest, err := entrypoint.Hash(ctx, path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(runtime.GameDir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = digest
		if widescreenName(filepath.Base(path)) {
			known[digest] = true
		}
		return nil
	}
	entries, err := os.ReadDir(runtime.GameDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if err := inspect(filepath.Join(runtime.GameDir, entry.Name()), info); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"scripts", "plugins", "modloader"} {
		root, err := runtime.compatibilityPath(name)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("cannot verify Widescreen Fix exclusion through symbolic link: %s", path)
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return inspect(path, info)
		})
		if err != nil {
			return nil, err
		}
	}
	var found []widescreenEntry
	for path, digest := range files {
		// Also quarantine byte-identical renamed copies of a named bundle.
		if widescreenName(filepath.Base(path)) || known[digest] {
			found = append(found, widescreenEntry{path, digest})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found, nil
}

// Move only ASIs, not bundles/settings/fonts, to non-autoload .bin files.
// The write-ahead inventory handles interruption before/after every rename.
func (runtime *Runtime) disableWidescreen(ctx context.Context) error {
	statePath, backupRoot := runtime.widescreenPaths()
	for _, path := range []string{statePath, backupRoot} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("Widescreen Fix compatibility recovery is pending: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	entries, err := runtime.findWidescreen(ctx)
	if err != nil {
		return fmt.Errorf("inspect Widescreen Fix for multiplayer: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	if err := runtime.requireClosedForCompatibility(); err != nil {
		return err
	}
	if _, err := runtime.compatibilityPath(".downgrader/multiplayer/widescreen-backup"); err != nil {
		return err
	}
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		return err
	}
	if err := writeJSON(ctx, statePath, widescreenState{Schema: 1, Entries: entries}); err != nil {
		return errors.Join(err, os.Remove(backupRoot))
	}
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, runtime.restoreWidescreen())
		}
		active, err := runtime.compatibilityPath(entry.Path)
		if err == nil {
			var digest string
			digest, err = entrypoint.Hash(ctx, active)
			if err == nil && digest != entry.SHA256 {
				err = fmt.Errorf("Widescreen Fix changed during preflight: %s", entry.Path)
			}
		}
		if err == nil {
			err = runtime.requireClosedForCompatibility()
		}
		if err == nil {
			var backup string
			backup, err = runtime.compatibilityPath(fmt.Sprintf(".downgrader/multiplayer/widescreen-backup/%03d.bin", index))
			if err == nil {
				err = moveCompatibilityFile(active, backup)
			}
		}
		if err != nil {
			return errors.Join(fmt.Errorf("disable Widescreen Fix: %w", err), runtime.restoreWidescreen())
		}
	}
	return nil
}

func (runtime *Runtime) restoreWidescreen() error {
	statePath, backupRoot := runtime.widescreenPaths()
	if _, err := runtime.compatibilityPath(".downgrader/multiplayer/widescreen-disabled.json"); err != nil {
		return err
	}
	data, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state widescreenState
	if err := json.Unmarshal(data, &state); err != nil || state.Schema != 1 || len(state.Entries) == 0 {
		return fmt.Errorf("invalid Widescreen Fix recovery inventory")
	}
	type move struct{ from, to, digest string }
	var moves []move
	seen := make(map[string]bool)
	// Validate every entry before restoring any: unknown/changed files never overwritten.
	for index, entry := range state.Entries {
		digest, err := hex.DecodeString(entry.SHA256)
		path := filepath.ToSlash(entry.Path)
		key := strings.ToLower(path)
		allowed := !strings.Contains(path, "/") || strings.HasPrefix(key, "scripts/") || strings.HasPrefix(key, "plugins/") || strings.HasPrefix(key, "modloader/")
		if err != nil || len(digest) != 32 || !strings.EqualFold(filepath.Ext(path), ".asi") || !allowed || seen[key] {
			return fmt.Errorf("invalid Widescreen Fix recovery entry")
		}
		seen[key] = true
		active, err := runtime.compatibilityPath(path)
		if err != nil {
			return err
		}
		backup, err := runtime.compatibilityPath(fmt.Sprintf(".downgrader/multiplayer/widescreen-backup/%03d.bin", index))
		if err != nil {
			return err
		}
		original, activeErr := entrypoint.Hash(context.Background(), active)
		saved, backupErr := entrypoint.Hash(context.Background(), backup)
		if activeErr == nil {
			if original != entry.SHA256 || !errors.Is(backupErr, os.ErrNotExist) {
				return fmt.Errorf("cannot restore Widescreen Fix: %s changed or both copies exist; original retained at %s", path, backupRoot)
			}
			continue // Rename was never made, or an earlier recovery restored this row.
		}
		if !errors.Is(activeErr, os.ErrNotExist) {
			return activeErr
		}
		if backupErr != nil || saved != entry.SHA256 {
			return fmt.Errorf("Widescreen Fix backup missing or changed: %s", backup)
		}
		if _, err := os.Stat(filepath.Dir(active)); err != nil {
			return fmt.Errorf("Widescreen Fix original directory unavailable: %w", err)
		}
		moves = append(moves, move{backup, active, entry.SHA256})
	}
	for _, move := range moves {
		if err := runtime.requireClosedForCompatibility(); err != nil {
			return err
		}
		for _, path := range []string{move.from, move.to} {
			relative, err := filepath.Rel(runtime.GameDir, path)
			if err != nil {
				return err
			}
			if _, err := runtime.compatibilityPath(filepath.ToSlash(relative)); err != nil {
				return err
			}
		}
		digest, err := entrypoint.Hash(context.Background(), move.from)
		if err != nil || digest != move.digest {
			return fmt.Errorf("Widescreen Fix backup changed during recovery: %s", move.from)
		}
		if err := moveCompatibilityFile(move.from, move.to); err != nil {
			return fmt.Errorf("restore Widescreen Fix without overwriting replacements: %w", err)
		}
	}
	if err := runtime.requireClosedForCompatibility(); err != nil {
		return err
	}
	// Do not recursively delete unrecorded files in a recovery folder.
	if err := os.Remove(backupRoot); err != nil {
		return fmt.Errorf("clear Widescreen Fix recovery folder: %w", err)
	}
	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("clear Widescreen Fix recovery inventory: %w", err)
	}
	return nil
}
