package patch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

func ValidateAssets(root string) (game.Manifest, error) {
	m, e := game.Load(filepath.Join(root, "manifest.json"))
	if e != nil {
		return m, e
	}
	for _, f := range m.Files {
		n := f.Path + ".xdelta"
		if f.Action == "copy" {
			n = "gta_sa.exe"
		}
		p, e := fsutil.Join(root, n)
		if e != nil {
			return m, e
		}
		st, e := os.Stat(p)
		if e != nil {
			return m, e
		}
		if !st.Mode().IsRegular() {
			return m, fmt.Errorf("invalid patch asset: %s", n)
		}
	}
	return m, nil
}
func DownloadAssets(ctx context.Context, c *download.Client, url, target string, p download.Progress) error {
	parent := filepath.Dir(target)
	if e := os.MkdirAll(parent, 0755); e != nil {
		return e
	}
	work, e := os.MkdirTemp(parent, ".patch-download-*")
	if e != nil {
		return e
	}
	defer cleanupWork(work)
	archive := filepath.Join(work, "Patches.zip")
	if e = c.File(ctx, url, archive, p); e != nil {
		return e
	}
	return installArchive(ctx, archive, target, work)
}
func ImportAssets(ctx context.Context, archive, target string) error {
	if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return e
	}
	work, e := os.MkdirTemp(filepath.Dir(target), ".patch-import-*")
	if e != nil {
		return e
	}
	defer cleanupWork(work)
	return installArchive(ctx, archive, target, work)
}
func installArchive(ctx context.Context, archive, target, work string) error {
	unpack := filepath.Join(work, "extracted")
	if e := fsutil.Extract(ctx, archive, unpack, "zip", nil); e != nil {
		return e
	}
	root := unpack
	if _, e := os.Stat(filepath.Join(root, "manifest.json")); os.IsNotExist(e) {
		root = filepath.Join(root, "Patches")
	}
	if _, e := ValidateAssets(root); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	previous := filepath.Join(work, "previous")
	hadPrevious := false
	if _, e := os.Lstat(target); e == nil {
		if e = os.Rename(target, previous); e != nil {
			return e
		}
		hadPrevious = true
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := os.Rename(root, target); e != nil {
		if hadPrevious {
			if rollback := os.Rename(previous, target); rollback != nil {
				return fmt.Errorf("install: %v; rollback: %w (previous assets: %s)", e, rollback, previous)
			}
		}
		return e
	}
	if hadPrevious {
		return os.RemoveAll(previous)
	}
	return nil
}

// Do not delete a previous asset directory if an unusual filesystem failure
// also prevented rollback. Leave the path from the error available for recovery.
func cleanupWork(work string) {
	if _, err := os.Stat(filepath.Join(work, "previous")); os.IsNotExist(err) {
		_ = os.RemoveAll(work)
	}
}
