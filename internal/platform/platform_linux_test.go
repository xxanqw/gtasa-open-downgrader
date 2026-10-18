package platform

import (
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestDetectUsesSteamAppManifestOnLinux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")
	library := filepath.Join(t.TempDir(), "Secondary Library")
	game := filepath.Join(library, "steamapps", "common", "Renamed GTA SA")
	testutil.Write(t, root, "steamapps/libraryfolders.vdf", `"libraryfolders" { "1" { "path" "`+filepath.ToSlash(library)+`" } }`)
	testutil.Write(t, library, "steamapps/appmanifest_12120.acf", `"AppState" { "appid" "12120" "installdir" "Renamed GTA SA" }`)
	testutil.Write(t, game, "gta-sa.exe", "game")
	if got := Detect(); got != game {
		t.Fatalf("detected %q, want game from Steam manifest %q", got, game)
	}
}
