package main

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherprefs"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func testLauncher(t *testing.T) *launcherWindow {
	t.Helper()
	a := test.NewApp()
	a.Settings().SetTheme(appwidget.CompactTheme{Theme: theme.DefaultTheme()})
	w := a.NewWindow("Launcher test")
	ui := newLauncherWindow(a, w, t.TempDir(), "windows", false, nil, launcherActions{single: func(context.Context) error { return nil }, multiplayer: func(context.Context, multiplayer.ClientID) error { return nil }})
	w.Show()
	t.Cleanup(func() {
		appwidget.DismissModals(w)
		w.SetCloseIntercept(nil)
		w.Close()
		a.Quit()
	})
	return ui
}

func TestRememberedChoiceAllowsKeyboardBypass(t *testing.T) {
	ui := testLauncher(t)
	launched := 0
	ui.remembered = launcherprefs.SinglePlayer
	ui.activate[launcherprefs.SinglePlayer] = func() { launched++ }
	ui.startRemembered()
	if launched != 0 || ui.autoLaunchCancel == nil {
		t.Fatal("saved mode launched without a bypass")
	}
	button, ok := ui.window.Canvas().Focused().(*appwidget.Button)
	if !ok || button.Text != "Choose another mode" {
		t.Fatal("safe bypass should have keyboard focus")
	}
	button.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if launched != 0 || ui.autoLaunchCancel != nil || appwidget.ModalOpen(ui.window) {
		t.Fatal("bypass did not stop automatic launch")
	}
	if ui.remembered != launcherprefs.SinglePlayer {
		t.Fatal("one-time bypass must not silently erase preferences")
	}
}

func TestRememberedChoiceLaunchNowAndUnavailableMode(t *testing.T) {
	ui := testLauncher(t)
	launched := 0
	ui.activate[launcherprefs.SinglePlayer] = func() { launched++ }
	ui.remembered = launcherprefs.SinglePlayer
	ui.startRemembered()
	overlays := ui.window.Canvas().Overlays().List()
	button := launcherTestButton(overlays[len(overlays)-1], "Launch now")
	if button == nil {
		t.Fatal("missing launch-now action")
	}
	button.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if launched != 1 || ui.autoLaunchCancel != nil {
		t.Fatal("launch now should activate exactly once and stop countdown")
	}
	ui.remembered = launcherprefs.Choice(multiplayer.ClientSAMP)
	ui.startRemembered()
	if appwidget.ModalOpen(ui.window) || ui.autoLaunchCancel != nil || launched != 1 {
		t.Fatal("unavailable saved mode should leave chooser usable")
	}
}

func launcherTestButton(object fyne.CanvasObject, label string) *appwidget.Button {
	if b, ok := object.(*appwidget.Button); ok && b.Text == label {
		return b
	}
	var children []fyne.CanvasObject
	switch o := object.(type) {
	case *fyne.Container:
		children = o.Objects
	default:
		if w, ok := o.(fyne.Widget); ok {
			children = test.WidgetRenderer(w).Objects()
		}
	}
	for _, child := range children {
		if b := launcherTestButton(child, label); b != nil {
			return b
		}
	}
	return nil
}

func TestCaptureLauncher(t *testing.T) {
	dir := os.Getenv("GTASA_UI_CAPTURE_DIR")
	if dir == "" {
		t.Skip("optional UI capture")
	}
	ui := testLauncher(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	capture := func(name string) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, ui.window.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	capture("launcher")
	ui.remembered = launcherprefs.SinglePlayer
	ui.startRemembered()
	capture("launcher-auto")
}
