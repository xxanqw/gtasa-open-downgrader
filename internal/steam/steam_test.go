package steam

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

const testAppID = uint32(12120)

type steamFixture struct {
	root, library, game, prefix string
}

func newSteamFixture(t *testing.T) steamFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Steam")
	library := filepath.Join(t.TempDir(), "Extra Library")
	mkdir(t, filepath.Join(root, "steamapps"))
	mkdir(t, filepath.Join(root, "config"))
	mkdir(t, filepath.Join(library, "steamapps", "common", "Grand Theft Auto San Andreas"))
	mkdir(t, filepath.Join(library, "steamapps", "compatdata", "12120"))
	testutil.Write(t, root, "steamapps/libraryfolders.vdf", fmt.Sprintf(`
"libraryfolders"
{
    "0" { "path" %q "apps" {} }
    "1" { "path" %q "apps" { "12120" "12345" } }
}`, root, library))
	testutil.Write(t, library, "steamapps/appmanifest_12120.acf", `
"AppState"
{
    "appid" "12120"
    "name" "Grand Theft Auto: San Andreas"
    "installdir" "Grand Theft Auto San Andreas"
}`)
	return steamFixture{
		root: root, library: library,
		game:   filepath.Join(library, "steamapps", "common", "Grand Theft Auto San Andreas"),
		prefix: filepath.Join(library, "steamapps", "compatdata", "12120"),
	}
}

func (fixture steamFixture) configure(t *testing.T, app, tool string) {
	t.Helper()
	testutil.Write(t, fixture.root, "config/config.vdf", fmt.Sprintf(`
"InstallConfigStore" { "Software" { "Valve" { "Steam" {
    "CompatToolMapping" { %q { "name" %q "priority" "250" } }
} } } }`, app, tool))
}

func (fixture steamFixture) addTool(t *testing.T, relative, internal, display string, priority int) string {
	t.Helper()
	directory := filepath.Join(fixture.root, filepath.FromSlash(relative))
	testutil.Write(t, directory, "compatibilitytool.vdf", fmt.Sprintf(`
"compatibilitytools" { "compat_tools" {
    %q { "install_path" "." "display_name" %q "priority" %q }
} }`, internal, display, fmt.Sprint(priority)))
	proton := testutil.Write(t, directory, "proton", "#!/bin/sh\n")
	if err := os.Chmod(proton, 0755); err != nil {
		t.Fatal(err)
	}
	return proton
}

func TestFindSteamRootsDetectsNativeAndFlatpakAndDeduplicatesAliases(t *testing.T) {
	home := t.TempDir()
	native := filepath.Join(home, ".local", "share", "Steam")
	flatpak := filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam")
	mkdir(t, filepath.Join(native, "steamapps"))
	mkdir(t, filepath.Join(flatpak, "steamapps"))
	mkdir(t, filepath.Join(home, ".steam"))
	if err := os.Symlink(native, filepath.Join(home, ".steam", "root")); err != nil {
		t.Fatal(err)
	}
	roots := FindSteamRoots(home)
	canonical := make([]string, 0, len(roots))
	for _, root := range roots {
		canonical = append(canonical, canonicalPath(root))
	}
	if len(roots) != 2 || !slices.Contains(canonical, canonicalPath(native)) || !slices.Contains(canonical, canonicalPath(flatpak)) {
		t.Fatalf("Steam roots = %q", roots)
	}
}

func TestLibraryRootsParsesModernAndLegacyEntries(t *testing.T) {
	root, modern, legacy := t.TempDir(), t.TempDir(), t.TempDir()
	mkdir(t, filepath.Join(root, "steamapps"))
	testutil.Write(t, root, "steamapps/libraryfolders.vdf", fmt.Sprintf(`
"libraryfolders" {
    "0" { "path" %q }
    "1" { "path" %q }
    "2" %q
}`, root, modern, legacy))
	libraries, err := LibraryRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{root, modern, legacy} {
		if !slices.Contains(libraries, want) {
			t.Fatalf("missing library %q in %q", want, libraries)
		}
	}
}

