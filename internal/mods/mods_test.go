package mods

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestPackageLayouts(t *testing.T) {
	cases := []struct {
		name         string
		files        map[string]string
		want, absent string
	}{
		{"ASI Loader", map[string]string{"vorbisFile.dll": "loader", "vorbisHooked.dll": "hook", "readme.txt": "ignore"}, "vorbisFile.dll", "readme.txt"},
		{"ModLoader", map[string]string{"modloader.asi": "asi", "modloader/config.ini": "settings"}, "modloader/config.ini", "none"},
		{"SilentPatch", map[string]string{"nested/SilentPatchSA.asi": "asi", "readme.txt": "ignore"}, "modloader/SilentPatch/SilentPatchSA.asi", "modloader/SilentPatch/readme.txt"},
		{"Widescreen Fixes", map[string]string{"scripts/GTASA.WidescreenFix.asi": "asi", "vorbisFile.dll": "ignore"}, "modloader/WidescreenFix/GTASA.WidescreenFix.asi", "modloader/WidescreenFix/vorbisFile.dll"},
		{"SkyGFX", map[string]string{"skygfx/skygfx.asi": "asi"}, "modloader/SkyGFX/skygfx.asi", "none"},
		{"Frontend Mods", map[string]string{"models/txd/LOADSCS.txd": "textures"}, "modloader/FrontendMods/models/txd/LOADSCS.txd", "none"},
		{Framerate, map[string]string{"GTA SA/FramerateVigilante/FramerateVigilante.ini": "sa", "GTA VC/FramerateVigilante/FramerateVigilante.ini": "vc", "GTA III/FramerateVigilante/FramerateVigilante.III.asi": "iii"}, "modloader/FramerateVigilante/FramerateVigilante.ini", "modloader/FramerateVigilante/FramerateVigilante.III.asi"},
		{"GInput", map[string]string{"GInputSA.asi": "asi", "GInputAPI (for modders)/api.h": "ignore"}, "modloader/GInput/GInputSA.asi", "modloader/GInput/GInputAPI (for modders)/api.h"},
		{"Project 2DFX", map[string]string{"SALodLights.asi": "asi"}, "modloader/Project2DFX/SALodLights.asi", "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if e := Extract(context.Background(), root, c.name, testutil.Zip(t, c.files), "zip"); e != nil {
				t.Fatal(e)
			}
			if _, e := os.Stat(filepath.Join(root, c.want)); e != nil {
				t.Fatal(e)
			}
			if _, e := os.Stat(filepath.Join(root, c.absent)); !os.IsNotExist(e) {
				t.Fatal("unexpected member installed")
			}
			if c.name == Framerate && testutil.Read(t, root, c.want) != "sa" {
				t.Fatal("wrong game's configuration")
			}
		})
	}
}
func TestSevenZipPackage(t *testing.T) {
	root := t.TempDir()
	if e := Extract(context.Background(), root, Framerate, "testdata/framerate.7z", "7z"); e != nil {
		t.Fatal(e)
	}
	if testutil.Read(t, root, "modloader/FramerateVigilante/FramerateVigilante.ini") != "sa settings" {
		t.Fatal("7z mapping failed")
	}
}
func TestUnsafeModArchiveDoesNotModifyGame(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader.asi", "original")
	archive := testutil.Zip(t, map[string]string{"modloader.asi": "new", "../outside": "bad"})
	if e := Extract(context.Background(), root, "ModLoader", archive, "zip"); e == nil {
		t.Fatal("unsafe archive accepted")
	}
	if testutil.Read(t, root, "modloader.asi") != "original" {
		t.Fatal("modified game before validation")
	}
}
func TestTrackerPreservesUnknownFieldsAndVersions(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, TrackerPath, `{"installed_mods":["ModLoader"],"custom":true}`)
	tracker, e := LoadTracker(root)
	if e != nil || !tracker.Legacy() {
		t.Fatal(e)
	}
	tracker.Installed = append(tracker.Installed, "SilentPatch")
	tracker.Versions["SilentPatch"] = Release{Version: "new", Fingerprint: "github:123"}
	tracker.Schema = 2
	if e = tracker.Save(context.Background(), root, "v0.2.2"); e != nil {
		t.Fatal(e)
	}
	reloaded, e := LoadTracker(root)
	if e != nil || reloaded.Legacy() || string(reloaded.Extra["custom"]) != "true" || reloaded.Versions["SilentPatch"].Fingerprint != "github:123" {
		t.Fatalf("%+v %v", reloaded, e)
	}
}
func TestSkipForceAndLegacyMigration(t *testing.T) {
	for _, mode := range []string{"skip", "force", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			schema := 2
			if mode == "legacy" {
				schema = 1
			}
			tracker := Tracker{Schema: schema, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch", "GInput"}, Versions: map[string]Release{}}
			if e := tracker.Save(context.Background(), root, "old"); e != nil {
				t.Fatal(e)
			}
			var calls []string
			installer := Installer{Resolve: func(context.Context, string) (Release, error) {
				return Release{Version: "new", Fingerprint: "new"}, nil
			}, Install: func(ctx context.Context, root, name string, r Release) error { calls = append(calls, name); return nil }}
			if e := installer.InstallSelected(context.Background(), root, []string{"SilentPatch"}, mode == "force", nil); e != nil {
				t.Fatal(e)
			}
			want := []string(nil)
			if mode == "force" {
				want = []string{"ASI Loader", "ModLoader", "SilentPatch"}
			}
			if mode == "legacy" {
				want = []string{"ASI Loader", "ModLoader", "SilentPatch", "GInput"}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("got %v want %v", calls, want)
			}
			loaded, e := LoadTracker(root)
			if e != nil || loaded.Legacy() {
				t.Fatal("tracker not migrated")
			}
		})
	}
}
func TestCancellationKeepsCompletedTrackerAndPendingLegacy(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, TrackerPath, `{"installed_mods":["ASI Loader","ModLoader","GInput"],"custom":true}`)
	ctx, cancel := context.WithCancel(context.Background())
	installer := Installer{Resolve: func(context.Context, string) (Release, error) { return Release{Version: "current"}, nil }, Install: func(ctx context.Context, root, name string, r Release) error { cancel(); return nil }}
	e := installer.InstallSelected(ctx, root, []string{"ASI Loader"}, false, nil)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	tracker, e := LoadTracker(root)
	if e != nil {
		t.Fatal(e)
	}
	if !tracker.Legacy() || tracker.Versions["ASI Loader"].Version != "current" {
		t.Fatalf("lost partial migration state: %+v", tracker)
	}
}
func TestGitHubResolutionFingerprintAndCache(t *testing.T) {
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write([]byte(`{"tag_name":"gtasa","assets":[{"name":"GTASA.WidescreenFix.zip","id":3,"updated_at":"2026-09-01T00:00:00Z","size":4,"browser_download_url":"https://example.com/fix.zip"},{"name":"GTASA.WidescreenFrontend.zip","id":5,"updated_at":"2026-09-02T00:00:00Z","size":6,"browser_download_url":"https://example.com/frontend.zip"}]}`))
	}))
	defer s.Close()
	r := NewResolver(download.New())
	r.GitHubAPI = s.URL
	a, e := r.Resolve(context.Background(), "Widescreen Fixes")
	if e != nil {
		t.Fatal(e)
	}
	if a.Version != "2026-09-01" || a.Fingerprint != "github:3:2026-09-01T00:00:00Z:4" {
		t.Fatalf("%+v", a)
	}
	if _, e = r.Resolve(context.Background(), "Frontend Mods"); e != nil {
		t.Fatal(e)
	}
	if requests != 1 {
		t.Fatal("shared release was fetched twice")
	}
}
func TestShareModsAllowsOfficialStorageOnly(t *testing.T) {
	body := `<a href="https://wingsmob.com/advertisement">Ad</a><a href="https://sharem.s3.de.io.cloud.ovh.net/uploads/08721/newfile123?x=1&amp;y=2">Download</a>`
	got, e := shareDownload(body, "newfile123")
	if e != nil || !strings.Contains(got, "x=1&y=2") {
		t.Fatal(got, e)
	}
	if _, e = shareDownload(body, "different"); e == nil {
		t.Fatal("accepted other file ID")
	}
	if _, e = shareDownload(`<a href="https://r2.cloudflarestorage.com.evil.com/file">x</a>`, "id"); e == nil {
		t.Fatal("accepted fake storage host")
	}
}
func TestCorruptTrackerIsNotOverwritten(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, TrackerPath, "invalid json")
	if _, e := LoadTracker(root); e == nil {
		t.Fatal("corruption hidden")
	}
}

func TestPackageCannotOverwriteTrackerWithDifferentCase(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, TrackerPath, `{"schema_version":2,"installed_mods":[]}`)
	archive := testutil.Zip(t, map[string]string{"modloader/.DOWNGRADER": "malicious"})
	if err := Extract(context.Background(), root, "ModLoader", archive, "zip"); err == nil {
		t.Fatal("case-insensitive tracker overwrite accepted")
	}
}
