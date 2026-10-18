package mods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

func layout(mod, name string) string {
	mod = canonicalModName(mod)
	switch mod {
	case "ASI Loader":
		if name == "vorbisFile.dll" || name == "vorbisHooked.dll" || strings.HasPrefix(name, "scripts/") {
			return name
		}
	case "ModLoader":
		return name
	case "SilentPatch":
		if strings.HasSuffix(strings.ToLower(name), ".asi") || strings.HasSuffix(strings.ToLower(name), ".ini") {
			return "modloader/SilentPatch/" + path.Base(name)
		}
	case "Widescreen Fixes":
		if strings.HasPrefix(name, "scripts/") {
			return "modloader/WidescreenFix/" + strings.TrimPrefix(name, "scripts/")
		}
	case "SkyGFX":
		if _, rest, ok := strings.Cut(name, "/"); ok {
			return "modloader/SkyGFX/" + rest
		}
	case "Frontend Mods":
		return "modloader/FrontendMods/" + name
	case HDRadarMap, OriginalHQIcons, AtmosphereHUD, NadalaoWeapons:
		return interfaceArchivePath(mod, name)
	case ExtendedFonts:
		return extendedArchivePath(name)
	case Framerate:
		base := strings.ToLower(path.Base(name))
		if base == "frameratevigilante.sa.asi" || base == "frameratevigilante.ini" {
			for _, part := range strings.Split(path.Dir(name), "/") {
				if strings.EqualFold(part, "gta sa") {
					return "modloader/FramerateVigilante/" + path.Base(name)
				}
			}
		}
	case "GInput":
		if !strings.HasPrefix(name, "GInputAPI (for modders)/") {
			return "modloader/GInput/" + name
		}
	case "Project 2DFX":
		return "modloader/Project2DFX/" + name
	case SanLtdRussian:
		const prefix = "00 - Copy to game folder/"
		if !strings.HasPrefix(name, prefix) {
			return ""
		}
		rel := strings.TrimPrefix(name, prefix)
		lower := strings.ToLower(rel)
		switch {
		case lower == "data/fonts.dat",
			lower == "text/american.gxt",
			lower == "text/spanish.gxt",
			lower == "models/fonts.txd",
			lower == "models/fronten1.txd",
			lower == "models/hud.txd",
			strings.HasPrefix(lower, "models/txd/") && strings.HasSuffix(lower, ".txd"):
			return "modloader/SALtdRussian/" + rel
		}
	}
	return ""
}

// Extract stages and validates the whole package, then rolls back the overlay if
// any write fails or is cancelled. A mod is tracked only after all writes succeed.
func Extract(ctx context.Context, root, mod, archive, kind string) error {
	return extract(ctx, root, mod, archive, kind, false)
}

// ExtractUpdate replaces package files in place while preserving existing
// configuration. Supported INI files gain new upstream keys without losing
// local values; unknown configuration formats are left untouched.
func ExtractUpdate(ctx context.Context, root, mod, archive, kind string) error {
	return extract(ctx, root, mod, archive, kind, true)
}