func TestDetectProtonContextUsesExplicitCustomToolAndGameLibrary(t *testing.T) {
	fixture := newSteamFixture(t)
	proton := fixture.addTool(t, "compatibilitytools.d/Local-Custom-Build", "custom_internal_id", "My Local Proton", 250)
	fixture.addTool(t, "steamapps/common/Proton 9.0", "proton_9", "Proton 9.0-4", 250)
	fixture.configure(t, "12120", "custom_internal_id")

	context, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
	if err != nil {
		t.Fatal(err)
	}
	if context.GameDir != fixture.game || context.LibraryRoot != fixture.library || context.CompatDataPath != fixture.prefix {
		t.Fatalf("wrong application paths: %+v", context)
	}
	if context.ToolName != "custom_internal_id" || context.ToolDisplayName != "My Local Proton" || context.ProtonExecutable != proton || context.Selection != "per-game mapping" {
		t.Fatalf("explicit custom tool not retained: %+v", context)
	}
}

func TestDetectProtonContextUsesSteamDefaultValveTool(t *testing.T) {
	fixture := newSteamFixture(t)
	proton := fixture.addTool(t, "steamapps/common/Proton 9.0", "proton_9", "Proton 9.0-4", 250)
	fixture.configure(t, "0", "proton_9")
	context, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
	if err != nil {
		t.Fatal(err)
	}
	if context.ProtonExecutable != proton || context.Selection != "Steam default mapping" {
		t.Fatalf("Steam default was not selected: %+v", context)
	}
}

func TestDetectProtonContextFallsBackByToolPriorityThenVersion(t *testing.T) {
	fixture := newSteamFixture(t)
	fixture.addTool(t, "steamapps/common/Proton 9.0", "proton_9", "Proton 9", 100)
	want := fixture.addTool(t, "compatibilitytools.d/Proton-CachyOS", "proton-cachyos-10", "Proton-CachyOS 10", 300)
	context, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
	if err != nil {
		t.Fatal(err)
	}
	if context.ProtonExecutable != want || context.Selection != "installed-tool fallback" {
		t.Fatalf("metadata fallback = %+v", context)
	}
}

func TestCompatibilityToolSelectionUsesMetadataNames(t *testing.T) {
	tools := []CompatibilityTool{
		{Name: "proton_experimental", DisplayName: "Proton Experimental"},
		{Name: "GE-Proton9-27", DisplayName: "GE-Proton 9-27"},
		{Name: "proton-cachyos-10.0", DisplayName: "Proton-CachyOS 10.0"},
		{Name: "personal_build_42", DisplayName: "A locally built Proton"},
	}
	for _, selected := range []string{"proton_experimental", "GE-Proton9-27", "proton-cachyos-10.0", "personal_build_42"} {
		t.Run(selected, func(t *testing.T) {
			tool, source, err := selectCompatibilityTool(testAppID, map[string]compatMapping{"12120": {Name: selected}}, tools)
			if err != nil {
				t.Fatal(err)
			}
			if tool.Name != selected || source != "per-game mapping" {
				t.Fatalf("selected %+v through %q", tool, source)
			}
		})
	}
}

func TestDirectCustomToolManifestCanRedirectToAbsoluteInstallPath(t *testing.T) {
	fixture := newSteamFixture(t)
	toolRoot := filepath.Join(t.TempDir(), "outside Steam", "My Proton")
	proton := testutil.Write(t, toolRoot, "proton", "#!/bin/sh\n")
	if err := os.Chmod(proton, 0755); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, fixture.root, "compatibilitytools.d/compatibilitytool.vdf", fmt.Sprintf(`
"compatibilitytools" { "compat_tools" {
    "redirected_custom_tool" { "install_path" %q "display_name" "Redirected Proton" }
} }`, toolRoot))
	fixture.configure(t, "12120", "redirected_custom_tool")

	context, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
	if err != nil {
		t.Fatal(err)
	}
	if context.ToolRoot != toolRoot || context.ProtonExecutable != proton {
		t.Fatalf("redirected tool = %+v", context)
	}
}

