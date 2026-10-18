package mods

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestShareModsBrowserPathKeepsChecksumsAndBypassesAutomatedRequests(t *testing.T) {
	data, err := os.ReadFile(testutil.Zip(t, map[string]string{"readme.txt": "reviewed"}))
	if err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; http.Error(w, "browser challenge", 403) }))
	defer server.Close()
	resolver := NewResolver(download.New())
	called := 0
	resolver.BrowserDownload = func(ctx context.Context, raw, dest, expected string, progress download.Progress) error {
		called++
		if raw != server.URL || expected != checksum {
			t.Fatalf("wrong browser request: %q,%q", raw, expected)
		}
		return os.WriteFile(dest, data, 0600)
	}
	destination := filepath.Join(t.TempDir(), "package")
	release := Release{URL: server.URL, FileID: "officialfile"}
	if err := resolver.Download(context.Background(), release, destination, checksum, nil); err != nil {
		t.Fatal(err)
	}
	if called != 1 || requests != 0 {
		t.Fatalf("browser=%d network=%d", called, requests)
	}
	resolver.BrowserDownload = func(context.Context, string, string, string, download.Progress) error { return context.Canceled }
	if err := resolver.Download(context.Background(), release, destination, checksum, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	resolver.BrowserDownload = func(_ context.Context, _ string, dest, _ string, _ download.Progress) error {
		return os.WriteFile(dest, []byte("tampered"), 0600)
	}
	if err := resolver.Download(context.Background(), release, destination, checksum, nil); err == nil {
		t.Fatal("browser callback bypassed pinned checksum")
	}
}
func TestNormalModDownloadStaysAutomatic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("automatic download")) }))
	defer server.Close()
	resolver := NewResolver(download.New())
	resolver.BrowserDownload = func(context.Context, string, string, string, download.Progress) error {
		t.Fatal("browser used for unprotected source")
		return nil
	}
	destination := filepath.Join(t.TempDir(), "archive")
	if err := resolver.Download(context.Background(), Release{URL: server.URL}, destination, "", nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "automatic download" {
		t.Fatalf("download=%q,%v", data, err)
	}
}
