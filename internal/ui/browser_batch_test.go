package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func TestBrowserQueueDeduplicatesSharedArchives(t *testing.T) {
	sha := strings.Repeat("a", 64)
	requests := []mods.BrowserArchive{
		{Name: "Radar", URL: "https://sharemods.com/abc/interface.7z.html", Checksum: sha},
		{Name: "HUD", URL: "https://sharemods.com/abc/interface.7z.html", Checksum: sha},
		{Name: "Helper", URL: "https://sharemods.com/def/helper.7z.html", Checksum: sha},
	}
	entries, err := browserQueueEntries(requests, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].request.Name != "Radar, HUD" {
		t.Fatalf("incorrect shared archive grouping: %+v", entries)
	}
	for _, raw := range []string{"https://sharemods.com.evil.test/id/mod.zip.html", "http://sharemods.com/id/mod.zip.html", "https://user@sharemods.com/id/mod.zip.html"} {
		if _, err := browserQueueEntries([]mods.BrowserArchive{{URL: raw}}, t.TempDir(), t.TempDir()); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	requests[0].Checksum = "../../invalid"
	if _, err := browserQueueEntries(requests, t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("accepted invalid checksum")
	}
}

func TestBrowserQueueAcceptsOutOfOrderDownloads(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	entries, err := browserQueueEntries([]mods.BrowserArchive{
		{Name: "First", URL: "https://sharemods.com/a/first.zip.html"},
		{Name: "Second", URL: "https://sharemods.com/b/second.zip.html"},
	}, work, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	results := []chan error{make(chan error, 1), make(chan error, 1)}
	for index, entry := range entries {
		entry.watcher, err = download.WatchDownloads(dir, entry.filename)
		if err != nil {
			t.Fatal(err)
		}
		go func() { results[index] <- waitQueueArchive(ctx, entry, func(string) {}) }()
	}
	archive := browserWaitZIP(t, "reviewed content")
	// A later queue item finishes while the earlier item is still missing.
	if err := os.WriteFile(filepath.Join(dir, "second.zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-results[1]:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-results[0]:
		t.Fatalf("first item unexpectedly finished: %v", err)
	default:
	}
	if err := os.WriteFile(filepath.Join(dir, "first.zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-results[0]:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for _, entry := range entries {
		data, err := os.ReadFile(entry.destination)
		if err != nil || !bytes.Equal(data, archive) {
			t.Fatalf("wrong archive %s: %v", entry.filename, err)
		}
	}
}

func TestBrowserQueueExistingImportRejectsWrongChecksumAndCanRetry(t *testing.T) {
	archive := browserWaitZIP(t, "reviewed content")
	entries, err := browserQueueEntries([]mods.BrowserArchive{{Name: "Mod", URL: "https://sharemods.com/a/mod.zip.html", Checksum: fmt.Sprintf("%x", sha256.Sum256(archive))}}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	dir := t.TempDir()
	entry.watcher, err = download.WatchDownloads(dir, entry.filename)
	if err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "existing.zip")
	if err := os.WriteFile(existing, browserWaitZIP(t, "wrong content"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rejected := make(chan struct{}, 1)
	result := make(chan error, 1)
	go func() {
		result <- waitQueueArchive(ctx, entry, func(message string) {
			if strings.Contains(message, "rejected") {
				rejected <- struct{}{}
			}
		})
	}()
	entry.imports <- browserQueueSource{file: existing}
	select {
	case <-rejected:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := os.Stat(entry.destination); !os.IsNotExist(err) {
		t.Fatal("wrong checksum committed")
	}
	if err := os.WriteFile(existing, archive, 0600); err != nil {
		t.Fatal(err)
	}
	entry.imports <- browserQueueSource{file: existing}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := os.Stat(existing); err != nil {
		t.Fatal("user archive removed")
	}
}

func TestBrowserQueueCancellationJoinsWorkers(t *testing.T) {
	entries, err := browserQueueEntries([]mods.BrowserArchive{{URL: "https://sharemods.com/a/mod.zip.html"}}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	entry.watcher, err = download.WatchDownloads(t.TempDir(), entry.filename)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- waitQueueArchive(ctx, entry, func(string) {}) }()
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
	if _, err := os.Stat(entry.destination); !os.IsNotExist(err) {
		t.Fatal("cancelled watcher committed an archive")
	}
}

func TestCaptureBrowserQueue(t *testing.T) {
	if os.Getenv("GTASA_UI_CAPTURE_DIR") == "" {
		t.Skip("optional UI captures")
	}
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		w := newTestWindow(t)
		w.app.Settings().SetTheme(appwidget.CompactTheme{Theme: theme.DefaultTheme()})
		entries, err := browserQueueEntries([]mods.BrowserArchive{
			{Name: "HD Radar Map (Original HD), Atmosphere HUD Textures", URL: "https://sharemods.com/a/Atmosphere_Interface_Pack.7z.html"},
			{Name: "Original HQ Radar Icons", URL: "https://sharemods.com/b/SA_Original_Radar_Icons_HQ.7z.html"},
			{Name: "nadalao Weapon Icons helper", URL: "https://sharemods.com/c/Weapon_Icons_TXD.7z.html"},
		}, t.TempDir(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			entry.state = widget.NewLabel("Needs browser download")
			entry.state.Wrapping = fyne.TextWrapWord
		}
		entries[1].ready = true
		entries[1].state.SetText("Verified · ready to install")
		modal := appwidget.ShowModal(w.window, appwidget.ModalOptions{Title: "Download selected mod archives", Subtitle: "1 of 3 ready · Watching /home/player/Downloads", Body: w.browserQueueBody(entries, func(int) {}), Actions: []appwidget.ModalAction{{Text: "Cancel"}, {Text: "Change download folder…"}, {Text: "Open remaining pages", Importance: widget.HighImportance}}})
		for _, size := range []fyne.Size{fyne.NewSize(960, 720), fyne.NewSize(640, 480)} {
			w.window.Resize(size)
			captureUX(t, w, fmt.Sprintf("download-queue-%d-%d", variant, int(size.Width)))
		}
		modal.Close("")
	}
}

func TestBrowserQueueReusesVerifiedCacheWithoutPrompt(t *testing.T) {
	w := newTestWindow(t)
	w.cache = t.TempDir()
	w.app.Preferences().SetString("mod-download-directory", filepath.Join(t.TempDir(), "missing-download-directory"))
	archive := browserWaitZIP(t, "cached reviewed content")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archive))
	cacheDir := filepath.Join(w.cache, "mod-archives")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, checksum+".zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw := "https://sharemods.com/a/mod.zip.html"
	downloader, cleanup, err := w.prepareBrowserDownloads(ctx, []mods.BrowserArchive{{Name: "Mod", URL: raw, Checksum: checksum}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	destination := filepath.Join(t.TempDir(), "package.zip")
	if err := downloader(ctx, raw, destination, checksum, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, archive) {
		t.Fatalf("wrong cached package: %v", err)
	}
	if w.promptActive {
		t.Fatal("cached package opened a prompt")
	}
	cleanup()
	if err := downloader(ctx, raw, destination, checksum, nil); err == nil {
		t.Fatal("session files survived cleanup")
	}
}

func TestBrowserQueueFolderSnapshotPrecedesNewDownload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	entries, err := browserQueueEntries([]mods.BrowserArchive{{URL: "https://sharemods.com/a/mod.zip.html"}}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	entry.watcher, err = download.WatchDownloads(t.TempDir(), entry.filename)
	if err != nil {
		t.Fatal(err)
	}
	nextDir := t.TempDir()
	watcher, err := download.WatchDownloads(nextDir, entry.filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := browserWaitZIP(t, "completed immediately after folder selection")
	if err := os.WriteFile(filepath.Join(nextDir, "mod.zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	// The watcher starts after the file finishes, using the earlier snapshot.
	entry.imports <- browserQueueSource{watcher: watcher, directory: nextDir}
	if err := waitQueueArchive(ctx, entry, func(string) {}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(entry.destination)
	if err != nil || !bytes.Equal(data, archive) {
		t.Fatalf("missed new download: %v", err)
	}
}
