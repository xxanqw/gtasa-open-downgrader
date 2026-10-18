package mods

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUninstallPlanRequiresEntireDependentSelection(t *testing.T) {
	tracker := Tracker{Installed: []string{"ASI Loader", "ModLoader", "SilentPatch", OriginalHQIcons, AtmosphereHUD}}
	for _, selection := range [][]string{{"ASI Loader"}, {"ModLoader"}, {OriginalHQIcons}, {"ModLoader", "SilentPatch"}} {
		if _, err := UninstallPlan(tracker, selection); err == nil {
			t.Fatal("accepted incomplete dependency removal", selection)
		}
	}
	order, err := UninstallPlan(tracker, tracker.Installed)
	if err != nil {
		t.Fatal(err)
	}
	index := map[string]int{}
	for i, name := range order {
		index[name] = i
	}
	if index[AtmosphereHUD] >= index[OriginalHQIcons] || index[OriginalHQIcons] >= index["ModLoader"] || index["SilentPatch"] >= index["ModLoader"] || index["ModLoader"] >= index["ASI Loader"] {
		t.Fatal("unsafe order", order)
	}
	if UninstallBlocker(tracker, "ModLoader", tracker.Installed) != "" {
		t.Fatal("complete batch still blocks loader")
	}
}
func TestBatchRemovalBacksUpAndPreservesUnselectedFiles(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch"}}
	if err := tracker.Save(ctx, root, "test"); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"vorbisFile.dll": "ASI", "modloader.asi": "loader", "modloader/SilentPatch/SilentPatchSA.asi": "patch", "modloader/SilentPatch/custom.ini": "user settings", "unrelated.txt": "keep"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := UninstallSelected(ctx, root, []string{"ModLoader"}, "test"); err == nil {
		t.Fatal("partial loader removal accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "modloader.asi")); err != nil {
		t.Fatal("preflight refusal moved files")
	}
	done, err := UninstallSelected(ctx, root, tracker.Installed, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 3 {
		t.Fatal(done)
	}
	b, err := os.ReadFile(filepath.Join(done["SilentPatch"].BackupDir, "modloader", "SilentPatch", "custom.ini"))
	if err != nil || string(b) != "user settings" {
		t.Fatal("custom files not backed up", err)
	}
	after, err := LoadTracker(root)
	if err != nil || len(after.Installed) != 0 {
		t.Fatal(after, err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "unrelated.txt")); err != nil || string(b) != "keep" {
		t.Fatal("unrelated file changed")
	}
}
func TestCancelledBatchDoesNotModifyGame(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"SilentPatch"}}
	tracker.Save(context.Background(), root, "test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if done, err := UninstallSelected(ctx, root, tracker.Installed, "test"); err == nil || len(done) != 0 {
		t.Fatal(done, err)
	}
	after, err := LoadTracker(root)
	if err != nil || !reflect.DeepEqual(after.Installed, tracker.Installed) {
		t.Fatal("cancelled batch modified tracking")
	}
}
