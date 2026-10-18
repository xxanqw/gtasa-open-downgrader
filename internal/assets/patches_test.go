//go:build !offline && !launcher

package assets_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestPatchDirectoryRejectsStaleCache(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	cache := filepath.Join(work, "cache")
	root := filepath.Join(cache, "Patches")
	m := game.Manifest{Version: "1.0", Files: []game.File{{Path: "gta_sa.exe", Action: "copy", SourceHash: testutil.Hash("source"), TargetHash: testutil.Hash("target")}}}
	data, _ := json.Marshal(m)
	testutil.Write(t, root, "manifest.json", string(data))
	testutil.Write(t, root, "gta_sa.exe", "target")
	if _, err := assets.PatchDirectory(context.Background(), cache); err == nil {
		t.Fatal("selected stale Global-only cache")
	}
	if _, err := patch.ValidateAssets(root); err != nil {
		t.Fatalf("legacy cache unexpectedly removed: %v", err)
	}
	m.Version = "2.0"
	m.ReleaseID = patch.ReleaseID
	m.SourceVersions = []game.SourceVersion{{ID: "steam_r2_global", Name: "Global", EXEHash: testutil.Hash("source")}, {ID: "steam_r2_german", Name: "German", EXEHash: testutil.Hash("german")}}
	m.Files[0].Sources = []game.PatchSource{{Hash: testutil.Hash("source")}, {Hash: testutil.Hash("german")}}
	m.Files[0].ApplicableVersions = []string{"steam_r2_global", "steam_r2_german"}
	data, _ = json.Marshal(m)
	testutil.Write(t, root, "manifest.json", string(data))
	if _, err := assets.PatchDirectory(context.Background(), cache); err == nil {
		t.Fatal("selected stale v2 cache with otherwise complete release metadata")
	}
	m.Version = "2.1"
	data, _ = json.Marshal(m)
	testutil.Write(t, root, "manifest.json", string(data))
	selected, err := assets.PatchDirectory(context.Background(), cache)
	if err != nil || selected != root {
		t.Fatalf("selected %q, %v; want %q", selected, err, root)
	}
}
