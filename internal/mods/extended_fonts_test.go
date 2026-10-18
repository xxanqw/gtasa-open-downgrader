package mods

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

// Synthetic contracts test transaction mechanics only; they are NOT game/TXD/GXT
// validity or rendering fixtures. Production always uses the reviewed hashes.
func syntheticExtended(t *testing.T) (string, map[string]string) {
	t.Helper()
	stage := t.TempDir()
	inventory := map[string]string{}
	for name := range extendedInventory {
		data := "synthetic transaction fixture: " + name
		testutil.Write(t, stage, name, data)
		sum := sha256.Sum256([]byte(data))
		inventory[name] = hex.EncodeToString(sum[:])
	}
	return stage, inventory
}
func closedTest() error { return nil }
func legacyExtendedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for p, data := range map[string]string{"text/american.gxt": "vanilla GXT", "models/fonts.txd": "vanilla font", "data/fonts.dat": "vanilla widths", "scripts/unrelated.asi": "other mod", "gta_sa.set": "untouched preferences", "modloader/HDFonts/models/fonts.txd": "old HD font", hdFontStore + "/plain/fonts.dat": "old style widths"} {
		testutil.Write(t, root, p, data)
	}
	tr := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", HDFonts, "GInput"}, Versions: map[string]Release{HDFonts: {Version: "old", Style: "vintage"}}, Extra: map[string]json.RawMessage{"user_setting": json.RawMessage(`true`)}}
	if err := tr.Save(context.Background(), root, "old"); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestExtendedFontsAtomicLegacyMigrationAndUninstall(t *testing.T) {
	root := legacyExtendedRoot(t)
	stage, inv := syntheticExtended(t)
	if err := commitExtended(context.Background(), root, stage, Release{Version: "test"}, "test", inv, closedTest); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTracker(root)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Has(HDFonts) || !tr.Has(ExtendedFonts) || !tr.Has("GInput") || string(tr.Extra["user_setting"]) != "true" {
		t.Fatalf("bad tracker %+v", tr)
	}
	for _, path := range []string{"modloader/HDFonts", hdFontStore} {
		if _, err = os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatal("old HD Fonts still active")
		}
	}
	for p, data := range map[string]string{"text/american.gxt": "vanilla GXT", "models/fonts.txd": "vanilla font", "data/fonts.dat": "vanilla widths", "scripts/unrelated.asi": "other mod", "gta_sa.set": "untouched preferences"} {
		if testutil.Read(t, root, p) != data {
			t.Fatal("unrelated file changed", p)
		}
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups/mods/extended-fonts-*"))
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
	if testutil.Read(t, backups[0], "previous/modloader/HDFonts/models/fonts.txd") != "old HD font" {
		t.Fatal("missing durable HD backup")
	}
	if err = validateExtended(filepath.Join(root, filepath.FromSlash(extendedBundle)), inv); err != nil {
		t.Fatal(err)
	}
	result, err := Uninstall(context.Background(), root, ExtendedFonts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if testutil.Read(t, result.BackupDir, extendedBundle+"/ExtendedLanguageLoader.asi") == "" {
		t.Fatal("ASI not backed up")
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(extendedBundle))); !os.IsNotExist(err) {
		t.Fatal("active ELL bundle remains")
	}
	if testutil.Read(t, root, "models/fonts.txd") != "vanilla font" {
		t.Fatal("uninstall changed vanilla assets")
	}
}
func TestExtendedFontsRollbackOnTrackerFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"save failure", "game starts", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root := legacyExtendedRoot(t)
			before := testutil.Read(t, root, TrackerPath)
			stage, inv := syntheticExtended(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			closed := func() error {
				calls++
				if calls == 6 && mode != "save failure" {
					if mode == "cancel" {
						cancel()
						return context.Canceled
					}
					return errors.New("game started")
				}
				return nil
			}
			err := commitExtendedTransaction(ctx, root, stage, Release{Version: "test"}, inv, closed, func(Tracker) error { return errors.New("simulated atomic tracker write failure") })
			if err == nil {
				t.Fatal("accepted failing transaction")
			}
			if testutil.Read(t, root, TrackerPath) != before {
				t.Fatal("changed tracker on rollback")
			}
			if testutil.Read(t, root, "modloader/HDFonts/models/fonts.txd") != "old HD font" || testutil.Read(t, root, hdFontStore+"/plain/fonts.dat") != "old style widths" {
				t.Fatal("legacy files lost")
			}
			if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(extendedBundle))); !os.IsNotExist(err) {
				t.Fatal("partial ELL activation")
			}
		})
	}
}
func TestExtendedFontsRejectsDuplicateAndUnmanagedInstallations(t *testing.T) {
	for _, path := range []string{"ExtendedLanguageLoader.asi", "scripts/ExtendedLanguageLoader.asi", "modloader/Other/ExtendedLanguageLoader.asi", "scripts/renamed.asi", extendedBundle + "/ExtendedLanguageLoader.asi", "modloader/HDFonts/fonts.txd", hdFontStore + "/user-font"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			stage, inv := syntheticExtended(t)
			testutil.Write(t, root, path, "ExtendedLanguageLoaderStatus\x00ExtendedLanguageLoaderFontProfile")
			err := commitExtended(context.Background(), root, stage, Release{Version: "test"}, "test", inv, closedTest)
			if err == nil {
				t.Fatal("conflicting installation accepted")
			}
			if testutil.Read(t, root, path) != "ExtendedLanguageLoaderStatus\x00ExtendedLanguageLoaderFontProfile" {
				t.Fatal("unmanaged files changed")
			}
		})
	}
}
func TestExtendedFontsRejectsModifiedMapOrIncompleteBundle(t *testing.T) {
	for _, name := range []string{"text/languages.ini", "language-fonts/ExtendedFonts/extended_fonts.json", "language-fonts/ExtendedFonts/extended_metrics.json", "ExtendedLanguageLoader.asi"} {
		t.Run(name, func(t *testing.T) {
			root := legacyExtendedRoot(t)
			stage, inv := syntheticExtended(t)
			before := testutil.Read(t, root, TrackerPath)
			if err := os.Remove(filepath.Join(stage, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
			if err := commitExtended(context.Background(), root, stage, Release{}, "test", inv, closedTest); err == nil {
				t.Fatal("incomplete bundle accepted")
			}
			if testutil.Read(t, root, TrackerPath) != before {
				t.Fatal("incomplete package changed tracker")
			}
		})
	}
	root := t.TempDir()
	stage, inv := syntheticExtended(t)
	if err := commitExtended(context.Background(), root, stage, Release{Version: "old"}, "test", inv, closedTest); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, extendedBundle+"/text/languages.ini", "customized language list")
	if err := commitExtended(context.Background(), root, stage, Release{Version: "new"}, "test", inv, closedTest); err == nil {
		t.Fatal("customized config overwritten")
	}
	if testutil.Read(t, root, extendedBundle+"/text/languages.ini") != "customized language list" {
		t.Fatal("modified user INI")
	}
}
func TestExtendedFontsManagedUpdateBacksUpWholeBundle(t *testing.T) {
	root := t.TempDir()
	stage, inv := syntheticExtended(t)
	if err := commitExtended(context.Background(), root, stage, Release{Version: "old"}, "test", inv, closedTest); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, extendedBundle+"/ExtendedLanguageLoader.log", "old diagnostic log")
	if err := commitExtended(context.Background(), root, stage, Release{Version: "new"}, "test", inv, closedTest); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTracker(root)
	if err != nil || tr.Versions[ExtendedFonts].Version != "new" {
		t.Fatal(tr, err)
	}
	logs, err := filepath.Glob(filepath.Join(root, "backups/mods/extended-fonts-*/previous", filepath.FromSlash(extendedBundle), "ExtendedLanguageLoader.log"))
	if err != nil || len(logs) != 1 {
		t.Fatal("missing previous bundle backup", logs, err)
	}
}
func TestExtendedFontsConflictRefusedBeforeDependencyDownloads(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "scripts/ExtendedLanguageLoader.asi", "existing loader")
	resolved := 0
	i := Installer{Resolve: func(context.Context, string) (Release, error) {
		resolved++
		return Release{}, errors.New("unexpected resolution/download")
	}}
	if err := i.InstallSelected(context.Background(), root, []string{ExtendedFonts}, true, nil); err == nil || !strings.Contains(err.Error(), "conflicting loader") {
		t.Fatal(err)
	}
	if resolved != 0 {
		t.Fatal("touched dependencies before ELL preflight")
	}
	if _, err := os.Stat(filepath.Join(root, TrackerPath)); !os.IsNotExist(err) {
		t.Fatal("changed tracker before ELL preflight")
	}
}

