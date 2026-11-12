package mods

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

func isLimitAdjusterFile(name string) bool {
	switch strings.ToLower(name) {
	case "iii.vc.sa.limitadjuster.asi", "iii.vc.sa.limitadjuster.ini", "limit_adjuster_gta3vcsa.asi", "limit_adjuster_gta3vcsa.ini":
		return true
	}
	return false
}

// Older Project 2DFX installs owned a bundled adjuster in ModLoader. Migrate
// only that tracked copy, retaining its settings and an outside-autoload backup.
// Any other copy requires manual reconciliation instead of loading two plugins.
func prepareLimitAdjuster(ctx context.Context, root, staged string) (func() error, error) {
	noop := func() error { return nil }
	plugin := "scripts/III.VC.SA.LimitAdjuster.asi"
	if info, err := os.Stat(filepath.Join(staged, filepath.FromSlash(plugin))); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("Open Limit Adjuster archive has no nonempty plugin")
	}
	tracker, err := LoadTracker(root)
	if err != nil {
		return nil, err
	}
	var legacy []string
	inspect := func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !isLimitAdjusterFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			return fmt.Errorf("limit adjuster is not a regular file: %s", rel)
		}
		if rel == plugin || rel == "scripts/III.VC.SA.LimitAdjuster.ini" {
			if !tracker.Has(OpenLimitAdjuster) {
				return fmt.Errorf("unmanaged Open Limit Adjuster at %s; back up and reconcile this copy before installing", rel)
			}
			return nil
		}
		if tracker.Has("Project 2DFX") && strings.HasPrefix(rel, "modloader/Project2DFX/") && filepath.ToSlash(filepath.Dir(rel)) == "modloader/Project2DFX" {
			legacy = append(legacy, rel)
			return nil
		}
		return fmt.Errorf("another limit adjuster exists at %s; back up and reconcile it before installing Open Limit Adjuster", rel)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			if err := inspect(filepath.Join(root, entry.Name()), entry, nil); err != nil {
				return nil, err
			}
		}
	}
	for _, dir := range []string{"scripts", "modloader"} {
		full, err := fsutil.Join(root, dir)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(full, inspect)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	if len(legacy) == 0 {
		return noop, nil
	}
	// Keep customized settings rather than replacing them with release defaults.
	currentINI, err := fsutil.Join(root, "scripts/III.VC.SA.LimitAdjuster.ini")
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(currentINI); os.IsNotExist(err) {
		for _, rel := range legacy {
			if strings.EqualFold(filepath.Ext(rel), ".ini") {
				source, err := fsutil.Join(root, rel)
				if err != nil {
					return nil, err
				}
				if err := fsutil.CopyFile(ctx, source, filepath.Join(staged, "scripts", "III.VC.SA.LimitAdjuster.ini")); err != nil {
					return nil, err
				}
				break
			}
		}
	} else if err != nil {
		return nil, err
	}
	backupBase, err := fsutil.Join(root, "backups/mods")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(backupBase, 0755); err != nil {
		return nil, err
	}
	backup, err := os.MkdirTemp(backupBase, "limit-adjuster-migration-*")
	if err != nil {
		return nil, err
	}
	type move struct{ source, backup string }
	var moved []move
	rollback := func() error {
		var failures []error
		for index := len(moved) - 1; index >= 0; index-- {
			item := moved[index]
			if err := os.Rename(item.backup, item.source); err != nil {
				failures = append(failures, err)
			}
		}
		if len(failures) == 0 {
			_ = os.RemoveAll(backup)
			return nil
		}
		return fmt.Errorf("limit adjuster recovery incomplete; original files retained at %s: %w", backup, errors.Join(failures...))
	}
	fail := func(err error) (func() error, error) { return nil, errors.Join(err, rollback()) }
	for _, rel := range legacy {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := platform.RequireGameClosed(); err != nil {
			return fail(err)
		}
		source, err := fsutil.Join(root, rel)
		if err != nil {
			return fail(err)
		}
		destination := filepath.Join(backup, filepath.Base(source))
		if err := os.Rename(source, destination); err != nil {
			return fail(err)
		}
		moved = append(moved, move{source, destination})
	}
	return rollback, nil
}
