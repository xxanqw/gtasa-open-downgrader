//go:build !windows

package steam

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsPathUsesLongestProtonDriveMapping(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	game := filepath.Join(library, "steamapps", "common", "Grand Theft Auto San Andreas")
	if err := os.MkdirAll(game, 0755); err != nil {
		t.Fatal(err)
	}
	dosDevices := filepath.Join(root, "compatdata", "pfx", "dosdevices")
	if err := os.MkdirAll(dosDevices, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(dosDevices, "z:")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(library, filepath.Join(dosDevices, "g:")); err != nil {
		t.Fatal(err)
	}
	proton := ProtonContext{CompatDataPath: filepath.Join(root, "compatdata")}
	got, err := proton.WindowsPath(game)
	if err != nil {
		t.Fatal(err)
	}
	want := `G:\steamapps\common\Grand Theft Auto San Andreas`
	if got != want {
		t.Fatalf("Windows path = %q, want %q", got, want)
	}
}

func TestRegistrySetterRejectsUnrelatedKeys(t *testing.T) {
	if err := (&ProtonContext{}).SetRegistryString(context.Background(), `HKLM\SYSTEM\Unsafe`, "Value", "data"); err == nil {
		t.Fatal("unrelated Registry key accepted")
	}
}

func TestEnsureDriveMappingCreatesVisibleDedicatedDrive(t *testing.T) {
	root, game := t.TempDir(), t.TempDir()
	dosDevices := filepath.Join(root, "pfx", "dosdevices")
	if err := os.MkdirAll(dosDevices, 0755); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(root, "occupied")
	if err := os.MkdirAll(occupied, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(occupied, filepath.Join(dosDevices, "g:")); err != nil {
		t.Fatal(err)
	}
	proton := ProtonContext{CompatDataPath: root}
	got, err := proton.EnsureDriveMapping(game)
	if err != nil {
		t.Fatal(err)
	}
	if got != `H:\` {
		t.Fatalf("drive = %q, want H:\\", got)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(dosDevices, "h:"))
	if err != nil {
		t.Fatal(err)
	}
	game, _ = filepath.Abs(game)
	if resolved != game {
		t.Fatalf("mapping target = %q, want %q", resolved, game)
	}
	again, err := proton.EnsureDriveMapping(game)
	if err != nil || again != got {
		t.Fatalf("idempotent mapping = %q, %v", again, err)
	}
}
