package mods

import (
	"context"
	"errors"
	"testing"
)

func TestBrowserPreparationIncludesDependenciesAndHelpersBeforeInstall(t *testing.T) {
	root := t.TempDir()
	prepared, cleaned := false, false
	resolved := map[string]int{}
	installer := Installer{
		Resolve: func(ctx context.Context, name string) (Release, error) {
			resolved[name]++
			return NewResolver(nil).Resolve(ctx, name)
		},
		PrepareBrowserDownloads: func(ctx context.Context, archives []BrowserArchive) (BrowserDownloadFunc, func(), error) {
			prepared = true
			found := map[string]BrowserArchive{}
			for _, archive := range archives {
				found[archive.Name] = archive
			}
			for _, name := range []string{HDRadarMap, AtmosphereHUD, OriginalHQIcons, NadalaoWeapons, NadalaoWeapons + " helper"} {
				if found[name].URL == "" || found[name].Checksum == "" {
					t.Fatalf("missing reviewed archive: %s", name)
				}
			}
			if found[HDRadarMap].URL != found[AtmosphereHUD].URL {
				t.Fatal("expected shared archive")
			}
			return nil, func() { cleaned = true }, nil
		},
		Install: func(context.Context, string, string, Release) error {
			if !prepared {
				t.Fatal("installed before preparation")
			}
			return nil
		},
	}
	if err := installer.InstallSelected(context.Background(), root, []string{HDRadarMap, AtmosphereHUD, NadalaoWeapons}, false, nil); err != nil {
		t.Fatal(err)
	}
	if !cleaned {
		t.Fatal("session cleanup was not called")
	}
	for name, count := range resolved {
		if count != 1 {
			t.Fatalf("%s resolved %d times", name, count)
		}
	}
	// Already installed packages should not trigger another preparation.
	installer.PrepareBrowserDownloads = func(context.Context, []BrowserArchive) (BrowserDownloadFunc, func(), error) {
		t.Fatal("prepared installed archives")
		return nil, nil, nil
	}
	if err := installer.InstallSelected(context.Background(), root, []string{HDRadarMap, AtmosphereHUD, NadalaoWeapons}, false, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserPreparationCancellationDoesNotInstall(t *testing.T) {
	installer := Installer{
		Resolve: func(ctx context.Context, name string) (Release, error) { return NewResolver(nil).Resolve(ctx, name) },
		PrepareBrowserDownloads: func(context.Context, []BrowserArchive) (BrowserDownloadFunc, func(), error) {
			return nil, nil, context.Canceled
		},
		Install: func(context.Context, string, string, Release) error {
			t.Fatal("installed after preparation cancelled")
			return nil
		},
	}
	if err := installer.InstallSelected(context.Background(), t.TempDir(), []string{NadalaoWeapons}, false, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestBrowserUpdatePreparesOnlyChangedSelectedPackages(t *testing.T) {
	root := t.TempDir()
	tracker := Tracker{Schema: 2, Installed: []string{HDRadarMap, OriginalHQIcons, NadalaoWeapons}, Versions: map[string]Release{
		HDRadarMap: {Fingerprint: "old"}, OriginalHQIcons: {Fingerprint: "current"}, NadalaoWeapons: {Fingerprint: "old"},
	}}
	if err := tracker.Save(context.Background(), root, "test"); err != nil {
		t.Fatal(err)
	}
	installed := 0
	installer := Installer{
		Resolve: func(ctx context.Context, name string) (Release, error) {
			if name == NadalaoWeapons {
				t.Fatal("resolved unselected mod")
			}
			rel, err := NewResolver(nil).Resolve(ctx, name)
			if name == OriginalHQIcons {
				rel.Fingerprint = "current"
			}
			return rel, err
		},
		PrepareBrowserDownloads: func(ctx context.Context, archives []BrowserArchive) (BrowserDownloadFunc, func(), error) {
			if len(archives) != 1 || archives[0].Name != HDRadarMap {
				t.Fatalf("wrong update archives: %+v", archives)
			}
			return nil, nil, nil
		},
		Install: func(ctx context.Context, root, name string, rel Release) error {
			installed++
			if name != HDRadarMap {
				t.Fatal(name)
			}
			return nil
		},
	}
	result, err := installer.UpdateSelected(context.Background(), root, []string{HDRadarMap, OriginalHQIcons}, nil)
	if err != nil || result.Updated != 1 || result.Current != 1 || installed != 1 {
		t.Fatalf("result=%+v installed=%d error=%v", result, installed, err)
	}
}
