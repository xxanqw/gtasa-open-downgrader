package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
)

func TestDownloadTraceExplainsHTTPFailuresAndTruncation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "metadata-123")
		if r.URL.Path == "/missing" {
			w.WriteHeader(404)
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/short?signature=secret", http.StatusFound)
			return
		}
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("short"))
	}))
	defer s.Close()
	j := &applog.Journal{}
	client := New()
	for _, path := range []string{"/missing?token=secret", "/redirect"} {
		if err := client.File(j.Context(context.Background()), s.URL+path, filepath.Join(t.TempDir(), "archive"), nil); err == nil {
			t.Fatal("expected failure")
		}
	}
	text, _ := j.Snapshot()
	for _, expected := range []string{"status=404", `request_id="metadata-123"`, "/missing", "http.redirect", "received=5 expected=100", "unexpected EOF", "duration_ms="} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in %s", expected, text)
		}
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "signature=") || strings.Contains(text, "token=") {
		t.Fatal("signed URL leaked", text)
	}
}
