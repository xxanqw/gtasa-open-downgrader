//go:build linux

package download

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDownloadsDirectoryUsesXDGWithoutExecutingShell(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	for _, tc := range []struct{ value, want string }{
		{"$HOME/Завантаження", filepath.Join(home, "Завантаження")},
		{filepath.Join(home, "Custom Downloads"), filepath.Join(home, "Custom Downloads")},
		{"$(unsafe-command)", filepath.Join(home, "Downloads")},
	} {
		if err := os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte("XDG_DOWNLOAD_DIR="+strconv.Quote(tc.value)+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := DownloadsDirectory()
		if err != nil || got != tc.want {
			t.Fatalf("download directory=%q,%v; want %q", got, err, tc.want)
		}
	}
}
