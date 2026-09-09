package ui

import (
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
)

func TestToolsKeepGameValidationAndQueueOtherPrompts(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Show()
	w.launchOptions()
	snapshot(t, w, "GTASA_LAUNCH_OPTIONS_SNAPSHOT")
	tapPrompt(t, w, "Close")
	w.tools()
	for _, action := range w.toolActions() {
		_, label := toolPresentation(action.label)
		if promptButton(w.window.Canvas().Overlays().Top(), label) == nil {
			t.Fatalf("tool lost: %s", label)
		}
	}
	snapshot(t, w, "GTASA_TOOLS_SNAPSHOT")
	tapPrompt(t, w, "Restore original files")
	found := false
	walkObjects(w.window.Canvas().Overlays().Top(), func(o fyne.CanvasObject) bool {
		if label, ok := o.(*widget.Label); ok && strings.Contains(label.Text, "scan a game directory first") {
			found = true
			return true
		}
		return false
	})
	if !found {
		t.Fatal("restore bypassed game directory validation")
	}
	tapPrompt(t, w, "Close")
	w.tools()
	w.info("Update", "An update is available.")
	if len(w.prompts) != 1 {
		t.Fatal("background information overlapped Tools")
	}
	tapPrompt(t, w, "Close")
	if !w.promptActive || len(w.prompts) != 0 {
		t.Fatal("queued information did not open after Tools")
	}
	tapPrompt(t, w, "Close")
}
func TestAboutContainsOriginalCreditsAndCompleteLicenses(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Show()
	source, err := os.ReadFile("../../LICENSE")
	if err != nil || string(source) != assets.AppLicense {
		t.Fatal("embedded app license differs from repository license")
	}
	for _, credit := range []string{"RockstarNexus", "Ivan Potiienko (xxanqw)", "https://github.com/xxanqw", "GTA SA community", "MIT License"} {
		if !strings.Contains(aboutCredits, credit) {
			t.Fatalf("missing original credit %s", credit)
		}
	}
	w.showAbout()
	var licenses *widget.Select
	walkObjects(w.window.Canvas().Overlays().Top(), func(o fyne.CanvasObject) bool {
		if v, ok := o.(*widget.Select); ok {
			licenses = v
			return true
		}
		return false
	})
	if licenses == nil || len(licenses.Options) != 3 {
		t.Fatal("About lacks bundled license selector")
	}
	for i, want := range []string{assets.AppLicense, assets.XdeltaLicense, assets.FontAwesomeLicense} {
		licenses.SetSelected(licenses.Options[i])
		found := false
		walkObjects(w.window.Canvas().Overlays().Top(), func(o fyne.CanvasObject) bool {
			if label, ok := o.(*widget.Label); ok && label.Text == want && label.Selectable {
				found = true
				return true
			}
			return false
		})
		if !found {
			t.Fatalf("license %d is incomplete or not selectable", i)
		}
		tapPrompt(t, w, "Copy license")
		if a.Clipboard().Content() != want {
			t.Fatalf("license %d was not copied in full", i)
		}
	}
	licenses.SetSelected(appLicenseName)
	snapshot(t, w, "GTASA_ABOUT_SNAPSHOT")
	tapPrompt(t, w, "Close")
	if w.promptActive {
		t.Fatal("About did not close")
	}
	w.window.Resize(fyne.NewSize(720, 480))
	w.showAbout()
	okButton := promptButton(w.window.Canvas().Overlays().Top(), "Close")
	pos := a.Driver().AbsolutePositionForObject(okButton)
	if pos.X < 0 || pos.Y < 0 || pos.X+okButton.Size().Width > 720 || pos.Y+okButton.Size().Height > 480 {
		t.Fatal("small window hides About actions")
	}
	tapPrompt(t, w, "Close")
}
