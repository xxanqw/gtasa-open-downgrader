package ui

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func reviewRows() []game.Row {
	rows := []game.Row{{Path: "gta-sa.exe", Status: "Original (Needs Patch)", NeedsPatch: "Yes"}, {Path: "anim/anim.img", Status: "Original (Needs Patch)", NeedsPatch: "Yes"}}
	for index := 0; index < 14; index++ {
		rows = append(rows, game.Row{Path: fmt.Sprintf("audio/streams/TRACK_%02d", index), Status: "Original (Needs Patch)", NeedsPatch: "Yes"})
	}
	return append(rows, game.Row{Path: "unchanged.dat", NeedsPatch: "No"})
}

func TestStructuredReviewDisclosesFilesAndKeepsDecisionVisible(t *testing.T) {
	w := newTestWindow(t)
	for _, size := range []fyne.Size{fyne.NewSize(960, 720), fyne.NewSize(640, 480)} {
		w.window.Resize(size)
		options := downgradeReviewModal("/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas", "NewSteam R2 | Writable", reviewRows())
		options.Title = "Review downgrade"
		called, accepted := false, false
		options.Actions = []appwidget.ModalAction{{Text: "Cancel"}, {ID: "accept", Text: "Downgrade game", Importance: widget.HighImportance}}
		options.OnClosed = func(result string) { called = true; accepted = result == "accept" }
		modal := appwidget.ShowModal(w.window, *options)
		overlay := w.window.Canvas().Overlays().Top()
		files := findButton(overlay, "Files to change (16)")
		if files == nil {
			t.Fatal("file disclosure missing")
		}
		if options.Sections[0].Content.Visible() {
			t.Fatal("file list should start collapsed")
		}
		cancel := findButton(overlay, "Cancel")
		if cancel == nil || w.window.Canvas().Focused() != cancel {
			t.Fatal("Cancel must remain the initial action")
		}
		captureUX(t, w, fmt.Sprintf("review-summary-%d", int(size.Width)))
		test.Tap(files)
		if !options.Sections[0].Content.Visible() || called {
			t.Fatal("opening file details confirmed or did not reveal files")
		}
		captureUX(t, w, fmt.Sprintf("review-files-%d", int(size.Width)))
		// Controls are inside the canvas even with a long file list.
		accept := findButton(overlay, "Downgrade game")
		for _, button := range []*appwidget.Button{cancel, accept} {
			position := w.app.Driver().AbsolutePositionForObject(button)
			if position.Y+button.Size().Height > w.window.Canvas().Size().Height {
				t.Fatal("decision button escaped viewport")
			}
		}
		test.Tap(cancel)
		if !called || accepted {
			t.Fatal("cancel accepted the operation")
		}
		modal.Close("")
	}
}

func TestStructuredPromptRoutesOnlyExplicitAcceptance(t *testing.T) {
	w := newTestWindow(t)
	accepted := false
	w.prompt(promptRequest{kind: promptConfirm, title: "Review downgrade", accept: "Downgrade game", modal: downgradeReviewModal("/game", "Steam", reviewRows()), done: func(_ string, ok bool) { accepted = ok }})
	test.Tap(findButton(w.window.Canvas().Overlays().Top(), "Downgrade game"))
	if !accepted || w.promptActive {
		t.Fatal("explicit confirmation was not routed")
	}
}

func TestCaptureStructuredInstallReview(t *testing.T) {
	w := seedCompactMods(t)
	w.window.Resize(fyne.NewSize(640, 480))
	options := modInstallReviewModal("/mnt/nvme/SteamLibrary/steamapps/common/Grand Theft Auto San Andreas", []string{"ASI Loader", "ModLoader", "SilentPatch", "SkyGFX", "Widescreen Fixes"}, false)
	options.Title = "Install selected mods"
	options.Actions = []appwidget.ModalAction{{Text: "Cancel"}, {ID: "accept", Text: "Install mods", Importance: widget.HighImportance}}
	modal := appwidget.ShowModal(w.window, *options)
	captureUX(t, w, "review-install-compact")
	modal.Close("")
}

func TestModifiedFilesRemainVisibleWithoutOpeningDetails(t *testing.T) {
	options := downgradeReviewModal("/game", "Steam", []game.Row{
		{Path: "gta-sa.exe", Status: "Modified", NeedsPatch: "Yes (Force)"},
		{Path: "unchanged", NeedsPatch: "No"},
	})
	if !strings.Contains(options.Subtitle, "1 modified") || !strings.Contains(options.Subtitle, "1 files to patch") {
		t.Fatal("review hides modified-file scope")
	}
	if options.Accessory == nil {
		t.Fatal("cancellation warning must remain visible")
	}
}
