//go:build !launcher

package assets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
)

func PatchDirectory(ctx context.Context, cache string) (string, error) {
	exe, _ := os.Executable()
	for _, p := range []string{"Patches", filepath.Join(filepath.Dir(exe), "Patches"), filepath.Join(cache, "Patches")} {
		if _, e := patch.ValidateReleaseAssets(p); e == nil {
			return filepath.Abs(p)
		}
	}
	if !Offline {
		return "", fmt.Errorf("current patch assets are missing; download Patches_v2.1.zip or import a local archive")
	}
	dst := filepath.Join(cache, "Patches")
	if e := os.MkdirAll(cache, 0755); e != nil {
		return "", e
	}
	work, e := os.MkdirTemp(cache, ".embedded-patches-*")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(work)
	e = fs.WalkDir(Patches, "Patches", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel("Patches", p)
		if e != nil {
			return e
		}
		target, e := fsutil.Join(work, rel)
		if e != nil {
			return e
		}
		f, e := Patches.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		return fsutil.Write(ctx, target, 0644, func(w io.Writer) error { _, e := fsutil.Copy(ctx, w, f); return e })
	})
	if e != nil {
		return "", e
	}
	if _, e = patch.ValidateReleaseAssets(work); e != nil {
		return "", fmt.Errorf("offline build has invalid embedded patches: %w", e)
	}
	// A malformed cache is kept aside until the valid embedded copy is installed.
	previous := dst + ".previous"
	if _, e = os.Stat(previous); e == nil {
		return "", fmt.Errorf("previous patches exist at %s; inspect them before retrying", previous)
	}
	had := false
	if _, e = os.Stat(dst); e == nil {
		if e = os.Rename(dst, previous); e != nil {
			return "", e
		}
		had = true
	}
	if e = os.Rename(work, dst); e != nil {
		if had {
			_ = os.Rename(previous, dst)
		}
		return "", e
	}
	if had {
		_ = os.RemoveAll(previous)
	}
	return dst, nil
}
