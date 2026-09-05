package testutil

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func Hash(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) }
func Write(t testing.TB, root, rel, data string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(data), 0644); e != nil {
		t.Fatal(e)
	}
	return p
}
func Read(t testing.TB, root, rel string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func Zip(t testing.TB, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "archive.zip")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for name, data := range files {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(data)); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return p
}
