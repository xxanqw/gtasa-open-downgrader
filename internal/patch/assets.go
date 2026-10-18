package patch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

const ReleaseID = "steam-r2-global-german-v2-complete"
const OfficialArchiveSHA256 = "bb974902ba4c8e188c38e25009112299b52ef63df5493c41ab6592971947e243"

func verifyOfficialArchive(ctx context.Context, archive string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := fsutil.Copy(ctx, hash, file); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != OfficialArchiveSHA256 {
		return fmt.Errorf("official patch archive SHA-256 mismatch: got %s", got)
	}
	return nil
}

func ValidateReleaseAssets(root string) (game.Manifest, error) {
	m, err := ValidateAssets(root)
	if err != nil {
		return m, err
	}
	if m.Version != "2.1" || m.ReleaseID != ReleaseID {
		return m, fmt.Errorf("patch assets are not the current Global/German v2.1 release; download Patches_v2.1.zip")
	}
	versions := map[string]bool{}
	for _, version := range m.SourceVersions {
		versions[version.ID] = true
	}
	if !versions["steam_r2_global"] || !versions["steam_r2_german"] {
		return m, fmt.Errorf("release patch assets are missing a source edition")
	}
	for _, f := range m.Files {
		if len(f.ApplicableVersions) == 0 {
			return m, fmt.Errorf("release patch assets omit source applicability for %s", f.Path)
		}
	}
	for _, version := range m.SourceVersions {
		if !versions[version.ID] {
			continue
		}
		covered := false
		for _, f := range m.Files {
			if game.IsEXE(f.Path) && f.Action == "copy" && f.AppliesTo(version.ID) && (f.MatchesSource(version.EXEHash) || f.TargetHash == version.EXEHash) {
				covered = true
				break
			}
		}
		if !covered {
			return m, fmt.Errorf("release patch assets do not cover executable for %s", version.ID)
		}
	}
	return m, nil
}

func ValidateAssets(root string) (game.Manifest, error) {
	m, e := game.Load(filepath.Join(root, "manifest.json"))
	if e != nil {
		return m, e
	}
	for _, f := range m.Files {
		for _, n := range f.PatchAssets() {
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
	archive := filepath.Join(work, "Patches_v2.1.zip")
	if e = c.File(ctx, url, archive, p); e != nil {
		return e
	}
	if url == download.PatchURL {
		if e = verifyOfficialArchive(ctx, archive); e != nil {
			return e
		}
	}
	return installArchive(ctx, archive, target, work, true)
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
	return installArchive(ctx, archive, target, work, false)
}

// ImportReleaseAssets verifies a release archive before replacing any existing assets.
func ImportReleaseAssets(ctx context.Context, archive, target string) error {
	if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return e
	}
	work, e := os.MkdirTemp(filepath.Dir(target), ".patch-import-*")
	if e != nil {
		return e
	}
	defer cleanupWork(work)
	return installArchive(ctx, archive, target, work, true)
}
func installArchive(ctx context.Context, archive, target, work string, release bool) error {
	unpack := filepath.Join(work, "extracted")
	if e := fsutil.Extract(ctx, archive, unpack, "zip", nil); e != nil {
		return e
	}
	root, e := patchRoot(unpack)
	if e != nil {
		return e
	}
	if release {
		_, e = ValidateReleaseAssets(root)
	} else {
		_, e = ValidateAssets(root)
	}
	if e != nil {
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

func patchRoot(unpack string) (string, error) {
	for _, root := range []string{unpack, filepath.Join(unpack, "Patches")} {
		if st, err := os.Stat(filepath.Join(root, "manifest.json")); err == nil && st.Mode().IsRegular() {
			return root, nil
		}
	}
	entries, err := os.ReadDir(unpack)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(unpack, entry.Name())
		if st, statErr := os.Stat(filepath.Join(root, "manifest.json")); statErr == nil && st.Mode().IsRegular() {
			return root, nil
		}
	}
	return "", fmt.Errorf("patch archive does not contain manifest.json")
}

// Do not delete a previous asset directory if an unusual filesystem failure
// also prevented rollback. Leave the path from the error available for recovery.
func cleanupWork(work string) {
	if _, err := os.Stat(filepath.Join(work, "previous")); os.IsNotExist(err) {
		_ = os.RemoveAll(work)
	}
}