func TestExtendedFontsPinnedPairAndBadDownload(t *testing.T) {
	client := download.New()
	resolver := NewResolver(client)
	rel, err := resolver.Resolve(context.Background(), ExtendedFonts)
	if err != nil {
		t.Fatal(err)
	}
	if rel.URL != extendedFontsURL || !strings.Contains(rel.Fingerprint, extendedLoaderSHA) || !strings.Contains(rel.Fingerprint, extendedFontsSHA) {
		t.Fatal("pair is not integrity-pinned", rel)
	}
	if got := Selection([]string{ExtendedFonts}, Tracker{}); !reflect.DeepEqual(got, []string{"ASI Loader", "ModLoader", ExtendedFonts}) {
		t.Fatal(got)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("changed package or HTML landing page"))
	}))
	defer server.Close()
	root := legacyExtendedRoot(t)
	before := testutil.Read(t, root, TrackerPath)
	rel.URL = server.URL
	i := Installer{Client: client, AppVersion: "test"}
	if err = i.installExtended(context.Background(), root, rel, nil); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatal(err)
	}
	if testutil.Read(t, root, TrackerPath) != before || testutil.Read(t, root, "modloader/HDFonts/models/fonts.txd") != "old HD font" {
		t.Fatal("bad download changed installed files")
	}
}
func TestExtendedFontsImportRejectsFontOnlyUnsafeAndUnknownAssets(t *testing.T) {
	for _, files := range []map[string]string{
		{"text/languages.ini": "font-only"},
		{"../unsafe.asi": "bad"},
		{"ExtendedLanguageLoader.asi": "unreviewed ASI", "models/fonts.txd": "native overwrite", "extra.asi": "other mod"},
	} {
		root := t.TempDir()
		testutil.Write(t, root, "models/fonts.txd", "vanilla")
		if err := Import(context.Background(), root, ExtendedFonts, testutil.Zip(t, files), "test"); err == nil {
			t.Fatal("unverified/incomplete archive accepted")
		}
		if testutil.Read(t, root, "models/fonts.txd") != "vanilla" {
			t.Fatal("native font overwritten")
		}
		if _, err := os.Stat(filepath.Join(root, TrackerPath)); !os.IsNotExist(err) {
			t.Fatal("failed import changed tracker")
		}
	}
}

