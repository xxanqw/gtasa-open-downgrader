package game

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestScanStatusesAndExecutableAlias(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "original")
	testutil.Write(t, root, "models/done", "target")
	testutil.Write(t, root, "models/custom", "changed")
	m := Manifest{Files: []File{{Path: "gta_sa.exe", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "models/done", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "models/custom", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "missing", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}}}
	r, e := Inspect(context.Background(), root, m, nil)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"Original (Needs Patch)", "Already Downgraded", "Modified", "Missing"}
	for i, row := range r.Rows {
		if row.Status != want[i] {
			t.Errorf("row %d: %s", i, row.Status)
		}
	}
	if r.ReadOnly {
		t.Fatal("temporary game should be writable")
	}
}
func TestScanUsesRealGameAndIgnoresManagedLauncher(t *testing.T) {
	root, launcherDir := t.TempDir(), t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "real game")
	testutil.Write(t, root, entrypoint.SteamExecutable, "original Steam file")
	launcher := testutil.Write(t, launcherDir, entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Files: []File{{Path: entrypoint.SteamExecutable, SourceHash: testutil.Hash("real game"), TargetHash: testutil.Hash("target")}}}
	result, err := Inspect(context.Background(), root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Rows[0].CurrentHash; got != testutil.Hash("real game") {
		t.Fatalf("scanned %q, want real game hash %q", got, testutil.Hash("real game"))
	}
}
func TestVersionDetectionPrioritizesSteamExecutable(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, entrypoint.SteamExecutable, "German Steam")
	testutil.Write(t, root, entrypoint.RealGameExecutable, "orphan downgraded")
	steamHash, orphanHash := testutil.Hash("German Steam"), testutil.Hash("orphan downgraded")
	oldSteam, hadSteam := Versions[steamHash]
	oldOrphan, hadOrphan := Versions[orphanHash]
	Versions[steamHash] = "NewSteam R2 (German)"
	Versions[orphanHash] = "v1.0 US (Clean)"
	defer func() {
		if hadSteam {
			Versions[steamHash] = oldSteam
		} else {
			delete(Versions, steamHash)
		}
		if hadOrphan {
			Versions[orphanHash] = oldOrphan
		} else {
			delete(Versions, orphanHash)
		}
	}()
	result, err := Inspect(context.Background(), root, Manifest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "NewSteam R2 (German)" {
		t.Fatalf("detected %q", result.Version)
	}
}
func TestPublishedGermanSteamHashesAreKnown(t *testing.T) {
	for _, hash := range []string{"d9cb35c898d3298ca904a63e10ee18d7", "d9cb35c898d3298ca904a63e10ee18d9"} {
		if got := Versions[hash]; got != "NewSteam R2 (German)" {
			t.Fatalf("German hash %s = %q", hash, got)
		}
	}
}
func TestManifestRejectsInvalidInputs(t *testing.T) {
	for _, f := range []File{{Path: "../bad", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}, {Path: "a", SourceHash: "wrong", TargetHash: testutil.Hash("b")}, {Path: "backups/a", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}, {Path: "a", Action: "delete", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}} {
		m := Manifest{Files: []File{f}}
		if e := m.Validate(); e == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
	if e := (&Manifest{}).Validate(); e == nil {
		t.Fatal("accepted empty manifest")
	}
}
func TestManifestAcceptsGeneratorMissingCopy(t *testing.T) {
	m := Manifest{Files: []File{{Path: "gta_sa.exe", Action: "copy", SourceHash: "MISSING", TargetHash: testutil.Hash("target")}}}
	if e := m.Validate(); e != nil {
		t.Fatal(e)
	}
	if m.Files[0].Payload != "gta_sa.exe" {
		t.Fatalf("default payload = %q", m.Files[0].Payload)
	}
}
func TestManifestV2RecognizesGermanSourceAndNormalizesAssets(t *testing.T) {
	global, german, target := testutil.Hash("global"), testutil.Hash("german"), testutil.Hash("target")
	m := Manifest{
		Version:        "2.0",
		SourceVersions: []SourceVersion{{ID: "steam_r2_german", Name: "NewSteam R2 (German)", EXEHash: strings.ToUpper(german)}},
		Files: []File{{
			Path: "models/gta3.img", Action: "patch", SourceHash: global, TargetHash: target,
			Sources: []PatchSource{{Hash: global, Patch: "models/gta3.img.xdelta"}, {Hash: strings.ToUpper(german), Patch: `variants\steam_r2_german\models\gta3.img.xdelta`}},
		}},
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	f := m.Files[0]
	got := f.PatchAsset(german)
	if !f.MatchesSource(german) || got != "variants/steam_r2_german/models/gta3.img.xdelta" {
		t.Fatalf("German source was not selected: matches=%t patch=%q", f.MatchesSource(german), got)
	}
	if got := f.PatchAsset(testutil.Hash("modified")); got != "models/gta3.img.xdelta" {
		t.Fatalf("force fallback patch = %q", got)
	}
}
func TestScanUsesAllManifestV2SourceHashes(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "german exe")
	testutil.Write(t, root, "models/global.img", "global")
	testutil.Write(t, root, "models/german.img", "german")
	testutil.Write(t, root, "models/modified.img", "modified")
	target := testutil.Hash("target")
	m := Manifest{Files: []File{
		{Path: "models/global.img", SourceHash: testutil.Hash("global"), TargetHash: target, Sources: []PatchSource{{Hash: testutil.Hash("global"), Patch: "global.xdelta"}}},
		{Path: "models/german.img", SourceHash: testutil.Hash("global other"), TargetHash: target, Sources: []PatchSource{{Hash: testutil.Hash("global other"), Patch: "global-other.xdelta"}, {Hash: testutil.Hash("german"), Patch: "german.xdelta"}}},
		{Path: "models/modified.img", SourceHash: testutil.Hash("original"), TargetHash: target, Sources: []PatchSource{{Hash: testutil.Hash("original"), Patch: "original.xdelta"}}},
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(context.Background(), root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Original (Needs Patch)", "Original (Needs Patch)", "Modified"}
	for index, row := range result.Rows {
		if row.Status != want[index] {
			t.Errorf("row %d status = %q, want %q", index, row.Status, want[index])
		}
	}
}
func TestManifestV2RejectsUnsafeOrAmbiguousSources(t *testing.T) {
	hash, target := testutil.Hash("source"), testutil.Hash("target")
	files := []File{
		{Path: "data.bin", SourceHash: hash, TargetHash: target, Sources: []PatchSource{{Hash: hash, Patch: "../outside.xdelta"}}},
		{Path: "data.bin", SourceHash: hash, TargetHash: target, Sources: []PatchSource{{Hash: hash, Patch: "one.xdelta"}, {Hash: hash, Patch: "two.xdelta"}}},
		{Path: "gta_sa.exe", Action: "copy", SourceHash: hash, TargetHash: target, Payload: "../outside.exe"},
	}
	for _, file := range files {
		if err := (&Manifest{Version: "2.0", Files: []File{file}}).Validate(); err == nil {
			t.Fatalf("accepted unsafe manifest file: %+v", file)
		}
	}
}
func TestManifestRejectsUnmappedPrimaryDeltaAndUnknownEdition(t *testing.T) {
	global, german, target := testutil.Hash("global"), testutil.Hash("german"), testutil.Hash("target")
	m := Manifest{Version: "2.0", SourceVersions: []SourceVersion{{ID: "german", Name: "German", EXEHash: german}}, Files: []File{{Path: "models/data", SourceHash: global, TargetHash: target, Sources: []PatchSource{{Hash: german, Patch: "german.xdelta"}}}}}
	if err := m.Validate(); err == nil {
		t.Fatal("unmapped primary hash accepted")
	}
	m.Files[0].SourceHash = target // Already-target files do not need a Global delta.
	m.Files[0].ApplicableVersions = []string{"missing"}
	if err := m.Validate(); err == nil {
		t.Fatal("unknown source edition accepted")
	}
	m.Files[0].ApplicableVersions = []string{"german"}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestScanMarksEditionOnlyFilesNotApplicable(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "global exe")
	m := Manifest{SourceVersions: []SourceVersion{{ID: "global", Name: "Global", EXEHash: testutil.Hash("global exe")}, {ID: "german", Name: "German", EXEHash: testutil.Hash("german exe")}}, Files: []File{{Path: "models/only-german.img", SourceHash: testutil.Hash("german file"), TargetHash: testutil.Hash("target"), ApplicableVersions: []string{"german"}}}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(context.Background(), root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Rows[0].Status; got != "Not in this edition" {
		t.Fatalf("missing edition-only file marked %q", got)
	}
}

func TestScanCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	testutil.Write(t, root, "gta_sa.exe", "x")
	_, e := Inspect(ctx, root, Manifest{}, nil)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestUnreadableManifestIsReported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "manifest.json")
	os.WriteFile(p, []byte("invalid"), 0600)
	if _, e := Load(p); e == nil {
		t.Fatal("invalid manifest accepted")
	}
}
