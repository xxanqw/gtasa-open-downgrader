package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

const (
	extendedBundle    = "modloader/ExtendedLanguageLoader"
	extendedFontsURL  = "https://files.xxanqw.me/s/mVc8bf7w64"
	extendedFontsSHA  = "268e1df8a0c9d0a915db56d20540ffffb4c548fac2c197d64ea2868747f66152"
	extendedLoaderURL = "https://github.com/xxanqw/ExtendedLanguageLoader/releases/download/v0.4.2/ExtendedLanguageLoader-v0.4.2.zip"
	extendedLoaderSHA = "d2d008f3abfd721dd2d33046e827eb092836ade4734e59b95a5730cbbc3ead7b"
)

// Reviewed, matched finite-map files. No artwork is embedded in the downgrader.
// Per-file hashes also validate complete offline imports, not just online ZIPs.
var extendedInventory = map[string]string{
	"README.txt":                 "cb734a954b5c48caf0781574c9a51c274026f59d8491e2d33abe063c26f793d3",
	"ExtendedLanguageLoader.asi": "120c68a971957197250a2f912d1d77a96f62bc746d2dd8a9e14dd6f0172ff9bb",
	"LICENSE-ELL.txt":            "b14f4f8286dba4e7a0aaea428c7b4ca67789fedd54be21ae9130a81be557c0b2",
	"NOTICE-ELL.txt":             "4d531487f055c3f5c7fb59eb4be79b85ccf1d4a8e07dce4e30ff7b191bf5638d",
	"language-fonts/ExtendedFonts/extended_fonts.json":   "46ef6c57a91f54157bcb4010343de3a51e5ad1e34636d9a96c3e151415a61689",
	"language-fonts/ExtendedFonts/extended_fonts.txd":    "215fdd35a2a22c57ae4b2d85b8088ac635fae93ee686c3b1c074e5b87743bb9b",
	"language-fonts/ExtendedFonts/extended_metrics.json": "203888d4d82358f089097ed87c764e2f42979dd9a15e607fe4df9e38d3f5e269",
	"text/languages.ini":      "4095020a6d26695a36b55b32b95473b6d35225d31c15469c538d9e808b25efb9",
	"text/american-ell.gxt":   "c9164470947ff1546a21e1cd3a6aa637b90f7411278e0491376be5aa3be4cabd",
	"text/bel-ell.gxt":        "29859efc7faaa87823122e967d239aee7ef3309491a3f852404162534a798c17",
	"text/dutch-ell.gxt":      "5c6d51cf7cde89381d5108eb11d669887536c70572cd92a1a880b7046bb428b9",
	"text/french-ell.gxt":     "25a05a7ac33ea230674508010f6069654c529f5282d733294ee055c5e11a48e1",
	"text/german-ell.gxt":     "50dcbc685822a5d683b0acf259c78b4510cd24a8c42b48b59459c1d8470a2914",
	"text/indonesian-ell.gxt": "64330a3bbf3397f68679e6f17546ba963edc20494ab5abcc20b553288fa5d6a6",
	"text/italian-ell.gxt":    "dc8ce69ec23355ec1e592d7a3db43a73a1f30919bac76c0e77c8e0979261d577",
	"text/romanian-ell.gxt":   "e818d18c4c9638509070a68a3619f0b9102d5e9e5167b4eba0d9e154ffc8bcd2",
	"text/rus-ell.gxt":        "e2f6fd394a0849e39212fc23500339673176217c32264927112e30365a145dca",
	"text/spanish-ell.gxt":    "6db56c7969cbf116a928c068316ef3eabfabcd255cff62ceb56e61c01f3252e5",
	"text/ukr-ell.gxt":        "38750c082b69761a069962e5b6cc34950531abfc15601d5ee2f83c60739d30b8",
}

func extendedArchivePath(name string) string {
	switch name {
	case "LICENSE":
		name = "LICENSE-ELL.txt"
	case "NOTICE.md":
		name = "NOTICE-ELL.txt"
	}
	if _, ok := extendedInventory[name]; !ok {
		return ""
	}
	return extendedBundle + "/" + name
}
func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func checkSHA(path, want string) error {
	got, err := fileSHA(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("SHA-256 mismatch for %s; expected reviewed package, refusing installation", filepath.Base(path))
	}
	return nil
}
func validateExtended(staged string, inventory map[string]string) error {
	for name, sha := range inventory {
		p, err := fsutil.Join(staged, name)
		if err != nil {
			return err
		}
		if err = checkSHA(p, sha); err != nil {
			return fmt.Errorf("incomplete/mismatched ELL bundle (%s): %w", name, err)
		}
	}
	return nil
}

