package platform

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestLAAValidatesPEAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	p := testutil.Write(t, root, "bad.exe", "not a PE")
	if e := ApplyLAA(context.Background(), p); e == nil {
		t.Fatal("bad PE accepted")
	}
	b := make([]byte, 128)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[60:64], 64)
	copy(b[64:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[86:88], 0x102)
	p = filepath.Join(root, "good.exe")
	os.WriteFile(p, b, 0755)
	if e := ApplyLAA(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(p)
	if binary.LittleEndian.Uint16(got[86:88]) != 0x122 {
		t.Fatal("wrong PE flags")
	}
	if e := ApplyLAA(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	again, _ := os.ReadFile(p)
	if !reflect.DeepEqual(got, again) {
		t.Fatal("not idempotent")
	}
}
func TestSteamGameDiscoveryFollowsAppManifest(t *testing.T) {
	root := t.TempDir()
	extra := t.TempDir()
	testutil.Write(t, root, "steamapps/libraryfolders.vdf", `"libraryfolders" { "1" { "path" "`+filepath.ToSlash(extra)+`" } }`)
	game := filepath.Join(extra, "steamapps", "common", "My Custom GTA Folder")
	testutil.Write(t, game, "gta-sa.exe", "game")
	testutil.Write(t, extra, "steamapps/appmanifest_12120.acf", `"AppState" { "appid" "12120" "installdir" "My Custom GTA Folder" }`)
	if paths := SteamGameDirs(root); len(paths) != 1 || paths[0] != game {
		t.Fatalf("appmanifest discovery: %q, want %q", paths, game)
	}
	// An executable in the old default path is not evidence of a Steam install.
	orphan := t.TempDir()
	testutil.Write(t, orphan, "steamapps/common/Grand Theft Auto San Andreas/gta-sa.exe", "game")
	if paths := SteamGameDirs(orphan); len(paths) != 0 {
		t.Fatalf("guessed Steam game directory without metadata: %q", paths)
	}
}