func extract(ctx context.Context, root, mod, archive, kind string, update bool) error {
	mod = canonicalModName(mod)
	if mod == ExtendedFonts {
		return importExtended(ctx, root, archive, kind, "")
	}
	if isInterfaceMod(mod) {
		return importInterface(ctx, root, mod, archive, kind, "")
	}
	if !Supported(mod) {
		return fmt.Errorf("unknown mod %q", mod)
	}
	if err := platform.RequireGameClosed(); err != nil {
		return err
	}
	work, e := os.MkdirTemp(root, ".downgrader-mod-*")
	if e != nil {
		return e
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(work)
		}
	}()
	staged := filepath.Join(work, "new")
	old := filepath.Join(work, "old")
	if e = fsutil.Extract(ctx, archive, staged, kind, func(n string) string { return layout(mod, n) }); e != nil {
		return e
	}
	var files []string
	e = filepath.WalkDir(staged, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			rel, e := filepath.Rel(staged, p)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			if strings.EqualFold(rel, TrackerPath) || strings.HasPrefix(strings.ToLower(rel), "backups/") || strings.HasPrefix(strings.ToLower(rel), ".downgrader") {
				return fmt.Errorf("mod package uses reserved path: %s", rel)
			}
			files = append(files, rel)
		}
		return nil
	})
	if e != nil {
		return e
	}
	if len(files) == 0 {
		return fmt.Errorf("package contains no files for %s", mod)
	}
	if e = checkInterfaceOverlap(root, files); e != nil {
		return e
	}
	existed := map[string]bool{}
	var writes []string
	for _, rel := range files {
		dst, e := fsutil.Join(root, rel)
		if e != nil {
			return e
		}
		if _, e = os.Stat(dst); e == nil {
			existed[rel] = true
			if update {
				stage := filepath.Join(staged, filepath.FromSlash(rel))
				write, err := prepareUpdateFile(ctx, dst, stage)
				if err != nil {
					return err
				}
				if !write {
					continue
				}
			}
			if e = fsutil.CopyFile(ctx, dst, filepath.Join(old, filepath.FromSlash(rel))); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		writes = append(writes, rel)
	}
	var applied []string
	for _, rel := range writes {
		dst, err := fsutil.Join(root, rel)
		if err == nil {
			err = platform.RequireGameClosed()
		}
		if err == nil {
			err = fsutil.CopyFile(ctx, filepath.Join(staged, filepath.FromSlash(rel)), dst)
		}
		if err != nil {
			var rollbackErrors []error
			for i := len(applied) - 1; i >= 0; i-- {
				name := applied[i]
				target, rollback := fsutil.Join(root, name)
				if rollback == nil {
					if existed[name] {
						rollback = fsutil.CopyFile(context.Background(), filepath.Join(old, filepath.FromSlash(name)), target)
					} else {
						rollback = os.Remove(target)
						if os.IsNotExist(rollback) {
							rollback = nil
						}
					}
				}
				if rollback != nil {
					rollbackErrors = append(rollbackErrors, rollback)
				}
			}
			if len(rollbackErrors) > 0 {
				keep = true
				return fmt.Errorf("install: %v; rollback incomplete (%v); original files retained at %s", err, errors.Join(rollbackErrors...), old)
			}
			return err
		}
		applied = append(applied, rel)
	}
	return nil
}

type Installer struct {
	Client     *download.Client
	AppVersion string
	Resolve    func(context.Context, string) (Release, error)
	Install    func(context.Context, string, string, Release) error
}
type Event func(name, status string, done, total int64)

type UpdateResult struct {
	Updated, Current, Untracked int
}

// UpdateInstalled checks only recorded installations. Packages without a
// fingerprint cannot be classified safely and require an explicit reinstall.
func (i Installer) UpdateInstalled(ctx context.Context, root string, event Event) (UpdateResult, error) {
	return i.updateSelected(ctx, root, nil, event)
}

// UpdateSelected scopes update checks and installation to the named installed mods.
func (i Installer) UpdateSelected(ctx context.Context, root string, selected []string, event Event) (UpdateResult, error) {
	allowed := make(map[string]bool, len(selected))
	for _, name := range selected {
		if !Supported(name) {
			return UpdateResult{}, fmt.Errorf("unsupported selected mod: %s", name)
		}
		allowed[name] = true
	}
	return i.updateSelected(ctx, root, allowed, event)
}
func (i Installer) updateSelected(ctx context.Context, root string, allowed map[string]bool, event Event) (UpdateResult, error) {
	var result UpdateResult
	tracker, err := LoadTracker(root)
	if err != nil {
		return result, err
	}
	resolver := NewResolver(i.Client)
	resolve := i.Resolve
	if resolve == nil {
		resolve = resolver.Resolve
	}
	var failures []error
	for _, name := range tracker.Installed {
		if allowed != nil && !allowed[name] {
			continue
		}
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if !Supported(name) {
			continue
		}
		installed := tracker.Versions[name]
		if installed.Fingerprint == "" {
			result.Untracked++
			if event != nil {
				event(name, "Version untracked; reinstall manually", 0, 0)
			}
			continue
		}
		if event != nil {
			event(name, "Checking for updates", 0, 0)
		}
		latest, resolveErr := resolve(ctx, name)
		if resolveErr != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			failures = append(failures, fmt.Errorf("%s: %w", name, resolveErr))
			if event != nil {
				event(name, "Update check unavailable", 0, 0)
			}
			continue
		}
		if latest.Fingerprint == "" {
			result.Untracked++
			if event != nil {
				event(name, "Release version untracked", 0, 0)
			}
			continue
		}
		if installed.Fingerprint == latest.Fingerprint {
			result.Current++
			if event != nil {
				event(name, "Already up to date", 0, 0)
			}
			continue
		}
		if i.Install != nil {
			err = i.Install(ctx, root, name, latest)
		} else {
			err = i.install(ctx, resolver, root, name, latest, event, true)
		}
		if err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, errors.Join(append(failures, fmt.Errorf("%s: %w", name, err))...)
		}
		if hasTransactionalTracker(name) && i.Install == nil {
			tracker, err = LoadTracker(root)
			if err != nil {
				return result, err
			}
		}
		latest.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		tracker.Versions[name] = latest
		if err = tracker.Save(context.Background(), root, i.AppVersion); err != nil {
			return result, fmt.Errorf("record %s update: %w", name, err)
		}
		result.Updated++
		if event != nil {
			event(name, "Updated", 1, 1)
		}
	}
	return result, errors.Join(failures...)
}

