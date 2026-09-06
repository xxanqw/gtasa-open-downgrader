package ui

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func TestNativeToolsSelectionOpensPicker(t *testing.T) {
	a := newDialogTestApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	requests := make(chan promptRequest, 2)
	replies := make(chan string, 2)
	w.dialogs = dialogBackend{available: func() bool { return true }, show: func(ctx context.Context, r promptRequest, _ any) (string, error) {
		requests <- r
		select {
		case value := <-replies:
			if value == "" {
				return "", zenity.ErrCanceled
			}
			return value, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	w.tools()
	r := receive(t, requests)
	if r.kind != promptChoice || !slices.Contains(r.choices, "Restore downgrade backups") || !slices.Contains(r.choices, "Apply 4GB / Large Address Aware flag") {
		t.Fatal("native tools lost game operations")
	}
	replies <- "Select an existing Patches folder"
	a.dispatch(t)
	r = receive(t, requests)
	if r.kind != promptFolder {
		t.Fatal("selected tool did not open its native picker")
	}
	replies <- ""
	w.jobs.Close()
	a.dispatch(t)
	if w.patchDir != "" {
		t.Fatal("cancelled tool changed patch assets")
	}
}

type linkCaptureApp struct {
	fyne.App
	opened *url.URL
}

func (a *linkCaptureApp) OpenURL(u *url.URL) error { a.opened = u; return nil }

func TestNativeAboutPreservesInformationAndBundledLicense(t *testing.T) {
	a := newDialogTestApp()
	defer a.Quit()
	capture := &linkCaptureApp{App: a}
	w := New(capture)
	w.cache = t.TempDir()
	requests := make(chan promptRequest, 1)
	w.dialogs = dialogBackend{available: func() bool { return true }}
	// Capture and inspect the actual native request without opening a system window.
	w.dialogs.show = func(ctx context.Context, r promptRequest, _ any) (string, error) {
		requests <- r
		return "Copy app information", nil
	}
	w.showAbout()
	r := receive(t, requests)
	w.jobs.Close()
	a.dispatch(t)
	if r.kind != promptChoice || !strings.Contains(r.text, updater.Version) || !slices.Contains(r.choices, "Read bundled xdelta3 license") {
		t.Fatal("About lost version or license access")
	}
	if !strings.Contains(a.Clipboard().Content(), updater.Version) {
		t.Fatal("About information was not copied")
	}
	w.openLicense()
	if capture.opened == nil || capture.opened.Scheme != "file" {
		t.Fatal("license did not open in the system viewer")
	}
	data, err := os.ReadFile(filepath.Join(w.cache, "xdelta3-LICENSE.txt"))
	if err != nil || string(data) != assets.XdeltaLicense {
		t.Fatalf("bundled license changed: %v", err)
	}
}
