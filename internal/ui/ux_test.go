package ui

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func newTestWindow(t *testing.T) *Window {
	t.Helper()
	a := test.NewApp()
	w := newWindow(a, &applog.Journal{})
	w.window.Show()
	t.Cleanup(func() {
		w.jobs.Close()
		w.indeterminate.Stop()
		appwidget.DismissModals(w.window)
		w.window.SetCloseIntercept(nil)
		w.window.Close()
		a.Quit()
	})
	return w
}

func verifiedTestGame(t *testing.T, w *Window) string {
	t.Helper()
	root := t.TempDir()
	w.path.SetText(root)
	w.scanPath, w.patchDir = root, "test-patches"
	hash := "0123456789abcdef0123456789abcdef"
	w.rows = []game.Row{{Path: "gta_sa.exe", Status: "Already Downgraded", NeedsPatch: "No", CurrentHash: hash, TargetHash: hash}}
	w.version.SetText("v1.0 US | Writable")
	w.modStatus.SetText("No downgrader-installed mods detected.")
	w.refreshEnabled()
	return root
}

func findButton(object fyne.CanvasObject, label string) *appwidget.Button {
	if b, ok := object.(*appwidget.Button); ok && b.Text == label {
		return b
	}
	var children []fyne.CanvasObject
	switch o := object.(type) {
	case *fyne.Container:
		children = o.Objects
	case *framedPanel:
		children = []fyne.CanvasObject{o.content}
	default:
		if w, ok := o.(fyne.Widget); ok {
			children = test.WidgetRenderer(w).Objects()
		}
	}
	for _, child := range children {
		if b := findButton(child, label); b != nil {
			return b
		}
	}
	return nil
}

type uiTestRunner struct{}

func (uiTestRunner) Prepare(context.Context, multiplayer.ClientID, multiplayer.Environment) ([]string, error) {
	return nil, nil
}
func (uiTestRunner) Run(context.Context, string, ...string) error { return nil }

func TestCancelMultiplayerRemovalKeepsCards(t *testing.T) {
	for _, action := range []string{"Remove & restore", "Uninstall"} {
		t.Run(action, func(t *testing.T) {
			w := newTestWindow(t)
			root := verifiedTestGame(t, w)
			w.showMultiplayerInline(root, entrypoint.Status{State: entrypoint.Installed}, multiplayer.Environment{}, uiTestRunner{}, "", nil,
				[]multiplayer.Client{{ID: multiplayer.ClientSAMP, Name: "SA-MP", Installed: true}})
			b := findButton(w.multiplayerBody, action)
			if b == nil {
				t.Fatalf("missing %s", action)
			}
			if b.Disabled() {
				t.Fatal("fixture action should be available")
			}
			before := w.multiplayerBody.Objects[0]
			test.Tap(b)
			if !b.Disabled() {
				t.Fatal("cards must be disabled during confirmation")
			}
			if !w.promptActive {
				t.Fatal("expected confirmation")
			}
			focused, ok := w.window.Canvas().Focused().(*appwidget.Button)
			if !ok || focused.Text != "Cancel" {
				t.Fatal("confirmation must focus Cancel")
			}
			focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if w.multiplayerBody.Objects[0] != before || w.busy || w.promptActive || b.Disabled() {
				t.Fatal("cancellation replaced cards or started work")
			}
		})
	}
}

func TestMainProgressStatusAndOutcomeSurviveVerification(t *testing.T) {
	w := newTestWindow(t)
	w.busy = true
	message := "Waiting for very-long-reviewed-archive.zip in /home/player/Downloads — finish the browser download"
	w.setStatus(message)
	if w.status.Text != message || !w.progressTrack.Visible() {
		t.Fatal("full operation instructions were lost")
	}
	w.window.Resize(fyne.NewSize(640, 480))
	longStatus := "models/a-long-game-file-path/with-additional-directory-names/gta_int.img · Backup & patching the original game archive"
	w.setStatus(longStatus)
	if w.status.Text != longStatus || w.status.Truncation != fyne.TextTruncateOff || w.status.Wrapping != fyne.TextWrapWord {
		t.Fatal("main progress status must retain and wrap the full action")
	}
	w.setIndeterminate(true)
	if !w.indeterminate.Visible() || w.progress.Visible() {
		t.Fatal("unknown progress must be indeterminate")
	}
	w.setProgress(.5)
	if w.indeterminate.Visible() || !w.progress.Visible() {
		t.Fatal("known progress must be determinate")
	}
	w.recordOperationOutcome("Installing mods", context.Canceled)
	outcome := w.operationOutcome
	w.recordOperationOutcome("Scanning game files", nil)
	w.setStatus("Scanning game files")
	if w.operationOutcome != outcome || !strings.Contains(outcome, "does not undo") {
		t.Fatal("scan erased cancellation consequences")
	}
	w.busy = false
	w.refreshEnabled()
	if w.status.Text != outcome {
		t.Fatal("outcome must remain visible after work stops")
	}
	w.outcomeDismiss.OnTapped()
	if w.operationOutcome != "" || w.progressTrack.Visible() {
		t.Fatal("dismiss should clear the interruption warning")
	}
}

