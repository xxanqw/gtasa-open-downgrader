package mods

import (
	"context"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredHDFontsIsNotInstallableButStillRemovable(t *testing.T) {
	if Supported(HDFonts) {
		t.Fatal("retired catalog item selectable")
	}
	root := t.TempDir()
	testutil.Write(t, root, "modloader/HDFonts/models/fonts.txd", "old native fonts")
	testutil.Write(t, root, hdFontStore+"/plain/fonts.dat", "old variants")
	tracker := Tracker{Schema: 2, Installed: []string{HDFonts}, Versions: map[string]Release{HDFonts: {Style: "plain"}}}
	if err := tracker.Save(context.Background(), root, "old"); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), root, HDFonts, testutil.Zip(t, map[string]string{"fonts.txd": "ignored"}), "zip"); err == nil {
		t.Fatal("retired package install accepted")
	}
	result, err := Uninstall(context.Background(), root, HDFonts, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"modloader/HDFonts", hdFontStore} {
		if _, err = os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatal("legacy path remains", path)
		}
	}
	if testutil.Read(t, result.BackupDir, "modloader/HDFonts/models/fonts.txd") != "old native fonts" {
		t.Fatal("lost legacy backup")
	}
}
