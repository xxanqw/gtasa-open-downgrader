package patch_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
func TestManifestV2SelectsGermanVariantPatch(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "models/gta3.img", "german source")
	globalPatch := testutil.Write(t, patches, "models/gta3.img.xdelta", "global delta")
	germanPatch := testutil.Write(t, patches, "variants/steam_r2_german/models/gta3.img.xdelta", "German delta")
	m := game.Manifest{Version: "2.0", Files: []game.File{{
		Path: "models/gta3.img", Action: "patch", SourceHash: testutil.Hash("global source"), TargetHash: testutil.Hash("target"),
		Sources: []game.PatchSource{{Hash: testutil.Hash("global source"), Patch: "models/gta3.img.xdelta"}, {Hash: testutil.Hash("german source"), Patch: "variants/steam_r2_german/models/gta3.img.xdelta"}},
	}}}
	var selected string
	engine := patch.Engine{Xdelta: "xdelta3", Run: func(_ context.Context, _ string, args ...string) error {
		selected = args[3]
		return os.WriteFile(args[4], []byte("target"), 0644)
	}}
	result, err := engine.Apply(context.Background(), root, patches, m, nil)
	if err != nil || result.Succeeded != 1 {
		t.Fatalf("apply German variant: %+v, %v", result, err)
	}
	if selected != germanPatch || selected == globalPatch {
		t.Fatalf("selected patch %q, want %q", selected, germanPatch)
	}
	if got := testutil.Read(t, root, "models/gta3.img"); got != "target" {
		t.Fatalf("patched content = %q", got)
	}
}
func TestManifestV2CopyUsesDeclaredPayload(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "German executable")
	testutil.Write(t, patches, "payloads/hoodlum.exe", "target")
	m := game.Manifest{Version: "2.0", Files: []game.File{{
		Path: "gta-sa.exe", Action: "copy", SourceHash: testutil.Hash("global executable"), TargetHash: testutil.Hash("target"), Payload: "payloads/hoodlum.exe",
		Sources: []game.PatchSource{{Hash: testutil.Hash("global executable")}, {Hash: testutil.Hash("German executable")}},
	}}}
	if _, err := (patch.Engine{}).Apply(context.Background(), root, patches, m, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gta-sa.exe", "gta_sa.exe"} {
		if got := testutil.Read(t, root, name); got != "target" {
			t.Fatalf("%s = %q", name, got)
		}
	}
}
func TestValidateAssetsRequiresEveryManifestV2Variant(t *testing.T) {
	root := t.TempDir()
	m := game.Manifest{Version: "2.0", Files: []game.File{{
		Path: "models/gta3.img", SourceHash: testutil.Hash("global"), TargetHash: testutil.Hash("target"),
		Sources: []game.PatchSource{{Hash: testutil.Hash("global"), Patch: "models/gta3.img.xdelta"}, {Hash: testutil.Hash("german"), Patch: "variants/steam_r2_german/models/gta3.img.xdelta"}},
	}}}
	b, _ := json.Marshal(m)
	testutil.Write(t, root, "manifest.json", string(b))
	testutil.Write(t, root, "models/gta3.img.xdelta", "global")
	if _, err := patch.ValidateAssets(root); err == nil {
		t.Fatal("accepted assets with missing German variant")
	}
	testutil.Write(t, root, "variants/steam_r2_german/models/gta3.img.xdelta", "german")
	if _, err := patch.ValidateAssets(root); err != nil {
		t.Fatal(err)
	}
}
func TestEditionScopedDowngradeAndRevert(t *testing.T) {
	ctx := context.Background()
	bin, cleanup, err := assets.Xdelta(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	patches := t.TempDir()
	testutil.Write(t, patches, "gta_sa.exe", "target executable")
	for _, tc := range []struct{ name, exe, own, other string }{
		{"steam_r2_global", "global executable", "global.img", "german.img"},
		{"steam_r2_german", "german executable", "german.img", "global.img"},
	} {
		path := filepath.Join("models", tc.own)
		source := testutil.Write(t, t.TempDir(), tc.own, "source "+tc.own)
		target := testutil.Write(t, t.TempDir(), tc.own, "target "+tc.own)
		delta := filepath.Join(patches, path+".xdelta")
		if err := os.MkdirAll(filepath.Dir(delta), 0755); err != nil {
			t.Fatal(err)
		}
		if err := patch.Run(ctx, bin, "-e", "-s", source, target, delta); err != nil {
			t.Fatal(err)
		}
	}
	m := game.Manifest{Version: "2.1", ReleaseID: patch.ReleaseID,
		SourceVersions: []game.SourceVersion{
			{ID: "steam_r2_global", Name: "Global", EXEHash: testutil.Hash("global executable")},
			{ID: "steam_r2_german", Name: "German", EXEHash: testutil.Hash("german executable")},
		},
		Files: []game.File{{Path: "gta-sa.exe", Action: "copy", SourceHash: testutil.Hash("global executable"), TargetHash: testutil.Hash("target executable"), Sources: []game.PatchSource{{Hash: testutil.Hash("global executable")}, {Hash: testutil.Hash("german executable")}}, ApplicableVersions: []string{"steam_r2_global", "steam_r2_german"}}},
	}
	for _, tc := range []struct{ id, name string }{{"steam_r2_global", "global.img"}, {"steam_r2_german", "german.img"}} {
		m.Files = append(m.Files, game.File{Path: "models/" + tc.name, SourceHash: testutil.Hash("source " + tc.name), TargetHash: testutil.Hash("target " + tc.name), Sources: []game.PatchSource{{Hash: testutil.Hash("source " + tc.name), Patch: "models/" + tc.name + ".xdelta"}}, ApplicableVersions: []string{tc.id}})
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, patches, "manifest.json", string(data))
	if _, err := patch.ValidateReleaseAssets(patches); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, exe, own, other string }{
		{"steam_r2_global", "global executable", "global.img", "german.img"},
		{"steam_r2_german", "german executable", "german.img", "global.img"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.Write(t, root, "gta-sa.exe", tc.exe)
			testutil.Write(t, root, "models/"+tc.own, "source "+tc.own)
			loaded, err := game.Load(filepath.Join(patches, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			for run := 0; run < 2; run++ {
				result, err := (patch.Engine{Xdelta: bin}).Apply(ctx, root, patches, loaded, nil)
				if err != nil || result.Failed != 0 {
					t.Fatalf("apply %d: %+v %v", run, result, err)
				}
				if got := testutil.Read(t, root, "models/"+tc.own); got != "target "+tc.own {
					t.Fatalf("patched file: %q", got)
				}
				if _, err := os.Stat(filepath.Join(root, "models", tc.other)); !os.IsNotExist(err) {
					t.Fatalf("wrong edition file created: %v", err)
				}
			}
			if err := patch.Revert(ctx, root, nil); err != nil {
				t.Fatal(err)
			}
			if got := testutil.Read(t, root, "models/"+tc.own); got != "source "+tc.own {
				t.Fatalf("revert: %q", got)
			}
		})
	}
}

func TestEditionUnknownFailsBeforeModifyingGame(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "unrecognized")
	testutil.Write(t, patches, "gta_sa.exe", "target")
	m := game.Manifest{SourceVersions: []game.SourceVersion{{ID: "global", Name: "Global", EXEHash: testutil.Hash("global")}}, Files: []game.File{{Path: "gta-sa.exe", Action: "copy", SourceHash: testutil.Hash("global"), TargetHash: testutil.Hash("target"), ApplicableVersions: []string{"global"}}}}
	if _, err := (patch.Engine{}).Apply(context.Background(), root, patches, m, nil); err == nil {
		t.Fatal("patched installation with unknown source edition")
	}
	if got := testutil.Read(t, root, "gta-sa.exe"); got != "unrecognized" {
		t.Fatalf("game changed: %q", got)
	}
}

