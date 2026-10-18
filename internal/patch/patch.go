// Package patch validates outputs before replacing game files and preserves originals.
package patch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

type Progress func(index int, status, message string)
type Result struct{ Succeeded, Failed int }
type Runner func(context.Context, string, ...string) error

func Run(ctx context.Context, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	platform.HideConsole(cmd)
	out, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if e != nil {
		return fmt.Errorf("xdelta3: %w: %s", e, out)
	}
	return nil
}

type Engine struct {
	Xdelta string
	Run    Runner
}

func (e Engine) Apply(ctx context.Context, root, assets string, m game.Manifest, progress Progress) (Result, error) {
	var result Result
	if err := m.Validate(); err != nil {
		return result, err
	}
	if e.Run == nil {
		e.Run = Run
	}
	edition, err := m.SourceEdition(ctx, root)
	if err != nil {
		return result, err
	}
	for _, f := range m.Files {
		if len(f.ApplicableVersions) > 0 && edition == "" {
			return result, fmt.Errorf("cannot determine source edition for %s; restore an original executable or backup", f.Path)
		}
	}
	var failures []error
	for i, f := range m.Files {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !f.AppliesTo(edition) {
			if progress != nil {
				progress(i, "Not in this edition", "")
			}
			continue
		}
		if progress != nil {
			progress(i, "Backup & patching…", "")
		}
		err := e.applyFile(ctx, root, assets, f)
		if err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			result.Failed++
			failures = append(failures, fmt.Errorf("%s: %w", f.Path, err))
			if progress != nil {
				progress(i, "Failed", err.Error())
			}
		} else {
			result.Succeeded++
			if progress != nil {
				progress(i, "Success", "")
			}
		}
	}
	return result, errors.Join(failures...)
}
func (e Engine) applyFile(ctx context.Context, root, assets string, f game.File) error {
	src, err := game.Source(root, f.Path)
	if err != nil {
		return err
	}
	h, err := fsutil.Hash(ctx, src)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	targets := []string{filepath.Base(src)}
	if !game.IsEXE(f.Path) {
		targets = []string{f.Path}
	} else if !entrypoint.MetadataPresent(root) {
		targets = append(targets, game.OtherEXE(filepath.Base(src)))
	}
	if h == f.TargetHash || game.IsEXE(f.Path) && h == game.LAAHash {
		// Already downgraded: create only a missing executable alias.
		if len(targets) == 2 {
			dst, err := fsutil.Join(root, targets[1])
			if err != nil {
				return err
			}
			if _, err = os.Stat(dst); os.IsNotExist(err) {
				if err = Backup(ctx, root, targets[1]); err != nil {
					return err
				}
				return fsutil.CopyFile(ctx, src, dst)
			}
		}
		return nil
	}
	if err = os.MkdirAll(filepath.Dir(src), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(src), ".downgrader-patch-*")
	if err != nil {
		return err
	}
	out := temp.Name()
	temp.Close()
	os.Remove(out)
	defer os.Remove(out)
	if f.Action == "copy" {
		p, err := fsutil.Join(assets, f.Payload)
		if err != nil {
			return err
		}
		err = fsutil.CopyFile(ctx, p, out)
		if err != nil {
			return err
		}
	} else {
		if h == "" {
			return fmt.Errorf("source file is missing")
		}
		p, err := fsutil.Join(assets, f.PatchAsset(h))
		if err != nil {
			return err
		}
		if err = e.Run(ctx, e.Xdelta, "-d", "-s", src, p, out); err != nil {
			return err
		}
	}
	actual, err := fsutil.Hash(ctx, out)
	if err != nil {
		return err
	}
	if actual != f.TargetHash {
		return fmt.Errorf("output hash mismatch: got %s, expected %s", actual, f.TargetHash)
	}
	for _, rel := range targets {
		if err = Backup(ctx, root, rel); err != nil {
			return err
		}
	}
	for _, rel := range targets {
		dst, err := fsutil.Join(root, rel)
		if err != nil {
			return err
		}
		if err = fsutil.CopyFile(ctx, out, dst); err != nil {
			return err
		}
	}
	return nil
}

const indexName = ".downgrader-index.json"

type original struct {
	Existed bool `json:"existed"`
}

func readIndex(root string) (map[string]original, error) {
	p, e := fsutil.Join(root, "backups/"+indexName)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	m := map[string]original{}
	if os.IsNotExist(e) {
		return m, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	if m == nil {
		return nil, fmt.Errorf("invalid backup index")
	}
	return m, nil
}
func Backup(ctx context.Context, root, rel string) error {
	src, e := fsutil.Join(root, rel)
	if e != nil {
		return e
	}
	dst, e := fsutil.Join(root, "backups/"+rel)
	if e != nil {
		return e
	}
	m, e := readIndex(root)
	if e != nil {
		return e
	}
	if _, ok := m[rel]; ok {
		return nil
	}
	existed := false
	if st, err := os.Stat(dst); err == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("invalid backup: %s", dst)
		}
		existed = true
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err = os.Stat(src); err == nil {
		existed = true
		if err = fsutil.CopyFile(ctx, src, dst); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	m[rel] = original{existed}
	data, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	idx, e := fsutil.Join(root, "backups/"+indexName)
	if e != nil {
		return e
	}
	return fsutil.Write(ctx, idx, 0644, func(w io.Writer) error { _, e := w.Write(data); return e })
}
func Revert(ctx context.Context, root string, progress func(int, int)) error {
	backup, e := fsutil.Join(root, "backups")
	if e != nil {
		return e
	}
	if _, e = os.Stat(backup); e != nil {
		return e
	}
	m, e := readIndex(root)
	if e != nil {
		return e
	}
	// Include backups produced by Python, which did not have an index.
	e = filepath.WalkDir(backup, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup contains symbolic link: %s", p)
		}
		if d.IsDir() {
			// Mod uninstall snapshots are kept in backups/mods and are not
			// original game files managed by the downgrade journal.
			if p == filepath.Join(backup, "mods") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(backup, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel != indexName {
			m[rel] = original{true}
		}
		return nil
	})
	if e != nil {
		return e
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, rel := range keys {
		if e = ctx.Err(); e != nil {
			return e
		}
		if entrypoint.MetadataPresent(root) && rel == entrypoint.SteamExecutable {
			if progress != nil {
				progress(i+1, len(keys))
			}
			continue
		}
		dst, e := fsutil.Join(root, rel)
		if e != nil {
			return e
		}
		if m[rel].Existed {
			src, e := fsutil.Join(backup, rel)
			if e != nil {
				return e
			}
			e = fsutil.CopyFile(ctx, src, dst)
			if e != nil {
				return e
			}
		} else {
			if e = os.Remove(dst); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
		if progress != nil {
			progress(i+1, len(keys))
		}
	}
	return nil
}
func Cleanup(root string) error {
	p, e := fsutil.Join(root, "backups")
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	keepMods := false
	for _, entry := range entries {
		if entry.Name() == "mods" {
			keepMods = true
			continue
		}
		if e := os.RemoveAll(filepath.Join(p, entry.Name())); e != nil {
			return e
		}
	}
	if keepMods {
		return nil
	}
	return os.Remove(p)
}
