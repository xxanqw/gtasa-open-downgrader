package fsutil

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestUnsafePaths(t *testing.T) {
	for _, p := range []string{"../out", "nested/../../out", "/absolute", `C:\out`, `..\out`, "x:stream", "CON.txt", "dir/NUL", "foo. ", ""} {
		t.Run(p, func(t *testing.T) {
			if _, e := Relative(p); e == nil {
				t.Fatalf("accepted %q", p)
			}
		})
	}
}
func TestWriteCancellationPreservesDestination(t *testing.T) {
	root := t.TempDir()
	dst := testutil.Write(t, root, "file", "original")
	ctx, cancel := context.WithCancel(context.Background())
	err := Write(ctx, dst, 0644, func(w io.Writer) error { w.Write([]byte("replacement")); cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if testutil.Read(t, root, "file") != "original" {
		t.Fatal("overwrote original")
	}
	files, _ := os.ReadDir(root)
	if len(files) != 1 {
		t.Fatal("temporary files leaked")
	}
}
func TestExtractionRejectsSymlink(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(os.ModeSymlink | 0777)
	w, _ := z.CreateHeader(h)
	w.Write([]byte("../outside"))
	z.Close()
	f.Close()
	if e := Extract(context.Background(), p, t.TempDir(), "zip", nil); e == nil {
		t.Fatal("accepted symlink")
	}
}
func TestExtractionChecksFilteredPaths(t *testing.T) {
	p := testutil.Zip(t, map[string]string{"../bad": "x"})
	if e := Extract(context.Background(), p, t.TempDir(), "zip", func(string) string { return "" }); e == nil {
		t.Fatal("filtered unsafe entry was accepted")
	}
}
func TestJoinRejectsExistingLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink privilege varies")
	}
	root := t.TempDir()
	if e := os.Symlink(t.TempDir(), filepath.Join(root, "models")); e != nil {
		t.Fatal(e)
	}
	if _, e := Join(root, "models/test"); e == nil {
		t.Fatal("followed link")
	}
}
func TestExtractionRejectsDuplicateMappedTargets(t *testing.T) {
	p := testutil.Zip(t, map[string]string{"a/x": "a", "b/x": "b"})
	if e := Extract(context.Background(), p, t.TempDir(), "zip", func(string) string { return "same" }); e == nil {
		t.Fatal("duplicate output accepted")
	}
}
