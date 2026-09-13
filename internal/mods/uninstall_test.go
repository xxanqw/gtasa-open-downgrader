package mods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestUninstallIsolatedModBacksUpFolderAndUpdatesTracker(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch"}, Versions: map[string]Release{"SilentPatch": {Version: "1.0"}}}
	if err := tracker.Save(context.Background(), root, "old"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "modloader/SilentPatch/SilentPatchSA.asi", "mod")
	testutil.Write(t, root, "modloader/SilentPatch/local.ini", "personal")
	testutil.Write(t, root, "modloader/OtherMod/untouched.asi", "other")
	result, err := Uninstall(context.Background(), root, "SilentPatch", "new")
	if err != nil || result.Moved != 1 || result.BackupDir == "" {
		t.Fatalf("uninstall: %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "modloader/SilentPatch")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mod folder remains: %v", err)
	}
	if testutil.Read(t, result.BackupDir, "modloader/SilentPatch/local.ini") != "personal" || testutil.Read(t, root, "modloader/OtherMod/untouched.asi") != "other" {
		t.Fatal("uninstall lost personal files or touched another mod")
	}
	after, err := LoadTracker(root)
	if err != nil || after.Has("SilentPatch") || after.Versions["SilentPatch"].Version != "" || !after.Has("ModLoader") {
		t.Fatalf("tracker still records uninstalled mod: %+v, %v", after, err)
	}
}

func TestUninstallProtectsLoaderDependencies(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch"}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "vorbisFile.dll", "loader")
	testutil.Write(t, root, "modloader.asi", "loader")
	for _, name := range []string{"ASI Loader", "ModLoader"} {
		if _, err := Uninstall(context.Background(), root, name, "app"); err == nil || !strings.Contains(err.Error(), "remove the other installed mods first") {
			t.Fatalf("dependency was not protected for %s: %v", name, err)
		}
	}
	if testutil.Read(t, root, "vorbisFile.dll") != "loader" || testutil.Read(t, root, "modloader.asi") != "loader" {
		t.Fatal("dependent loader files changed")
	}
}

func TestUninstallLoaderRetainsSharedConfig(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader"}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "modloader.asi", "loader")
	testutil.Write(t, root, "modloader/config.ini", "personal")
	result, err := Uninstall(context.Background(), root, "ModLoader", "app")
	if err != nil || result.Moved != 1 {
		t.Fatalf("uninstall loader: %+v, %v", result, err)
	}
	if testutil.Read(t, root, "modloader/config.ini") != "personal" || testutil.Read(t, result.BackupDir, "modloader.asi") != "loader" {
		t.Fatal("shared config was removed or loader binary not backed up")
	}
}

func TestLegacySanLtdNameMigratesWithoutChangingInstallPath(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, TrackerPath, `{"schema_version":2,"installed_mods":["SALtd Russian"],"mod_versions":{"SALtd Russian":{"version":"0.56","fingerprint":"old"}}}`)
	testutil.Write(t, root, "modloader/SALtdRussian/text/american.gxt", "translated")
	tracker, err := LoadTracker(root)
	if err != nil || !tracker.Has(SanLtdRussian) || tracker.Versions[SanLtdRussian].Version != "0.56" {
		t.Fatalf("old tracker did not load as SanLtd: %+v, %v", tracker, err)
	}
	result, err := Uninstall(context.Background(), root, SanLtdRussian, "app")
	if err != nil || testutil.Read(t, result.BackupDir, "modloader/SALtdRussian/text/american.gxt") != "translated" {
		t.Fatalf("old folder was not uninstalled: %+v, %v", result, err)
	}
	after, err := LoadTracker(root)
	if err != nil || after.Has(SanLtdRussian) || strings.Contains(testutil.Read(t, root, TrackerPath), "SALtd Russian") {
		t.Fatalf("old tracker name remained: %+v, %v", after, err)
	}
}

func TestCancelledUninstallPreservesModAndTracker(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"SilentPatch"}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "modloader/SilentPatch/SilentPatchSA.asi", "mod")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Uninstall(ctx, root, "SilentPatch", "app"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	after, err := LoadTracker(root)
	if err != nil || !after.Has("SilentPatch") || testutil.Read(t, root, "modloader/SilentPatch/SilentPatchSA.asi") != "mod" {
		t.Fatal("cancelled uninstall changed installed mod")
	}
}
