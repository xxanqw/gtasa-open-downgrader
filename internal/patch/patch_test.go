package patch_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func manifest(path, action, source, target string) game.Manifest {
	return game.Manifest{Files: []game.File{{Path: path, Action: action, SourceHash: testutil.Hash(source), TargetHash: testutil.Hash(target)}}}
}
func TestRealXdeltaRoundTripAndRevert(t *testing.T) {
	ctx := context.Background()
	bin, cleanup, e := assets.Xdelta(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	root, patches := t.TempDir(), t.TempDir()
	source := testutil.Write(t, root, "models/data", "original game data")
	target := testutil.Write(t, t.TempDir(), "target", "downgraded game data")
	delta := filepath.Join(patches, "models", "data.xdelta")
	os.MkdirAll(filepath.Dir(delta), 0755)
	if e = patch.Run(ctx, bin, "-e", "-s", source, target, delta); e != nil {
		t.Fatal(e)
	}
	m := manifest("models/data", "patch", "original game data", "downgraded game data")
	engine := patch.Engine{Xdelta: bin}
	r, e := engine.Apply(ctx, root, patches, m, nil)
	if e != nil || r.Succeeded != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	if testutil.Read(t, root, "models/data") != "downgraded game data" {
		t.Fatal("patch not applied")
	}
	if _, e = engine.Apply(ctx, root, patches, m, nil); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "backups/models/data") != "original game data" {
		t.Fatal("original backup was replaced")
	}
	if e = patch.Revert(ctx, root, nil); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "models/data") != "original game data" {
		t.Fatal("revert failed")
	}
}
func TestCopyAliasBackupsAndRevert(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "original")
	testutil.Write(t, patches, "gta_sa.exe", "target")
	m := manifest("gta_sa.exe", "copy", "original", "target")
	if _, e := (patch.Engine{}).Apply(context.Background(), root, patches, m, nil); e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"gta_sa.exe", "gta-sa.exe"} {
		if testutil.Read(t, root, n) != "target" {
			t.Fatal(n)
		}
	}
	if e := patch.Revert(context.Background(), root, nil); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "gta-sa.exe") != "original" {
		t.Fatal("wrong original name")
	}
	if _, e := os.Stat(filepath.Join(root, "gta_sa.exe")); !os.IsNotExist(e) {
		t.Fatal("new alias survived revert")
	}
}
func TestManagedLauncherIsNotPatched(t *testing.T) {
	root, patches, launcherDir := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "original")
	testutil.Write(t, root, entrypoint.SteamExecutable, "Steam original")
	launcher := testutil.Write(t, launcherDir, entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, patches, entrypoint.RealGameExecutable, "target")
	m := manifest(entrypoint.SteamExecutable, "copy", "original", "target")
	if _, err := (patch.Engine{}).Apply(context.Background(), root, patches, m, nil); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, entrypoint.SteamExecutable); got != "launcher" {
		t.Fatalf("launcher was overwritten: %q", got)
	}
	if got := testutil.Read(t, root, entrypoint.RealGameExecutable); got != "target" {
		t.Fatalf("real game = %q", got)
	}
}
func TestRevertDoesNotOverwriteManagedLauncher(t *testing.T) {
	root, patches, launcherDir := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "original")
	testutil.Write(t, root, entrypoint.SteamExecutable, "Steam original")
	testutil.Write(t, patches, entrypoint.RealGameExecutable, "target")
	if _, err := (patch.Engine{}).Apply(context.Background(), root, patches, manifest(entrypoint.RealGameExecutable, "copy", "original", "target"), nil); err != nil {
		t.Fatal(err)
	}
	launcher := testutil.Write(t, launcherDir, entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	if err := patch.Revert(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, entrypoint.SteamExecutable); got != "launcher" {
		t.Fatalf("revert overwrote launcher: %q", got)
	}
	if got := testutil.Read(t, root, entrypoint.RealGameExecutable); got != "original" {
		t.Fatalf("real game was not restored: %q", got)
	}
}
func TestBothExecutableOriginalsPreserved(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "original dash")
	testutil.Write(t, root, "gta_sa.exe", "original underscore")
	testutil.Write(t, patches, "gta_sa.exe", "target")
	m := manifest("gta_sa.exe", "copy", "original underscore", "target")
	if _, e := (patch.Engine{}).Apply(context.Background(), root, patches, m, nil); e != nil {
		t.Fatal(e)
	}
	if e := patch.Revert(context.Background(), root, nil); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "gta-sa.exe") != "original dash" || testutil.Read(t, root, "gta_sa.exe") != "original underscore" {
		t.Fatal("aliases lost distinct original content")
	}
}
func TestBadOutputDoesNotReplaceOriginal(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta_sa.exe", "original")
	testutil.Write(t, patches, "gta_sa.exe", "corrupt")
	r, e := (patch.Engine{}).Apply(context.Background(), root, patches, manifest("gta_sa.exe", "copy", "original", "target"), nil)
	if e == nil || r.Failed != 1 {
		t.Fatal("bad hash accepted")
	}
	if testutil.Read(t, root, "gta_sa.exe") != "original" {
		t.Fatal("overwrote original")
	}
}
func TestCancelledXdeltaLeavesOriginalAndNoTemp(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "data", "original")
	ctx, cancel := context.WithCancel(context.Background())
	engine := patch.Engine{Run: func(ctx context.Context, bin string, args ...string) error {
		os.WriteFile(args[len(args)-1], []byte("partial"), 0644)
		cancel()
		return ctx.Err()
	}}
	_, e := engine.Apply(ctx, root, t.TempDir(), manifest("data", "patch", "original", "target"), nil)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "data") != "original" {
		t.Fatal("cancelled patch overwrote original")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary output leaked")
	}
}
func TestRestorePythonBackups(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "data", "patched")
	testutil.Write(t, root, "backups/data", "original")
	if e := patch.Revert(context.Background(), root, nil); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "data") != "original" {
		t.Fatal("legacy restore failed")
	}
}
func TestPatchArchiveValidationAndRollback(t *testing.T) {
	m := manifest("gta_sa.exe", "copy", "original", "target")
	b, _ := json.Marshal(m)
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "flat", true: "nested"}[nested], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "Patches")
			testutil.Write(t, root, "old", "old")
			prefix := ""
			if nested {
				prefix = "Patches/"
			}
			archive := testutil.Zip(t, map[string]string{prefix + "manifest.json": string(b), prefix + "gta_sa.exe": "target"})
			if e := patch.ImportAssets(context.Background(), archive, root); e != nil {
				t.Fatal(e)
			}
			if _, e := patch.ValidateAssets(root); e != nil {
				t.Fatal(e)
			}
			if _, e := os.Stat(filepath.Join(root, "old")); !os.IsNotExist(e) {
				t.Fatal("obsolete file retained")
			}
		})
	}
	root := t.TempDir()
	testutil.Write(t, root, "manifest.json", "original")
	for _, files := range []map[string]string{{"manifest.json": string(b), "../out": "bad"}, {"manifest.json": string(b)}, {"manifest.json": "{}"}} {
		archive := testutil.Zip(t, files)
		if e := patch.ImportAssets(context.Background(), archive, root); e == nil {
			t.Fatal("bad archive accepted")
		}
		if testutil.Read(t, root, "manifest.json") != "original" {
			t.Fatal("old patches were lost")
		}
	}
}
