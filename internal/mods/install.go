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
)

func layout(mod, name string) string {
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
	}
	return ""
}

// Extract stages and validates the whole package, then rolls back the overlay if
// any write fails or is cancelled. A mod is tracked only after all writes succeed.
func Extract(ctx context.Context, root, mod, archive, kind string) error {
	if !Supported(mod) {
		return fmt.Errorf("unknown mod %q", mod)
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
			if rel == TrackerPath || strings.HasPrefix(strings.ToLower(rel), "backups/") || strings.HasPrefix(strings.ToLower(rel), ".downgrader") {
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
	existed := map[string]bool{}
	for _, rel := range files {
		dst, e := fsutil.Join(root, rel)
		if e != nil {
			return e
		}
		if _, e = os.Stat(dst); e == nil {
			existed[rel] = true
			if e = fsutil.CopyFile(ctx, dst, filepath.Join(old, filepath.FromSlash(rel))); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	var applied []string
	for _, rel := range files {
		dst, err := fsutil.Join(root, rel)
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

func (i Installer) InstallSelected(ctx context.Context, root string, selected []string, force bool, event Event) error {
	tracker, e := LoadTracker(root)
	if e != nil {
		return e
	}
	legacy := tracker.Legacy()
	names := Selection(selected, tracker)
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
			err = i.install(ctx, resolver, root, name, rel, event)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
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
func (i Installer) install(ctx context.Context, r *Resolver, root, name string, rel Release, event Event) error {
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
	return Extract(ctx, root, name, archive, rel.ArchiveType)
}
func Import(ctx context.Context, root, name, archive, version string) error {
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
	tracker.Versions[name] = Release{Version: "manual import", InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	if !tracker.Legacy() {
		tracker.Schema = 2
	}
	return tracker.Save(context.Background(), root, version)
}
