package mods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

// Synthetic staged assets exercise transactions, not RenderWare/game validity.
func syntheticInterface(t *testing.T, name string) string {
	t.Helper()
	stage := t.TempDir()
	file := "models/hud.txd"
	if name == AtmosphereHUD {
		file = "models/pcbtns.txd"
	}
	if name == HDRadarMap {
		file = "radar00.txd"
	}
	if name == NadalaoWeapons {
		file = "WeaponIcons.txd"
	}
	testutil.Write(t, stage, file, "synthetic native asset")
	files, err := interfaceFiles(stage)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(interfaceManifest{Schema: 1, Name: name, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, stage, interfaceMarker, string(b))
	return stage
}
func TestInterfacePinnedCatalogAndDependencyOrdering(t *testing.T) {
	r := NewResolver(nil)
	for name, spec := range interfacePacks {
		rel, err := r.Resolve(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		if rel.Checksum != spec.checksum || !strings.Contains(rel.Fingerprint, spec.checksum) {
			t.Fatal("unpinned source", name, rel)
		}
		if rel.FileID == "" || rel.ArchiveType != "7z" {
			t.Fatal("bad host resolution", rel)
		}
	}
	want := []string{"ASI Loader", "ModLoader", OriginalHQIcons, AtmosphereHUD}
	if got := Selection([]string{AtmosphereHUD}, Tracker{}); !reflect.DeepEqual(got, want) {
		t.Fatal("shared HUD dependency", got)
	}
	root := t.TempDir()
	tr := Tracker{Schema: 2, Installed: []string{OriginalHQIcons, AtmosphereHUD}, Versions: map[string]Release{}}
	if err := tr.Save(context.Background(), root, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(context.Background(), root, OriginalHQIcons, "test"); err == nil {
		t.Fatal("removed needed shared HUD")
	}
	if err := checkInterfaceSelection([]string{OriginalHQIcons}, Tracker{Installed: []string{SanLtdRussian}}); err == nil {
		t.Fatal("selected two HUD owners")
	}
}
func TestInterfaceWhitelistsExcludeFontsMenusRadarAndEmptyHelper(t *testing.T) {
	for _, name := range []string{OriginalHQIcons, AtmosphereHUD, NadalaoWeapons} {
		for _, file := range []string{"models/fonts.txd", "data/fonts.dat", "text/languages.ini", "extra.asi", "Atmosphere Interface Pack/HQ Radar/radar00.txd", "Atmosphere Interface Pack/models/fronten1.txd", "Atmosphere Interface Pack/models/fronten2.txd", "png/M4icon.png"} {
			if got := layout(name, file); got != "" {
				t.Fatal("unexpected member", name, file, got)
			}
		}
	}
	if got := layout(AtmosphereHUD, "Atmosphere Interface Pack/models/hud.txd"); got != "" {
		t.Fatal("duplicate HUD dictionary installed")
	}
	if got := layout(OriginalHQIcons, "SA Original Radar Icons HQ/hud.txd"); got != "modloader/OriginalHQRadarIcons/models/hud.txd" {
		t.Fatal(got)
	}
	for n := 0; n < 144; n++ {
		file := fmt.Sprintf("radar%02d.txd", n)
		if got := layout(HDRadarMap, "Atmosphere Interface Pack/HQ Radar/"+file); got != "modloader/HDRadarMap/"+file {
			t.Fatal("missing original radar tile", got)
		}
	}
	for _, file := range []string{"map.svg", "Atmosphere Interface Pack/HQ Radar/radar144.txd", "Atmosphere Interface Pack/models/hud.txd", "Atmosphere Interface Pack/models/fronten2.txd", "models/fonts.txd", "extra.asi"} {
		if got := layout(HDRadarMap, file); got != "" {
			t.Fatal("radar imported unrelated file", got)
		}
	}
	if err := Import(context.Background(), t.TempDir(), NadalaoWeapons, testutil.Zip(t, map[string]string{"Weapon Icons TXD/WeaponIcons.txd": "empty placeholder"}), "test"); err == nil {
		t.Fatal("empty/unmatched helper package accepted")
	}
}
func TestInterfaceAtomicActivationUpdateUninstallAndProtectedFiles(t *testing.T) {
	root := t.TempDir()
	for p, v := range map[string]string{"models/fonts.txd": "native fonts", "data/fonts.dat": "native widths", "text/american.gxt": "native text", "models/hud.txd": "native HUD", "gta_sa.set": "preferences", "modloader/ExtendedLanguageLoader/text/languages.ini": "accepted ELL config", "scripts/unrelated.asi": "other ASI"} {
		testutil.Write(t, root, p, v)
	}
	stage := syntheticInterface(t, OriginalHQIcons)
	if err := commitInterface(context.Background(), root, OriginalHQIcons, stage, Release{Version: "one"}, "test", closedTest, nil); err != nil {
		t.Fatal(err)
	}
	if err := commitInterface(context.Background(), root, OriginalHQIcons, stage, Release{Version: "two"}, "test", closedTest, nil); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups/mods/original-hq-radar-icons-*/previous/models/hud.txd"))
	if err != nil || len(backups) != 1 {
		t.Fatal("missing update backup", backups, err)
	}
	tr, err := LoadTracker(root)
	if err != nil || tr.Versions[OriginalHQIcons].Version != "two" {
		t.Fatal(tr, err)
	}
	if _, err = Uninstall(context.Background(), root, OriginalHQIcons, "test"); err != nil {
		t.Fatal(err)
	}
	for p, v := range map[string]string{"models/fonts.txd": "native fonts", "data/fonts.dat": "native widths", "text/american.gxt": "native text", "models/hud.txd": "native HUD", "gta_sa.set": "preferences", "modloader/ExtendedLanguageLoader/text/languages.ini": "accepted ELL config", "scripts/unrelated.asi": "other ASI"} {
		if testutil.Read(t, root, p) != v {
			t.Fatal("protected file changed", p)
		}
	}
}
func TestInterfaceConflictModifiedFilesAndOrdinaryPackageGuard(t *testing.T) {
	for name, file := range map[string]string{HDRadarMap: "modloader/Other/radar00.txd", OriginalHQIcons: "modloader/Other/hud.txd", AtmosphereHUD: "modloader/Other/pcbtns.txd", NadalaoWeapons: "scripts/WeaponIconsTXD.SA.asi"} {
		root := t.TempDir()
		testutil.Write(t, root, file, "existing user asset")
		if err := preflightInterface(root, name); err == nil {
			t.Fatal("overlap accepted", name)
		}
		if testutil.Read(t, root, file) != "existing user asset" {
			t.Fatal("unrelated file moved")
		}
	}
	root := t.TempDir()
	stage := syntheticInterface(t, OriginalHQIcons)
	if err := commitInterface(context.Background(), root, OriginalHQIcons, stage, Release{}, "test", closedTest, nil); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), root, SanLtdRussian, testutil.Zip(t, map[string]string{"00 - Copy to game folder/models/hud.txd": "other HUD", "00 - Copy to game folder/text/american.gxt": "other text"}), "zip"); err == nil {
		t.Fatal("ordinary package installed second HUD")
	}
	if _, err := os.Stat(filepath.Join(root, "modloader/SALtdRussian/text/american.gxt")); !os.IsNotExist(err) {
		t.Fatal("partial conflicting translation install")
	}
	testutil.Write(t, root, interfacePacks[OriginalHQIcons].folder+"/models/hud.txd", "customized")
	if err := preflightInterface(root, OriginalHQIcons); err == nil {
		t.Fatal("modified managed asset overwritten")
	}
}
func TestInterfaceRollbackRestoresDirectoryAndTracker(t *testing.T) {
	for _, mode := range []string{"tracker failure", "cancel", "game starts"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			stage := syntheticInterface(t, OriginalHQIcons)
			if err := commitInterface(context.Background(), root, OriginalHQIcons, stage, Release{Version: "old"}, "test", closedTest, nil); err != nil {
				t.Fatal(err)
			}
			before := testutil.Read(t, root, TrackerPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			checks := 0
			closed := func() error {
				checks++
				if checks == 4 && mode != "tracker failure" {
					if mode == "cancel" {
						cancel()
						return context.Canceled
					}
					return errors.New("game started")
				}
				return nil
			}
			err := commitInterface(ctx, root, OriginalHQIcons, stage, Release{Version: "new"}, "test", closed, func(Tracker) error { return errors.New("atomic tracker I/O failure") })
			if err == nil {
				t.Fatal("failed transaction accepted")
			}
			if testutil.Read(t, root, TrackerPath) != before || testutil.Read(t, root, interfacePacks[OriginalHQIcons].folder+"/models/hud.txd") != "synthetic native asset" {
				t.Fatal("rollback lost original package/tracker")
			}
		})
	}
}