func (i Installer) InstallSelected(ctx context.Context, root string, selected []string, force bool, event Event) error {
	tracker, e := LoadTracker(root)
	if e != nil {
		return e
	}
	legacy := tracker.Legacy()
	names := Selection(selected, tracker)
	if e = checkInterfaceSelection(names, tracker); e != nil {
		return e
	}
	if i.Install == nil {
		if e = platform.RequireGameClosed(); e != nil {
			return e
		}
		for _, name := range names {
			if isInterfaceMod(name) {
				if e = preflightInterface(root, name); e != nil {
					return e
				}
			}
			if name == ExtendedFonts && (!tracker.Has(name) || legacy || force) {
				if e = preflightExtended(root, tracker, extendedInventory); e != nil {
					return e
				}
			}
		}
	}
	resolver := NewResolver(i.Client)
	resolve := i.Resolve
	if resolve == nil {
		resolve = resolver.Resolve
	}
	for _, name := range names {
		if e = ctx.Err(); e != nil {
			return e
		}
		if tracker.Has(name) && !legacy && !force {
			if event != nil {
				event(name, "Already installed", 0, 0)
			}
			continue
		}
		if event != nil {
			event(name, "Resolving official release", 0, 0)
		}
		rel, err := resolve(ctx, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if i.Install != nil {
			err = i.Install(ctx, root, name, rel)
		} else {
			err = i.install(ctx, resolver, root, name, rel, event, false)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if hasTransactionalTracker(name) && i.Install == nil {
			tracker, err = LoadTracker(root)
			if err != nil {
				return err
			}
		}
		if !tracker.Has(name) {
			tracker.Installed = append(tracker.Installed, name)
		}
		rel.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		tracker.Versions[name] = rel
		// Persist completed packages even when cancellation arrives immediately after
		// the commit. Keep legacy status until every recorded mod has been migrated.
		if !legacy {
			tracker.Schema = 2
		}
		if err = tracker.Save(context.Background(), root, i.AppVersion); err != nil {
			return err
		}
		if event != nil {
			event(name, "Installed", 1, 1)
		}
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	tracker.Schema = 2
	return tracker.Save(context.Background(), root, i.AppVersion)
}
func (i Installer) install(ctx context.Context, r *Resolver, root, name string, rel Release, event Event, update bool) error {
	if err := platform.RequireGameClosed(); err != nil {
		return err
	}
	if i.Client == nil {
		i.Client = r.Client
	}
	if name == ExtendedFonts {
		return i.installExtended(ctx, root, rel, event)
	}
	if isInterfaceMod(name) {
		return i.installInterface(ctx, r, root, name, rel, event)
	}
	raw, e := r.DownloadURL(ctx, rel)
	if e != nil {
		return e
	}
	work, e := os.MkdirTemp("", "gtasa-mod-download-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(work)
	archive := filepath.Join(work, "package")
	e = i.Client.File(ctx, raw, archive, func(done, total int64) {
		if event != nil {
			event(name, "Downloading", done, total)
		}
	})
	if e != nil {
		return e
	}
	if event != nil {
		event(name, "Installing", 0, 0)
	}
	return extract(ctx, root, name, archive, rel.ArchiveType, update)
}
func Import(ctx context.Context, root, name, archive, version string) error {
	name = canonicalModName(name)
	if isInterfaceMod(name) {
		return importInterface(ctx, root, name, archive, strings.TrimPrefix(strings.ToLower(filepath.Ext(archive)), "."), version)
	}
	if name == ExtendedFonts {
		kind := strings.TrimPrefix(strings.ToLower(filepath.Ext(archive)), ".")
		return importExtended(ctx, root, archive, kind, version)
	}
	tracker, e := LoadTracker(root)
	if e != nil {
		return e
	}
	kind := strings.TrimPrefix(strings.ToLower(filepath.Ext(archive)), ".")
	if kind != "zip" && kind != "7z" {
		return fmt.Errorf("select a ZIP or 7z archive")
	}
	if e = Extract(ctx, root, name, archive, kind); e != nil {
		return e
	}
	if !tracker.Has(name) {
		tracker.Installed = append(tracker.Installed, name)
	}
	rel := Release{Version: "manual import", InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	tracker.Versions[name] = rel
	if !tracker.Legacy() {
		tracker.Schema = 2
	}
	return tracker.Save(context.Background(), root, version)
}
