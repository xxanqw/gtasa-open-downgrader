package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"testing"
)

func TestStartupSizeAlreadySettledBeforeShow(t *testing.T) {
	a := test.NewApp()
	w := newWindow(a, &applog.Journal{})
	defer a.Quit()
	defer w.jobs.Close()
	defer w.indeterminate.Stop()
	// The production startup state is applied before Show, including long
	// command-line folders and the transition from Ready into active progress.
	w.path.SetText("/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas")
	w.busy = true
	w.operationTitle = "Preparing patch assets"
	w.setStatus("Preparing patch assets")
	w.setIndeterminate(true)
	w.refreshEnabled()
	w.prepareStartupWindow()
	initial := w.window.Canvas().Size()
	if initial.Width < 940 || initial.Height < 640 {
		t.Fatalf("startup size too small: %v", initial)
	}
	w.window.Show()
	w.window.Canvas().Capture()
	w.refreshWorkspace()
	if shown := w.window.Canvas().Size(); shown != initial {
		t.Fatalf("window resized after show: %v -> %v", initial, shown)
	}
	if minimum := w.window.Content().MinSize(); minimum.Width > initial.Width || minimum.Height > initial.Height {
		t.Fatalf("first-frame content outgrew startup bounds: %v > %v", minimum, initial)
	}
	// Startup preparation does not lock the window size afterwards.
	w.window.Resize(fyne.NewSize(800, 600))
	if w.window.FixedSize() {
		t.Fatal("startup fix disabled manual resizing")
	}
	w.window.SetCloseIntercept(nil)
	w.window.Close()
}
