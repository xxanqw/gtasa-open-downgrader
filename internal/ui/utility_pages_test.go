package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"os"
	"path/filepath"
	"testing"
)

func TestUtilityPagesKeepActionsCompact(t *testing.T) {
	w := newTestWindow(t)
	root := verifiedTestGame(t, w)
	if err := os.WriteFile(filepath.Join(root, "gta_sa.exe"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, size := range []fyne.Size{fyne.NewSize(940, 640), fyne.NewSize(640, 480)} {
		w.window.Resize(size)
		for _, page := range []string{hubTools, hubRecovery} {
			w.showHub(page)
			captureUX(t, w, fmt.Sprintf("%s-%d", page, int(size.Width)))
			for _, label := range []string{"Apply 4GB patch", "Import mod archive", "Restore original files", "Delete backups…", "Delete user data…"} {
				button := findButton(w.hubPages[page], label)
				if button != nil && button.Size().Height > 40 {
					t.Fatalf("%s action stretched to %v", label, button.Size())
				}
			}
		}
	}
	w.showHub(hubRecovery)
	button := findButton(w.hubPages[hubRecovery], "Delete user data…")
	button.OnTapped()
	if !appwidget.ModalOpen(w.window) || !w.promptActive || findButton(w.window.Canvas().Overlays().Top(), "Delete user data permanently") == nil {
		t.Fatal("deletion bypasses confirmation")
	}
}
