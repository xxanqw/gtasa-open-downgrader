package ui

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func TestGameWelcomeTransitionsToResults(t *testing.T) {
	w := newTestWindow(t)
	w.path.SetText("")
	w.window.Resize(fyne.NewSize(640, 480))
	if !w.setupGuide.Visible() || w.gameResults.Visible() || w.gameHeader.Visible() {
		t.Fatal("empty game view should prioritize folder selection")
	}
	captureUX(t, w, "game-choose-compact")
	w.window.Resize(fyne.NewSize(940, 640))
	captureUX(t, w, "game-choose-desktop")
	w.gameWelcome.alternative.OnTapped()
	if !w.promptActive || !appwidget.ModalOpen(w.window) {
		t.Fatal("manual path entry is unavailable")
	}
	appwidget.DismissModals(w.window)
	w.patchDir = "test-patches"
	w.path.SetText("/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas")
	if w.gameHeader.Visible() || w.gameWelcome.action.Text != "Check game folder" {
		t.Fatal("selected path should offer a scan")
	}
	w.window.Resize(fyne.NewSize(640, 480))
	captureUX(t, w, "game-ready-compact")
	verifiedTestGame(t, w)
	if w.setupGuide.Visible() || !w.gameResults.Visible() {
		t.Fatal("scan results did not replace welcome")
	}
	w.busy = true
	w.operationTitle = "Scanning game files"
	w.setupActivity = setupScanning
	w.setStatus("Comparing game files against the manifest")
	w.setProgress(.35)
	w.refreshEnabled()
	if !w.setupGuide.Visible() || w.gameResults.Visible() || w.gameWelcome.mode != setupWorking {
		t.Fatal("scan should replace stale results with scanning state")
	}
	if !w.gameWelcome.action.Disabled() {
		t.Fatal("scan allowed a second action")
	}
	captureUX(t, w, "game-scanning-compact")
	w.window.Resize(fyne.NewSize(940, 640))
	captureUX(t, w, "game-scanning-desktop")
	w.busy = false
	w.setupActivity = setupIdle
	w.refreshEnabled()
	if w.setupGuide.Visible() || !w.gameResults.Visible() {
		t.Fatal("completed scan did not reveal results")
	}
}

func TestGameWelcomeOffersPatchesBeforeRescanningWithoutManifest(t *testing.T) {
	w := newTestWindow(t)
	w.path.SetText("/game")
	w.scanPath = "/game"
	w.version.SetText("NewSteam R2 | Writable")
	w.patchDir = ""
	w.rows = nil
	w.refreshEnabled()
	if !w.setupGuide.Visible() || w.gameWelcome.mode != setupNeedPatches || w.gameWelcome.action.Text != "Install patches" {
		t.Fatal("missing patch assets should offer installation instead of another empty scan")
	}
	w.path.SetText("/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas")
	w.scanPath = w.path.Text
	w.refreshEnabled()
	if w.gameWelcome.changeFolder.Size().Height > 40 {
		t.Fatalf("Change button stretched to %v", w.gameWelcome.changeFolder.Size())
	}
	captureUX(t, w, "game-install-patches")
	w.patchDir = "test-patches"
	w.refreshEnabled()
	if w.gameWelcome.mode != setupReadyToScan || w.gameWelcome.action.Text != "Check game files" {
		t.Fatal("scan should become available after patches are prepared")
	}
}

func TestSetupUsesVerifiedRequirements(t *testing.T) {
	w := newTestWindow(t)
	cases := []struct {
		name, path, identified, patches string
		busy, readonly                  bool
		activity                        setupActivity
		problem                         error
		want                            setupStep
	}{
		{name: "first opening", want: setupChooseFolder},
		{name: "bundled patches still need game", patches: "patches", want: setupChooseFolder},
		{name: "remembered path needs identification", path: "/game", want: setupCheckFolder},
		{name: "unchecked folder with patches", path: "/game", patches: "patches", want: setupCheckFolder},
		{name: "identified folder needs patches", path: "/game", identified: "/game", want: setupNeedPatches},
		{name: "both requirements ready", path: "/game", identified: "/game", patches: "patches", want: setupReadyToScan},
		{name: "different folder invalidates identification", path: "/other", identified: "/game", patches: "patches", want: setupCheckFolder},
		{name: "folder check failed", path: "/game", problem: errors.New("missing executable"), want: setupFolderProblem},
		{name: "read only folder", path: "/game", identified: "/game", readonly: true, want: setupReadOnly},
		{name: "discovery in progress", busy: true, activity: setupDiscovering, want: setupWorking},
		{name: "unrelated task does not become discovery", busy: true, want: setupChooseFolder},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w.busy = false
			w.path.SetText(tc.path)
			w.scanPath, w.patchDir = tc.identified, tc.patches
			w.busy, w.readonly, w.setupActivity, w.gameSetupError = tc.busy, tc.readonly, tc.activity, tc.problem
			if got := w.nextSetupStep(); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetupActionStaysInCompactViewport(t *testing.T) {
	w := newTestWindow(t)
	w.window.Resize(fyne.NewSize(640, 480))
	for _, path := range []string{"", "/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas"} {
		w.path.SetText(path)
		w.refreshEnabled()
		action := w.gameWelcome.action
		position := w.app.Driver().AbsolutePositionForObject(action)
		if position.Y+action.Size().Height > w.window.Canvas().Size().Height-40 {
			t.Fatalf("setup action is below viewport: %v + %v", position, action.Size())
		}
	}
}