// Optional exact-artifact acceptance: external copyrighted files never enter CI
// or this repository. No live GTA/gameplay is launched by this test.
func TestExtendedFontsActualArchives(t *testing.T) {
	fonts, loader := os.Getenv("ELL_FONTS_ARCHIVE"), os.Getenv("ELL_LOADER_ARCHIVE")
	if fonts == "" || loader == "" {
		t.Skip("set ELL_FONTS_ARCHIVE and ELL_LOADER_ARCHIVE for exact-artifact acceptance")
	}
	if err := checkSHA(fonts, extendedFontsSHA); err != nil {
		t.Fatal(err)
	}
	if err := checkSHA(loader, extendedLoaderSHA); err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	for _, archive := range []string{fonts, loader} {
		if err := fsutil.Extract(context.Background(), archive, stage, "zip", extendedArchivePath); err != nil {
			t.Fatal(err)
		}
	}
	root := legacyExtendedRoot(t)
	if err := commitExtended(context.Background(), root, filepath.Join(stage, filepath.FromSlash(extendedBundle)), Release{Version: "actual 0.1.0 / 0.4.2"}, "test", extendedInventory, closedTest); err != nil {
		t.Fatal(err)
	}
	if err := validateExtended(filepath.Join(root, filepath.FromSlash(extendedBundle)), extendedInventory); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(context.Background(), root, ExtendedFonts, "test"); err != nil {
		t.Fatal(err)
	}
	// Test the actual offline importer, including the real official ASI's
	// recognizable exports, without placing copyrighted assets in the repository.
	bundle := filepath.Join(t.TempDir(), "complete-ell.zip")
	if err := fsutil.Write(context.Background(), bundle, 0644, func(w io.Writer) error {
		z := zip.NewWriter(w)
		for name := range extendedInventory {
			member, err := z.Create(name)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(extendedBundle), filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			if _, err = member.Write(data); err != nil {
				return err
			}
		}
		return z.Close()
	}); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(root, "scripts", "renamed-language-loader.asi")
	if err := fsutil.CopyFile(context.Background(), filepath.Join(stage, filepath.FromSlash(extendedBundle), "ExtendedLanguageLoader.asi"), renamed); err != nil {
		t.Fatal(err)
	}
	if err := Import(context.Background(), root, ExtendedFonts, bundle, "test"); err == nil || !strings.Contains(err.Error(), "renamed ELL copy") {
		t.Fatal("real renamed ASI not refused", err)
	}
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	if err := Import(context.Background(), root, ExtendedFonts, bundle, "test"); err != nil {
		t.Fatal(err)
	}
	if err := validateExtended(filepath.Join(root, filepath.FromSlash(extendedBundle)), extendedInventory); err != nil {
		t.Fatal(err)
	}
	tracker, err := LoadTracker(root)
	if err != nil || tracker.Versions[ExtendedFonts].Fingerprint != "" || tracker.Versions[ExtendedFonts].Version != "manual import" {
		t.Fatal("incorrect offline provenance", tracker, err)
	}
}
