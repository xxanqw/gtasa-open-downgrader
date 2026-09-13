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
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestUpdateInstalledDownloadsAndMergesConfig(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ModLoader"}, Versions: map[string]Release{"ModLoader": {Version: "old", Fingerprint: "old"}}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "modloader/config.ini", "setting=personal\n")
	archive := testutil.Zip(t, map[string]string{"modloader/config.ini": "setting=default\nfeature=on\n", "modloader.asi": "new binary"})
	packageBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(packageBytes)
	}))
	defer server.Close()
	installer := Installer{
		Client: download.New(), AppVersion: "app",
		Resolve: func(context.Context, string) (Release, error) {
			return Release{Version: "new", Fingerprint: "new", URL: server.URL, ArchiveType: "zip"}, nil
		},
	}
	result, err := installer.UpdateInstalled(context.Background(), root, nil)
	if err != nil || result.Updated != 1 {
		t.Fatalf("update failed: %+v, %v", result, err)
	}
	if got := testutil.Read(t, root, "modloader/config.ini"); !strings.Contains(got, "setting=personal") || !strings.Contains(got, "feature=on") || strings.Contains(got, "setting=default") {
		t.Fatalf("config was not merged: %q", got)
	}
	if testutil.Read(t, root, "modloader.asi") != "new binary" {
		t.Fatal("new package binary was not installed")
	}
}

func TestUpdateInstalledChangesOnlyTrackedOutdatedMods(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch", "GInput"}, Versions: map[string]Release{
		"ASI Loader":  {Version: "old", Fingerprint: "same"},
		"ModLoader":   {Version: "old", Fingerprint: "old"},
		"SilentPatch": {Version: "old", Fingerprint: "old"},
		"GInput":      {Version: "manual import"},
	}}
	if err := tracker.Save(context.Background(), root, "old-app"); err != nil {
		t.Fatal(err)
	}
	var installed, resolved []string
	installer := Installer{
		AppVersion: "new-app",
		Resolve: func(_ context.Context, name string) (Release, error) {
			resolved = append(resolved, name)
			fingerprint := "new"
			if name == "ASI Loader" {
				fingerprint = "same"
			}
			return Release{Version: "latest", Fingerprint: fingerprint}, nil
		},
		Install: func(_ context.Context, _, name string, _ Release) error {
			installed = append(installed, name)
			return nil
		},
	}
	result, err := installer.UpdateInstalled(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != (UpdateResult{Updated: 2, Current: 1, Untracked: 1}) {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !reflect.DeepEqual(installed, []string{"ModLoader", "SilentPatch"}) || !reflect.DeepEqual(resolved, []string{"ASI Loader", "ModLoader", "SilentPatch"}) {
		t.Fatalf("updated %v, resolved %v", installed, resolved)
	}
	after, err := LoadTracker(root)
	if err != nil {
		t.Fatal(err)
	}
	if after.Versions["ModLoader"].Fingerprint != "new" || after.Versions["SilentPatch"].InstalledAt == "" || after.Versions["GInput"].Version != "manual import" {
		t.Fatalf("tracker not updated selectively: %+v", after.Versions)
	}
}

func TestUpdateInstalledContinuesAfterReleaseCheckFailure(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader"}, Versions: map[string]Release{
		"ASI Loader": {Fingerprint: "old"}, "ModLoader": {Fingerprint: "old"},
	}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	installer := Installer{
		Resolve: func(_ context.Context, name string) (Release, error) {
			if name == "ASI Loader" {
				return Release{}, errors.New("unavailable")
			}
			return Release{Fingerprint: "new"}, nil
		},
		Install: func(context.Context, string, string, Release) error { return nil },
	}
	result, err := installer.UpdateInstalled(context.Background(), root, nil)
	if result.Updated != 1 || err == nil || !strings.Contains(err.Error(), "ASI Loader") {
		t.Fatalf("partial update not reported: %+v, %v", result, err)
	}
	after, err := LoadTracker(root)
	if err != nil || after.Versions["ModLoader"].Fingerprint != "new" || after.Versions["ASI Loader"].Fingerprint != "old" {
		t.Fatalf("partial update lost: %+v, %v", after.Versions, err)
	}
}