func TestDetectProtonContextErrors(t *testing.T) {
	t.Run("Steam missing", func(t *testing.T) {
		_, err := NewDetector(filepath.Join(t.TempDir(), "missing")).DetectProtonContext(testAppID)
		if !errors.Is(err, ErrSteamNotFound) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("app manifest missing", func(t *testing.T) {
		root := t.TempDir()
		mkdir(t, filepath.Join(root, "steamapps"))
		_, err := NewDetector(root).DetectProtonContext(testAppID)
		if !errors.Is(err, ErrAppNotInstalled) || !strings.Contains(err.Error(), "appmanifest_12120.acf") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("prefix missing", func(t *testing.T) {
		fixture := newSteamFixture(t)
		if err := os.Remove(fixture.prefix); err != nil {
			t.Fatal(err)
		}
		_, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
		if !errors.Is(err, ErrPrefixMissing) || !strings.Contains(err.Error(), fixture.prefix) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("configured tool missing", func(t *testing.T) {
		fixture := newSteamFixture(t)
		fixture.configure(t, "12120", "GE-Proton-does-not-exist")
		_, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
		if !errors.Is(err, ErrToolMissing) || !strings.Contains(err.Error(), "GE-Proton-does-not-exist") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("malformed Steam config", func(t *testing.T) {
		fixture := newSteamFixture(t)
		fixture.addTool(t, "steamapps/common/Proton Experimental", "proton_experimental", "Proton Experimental", 250)
		testutil.Write(t, fixture.root, "config/config.vdf", `"InstallConfigStore" { "broken"`)
		_, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
		if err == nil || !strings.Contains(err.Error(), "config.vdf") || !strings.Contains(err.Error(), "parse") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestProtonCommandUsesExistingPrefixAndSelectedToolEnvironment(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "compatdata", "12120")
	mkdir(t, prefix)
	proton := testutil.Write(t, root, "tool/proton", "#!/bin/sh\n")
	if err := os.Chmod(proton, 0755); err != nil {
		t.Fatal(err)
	}
	exe := testutil.Write(t, root, "installer/setup.exe", "MZ")
	contextValue := ProtonContext{
		AppID: 12120, SteamRoot: filepath.Join(root, "Steam"), CompatDataPath: prefix,
		ToolDisplayName: "GE-Proton", ProtonExecutable: proton,
	}
	command, err := contextValue.Command(context.Background(), exe, "/silent")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(command.Args, []string{proton, "run", exe, "/silent"}) || command.Dir != filepath.Dir(exe) {
		t.Fatalf("command = %#v, dir = %q", command.Args, command.Dir)
	}
	environment := envMap(contextValue.Environment([]string{"PATH=/bin", "SteamAppId=old", "STEAM_COMPAT_DATA_PATH=old"}))
	for key, want := range map[string]string{
		"PATH": "/bin", "SteamAppId": "12120", "SteamGameId": "12120", "STEAM_COMPAT_APP_ID": "12120",
		"STEAM_COMPAT_DATA_PATH": prefix, "STEAM_COMPAT_CLIENT_INSTALL_PATH": contextValue.SteamRoot,
	} {
		if environment[key] != want {
			t.Errorf("%s = %q, want %q", key, environment[key], want)
		}
	}
	if strings.Contains(strings.Join(contextValue.Environment([]string{"SteamAppId=old"}), "\n"), "SteamAppId=old") {
		t.Fatal("old Proton environment was retained")
	}
}

func mkdir(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func envMap(values []string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		key, value, _ := strings.Cut(value, "=")
		result[key] = value
	}
	return result
}
