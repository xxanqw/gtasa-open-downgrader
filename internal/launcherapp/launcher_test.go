package launcherapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func TestSinglePlayerUsesRealExecutableAndPassesArguments(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	var got []string
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, dir, exe string, args ...string) error {
		got = append([]string{dir, exe}, args...)
		return nil
	}}
	if err := runtime.SinglePlayer(context.Background(), "-windowed"); err != nil {
		t.Fatal(err)
	}
	want := []string{root, filepath.Join(root, entrypoint.RealGameExecutable), "-windowed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("call = %#v, want %#v", got, want)
	}
}

func TestRunningUnderProton(t *testing.T) {
	t.Setenv("STEAM_COMPAT_DATA_PATH", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("PROTON_VERSION", "")
	if RunningUnderProton() {
		t.Fatal("native environment detected as Proton")
	}
	t.Setenv("STEAM_COMPAT_DATA_PATH", "/tmp/compatdata")
	if !RunningUnderProton() {
		t.Fatal("Steam compatibility environment was not detected")
	}
}

func TestSinglePlayerNeverFallsBackToLauncher(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.SteamExecutable), []byte("launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := New(root).SinglePlayer(context.Background()); err == nil {
		t.Fatal("launcher recursively executed itself")
	}
}

func TestSAMPStagesModsWithoutModLoader(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STEAM_COMPAT_DATA_PATH", filepath.Join(root, "prefix"))
	for name, contents := range map[string]string{
		"samp.exe":              "samp.exe",
		"samp.dll":              "samp.dll",
		"modloader.asi":         "modloader.asi",
		"stream.ini":            "original stream",
		"modloader/.downgrader": `{"schema_version":2,"installed_mods":["ASI Loader","ModLoader","SilentPatch","Widescreen Fixes","SkyGFX"]}`,
		"modloader/SilentPatch/SilentPatchSA.asi":         "silent patch",
		"modloader/SilentPatch/ReadMe.txt":                "documentation",
		"modloader/WidescreenFix/GTASA.WidescreenFix.asi": "widescreen",
		"modloader/WidescreenFix/GTASA.WidescreenFix.ini": "settings",
		"modloader/SkyGFX/stream.ini":                     "mod stream",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	snapshotCalls := 0
	runtime := &Runtime{
		GameDir: root,
		snapshotGameProcesses: func() (gameProcessSet, error) {
			snapshotCalls++
			if snapshotCalls == 1 {
				return gameProcessSet{}, nil
			}
			return gameProcessSet{41: {}}, nil
		},
		waitForGameProcess: func(_ context.Context, before gameProcessSet, launcherDone <-chan struct{}) (bool, error) {
			<-launcherDone
			if _, ok := before[41]; !ok {
				t.Fatal("pre-launch GTA process snapshot was not passed to waiter")
			}
			if data, err := os.ReadFile(filepath.Join(root, "SilentPatchSA.asi")); err != nil || string(data) != "silent patch" {
				t.Fatalf("multiplayer overlay was removed before GTA exited: %q, %v", data, err)
			}
			return true, nil
		},
		Execute: func(_ context.Context, _, _ string, _ ...string) error {
			active, disabled := runtimePaths(root)
			if _, err := os.Stat(active); !os.IsNotExist(err) {
				t.Fatalf("Mod Loader remained active during SA-MP launch: %v", err)
			}
			if data, err := os.ReadFile(disabled); err != nil || string(data) != "modloader.asi" {
				t.Fatalf("Mod Loader was not preserved: %q, %v", data, err)
			}
			if data, err := os.ReadFile(filepath.Join(root, "SilentPatchSA.asi")); err != nil || string(data) != "silent patch" {
				t.Fatalf("SilentPatch was not staged directly: %q, %v", data, err)
			}
			if data, err := os.ReadFile(filepath.Join(root, "stream.ini")); err != nil || string(data) != "mod stream" {
				t.Fatalf("replacement file was not staged: %q, %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(root, "GTASA.WidescreenFix.asi")); !os.IsNotExist(err) {
				t.Fatalf("Proton-incompatible Widescreen Fix ASI was staged: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "GTASA.WidescreenFix.ini")); !os.IsNotExist(err) {
				t.Fatalf("Proton-incompatible Widescreen Fix settings were staged: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "ReadMe.txt")); !os.IsNotExist(err) {
				t.Fatalf("documentation was staged into the game root: %v", err)
			}
			return nil
		},
	}
	if err := runtime.Multiplayer(context.Background(), multiplayer.ClientSAMP); err != nil {
		t.Fatal(err)
	}
	active, disabled := runtimePaths(root)
	if data, err := os.ReadFile(active); err != nil || string(data) != "modloader.asi" {
		t.Fatalf("Mod Loader was not restored: %q, %v", data, err)
	}
	if _, err := os.Stat(disabled); !os.IsNotExist(err) {
		t.Fatalf("disabled copy remained after launch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SilentPatchSA.asi")); !os.IsNotExist(err) {
		t.Fatalf("temporary SilentPatch file remained after launch: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "stream.ini")); err != nil || string(data) != "original stream" {
		t.Fatalf("overwritten file was not restored: %q, %v", data, err)
	}
}

func TestNativeOverlayKeepsWidescreenFix(t *testing.T) {
	t.Setenv("STEAM_COMPAT_DATA_PATH", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("PROTON_VERSION", "")
	root := t.TempDir()
	for name, contents := range map[string]string{
		"modloader/.downgrader":                           `{"schema_version":2,"installed_mods":["Widescreen Fixes"]}`,
		"modloader/WidescreenFix/GTASA.WidescreenFix.asi": "widescreen",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := New(root)
	if err := runtime.stageMultiplayerOverlay(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "GTASA.WidescreenFix.asi")); err != nil || string(data) != "widescreen" {
		t.Fatalf("native Widescreen Fix was not staged: %q, %v", data, err)
	}
	if err := runtime.restoreMultiplayerOverlay(); err != nil {
		t.Fatal(err)
	}
}

func TestMTATemporarilyDisablesAndRestoresModLoader(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"modloader.asi":                "loader",
		"clients/Multi Theft Auto.exe": "mta",
		multiplayer.ConfigPath:         `{"schema_version":1,"clients":{"mta":{"executable":"clients/Multi Theft Auto.exe"}}}`,
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &Runtime{
		GameDir: root,
		Execute: func(_ context.Context, _, _ string, _ ...string) error {
			active, _ := runtimePaths(root)
			if _, err := os.Stat(active); !os.IsNotExist(err) {
				t.Fatalf("Mod Loader remained active during MTA launch: %v", err)
			}
			return nil
		},
	}
	if err := runtime.Multiplayer(context.Background(), multiplayer.ClientMTA); err != nil {
		t.Fatal(err)
	}
	active, _ := runtimePaths(root)
	if data, err := os.ReadFile(active); err != nil || string(data) != "loader" {
		t.Fatalf("Mod Loader was not restored after MTA: %q, %v", data, err)
	}
}

func TestOpenMPUsesCommandLineConnection(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STEAM_COMPAT_DATA_PATH", filepath.Join(root, "prefix"))
	clientDir := filepath.Join(root, "clients")
	client := filepath.Join(clientDir, "omp-launcher.exe")
	for name, contents := range map[string]string{
		"clients/omp-launcher.exe":                "launcher",
		multiplayer.ConfigPath:                    `{"schema_version":1,"clients":{"openmp":{"executable":"clients/omp-launcher.exe"}}}`,
		"modloader.asi":                           "loader",
		"modloader/.downgrader":                   `{"schema_version":2,"installed_mods":["ASI Loader","ModLoader","SilentPatch"]}`,
		"modloader/SilentPatch/SilentPatchSA.asi": "silent patch",
		"samp.dll":                                "MZoriginal",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	localAppData := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", localAppData)
	support := filepath.Join(localAppData, "mp.open.launcher", "omp", "omp-client.dll")
	managed := filepath.Join(localAppData, "mp.open.launcher", "samp", "0.3.7-R5", "samp.dll")
	for path, contents := range map[string]string{support: "MZclient", managed: "MZmanaged"} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sharedRoot := filepath.Join(localAppData, "mp.open.launcher", "samp", "shared")
	for _, relative := range multiplayer.OpenMPSharedFiles {
		path := filepath.Join(sharedRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("shared"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	runtime := &Runtime{GameDir: root, snapshotGameProcesses: func() (gameProcessSet, error) {
		return gameProcessSet{}, nil
	}, waitForGameProcess: func(_ context.Context, _ gameProcessSet, launcherDone <-chan struct{}) (bool, error) {
		<-launcherDone
		if data, err := os.ReadFile(filepath.Join(root, "samp.dll")); err != nil || string(data) != "MZmanaged" {
			t.Fatalf("open.mp samp.dll was restored before GTA exited: %q, %v", data, err)
		}
		return true, nil
	}, Execute: func(_ context.Context, dir, exe string, args ...string) error {
		if _, err := os.Stat(filepath.Join(root, "modloader.asi")); !os.IsNotExist(err) {
			t.Fatalf("Mod Loader remained active during open.mp launch: %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "samp.dll")); err != nil || string(data) != "MZmanaged" {
			t.Fatalf("managed SA-MP client was not active during open.mp launch: %q, %v", data, err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "SilentPatchSA.asi")); err != nil || string(data) != "silent patch" {
			t.Fatalf("SilentPatch was not staged directly: %q, %v", data, err)
		}
		got = append([]string{dir, exe}, args...)
		return errors.New("bootstrap returned a Wine status after starting GTA")
	}}
	connection := multiplayer.OpenMPConnection{Host: "server.example", Port: "7777", Nickname: "Player", Password: "secret", ClientVersion: "037R5"}
	if err := runtime.OpenMP(context.Background(), connection); err != nil {
		t.Fatal(err)
	}
	want := []string{clientDir, client, "-h", "server.example", "-p", "7777", "-n", "Player", "-g", root, "-P", "secret"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("call = %#v, want %#v", got, want)
	}
	if data, err := os.ReadFile(filepath.Join(root, "samp.dll")); err != nil || string(data) != "MZoriginal" {
		t.Fatalf("original samp.dll was not restored: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "bass.dll")); err != nil || string(data) != "shared" {
		t.Fatalf("managed shared file was not installed: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "modloader.asi")); err != nil || string(data) != "loader" {
		t.Fatalf("Mod Loader was not restored: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "SilentPatchSA.asi")); !os.IsNotExist(err) {
		t.Fatalf("temporary SilentPatch file remained after open.mp launch: %v", err)
	}
}

func TestSinglePlayerRecoversInterruptedMultiplayerOverlay(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		entrypoint.RealGameExecutable:                    "game",
		"modloader/.downgrader":                          `{"schema_version":2,"installed_mods":["SilentPatch"]}`,
		"modloader/SilentPatch/SilentPatchSA.asi":        "silent patch",
		".downgrader/multiplayer/modloader.asi.disabled": "loader",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	stager := &Runtime{GameDir: root}
	if err := stager.stageMultiplayerOverlay(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, _, _ string, _ ...string) error {
		if _, err := os.Stat(filepath.Join(root, "SilentPatchSA.asi")); !os.IsNotExist(err) {
			t.Fatalf("stale multiplayer overlay was not recovered before launch: %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "modloader.asi")); err != nil || string(data) != "loader" {
			t.Fatalf("stale Mod Loader state was not recovered before launch: %q, %v", data, err)
		}
		return nil
	}}
	if err := runtime.SinglePlayer(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayDoesNotOverwriteUnexpectedChanges(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"modloader/.downgrader":                   `{"schema_version":2,"installed_mods":["SilentPatch"]}`,
		"modloader/SilentPatch/SilentPatchSA.asi": "silent patch",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &Runtime{GameDir: root}
	if err := runtime.stageMultiplayerOverlay(context.Background()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "SilentPatchSA.asi")
	if err := os.WriteFile(target, []byte("changed elsewhere"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runtime.restoreMultiplayerOverlay(); err == nil {
		t.Fatal("unexpected file change was overwritten")
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "changed elsewhere" {
		t.Fatalf("unexpected file change was not preserved: %q, %v", data, err)
	}
}

func TestSinglePlayerRecoversLegacyMenuConfigBackup(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		entrypoint.RealGameExecutable:                  "game",
		"modloader/.data/config.ini":                   "EnableMenu = false\n",
		".downgrader/multiplayer/modloader-config.ini": "EnableMenu = true\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, _, _ string, _ ...string) error {
		data, err := os.ReadFile(filepath.Join(root, "modloader", ".data", "config.ini"))
		if err != nil || string(data) != "EnableMenu = true\n" {
			t.Fatalf("legacy Mod Loader config was not recovered: %q, %v", data, err)
		}
		return nil
	}}
	if err := runtime.SinglePlayer(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSinglePlayerRecoversInterruptedMTACompatibilityMode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	_, disabled := runtimePaths(root)
	if err := os.MkdirAll(filepath.Dir(disabled), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disabled, []byte("loader"), 0644); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, _, _ string, _ ...string) error { return nil }}
	if err := runtime.SinglePlayer(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, _ := runtimePaths(root)
	if data, err := os.ReadFile(active); err != nil || string(data) != "loader" {
		t.Fatalf("stale Mod Loader was not recovered: %q, %v", data, err)
	}
}

func runtimePaths(root string) (string, string) {
	return filepath.Join(root, "modloader.asi"), filepath.Join(root, ".downgrader", "multiplayer", "modloader.asi.disabled")
}

func TestFindDowngraderUsesVerifiedMetadata(t *testing.T) {
	root, sourceDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(sourceDir, entrypoint.SteamExecutable)
	downgrader := filepath.Join(sourceDir, "gtasa-open-downgrader-windows.exe")
	if err := os.WriteFile(launcher, []byte("launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(downgrader, []byte("downgrader"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := entrypoint.InstallWithDowngrader(context.Background(), root, launcher, downgrader); err != nil {
		t.Fatal(err)
	}
	if got := FindDowngrader(root); got != downgrader {
		t.Fatalf("downgrader = %q", got)
	}
	if err := os.WriteFile(downgrader, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := FindDowngrader(root); got != "" {
		t.Fatalf("modified downgrader trusted: %q", got)
	}
}
