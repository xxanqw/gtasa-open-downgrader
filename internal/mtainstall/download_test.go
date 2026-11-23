package mtainstall

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestNightlySelectsNewestWindowsClient(t *testing.T) {
	client := &http.Client{Transport: testTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != nightlyIndex {
			t.Fatalf("unexpected request %s", req.URL)
		}
		return testResponse(`
			<a href="mtasa-1.7-untested-999-20261031.exe">Older</a>
			<a href="mtasa-1.7-untested-1200-20261108.exe">Latest</a>
			<a href="mtasa-1.7-untested-1199-20261107.exe">Older</a>
			<a href="mtasa-1.7-untested-1200-20261107.exe">Older on the same revision</a>
			<a href="mtasa_x64-1.7-untested-1201-20261109.exe">Server</a>
			<a href="mtasa-1.6-unstable-1201-20261109.exe">1.6 nightly</a>
			<a href="mtasa-1.7-untested-1201-20261109-maetro.exe">Win7/8</a>
			<a href="https://example.com/mtasa-1.7-untested-1201-20261109.exe">External</a>
		`), nil
	})}
	name, err := latestNightly(context.Background(), client)
	if err != nil || name != "mtasa-1.7-untested-1200-20261108.exe" {
		t.Fatalf("latestNightly = %q, %v", name, err)
	}
}

func TestDownloadRejectsInvalidSignatureBeforePublishing(t *testing.T) {
	cache := t.TempDir()
	client := &http.Client{Transport: testTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://mirror-cdn.multitheftauto.com/mtasa/main/mtasa-1.6.exe" {
			t.Fatalf("unexpected request %s", req.URL)
		}
		return testResponse("MZ invalid installer"), nil
	})}
	installer, err := Download(context.Background(), client, cache, "1.6", nil)
	if err == nil || !strings.Contains(err.Error(), "publisher and signed SHA-256") || installer.Path() != "" {
		t.Fatalf("Download = %q, %v", installer.Path(), err)
	}
	files, err := os.ReadDir(filepath.Join(cache, "multiplayer", "mta"))
	if err != nil || len(files) != 0 {
		t.Fatalf("unverified download left cache entries %v, %v", files, err)
	}
}

func TestVerifiedInstallerRejectsCachedModification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mtasa.exe")
	original := []byte("MZ verified installer bytes")
	installer := VerifiedInstaller{path: path, sha256: sha256.Sum256(original)}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := installer.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	original[len(original)-1] ^= 0x1
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := installer.Validate(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Validate = %v", err)
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (send testTransport) RoundTrip(req *http.Request) (*http.Response, error) { return send(req) }

func testResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}