func TestSuccessfulOperationsDoNotLeaveBanner(t *testing.T) {
	w := newTestWindow(t)
	for _, title := range []string{"Uninstalling MTA 1.6", "Installing mods", "Downloading patch assets"} {
		w.recordOperationOutcome(title, nil)
		w.setStatus(title + " complete")
		w.refreshEnabled()
		if w.operationOutcome != "" || w.progressTrack.Visible() || w.outcomeDismiss.Visible() {
			t.Fatalf("%s left a redundant success banner", title)
		}
		if w.fullStatus != title+" complete" {
			t.Fatal("success should still be recorded in the status/log")
		}
		if title == "Uninstalling MTA 1.6" {
			captureUX(t, w, "routine-success")
		}
	}
}

func TestDestructiveConfirmationHasExplicitActionAndSafeFocus(t *testing.T) {
	w := newTestWindow(t)
	called := false
	w.confirmAction("Delete saves, settings & gallery", "Delete user data permanently", "This cannot be undone.", func(ok bool) { called = ok })
	if w.window.Canvas().Focused().(*appwidget.Button).Text != "Cancel" {
		t.Fatal("unsafe default focus")
	}
	overlays := w.window.Canvas().Overlays().List()
	if findButton(overlays[len(overlays)-1], "Delete user data permanently") == nil {
		t.Fatal("missing explicit action")
	}
	w.window.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if called || w.promptActive {
		t.Fatal("Escape must cancel")
	}
}

