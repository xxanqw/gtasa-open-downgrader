package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func TestMultiplayerDialogKeepsClientActionsCompact(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Resize(fyne.NewSize(720, 520))
	w.window.Show()
	longRoot := filepath.Join("/home/player/.local/share/Steam/steamapps/compatdata/12120/pfx/drive_c", strings.Repeat("client/", 8))
	clients := []multiplayer.Client{
		{ID: multiplayer.ClientSAMP, Name: "SA-MP", Installed: true, Executable: filepath.Join(longRoot, "samp.exe"), WorkingDir: longRoot},
		{ID: multiplayer.ClientOpenMP, Name: "open.mp", Installed: true, Executable: filepath.Join(longRoot, "omp-launcher.exe"), WorkingDir: longRoot},
		{ID: multiplayer.ClientMTA, Name: "Multi Theft Auto", Installed: true, Executable: filepath.Join(longRoot, "Multi Theft Auto.exe"), WorkingDir: longRoot},
	}
	w.showMultiplayer(t.TempDir(), entrypoint.Status{State: entrypoint.Installed}, multiplayer.Environment{GameDir: t.TempDir()}, multiplayer.DirectRunner{}, "Proton Experimental", nil, clients)
	if !w.promptActive || w.activePrompt == nil {
		t.Fatal("multiplayer dialog did not open")
	}
	snapshot(t, w, "GTASA_MULTIPLAYER_SNAPSHOT")
	w.activePrompt()
}
