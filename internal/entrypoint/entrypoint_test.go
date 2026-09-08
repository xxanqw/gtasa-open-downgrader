package entrypoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallInspectAndRemoveRestoresExistingEntrypoint(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(t.TempDir(), "gta-sa.exe")
	put(t, filepath.Join(root, RealGameExecutable), "real game")
	put(t, filepath.Join(root, SteamExecutable), "steam original")
	put(t, launcher, "managed launcher")

	status, err := Install(context.Background(), root, launcher)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != Installed {
		t.Fatalf("state = %q", status.State)
	}
	if status, err = Install(context.Background(), root, launcher); err != nil || status.State != Installed {
		t.Fatalf("idempotent install = %#v, %v", status, err)
	}
	if err = Remove(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, SteamExecutable))
	if err != nil || string(data) != "steam original" {
		t.Fatalf("restored entrypoint = %q, %v", data, err)
	}
	if err = Remove(context.Background(), root); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
}

func TestInstallRequiresRealGame(t *testing.T) {
	launcher := filepath.Join(t.TempDir(), "launcher.exe")
	put(t, launcher, "launcher")
	if _, err := Install(context.Background(), t.TempDir(), launcher); err == nil {
		t.Fatal("Install succeeded without gta_sa.exe")
	}
}

func TestInstallRejectsLauncherAsRealGame(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(t.TempDir(), "launcher.exe")
	put(t, filepath.Join(root, RealGameExecutable), "same launcher")
	put(t, launcher, "same launcher")
	if _, err := Install(context.Background(), root, launcher); err == nil {
		t.Fatal("launcher accepted as real game")
	}
}

func TestInspectDetectsSteamReplacementAndRemoveRefusesIt(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(t.TempDir(), "launcher.exe")
	put(t, filepath.Join(root, RealGameExecutable), "real")
	put(t, launcher, "launcher")
	if _, err := Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, SteamExecutable), "restored by Steam")
	if got := Inspect(context.Background(), root); got.State != Replaced {
		t.Fatalf("state = %q (%s)", got.State, got.Detail)
	}
	if err := Remove(context.Background(), root); err == nil {
		t.Fatal("Remove overwrote an unrecognized entrypoint")
	}
}

func TestInstallAfterSteamReplacementPreservesFirstBackup(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(t.TempDir(), "launcher.exe")
	put(t, filepath.Join(root, RealGameExecutable), "real")
	put(t, filepath.Join(root, SteamExecutable), "first original")
	put(t, launcher, "launcher")
	if _, err := Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, SteamExecutable), "Steam verification")
	if _, err := Install(context.Background(), root, launcher); err != nil {
		t.Fatal(err)
	}
	if err := Remove(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, SteamExecutable))
	if string(data) != "first original" {
		t.Fatalf("backup was replaced: %q", data)
	}
}

func TestInstallDoesNotOverwriteUntrackedBackup(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(t.TempDir(), "launcher.exe")
	put(t, filepath.Join(root, RealGameExecutable), "real")
	put(t, filepath.Join(root, SteamExecutable), "Steam original")
	put(t, filepath.Join(root, backupPath), "untracked backup")
	put(t, launcher, "launcher")
	if _, err := Install(context.Background(), root, launcher); err == nil {
		t.Fatal("untracked backup was overwritten")
	}
	data, _ := os.ReadFile(filepath.Join(root, backupPath))
	if string(data) != "untracked backup" {
		t.Fatalf("backup changed: %q", data)
	}
}

func TestLoadMigratesMissingSchemaAndNames(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, MetadataPath), `{"launcher_sha256":"`+string(make([]byte, 64))+`"}`)
	if _, err := Load(root); err == nil {
		t.Fatal("invalid digest accepted")
	}
	put(t, filepath.Join(root, MetadataPath), `{"launcher_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	config, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.SchemaVersion != SchemaVersion || config.LauncherExecutable != SteamExecutable || config.GameExecutable != RealGameExecutable {
		t.Fatalf("not migrated: %#v", config)
	}
}

func TestRejectsUnsafeMetadata(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, MetadataPath), `{"schema_version":1,"launcher_executable":"../gta-sa.exe","game_executable":"gta_sa.exe","launcher_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	if _, err := Load(root); err == nil {
		t.Fatal("unsafe metadata accepted")
	}
}
