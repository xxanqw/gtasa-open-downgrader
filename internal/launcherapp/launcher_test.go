package launcherapp

import (
	"context"
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

func TestSAMPTemporarilyDisablesAndRestoresModLoader(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"samp.exe", "samp.dll", "modloader.asi"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &Runtime{
		GameDir: root,
		Execute: func(_ context.Context, _, _ string, _ ...string) error {
			active, disabled := runtimePaths(root)
			if _, err := os.Stat(active); !os.IsNotExist(err) {
				t.Fatalf("Mod Loader remained active during SA-MP launch: %v", err)
			}
			if _, err := os.Stat(disabled); err != nil {
				t.Fatalf("disabled Mod Loader is missing: %v", err)
			}
			return nil
		},
		AfterCompatibilityExit: func(context.Context) error { return nil },
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
		AfterCompatibilityExit: func(context.Context) error { return nil },
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
	clientDir := filepath.Join(root, "clients")
	client := filepath.Join(clientDir, "omp-launcher.exe")
	for name, contents := range map[string]string{
		"clients/omp-launcher.exe": "launcher",
		multiplayer.ConfigPath:     `{"schema_version":1,"clients":{"openmp":{"executable":"clients/omp-launcher.exe"}}}`,
		"samp.dll":                 "MZoriginal",
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
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, dir, exe string, args ...string) error {
		if data, err := os.ReadFile(filepath.Join(root, "samp.dll")); err != nil || string(data) != "MZmanaged" {
			t.Fatalf("managed SA-MP client was not active during open.mp launch: %q, %v", data, err)
		}
		got = append([]string{dir, exe}, args...)
		return nil
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
}

func TestSinglePlayerRecoversInterruptedSAMPCompatibilityMode(t *testing.T) {
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