func TestUpdateCancellationKeepsCompletedPackage(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader"}, Versions: map[string]Release{
		"ASI Loader": {Fingerprint: "old"}, "ModLoader": {Fingerprint: "old"},
	}}
	if err := tracker.Save(context.Background(), root, "app"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []string
	installer := Installer{
		Resolve: func(context.Context, string) (Release, error) { return Release{Fingerprint: "new"}, nil },
		Install: func(_ context.Context, _, name string, _ Release) error {
			calls = append(calls, name)
			cancel()
			return nil
		},
	}
	result, err := installer.UpdateInstalled(ctx, root, nil)
	if !errors.Is(err, context.Canceled) || result.Updated != 1 || !reflect.DeepEqual(calls, []string{"ASI Loader"}) {
		t.Fatalf("cancellation did not stop after first package: %+v, %v, %v", result, calls, err)
	}
	after, err := LoadTracker(root)
	if err != nil || after.Versions["ASI Loader"].Fingerprint != "new" || after.Versions["ModLoader"].Fingerprint != "old" {
		t.Fatalf("completed update not retained: %+v, %v", after.Versions, err)
	}
}

func TestExtractUpdateMergesINIAndPreservesOtherConfigs(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader/config.ini", "; local comment\r\n[Settings]\r\nquality=custom\r\n[Settings]\r\nlocal=yes\r\n")
	testutil.Write(t, root, "modloader/user.json", `{"custom":true}`)
	testutil.Write(t, root, "modloader.asi", "old binary")
	testutil.Write(t, root, "modloader/unchanged.dat", "same")
	unchanged := filepath.Join(root, "modloader/unchanged.dat")
	date := time.Now().Add(-time.Hour)
	if err := os.Chtimes(unchanged, date, date); err != nil {
		t.Fatal(err)
	}
	archive := testutil.Zip(t, map[string]string{
		"modloader/config.ini":    "[Settings]\nquality=upstream\nnewOption=enabled\n[Extra]\nvalue=added\n",
		"modloader/user.json":     `{"custom":false,"new":1}`,
		"modloader.asi":           "new binary",
		"modloader/unchanged.dat": "same",
		"modloader/new.cfg":       "fresh=1",
	})
	if err := ExtractUpdate(context.Background(), root, "ModLoader", archive, "zip"); err != nil {
		t.Fatal(err)
	}
	ini := testutil.Read(t, root, "modloader/config.ini")
	for _, want := range []string{"; local comment\r\n", "quality=custom", "local=yes", "newOption=enabled", "[Extra]\r\nvalue=added"} {
		if !strings.Contains(ini, want) {
			t.Fatalf("merged INI missing %q: %q", want, ini)
		}
	}
	if strings.Count(ini, "newOption=enabled") != 1 || strings.Contains(ini, "quality=upstream") {
		t.Fatalf("duplicated new key or replaced local value: %q", ini)
	}
	if testutil.Read(t, root, "modloader/user.json") != `{"custom":true}` || testutil.Read(t, root, "modloader.asi") != "new binary" || testutil.Read(t, root, "modloader/new.cfg") != "fresh=1" {
		t.Fatal("update did not preserve config or replace package files")
	}
	info, err := os.Stat(unchanged)
	if err != nil || !info.ModTime().Equal(date) {
		t.Fatalf("unchanged package file was rewritten: %v", err)
	}
}

func TestUpdatePreservesUnrecognizedINI(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader/config.ini", "local=value\n")
	archive := testutil.Zip(t, map[string]string{"modloader/config.ini": "include another.ini\nnew=value\n"})
	if err := ExtractUpdate(context.Background(), root, "ModLoader", archive, "zip"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, "modloader/config.ini"); got != "local=value\n" {
		t.Fatalf("unknown config syntax overwritten: %q", got)
	}
}
