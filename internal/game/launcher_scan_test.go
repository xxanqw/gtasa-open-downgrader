package game

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func editionManifest() Manifest {
	versions := []string{"global", "german"}
	return Manifest{Version: "2.1", SourceVersions: []SourceVersion{{ID: "global", Name: "Global", EXEHash: testutil.Hash("Steam original")}, {ID: "german", Name: "German", EXEHash: testutil.Hash("German original")}}, Files: []File{
		{Path: entrypoint.SteamExecutable, Action: "copy", SourceHash: testutil.Hash("Steam original"), TargetHash: testutil.Hash("target exe"), ApplicableVersions: versions},
		{Path: "models/data.img", SourceHash: testutil.Hash("original data"), TargetHash: testutil.Hash("target data"), ApplicableVersions: versions},
	}}
}
func TestLauncherInstallationDoesNotChangeVerifiedGameRowsOrSourceEdition(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	m := editionManifest()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, entrypoint.SteamExecutable, "target exe")
	testutil.Write(t, root, entrypoint.RealGameExecutable, "target exe")
	testutil.Write(t, root, "backups/gta-sa.exe", "Steam original")
	testutil.Write(t, root, "models/data.img", "target data")
	before, err := Inspect(ctx, root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	launcher := testutil.Write(t, t.TempDir(), entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(ctx, root, launcher); err != nil {
		t.Fatal(err)
	}
	edition, err := m.SourceEdition(ctx, root)
	if err != nil || edition != "global" {
		t.Fatalf("installing launcher lost original edition: %q, %v", edition, err)
	}
	after, err := Inspect(ctx, root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range after.Rows {
		if row != before.Rows[i] || row.Status != "Already Downgraded" {
			t.Fatalf("row changed after installing launcher: before=%+v, after=%+v", before.Rows[i], row)
		}
	}
}
func TestExactTargetsRemainVerifiedWithoutOriginalEditionEvidence(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "target exe")
	testutil.Write(t, root, "models/data.img", "target data")
	launcher := testutil.Write(t, t.TempDir(), entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	m := editionManifest()
	m.Validate()
	scan, err := Inspect(context.Background(), root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range scan.Rows {
		if row.Status != "Already Downgraded" || row.NeedsPatch != "No" {
			t.Fatal("exact verified target incorrectly needs original edition", row)
		}
	}
	testutil.Write(t, root, "models/data.img", "customized data")
	scan, err = Inspect(context.Background(), root, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Rows[1].Status != "Source edition unknown" || scan.Rows[1].NeedsPatch != "N/A" {
		t.Fatal("unknown-edition modified data was authorized", scan.Rows[1])
	}
}
func TestLauncherBytesAreNeverUsedAsOriginalEditionEvidence(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "unknown real game")
	launcher := testutil.Write(t, t.TempDir(), entrypoint.SteamExecutable, "Steam original")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	m := editionManifest()
	edition, err := m.SourceEdition(context.Background(), root)
	if err != nil || edition != "" {
		t.Fatal("reserved live launcher was mistaken for original GTA", edition, err)
	}
}
func TestManagedBackupCanIdentifySourceEditionButChangedBackupCannot(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "target exe")
	testutil.Write(t, root, entrypoint.SteamExecutable, "German original")
	launcher := testutil.Write(t, t.TempDir(), entrypoint.SteamExecutable, "launcher")
	if _, err := entrypoint.Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	m := editionManifest()
	edition, err := m.SourceEdition(context.Background(), root)
	if err != nil || edition != "german" {
		t.Fatal("verified managed backup was ignored", edition, err)
	}
	config, err := entrypoint.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, config.BackupExecutable, "Steam original")
	edition, err = m.SourceEdition(context.Background(), root)
	if err != nil || edition != "" {
		t.Fatal("changed managed backup was trusted", edition, err)
	}
}
func TestInstalledSourceEditionReadOnly(t *testing.T) {
	root, manifest := os.Getenv("GTA_READONLY_ROOT"), os.Getenv("GTA_READONLY_MANIFEST")
	if root == "" || manifest == "" {
		t.Skip("opt-in read-only edition diagnostic")
	}
	m, err := Load(manifest)
	if err != nil {
		t.Fatal(err)
	}
	edition, err := m.SourceEdition(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(fmt.Sprintf("source edition=%q; launcher metadata present=%t", edition, entrypoint.MetadataPresent(root)))
	if edition == "" {
		t.Fatal("could not identify installed source edition")
	}
}