// Fail closed instead of moving an unrelated/root/script/renamed loader. Existing
// user-managed bundles (including alternate font/language configs) are untouched.
func preflightExtended(root string, t Tracker, inventory map[string]string) error {
	target, err := fsutil.Join(root, extendedBundle)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(target); err == nil {
		if !t.Has(ExtendedFonts) {
			return fmt.Errorf("existing ELL bundle is not managed by this downgrader; back it up and reconcile it manually")
		}
		for _, name := range []string{"text/languages.ini", "language-fonts/ExtendedFonts/extended_fonts.json", "language-fonts/ExtendedFonts/extended_metrics.json"} {
			if err = checkSHA(filepath.Join(target, filepath.FromSlash(name)), inventory[name]); err != nil {
				return fmt.Errorf("customized ELL configuration/map: %w; automatic merging is unsafe", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, dir := range []string{".", "scripts", "modloader"} {
		base := filepath.Join(root, dir)
		err = filepath.WalkDir(base, func(p string, d os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if dir == "." && p != base {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ".asi") || filepath.Clean(p) == filepath.Join(target, "ExtendedLanguageLoader.asi") {
				return nil
			}
			lower := strings.ToLower(filepath.Base(p))
			if lower == "extendedlanguageloader.asi" || lower == "languageloadertest.asi" || lower == "languageloader.asi" {
				return fmt.Errorf("conflicting loader at %s; close GTA and remove the duplicate yourself", p)
			}
			b, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			if bytes.Contains(b, []byte("ExtendedLanguageLoaderStatus")) && bytes.Contains(b, []byte("ExtendedLanguageLoaderFontProfile")) {
				return fmt.Errorf("renamed ELL copy at %s; remove the duplicate before installing", p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, rel := range []string{"modloader/HDFonts", hdFontStore} {
		p, err := fsutil.Join(root, rel)
		if err != nil {
			return err
		}
		if _, err = os.Lstat(p); err == nil && !t.Has(HDFonts) {
			return fmt.Errorf("unmanaged legacy HD Fonts at %s; back up and remove it manually", p)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (i Installer) installExtended(ctx context.Context, root string, rel Release, event Event) error {
	if err := platform.RequireGameClosed(); err != nil {
		return err
	}
	tracker, err := LoadTracker(root)
	if err != nil {
		return err
	}
	if err = preflightExtended(root, tracker, extendedInventory); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "gtasa-ell-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	for _, source := range []struct{ url, sha, name string }{{rel.URL, extendedFontsSHA, "fonts.zip"}, {extendedLoaderURL, extendedLoaderSHA, "loader.zip"}} {
		archive := filepath.Join(work, source.name)
		if err = i.Client.File(ctx, source.url, archive, func(done, total int64) {
			if event != nil {
				event(ExtendedFonts, "Downloading "+source.name, done, total)
			}
		}); err != nil {
			return err
		}
		if err = checkSHA(archive, source.sha); err != nil {
			return err
		}
		if err = fsutil.Extract(ctx, archive, filepath.Join(work, "stage"), "zip", extendedArchivePath); err != nil {
			return err
		}
	}
	if event != nil {
		event(ExtendedFonts, "Validating matched fonts, translations and ELL 0.4.2", 0, 0)
	}
	return commitExtended(ctx, root, filepath.Join(work, "stage", filepath.FromSlash(extendedBundle)), rel, i.AppVersion, extendedInventory, platform.RequireGameClosed)
}

func importExtended(ctx context.Context, root, archive, kind, version string) error {
	if kind != "zip" && kind != "7z" {
		return fmt.Errorf("select a ZIP or 7z complete ELL bundle")
	}
	if err := platform.RequireGameClosed(); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "gtasa-ell-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err = fsutil.Extract(ctx, archive, work, kind, extendedArchivePath); err != nil {
		return err
	}
	rel := Release{Version: "manual import", SourceURL: extendedFontsURL}
	return commitExtended(ctx, root, filepath.Join(work, filepath.FromSlash(extendedBundle)), rel, version, extendedInventory, platform.RequireGameClosed)
}

// Atomically activate ONE complete bundle and its tracker. The old bundle and
// tracked HD Fonts are moved outside autoload locations into a durable backup.
// No per-file activation, no edits to root GXTs/fonts/preferences/other mods.
func commitExtended(ctx context.Context, root, staged string, rel Release, version string, inventory map[string]string, closed func() error) error {
	return commitExtendedTransaction(ctx, root, staged, rel, inventory, closed, func(t Tracker) error { return t.Save(context.Background(), root, version) })
}
func commitExtendedTransaction(ctx context.Context, root, staged string, rel Release, inventory map[string]string, closed func() error, save func(Tracker) error) error {
	if err := validateExtended(staged, inventory); err != nil {
		return err
	}
	if err := closed(); err != nil {
		return err
	}
	t, err := LoadTracker(root)
	if err != nil {
		return err
	}
	if err = preflightExtended(root, t, inventory); err != nil {
		return err
	}
	base, err := fsutil.Join(root, "backups/mods")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(base, 0755); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(base, "extended-fonts-"+time.Now().UTC().Format("20060102T150405")+"-*")
	if err != nil {
		return err
	}
	// This incoming copy is outside autoload directories, on the same filesystem.
	incoming := filepath.Join(backup, "incoming")
	if err = copyExtendedDir(ctx, staged, incoming); err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	if err = validateExtended(incoming, inventory); err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	trackerPath, err := fsutil.Join(root, TrackerPath)
	if err != nil {
		return err
	}
	oldTracker, readErr := os.ReadFile(trackerPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if readErr == nil {
		if err = os.WriteFile(filepath.Join(backup, "tracker-before.json"), oldTracker, 0644); err != nil {
			return err
		}
	}
	target, err := fsutil.Join(root, extendedBundle)
	if err != nil {
		return err
	}
	type move struct{ source, dest string }
	var moves []move
	active := false
	rollback := func(cause error) error {
		var failures []error
		if active {
			if e := os.RemoveAll(target); e != nil {
				failures = append(failures, e)
			}
		}
		for n := len(moves) - 1; n >= 0; n-- {
			m := moves[n]
			if e := os.Rename(m.dest, m.source); e != nil {
				failures = append(failures, e)
			}
		}
		// Tracker is written atomically only after activation; failure keeps its old data.
		if len(failures) > 0 {
			return fmt.Errorf("%w; rollback incomplete: %v; originals at %s", cause, errors.Join(failures...), backup)
		}
		_ = os.RemoveAll(backup)
		return cause
	}
	paths := []string{extendedBundle}
	if t.Has(HDFonts) {
		paths = append(paths, "modloader/HDFonts", hdFontStore)
	}
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return rollback(err)
		}
		if err = closed(); err != nil {
			return rollback(err)
		}
		p, err := fsutil.Join(root, path)
		if err != nil {
			return rollback(err)
		}
		if _, err = os.Lstat(p); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return rollback(err)
		}
		dest := filepath.Join(backup, "previous", filepath.FromSlash(path))
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return rollback(err)
		}
		if err = os.Rename(p, dest); err != nil {
			return rollback(err)
		}
		moves = append(moves, move{p, dest})
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = closed(); err != nil {
		return rollback(err)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return rollback(err)
	}
	if err = os.Rename(incoming, target); err != nil {
		return rollback(err)
	}
	active = true
	names := []string{}
	for _, name := range t.Installed {
		if name != HDFonts && name != ExtendedFonts {
			names = append(names, name)
		}
	}
	t.Installed = append(names, ExtendedFonts)
	delete(t.Versions, HDFonts)
	rel.Style = ""
	rel.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	t.Versions[ExtendedFonts] = rel
	// Preserve legacy migration state; InstallSelected finalizes schema only after all mods.
	if !t.Legacy() {
		t.Schema = 2
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = closed(); err != nil {
		return rollback(err)
	}
	if err = save(t); err != nil {
		return rollback(err)
	}
	return nil
}
func copyExtendedDir(ctx context.Context, src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("ELL staging contains a link/special file")
		}
		return fsutil.CopyFile(ctx, p, filepath.Join(dst, rel))
	})
}