func TestReleaseValidationRejectsOldAndIncompleteManifests(t *testing.T) {
	root := t.TempDir()
	m := manifest("gta_sa.exe", "copy", "original", "target")
	testutil.Write(t, root, "gta_sa.exe", "target")
	for _, candidate := range []game.Manifest{m, {Version: "2.0", ReleaseID: patch.ReleaseID, SourceVersions: []game.SourceVersion{{ID: "steam_r2_global", Name: "Global", EXEHash: testutil.Hash("original")}, {ID: "steam_r2_german", Name: "German", EXEHash: testutil.Hash("german")}}, Files: m.Files}, {Version: "2.1", ReleaseID: patch.ReleaseID, SourceVersions: []game.SourceVersion{{ID: "steam_r2_global", Name: "Global", EXEHash: testutil.Hash("original")}, {ID: "steam_r2_german", Name: "German", EXEHash: testutil.Hash("german")}}, Files: m.Files}} {
		b, _ := json.Marshal(candidate)
		testutil.Write(t, root, "manifest.json", string(b))
		if _, err := patch.ValidateReleaseAssets(root); err == nil {
			t.Fatal("accepted non-release or incomplete assets")
		}
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
func TestDowngradeRecoveryPreservesModUninstallSnapshots(t *testing.T) {
	root, patches := t.TempDir(), t.TempDir()
	testutil.Write(t, root, "gta_sa.exe", "original")
	testutil.Write(t, patches, "gta_sa.exe", "target")
	if _, err := (patch.Engine{}).Apply(context.Background(), root, patches, manifest("gta_sa.exe", "copy", "original", "target"), nil); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "backups/mods/SilentPatch-1/modloader/SilentPatch/file.asi", "saved mod")
	if err := patch.Revert(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, "gta_sa.exe"); got != "original" {
		t.Fatalf("restore game executable: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "SilentPatch-1", "modloader")); !os.IsNotExist(err) {
		t.Fatalf("mod backup unexpectedly copied into game: %v", err)
	}
	if err := patch.Cleanup(root); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, "backups/mods/SilentPatch-1/modloader/SilentPatch/file.asi"); got != "saved mod" {
		t.Fatalf("mod backup lost: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "gta_sa.exe")); !os.IsNotExist(err) {
		t.Fatal("downgrade backup survived cleanup")
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
func TestImportReleaseRejectsOldArchiveWithoutLosingCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Patches")
	testutil.Write(t, root, "keep", "existing")
	m := manifest("gta_sa.exe", "copy", "old", "new")
	b, _ := json.Marshal(m)
	archive := testutil.Zip(t, map[string]string{"Patches/manifest.json": string(b), "Patches/gta_sa.exe": "new"})
	if err := patch.ImportReleaseAssets(context.Background(), archive, root); err == nil {
		t.Fatal("accepted a v1 release archive")
	}
	if got := testutil.Read(t, root, "keep"); got != "existing" {
		t.Fatalf("cache replaced: %q", got)
	}
}

func TestPatchArchiveValidationAndRollback(t *testing.T) {
	m := manifest("gta_sa.exe", "copy", "original", "target")
	b, _ := json.Marshal(m)
	for _, prefix := range []string{"", "Patches/", "Patches_v2/"} {
		name := "flat"
		if prefix != "" {
			name = strings.TrimSuffix(prefix, "/")
		}
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "Patches")
			testutil.Write(t, root, "old", "old")
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
