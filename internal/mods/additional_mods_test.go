package mods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestAdditionalPluginReleasesAndLayouts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ThirteenAG/III.VC.SA.WindowedMode/releases/latest":
			w.Write([]byte(`{"tag_name":"v3.0.2","assets":[{"name":"III.VC.SA.WindowedMode.zip","id":1,"browser_download_url":"https://example.com/windowed.zip"}]}`))
		case "/GTAmodding/III.VC.SA.LimitAdjuster/releases/latest":
			w.Write([]byte(`{"tag_name":"v2.0.6","assets":[{"name":"III.VC.SA.LimitAdjuster.zip","id":2,"browser_download_url":"https://example.com/limits.zip"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	resolver := NewResolver(nil)
	resolver.GitHubAPI = server.URL
	for _, name := range []string{WindowedMode, OpenLimitAdjuster} {
		rel, err := resolver.Resolve(context.Background(), name)
		if err != nil || rel.URL == "" || rel.ArchiveType != "zip" {
			t.Fatal(name, rel, err)
		}
	}
	root := t.TempDir()
	archive := testutil.Zip(t, map[string]string{"III.VC.SA.WindowedMode.asi": "plugin", "readme.md": "ignore", "vorbisFile.dll": "ignore"})
	if err := Extract(context.Background(), root, WindowedMode, archive, "zip"); err != nil {
		t.Fatal(err)
	}
	if testutil.Read(t, root, "modloader/WindowedMode/III.VC.SA.WindowedMode.asi") != "plugin" {
		t.Fatal("missing plugin")
	}
	if _, err := os.Stat(filepath.Join(root, "vorbisFile.dll")); !os.IsNotExist(err) {
		t.Fatal("installed bundled loader")
	}
}

func TestLODVegetationDependencyAndReviewedInventory(t *testing.T) {
	want := []string{"ASI Loader", "ModLoader", OpenLimitAdjuster, LODVegetation}
	if got := Selection([]string{LODVegetation}, Tracker{}); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if reason := UninstallBlocker(Tracker{Installed: want}, OpenLimitAdjuster, nil); reason == "" {
		t.Fatal("allowed removing required adjuster")
	}
	rel, err := NewResolver(nil).Resolve(context.Background(), LODVegetation)
	if err != nil || rel.FileID != "7u9q36epczhn" || rel.Checksum != interfacePacks[LODVegetation].checksum {
		t.Fatal(rel, err)
	}
	if layout(LODVegetation, "LOD Vegetation/LODvegetation.ide") == "" {
		t.Fatal("IDE not mapped")
	}
	if layout(LODVegetation, "LOD Vegetation/unreviewed.asi") != "" {
		t.Fatal("unreviewed plugin allowed")
	}
	if layout("Project 2DFX", "III.VC.SA.LimitAdjuster.asi") != "" {
		t.Fatal("duplicate adjuster bundled")
	}
}

func TestLimitAdjusterMigratesManagedBundleAndRetainsSettings(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "Project 2DFX"}}
	if err := tracker.Save(context.Background(), root, "test"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "modloader/Project2DFX/III.VC.SA.LimitAdjuster.asi", "old plugin")
	testutil.Write(t, root, "modloader/Project2DFX/III.VC.SA.LimitAdjuster.ini", "[SALIMITS]\nBuildings = 25000\n")
	archive := testutil.Zip(t, map[string]string{"III.VC.SA.LimitAdjuster.asi": "new plugin", "III.VC.SA.LimitAdjuster.ini": "[SALIMITS]\nBuildings = unlimited\n"})
	if err := Extract(context.Background(), root, OpenLimitAdjuster, archive, "zip"); err != nil {
		t.Fatal(err)
	}
	if testutil.Read(t, root, "scripts/III.VC.SA.LimitAdjuster.asi") != "new plugin" {
		t.Fatal("missing early-loaded plugin")
	}
	if !strings.Contains(testutil.Read(t, root, "scripts/III.VC.SA.LimitAdjuster.ini"), "25000") {
		t.Fatal("settings lost")
	}
	if _, err := os.Stat(filepath.Join(root, "modloader/Project2DFX/III.VC.SA.LimitAdjuster.asi")); !os.IsNotExist(err) {
		t.Fatal("duplicate remains")
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups/mods/limit-adjuster-migration-*/III.VC.SA.LimitAdjuster.asi"))
	if err != nil || len(backups) != 1 {
		t.Fatal("missing backup", backups, err)
	}
	content, _ := os.ReadFile(backups[0])
	if string(content) != "old plugin" {
		t.Fatal("backup changed")
	}
}

func TestLimitAdjusterRefusesUnmanagedCopyAndRollsBackMigration(t *testing.T) {
	for _, variant := range []string{"unmanaged", "write failure"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			tracker := Tracker{Schema: 2, Installed: []string{"Project 2DFX"}}
			if err := tracker.Save(context.Background(), root, "test"); err != nil {
				t.Fatal(err)
			}
			testutil.Write(t, root, "modloader/Project2DFX/III.VC.SA.LimitAdjuster.asi", "old plugin")
			if variant == "unmanaged" {
				testutil.Write(t, root, "scripts/limit_adjuster_gta3vcsa.asi", "external")
			} else {
				if err := os.MkdirAll(filepath.Join(root, "scripts/III.VC.SA.LimitAdjuster.asi"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			archive := testutil.Zip(t, map[string]string{"III.VC.SA.LimitAdjuster.asi": "new plugin"})
			if err := Extract(context.Background(), root, OpenLimitAdjuster, archive, "zip"); err == nil {
				t.Fatal("unsafe install accepted")
			}
			if testutil.Read(t, root, "modloader/Project2DFX/III.VC.SA.LimitAdjuster.asi") != "old plugin" {
				t.Fatal("old plugin not restored")
			}
		})
	}
}

// Real archive verification is opt-in; no third-party assets enter the repo.
func TestReviewedLODVegetationArchive(t *testing.T) {
	archive := os.Getenv("LOD_VEGETATION_ARCHIVE")
	if archive == "" {
		t.Skip("set LOD_VEGETATION_ARCHIVE to the official v1.2 7z")
	}
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", OpenLimitAdjuster}}
	if err := tracker.Save(context.Background(), root, "test"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "scripts/III.VC.SA.LimitAdjuster.asi", "dependency")
	if err := Import(context.Background(), root, LODVegetation, archive, "test"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "modloader/LODVegetation")
	if err := validateInterfaceFolder(folder, LODVegetation); err != nil {
		t.Fatal(err)
	}
	files, err := interfaceFiles(folder)
	if err != nil || len(files) != len(lodVegetationHashes) {
		t.Fatal("incomplete package", len(files), err)
	}
	if _, err := Uninstall(context.Background(), root, OpenLimitAdjuster, "test"); err == nil {
		t.Fatal("removed vegetation dependency")
	}
	if _, err := Uninstall(context.Background(), root, LODVegetation, "test"); err != nil {
		t.Fatal(err)
	}
	if testutil.Read(t, root, "scripts/III.VC.SA.LimitAdjuster.asi") != "dependency" {
		t.Fatal("removed shared adjuster")
	}
}

func TestReviewedAdditionalPluginArchives(t *testing.T) {
	dir := os.Getenv("GTASA_PLUGIN_ARCHIVES")
	if dir == "" {
		t.Skip("set GTASA_PLUGIN_ARCHIVES to the downloaded official ZIP directory")
	}
	for _, item := range []struct{ name, archive, plugin string }{
		{WindowedMode, "WindowedMode.zip", "modloader/WindowedMode/III.VC.SA.WindowedMode.asi"},
		{OpenLimitAdjuster, "LimitAdjuster.zip", "scripts/III.VC.SA.LimitAdjuster.asi"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := t.TempDir()
			tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader"}}
			if err := tracker.Save(context.Background(), root, "test"); err != nil {
				t.Fatal(err)
			}
			if err := Import(context.Background(), root, item.name, filepath.Join(dir, item.archive), "test"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.plugin)))
			if err != nil || len(data) < 2 || string(data[:2]) != "MZ" {
				t.Fatal("missing Windows plugin", err)
			}
			if _, err := Uninstall(context.Background(), root, item.name, "test"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(item.plugin))); !os.IsNotExist(err) {
				t.Fatal("plugin remains after uninstall", err)
			}
		})
	}
}
