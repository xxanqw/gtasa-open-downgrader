package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
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

func TestSAMPInstallerGuideExplainsMappedDriveBeforeStarting(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Show()

	started := false
	w.showSAMPInstallerGuide(multiplayer.Environment{WindowsGameDir: `G:\Grand Theft Auto San Andreas`}, func() {
		started = true
	})
	if started {
		t.Fatal("SA-MP installer started before the guide was accepted")
	}
	for _, want := range []string{"mounted inside the Windows installer as drive G:", `G:\Grand Theft Auto San Andreas`, "expand My Computer"} {
		found := false
		walkObjects(w.window.Canvas().Overlays().Top(), func(o fyne.CanvasObject) bool {
			if label, ok := o.(*widget.Label); ok && strings.Contains(label.Text, want) {
				found = true
				return true
			}
			return false
		})
		if !found {
			t.Fatalf("SA-MP guide does not explain %q", want)
		}
	}
	snapshot(t, w, "GTASA_SAMP_GUIDE_SNAPSHOT")
	tapPrompt(t, w, "Open installer")
	if !started {
		t.Fatal("accepting the guide did not start the SA-MP installer")
	}
}