// External actual archives: no copyrighted fixtures enter CI or the repository.
func TestInterfaceActualArchives(t *testing.T) {
	dir := os.Getenv("HD_INTERFACE_ARCHIVES")
	if dir == "" {
		t.Skip("set HD_INTERFACE_ARCHIVES to external reviewed archives/source files")
	}
	ctx := context.Background()
	root := t.TempDir()
	tr := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader"}, Versions: map[string]Release{}}
	if err := tr.Save(ctx, root, "test"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{HDRadarMap, OriginalHQIcons, AtmosphereHUD, NadalaoWeapons} {
		t.Run(name, func(t *testing.T) {
			spec := interfacePacks[name]
			work := t.TempDir()
			staged := filepath.Join(work, filepath.FromSlash(spec.folder))
			archive := map[string]string{HDRadarMap: "Atmosphere_Interface_Pack.7z", OriginalHQIcons: "SA_Original_Radar_Icons_HQ.7z", AtmosphereHUD: "Atmosphere_Interface_Pack.7z", NadalaoWeapons: "nadalao_Weapon_Icons_-_icones_fieis_corrigidos.7z"}[name]
			p := filepath.Join(dir, archive)
			if err := checkSHA(p, spec.checksum); err != nil {
				t.Fatal(err)
			}
			if err := fsutil.Extract(ctx, p, work, "7z", func(m string) string { return interfaceArchivePath(name, m) }); err != nil {
				t.Fatal(err)
			}
			if name == NadalaoWeapons {
				p = filepath.Join(dir, "Weapon_Icons_TXD.7z")
				if err := checkSHA(p, weaponHelperSHA); err != nil {
					t.Fatal(err)
				}
				if err := fsutil.Extract(ctx, p, work, "7z", func(m string) string {
					if m == "Weapon Icons TXD/WeaponIconsTXD.SA.asi" {
						return spec.folder + "/WeaponIconsTXD.SA.asi"
					}
					if m == "Readme (or die).txt" {
						return spec.folder + "/README-helper.txt"
					}
					return ""
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := prepareInterface(ctx, name, staged, nil); err != nil {
				t.Fatal(err)
			}
			if output := os.Getenv("HD_INTERFACE_EXPORT"); output != "" {
				if err := copyExtendedDir(ctx, staged, filepath.Join(output, filepath.Base(spec.folder))); err != nil {
					t.Fatal(err)
				}
			}
			if name == HDRadarMap {
				for n := 0; n < 144; n++ {
					p := filepath.Join(staged, radarName(n))
					data, err := os.ReadFile(p)
					if err != nil {
						t.Fatal(err)
					}
					checkOriginalRadarTxd(t, data, n)
					if err := checkSHA(p, radarRasterHashes[n]); err != nil {
						t.Fatal("original raster bytes changed", err)
					}
				}
			}
			if err := commitInterface(ctx, root, name, staged, Release{Version: "actual"}, "test", closedTest, nil); err != nil {
				t.Fatal(err)
			}
			if err := validateInterfaceFolder(filepath.Join(root, filepath.FromSlash(spec.folder)), name); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Exactly one HUD dictionary, one helper and 144 matching radar dictionaries.
	hud, helpers, radar := 0, 0, 0
	if err := filepath.WalkDir(filepath.Join(root, "modloader"), func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			if strings.EqualFold(d.Name(), "hud.txd") {
				hud++
			}
			if strings.EqualFold(d.Name(), "WeaponIconsTXD.SA.asi") {
				helpers++
			}
			if strings.HasPrefix(d.Name(), "radar") && strings.HasSuffix(d.Name(), ".txd") {
				radar++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if hud != 1 || helpers != 1 || radar != 144 {
		t.Fatal("duplicate/missing active assets", hud, helpers, radar)
	}
	for _, name := range []string{NadalaoWeapons, AtmosphereHUD, OriginalHQIcons, HDRadarMap} {
		if _, err := Uninstall(ctx, root, name, "test"); err != nil {
			t.Fatal(err)
		}
	}
}
func radarName(n int) string { return fmt.Sprintf("radar%02d.txd", n) }
