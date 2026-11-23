package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestMTAManagerShowsActionsForSelectedVersion(t *testing.T) {
	for _, installed := range []bool{false, true} {
		state := "fresh"
		if installed {
			state = "installed"
		}
		t.Run(state, func(t *testing.T) {
			w := newTestWindow(t)
			root := verifiedTestGame(t, w)
			prefix := t.TempDir()
			versionCount := 2
			if installed {
				for _, version := range []string{"1.6", "1.7", "1.5"} {
					testutil.Write(t, prefix, "Program Files/MTA San Andreas "+version+"/Multi Theft Auto.exe", "MZfixture")
				}
				testutil.Write(t, root, "Custom MTA/Multi Theft Auto.exe", "MZfixture")
				testutil.Write(t, root, multiplayer.ConfigPath, `{"schema_version":1,"clients":{"mta":{"executable":"Custom MTA/Multi Theft Auto.exe"}}}`)
				versionCount = 4
			}
			environment := multiplayer.Environment{GameDir: root, PrefixDriveC: prefix}
			runner := multiplayer.DirectRunner{}
			body := w.multiplayerBody
			w.multiplayerBody = nil // The fixture supplies detection results.
			w.showHub(hubMultiplayer)
			w.multiplayerBody = body
			w.showMultiplayerInline(root, entrypoint.Status{}, environment, runner, "", nil, multiplayer.Detect(environment))
			w.window.Resize(fyne.NewSize(940, 640))
			w.showMTAManager(environment, runner)
			w.window.Canvas().Capture()
			overlay := w.window.Canvas().Overlays().Top()
			var versions *mtaVersionList
			for _, object := range test.LaidOutObjects(overlay) {
				if list, ok := object.(*mtaVersionList); ok {
					versions = list
					break
				}
			}
			if versions == nil || versions.Length() != versionCount {
				t.Fatalf("expected %d versions", versionCount)
			}
			if installed && (findButton(overlay, "Reinstall…") == nil || findButton(overlay, "Uninstall…") == nil || findButton(overlay, "Use in Steam…") != nil) {
				t.Fatal("active version did not show the installed actions")
			}
			if !installed && findButton(overlay, "Install 1.6…") == nil {
				t.Fatal("stable version did not show its install action")
			}
			captureUX(t, w, "mta-manager-"+state)
			w.window.Canvas().Focus(versions)
			versions.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
			versions.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			w.window.Canvas().Capture()
			if installed {
				use := findButton(overlay, "Use in Steam…")
				if use == nil || use.AccessibilityLabel() != "Use MTA 1.7 in the Steam launcher" {
					t.Fatal("the other installed version did not offer its Steam selection action")
				}
			}
			if !installed && (findButton(overlay, "Install 1.7…") == nil || findButton(overlay, "Install 1.6…") != nil) {
				t.Fatal("switching versions did not replace the install action")
			}
			if choice, err := multiplayer.LoadMTAChoice(root); err != nil || choice != "" {
				t.Fatalf("browsing versions changed the Steam choice to %q: %v", choice, err)
			}
			w.window.Resize(fyne.NewSize(640, 480))
			captureUX(t, w, "mta-manager-"+state+"-compact")
			w.closing = true // Keep the fixture from re-detecting Steam after the dialog closes.
			versions.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			if w.window.Canvas().Overlays().Top() != nil {
				t.Fatal("Escape did not close the dialog from the version picker")
			}
		})
	}
}
