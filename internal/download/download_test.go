package download

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestResolveICloud(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GUIDs []struct {
				Value string `json:"value"`
			} `json:"shortGUIDs"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		if r.Method != "POST" || len(body.GUIDs) != 1 || body.GUIDs[0].Value != "example" {
			t.Error("wrong iCloud request")
		}
		w.Write([]byte(`{"results":[{"rootRecord":{"fields":{"fileContent":{"value":{"downloadURL":"https://example.com/archive.zip"}}}}}]}`))
	}))
	defer s.Close()
	c := New()
	c.ICloudAPI = s.URL
	u, e := c.Resolve(context.Background(), "https://www.icloud.com/iclouddrive/example#Patches")
	if e != nil || u != "https://example.com/archive.zip" {
		t.Fatal(u, e)
	}
}
func TestDownloadCancellationAndHTTPFailurePreserveDestination(t *testing.T) {
	root := t.TempDir()
	dst := testutil.Write(t, root, "archive", "original")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Length", "99999999")
		w.Write([]byte(strings.Repeat("x", 4096)))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer s.Close()
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	e := c.File(ctx, s.URL, dst, func(d, total int64) { cancel() })
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "archive") != "original" {
		t.Fatal("cancelled download replaced destination")
	}
	if e = c.File(context.Background(), s.URL+"/fail", dst, nil); e == nil {
		t.Fatal("HTTP failure accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("download temporary file leaked")
	}
}
func TestTruncatedDownloadRejected(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("short"))
	}))
	defer s.Close()
	dst := filepath.Join(t.TempDir(), "archive")
	if e := New().File(context.Background(), s.URL, dst, nil); e == nil {
		t.Fatal("truncated response accepted")
	}
	if _, e := os.Stat(dst); !os.IsNotExist(e) {
		t.Fatal("partial download committed")
	}
}
func TestFolderResponseRejected(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"rootRecord":{"recordType":"structure"}}]}`))
	}))
	defer s.Close()
	c := New()
	c.ICloudAPI = s.URL
	if _, e := c.Resolve(context.Background(), "https://icloud.com/iclouddrive/folder"); e == nil {
		t.Fatal("folder accepted")
	}
}
