package game

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestScanStatusesAndExecutableAlias(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "gta-sa.exe", "original")
	testutil.Write(t, root, "models/done", "target")
	testutil.Write(t, root, "models/custom", "changed")
	m := Manifest{Files: []File{{Path: "gta_sa.exe", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "models/done", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "models/custom", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}, {Path: "missing", SourceHash: testutil.Hash("original"), TargetHash: testutil.Hash("target")}}}
	r, e := Inspect(context.Background(), root, m, nil)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"Original (Needs Patch)", "Already Downgraded", "Modified", "Missing"}
	for i, row := range r.Rows {
		if row.Status != want[i] {
			t.Errorf("row %d: %s", i, row.Status)
		}
	}
	if r.ReadOnly {
		t.Fatal("temporary game should be writable")
	}
}
func TestManifestRejectsInvalidInputs(t *testing.T) {
	for _, f := range []File{{Path: "../bad", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}, {Path: "a", SourceHash: "wrong", TargetHash: testutil.Hash("b")}, {Path: "backups/a", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}, {Path: "a", Action: "delete", SourceHash: testutil.Hash("a"), TargetHash: testutil.Hash("b")}} {
		m := Manifest{Files: []File{f}}
		if e := m.Validate(); e == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
	if e := (&Manifest{}).Validate(); e == nil {
		t.Fatal("accepted empty manifest")
	}
}
func TestManifestAcceptsGeneratorMissingCopy(t *testing.T) {
	m := Manifest{Files: []File{{Path: "gta_sa.exe", Action: "copy", SourceHash: "MISSING", TargetHash: testutil.Hash("target")}}}
	if e := m.Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestScanCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	testutil.Write(t, root, "gta_sa.exe", "x")
	_, e := Inspect(ctx, root, Manifest{}, nil)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestUnreadableManifestIsReported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "manifest.json")
	os.WriteFile(p, []byte("invalid"), 0600)
	if _, e := Load(p); e == nil {
		t.Fatal("invalid manifest accepted")
	}
}