func TestRecoveryActionsUseExplicitConfirmations(t *testing.T) {
	w := newTestWindow(t)
	root := verifiedTestGame(t, w)
	if err := os.WriteFile(filepath.Join(root, "gta_sa.exe"), []byte("test fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for action, label := range map[string]string{
		"Restore downgrade backups":         "Restore original files",
		"Delete downgrade backups":          "Delete downgrade backups",
		"Clear saves, settings and gallery": "Delete user data permanently",
	} {
		w.runHubTool(action)
		if !w.promptActive {
			t.Fatalf("%s did not request confirmation", action)
		}
		overlays := w.window.Canvas().Overlays().List()
		if findButton(overlays[len(overlays)-1], label) == nil {
			t.Errorf("%s has no explicit action", action)
		}
		w.window.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		if w.busy || w.promptActive {
			t.Fatal("default Enter must cancel without work")
		}
	}
}

func TestOutcomeClearsWhenAnUnscannedFolderChanges(t *testing.T) {
	w := newTestWindow(t)
	w.path.SetText(t.TempDir())
	w.recordOperationOutcome("Installing mods", context.Canceled)
	w.path.SetText(t.TempDir())
	if w.operationOutcome != "" {
		t.Fatal("previous installation result leaked into a different folder")
	}
}

func TestSetupIdentityAndReview(t *testing.T) {
	w := newTestWindow(t)
	if !w.setupGuide.Visible() || w.fileControls.Visible() || w.patchButton.Visible() {
		t.Fatal("empty setup should prioritize choosing a game")
	}
	if !w.hubNav[hubMods].Disabled() {
		t.Fatal("setup must not bypass verification")
	}
	root := verifiedTestGame(t, w)
	w.showHub(hubRecovery)
	if w.gameHeader.Visible() || findButton(w.window.Content(), "Change game") != nil {
		t.Fatal("Recovery should not duplicate the Game page's folder controls")
	}
	if w.path.Text != root {
		t.Fatal("navigation lost the selected installation")
	}
	rows := []game.Row{{Path: "data/main.scm", Status: "Ready", NeedsPatch: "Yes"}, {Path: "gta_sa.exe", Status: "Modified", NeedsPatch: "Yes (Force)"}, {Path: "unchanged", NeedsPatch: "No"}}
	options := downgradeReviewModal(root, "NewSteam R2 German", rows)
	var collectText func(fyne.CanvasObject) string
	collectText = func(object fyne.CanvasObject) string {
		switch object := object.(type) {
		case *widget.Label:
			return object.Text + "\n"
		case *fyne.Container:
			var text strings.Builder
			for _, child := range object.Objects {
				text.WriteString(collectText(child))
			}
			return text.String()
		}
		return ""
	}
	plan := options.Subtitle + collectText(options.Summary)
	for _, section := range options.Sections {
		plan += collectText(section.Content)
	}
	for _, text := range []string{root, "NewSteam R2 German", "v1.0 US", "data/main.scm", "Modified", filepath.Join(root, "backups"), "1 unchanged or not applicable"} {
		if !strings.Contains(plan, text) {
			t.Errorf("review missing %q", text)
		}
	}
	if strings.Contains(collectText(options.Sections[0].Content), "unchanged") {
		t.Fatal("unchanged file listed as a replacement")
	}
	if (fileTableTheme{w.app.Settings().Theme()}).Size(theme.SizeNameText) < 14 {
		t.Fatal("file table is smaller than base text")
	}
}

func TestModStateCommunicatesUncertainty(t *testing.T) {
	w := newTestWindow(t)
	name := "SilentPatch"
	cases := []struct {
		detail, label string
		importance    widget.Importance
	}{
		{"Not installed", "Available", widget.MediumImportance},
		{"Installed · 1.0", "Installed", widget.SuccessImportance},
		{"Installed · 1.0 · Check unavailable", "Installed\nCheck unavailable", widget.WarningImportance},
		{"Installed · version unknown", "Installed\nVersion unknown", widget.WarningImportance},
		{"Installed · 1.0 · Update available: 2.0", "Installed\nUpdate available", widget.WarningImportance},
	}
	for _, tc := range cases {
		w.setModLabel(name, tc.detail)
		state := w.modLabels[name]
		if state.Text != tc.label || state.Importance != tc.importance {
			t.Errorf("%q: got %q importance %v", tc.detail, state.Text, state.Importance)
		}
	}
}

func TestMixedModToolbarKeepsSelectionSummary(t *testing.T) {
	w := newTestWindow(t)
	verifiedTestGame(t, w)
	w.modTracker = mods.Tracker{Installed: []string{"ASI Loader", "ModLoader", "SilentPatch"}}
	for _, name := range w.modTracker.Installed {
		w.installedMods[name] = true
		w.setModLabel(name, "Installed · 1.0")
	}
	w.installedModCount = 3
	w.modStatus.SetText("3 installed mods checked.")
	for _, source := range mods.Catalog {
		w.modExplicit[source.Name] = true
	}
	w.refreshModBatch()
	w.showHub(hubMods)
	w.window.Resize(fyne.NewSize(640, 480))
	w.window.Canvas().Capture()
	if !strings.Contains(w.modTable.header.sortName.Text, w.modSelection.Text) {
		t.Fatal("compact header lost selection summary")
	}
	if w.bulkActions.Position().Y != w.updateModsButton.Position().Y {
		t.Fatal("selection actions should share the update button's header row")
	}
	if w.updateModsButton.Text != "Update selected" {
		t.Fatal("update scope is ambiguous")
	}
	captureUX(t, w, "mods-small")
}

func TestErrorRecoveryOffersNextSteps(t *testing.T) {
	if strings.Contains(errorRecovery("unsupported MTA release"), "NewSteam") {
		t.Fatal("unrelated unsupported client must not get game-edition advice")
	}
	for detail, step := range map[string]string{
		"permission denied": "writable", "unsupported source edition": "NewSteam",
		"archive checksum mismatch": "official", "HTTP timeout": "connection",
		"GTA is running": "Close GTA", "no space left on device": "free space", "unexpected failure": "Recovery",
	} {
		if !strings.Contains(errorRecovery(detail), step) {
			t.Errorf("no applicable next step for %q", detail)
		}
	}
}

// Optional local rendering, without committing platform-dependent golden files.
func captureUX(t *testing.T, w *Window, name string) {
	t.Helper()
	dir := os.Getenv("GTASA_UI_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.window.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureUXStates(t *testing.T) {
	if os.Getenv("GTASA_UI_CAPTURE_DIR") == "" {
		t.Skip("optional UI captures")
	}
	w := newTestWindow(t)
	captureUX(t, w, "setup")
	verifiedTestGame(t, w)
	w.showHub(hubRecovery)
	captureUX(t, w, "recovery")
	w.busy = true
	w.setStatus("Waiting for browser download — finish the download or open Download options.")
	w.browserDetails = "Expected file: important-mod-archive.zip\nWatching folder: /home/player/Downloads\nFinish the browser download. Only new completed matching archives are accepted. Cancel stops waiting, not the browser download."
	w.browserOptions = func() {}
	w.setIndeterminate(true)
	w.refreshEnabled()
	captureUX(t, w, "waiting")
	w.busy = false
	w.browserOptions = nil
	w.setIndeterminate(false)
	w.refreshEnabled()
	w.confirmAction("Delete saves, settings & gallery", "Delete user data permanently", "Permanently delete user data? This cannot be undone.", func(bool) {})
	captureUX(t, w, "delete-confirm")
}
