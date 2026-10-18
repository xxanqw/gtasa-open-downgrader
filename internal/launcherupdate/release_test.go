package launcherupdate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/service"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func client(body []byte, length int64) *http.Client {
	return &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: length, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	})}
}
func manifest(body []byte) Manifest {
	return Manifest{Schema: 1, Version: "1", Tag: "launcher-1", Name: AssetName, Size: int64(len(body)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(body)), URL: service.BaseURL + "/api/v1/releases/launcher-1/download/gta-sa.exe"}
}

func TestManifestRejectsUnsafeMetadata(t *testing.T) {
	base := manifest([]byte("launcher"))
	for _, change := range []func(*Manifest){
		func(m *Manifest) { m.Schema = 2 }, func(m *Manifest) { m.Version = "latest" },
		func(m *Manifest) { m.Name = "gta_sa.exe" }, func(m *Manifest) { m.Digest = "" },
		func(m *Manifest) { m.Size = maxSize + 1 }, func(m *Manifest) { m.Size = -1 },
		func(m *Manifest) { m.URL = "https://other.example/gta-sa.exe" },
		func(m *Manifest) { m.URL = service.BaseURL + "/api/v1/releases/download/gta-sa.exe" },
		func(m *Manifest) { m.URL += "?redirect=other" },
	} {
		candidate := base
		change(&candidate)
		if err := candidate.Validate(); err == nil {
			t.Fatalf("accepted %+v", candidate)
		}
	}
}

func TestCheckUsesFirstPartyServiceAndRejectsOversizedMetadata(t *testing.T) {
	body, _ := json.Marshal(manifest([]byte("launcher")))
	httpClient := client(body, int64(len(body)))
	original := httpClient.Transport
	httpClient.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != service.LauncherReleaseURL {
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return original.RoundTrip(r)
	})
	if _, err := Check(context.Background(), httpClient); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(context.Background(), client([]byte(strings.Repeat("x", 65537)), -1)); err == nil {
		t.Fatal("accepted oversized metadata")
	}
	if _, err := Check(context.Background(), client([]byte(`{}`), -1)); err == nil {
		t.Fatal("accepted empty metadata")
	}
}

func TestDownloadVerifiesPEIdentityVersionAndDigest(t *testing.T) {
	binary := testutil.Launcher(t, "1")
	body, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	m := manifest(body)
	if err := Verify(context.Background(), binary, m); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	staged, err := Download(context.Background(), client(body, m.Size), cache, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(staged)
	for _, change := range []func(*Manifest){
		func(m *Manifest) { m.Digest = "sha256:" + strings.Repeat("0", 64) },
		func(m *Manifest) {
			m.Version = "2"
			m.Tag = "launcher-2"
			m.URL = service.BaseURL + "/api/v1/releases/launcher-2/download/gta-sa.exe"
		},
	} {
		bad := m
		change(&bad)
		if _, err := Download(context.Background(), client(body, -1), cache, bad, nil); err == nil {
			t.Fatal("accepted wrong checksum/version")
		}
	}
	fake := []byte("MZ not actually a launcher")
	if _, err := Download(context.Background(), client(fake, -1), cache, manifest(fake), nil); err == nil {
		t.Fatal("accepted fake PE")
	}
	if _, err := Download(context.Background(), client(append(body, 'x'), -1), cache, m, nil); err == nil {
		t.Fatal("accepted oversized body")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Download(ctx, client(body, -1), cache, m, nil); err == nil {
		t.Fatal("ignored cancellation")
	}
	entries, _ := filepath.Glob(filepath.Join(cache, "*"))
	if len(entries) != 0 {
		t.Fatalf("failed downloads leaked: %v", entries)
	}
}

func TestVersionComparisonNeverDowngrades(t *testing.T) {
	if !Newer("2", "1") || !Newer("10", "2") || Newer("1", "2") || Newer("1", "1") || Newer("v0.3.0", "1") || Newer("invalid", "1") {
		t.Fatal("invalid version comparison")
	}
}
