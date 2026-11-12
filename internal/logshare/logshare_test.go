package logshare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
)

func TestUploadValidatesReceiptAndIssueDraft(t *testing.T) {
	text := "2026-11-12 [ERROR] example\n"
	var mode atomic.Value
	mode.Store("ok")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != text || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") || r.Method != "POST" {
			t.Error("wrong log request")
		}
		digest := sha256.Sum256(body)
		link := server.URL + "/logs/abcdefghijklmnopqrstuvwx"
		if mode.Load() == "foreign" {
			link = "https://other.example/logs/abcdefghijklmnopqrstuvwx"
		}
		receipt := Receipt{URL: link, DownloadURL: link + "/download", SHA256: hex.EncodeToString(digest[:]), Size: len(body), ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}
		if mode.Load() == "hash" {
			receipt.SHA256 = "wrong"
		}
		if mode.Load() == "expired" {
			receipt.ExpiresAt = time.Now().Add(-time.Hour)
		}
		json.NewEncoder(w).Encode(receipt)
	}))
	defer server.Close()
	client := download.New()
	for _, candidate := range []string{"ok", "foreign", "hash", "expired"} {
		mode.Store(candidate)
		receipt, err := Upload(context.Background(), client, server.URL+"/logs/upload", text)
		if candidate != "ok" {
			if err == nil {
				t.Fatal("accepted invalid receipt", candidate)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		u := IssueDraft("v-test", "linux/amd64", receipt.URL)
		if u.Host != "github.com" || !strings.Contains(u.Query().Get("body"), receipt.URL) || !strings.Contains(u.Query().Get("body"), "30 days") {
			t.Fatal("issue link not prefilled", u)
		}
	}
	if !strings.Contains(IssueDraft("v-test", "linux", "").Query().Get("body"), "Attach the exported") {
		t.Fatal("missing manual attachment path")
	}
}

func TestUploadDoesNotFollowRedirect(t *testing.T) {
	var received atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Store(true) }))
	defer other.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	_, err := Upload(context.Background(), download.New(), source.URL, "private log")
	if err == nil || received.Load() {
		t.Fatal("upload followed redirect", err)
	}
}
