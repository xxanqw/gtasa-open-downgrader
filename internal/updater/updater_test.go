package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
)

func TestVersionComparison(t *testing.T) {
	for _, c := range []struct {
		candidate, current string
		want               bool
	}{{"v0.2.1", "v0.2.0", true}, {"v0.2.0", "v0.2.0", false}, {"v0.1.1", "v0.2.0", false}, {"latest", "v0.2.0", false}, {"v0.3.0-rc.1", "v0.3.0", false}, {"v0.3.0", "v0.3.0-rc.1", true}} {
		if Newer(c.candidate, c.current) != c.want {
			t.Fatal(c)
		}
	}
}
func TestExactPlatformAndVariantSelection(t *testing.T) {
	r := Release{Assets: []Asset{{Name: "gtasa-open-downgrader-windows-offline.exe"}, {Name: "gtasa-open-downgrader-windows.exe"}, {Name: "gtasa-open-downgrader-linux-offline.AppImage"}, {Name: "gtasa-open-downgrader-linux.AppImage"}, {Name: "gtasa-open-downgrader-linux"}}}
	for _, c := range []struct {
		system            string
		offline, appimage bool
		want              string
	}{{"windows", false, false, "gtasa-open-downgrader-windows.exe"}, {"windows", true, false, "gtasa-open-downgrader-windows-offline.exe"}, {"linux", true, true, "gtasa-open-downgrader-linux-offline.AppImage"}, {"linux", false, false, "gtasa-open-downgrader-linux"}} {
		a, ok := Select(r, c.system, c.offline, c.appimage)
		if !ok || a.Name != c.want {
			t.Fatal(a, c)
		}
	}
	if _, ok := Select(Release{Assets: []Asset{{Name: "gtasa-open-downgrader-windows-offline.exe"}}}, "windows", false, false); ok {
		t.Fatal("fell back to wrong variant")
	}
}
func TestPrepareChecksDigestAndPreservesRunningBinary(t *testing.T) {
	body := []byte("MZ\x00\x00new executable")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer s.Close()
	root := t.TempDir()
	target := filepath.Join(root, "app.exe")
	os.WriteFile(target, []byte("old"), 0755)
	a := Asset{URL: s.URL, Size: int64(len(body)), Digest: "sha256:bad"}
	if _, e := Prepare(context.Background(), download.New(), a, target, nil); e == nil {
		t.Fatal("bad digest accepted")
	}
	a.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	staged, e := Prepare(context.Background(), download.New(), a, target, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(target)
	if string(b) != "old" {
		t.Fatal("running binary changed before helper")
	}
	b, _ = os.ReadFile(staged)
	if string(b) != string(body) {
		t.Fatal("bad staged update")
	}
}
