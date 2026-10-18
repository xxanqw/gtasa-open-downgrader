package launcherapp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
)

func TestMTAChildEnvironmentIsScopedAndLeavesParentUntouched(t *testing.T) {
	original := []string{
		`Path=C:\Windows\System32`, `SystemRoot=C:\Windows`, `USERPROFILE=C:\Users\Player`,
		`TEMP=C:\Users\Player\Temp`, `=C:=C:\Games\GTA`, `SteamPath=C:\Steam`,
		"sTeAmApPiD=12120", "SteamGameId=12120", "SteamOverlayGameId=12120",
		"SteamOverlayPipe=private", "SteamClientLaunch=1", "SteamEnv=1",
		`SteamClientDll=C:\Steam\steamclient.dll`, `SteamClientDll64=C:\Steam\steamclient64.dll`,
		"SteamNoOverlayUI=0", "STEAMNOOVERLAYUIDRAWING=0",
	}
	inherited := append([]string(nil), original...)
	child := mtaChildEnvironment(`C:\MTA San Andreas 1.6\Multi Theft Auto.exe`, "windows", false, inherited)
	if !reflect.DeepEqual(inherited, original) {
		t.Fatal("parent environment changed")
	}
	values := map[string]string{}
	for _, entry := range child {
		key, value, _ := strings.Cut(entry, "=")
		key = strings.ToLower(key)
		if _, ok := values[key]; ok {
			t.Fatalf("duplicate environment key %q", key)
		}
		values[key] = value
	}
	for _, key := range []string{"steamappid", "steamgameid", "steamoverlaygameid", "steamoverlaypipe", "steamclientlaunch", "steamenv", "steamclientdll", "steamclientdll64"} {
		if _, ok := values[key]; ok {
			t.Fatalf("inherited Steam launch field %s", key)
		}
	}
	for _, key := range []string{"steamnooverlayui", "steamnooverlayuidrawing"} {
		if values[key] != "1" {
			t.Fatalf("overlay suppression %s=%q", key, values[key])
		}
	}
	for _, entry := range original[:6] {
		found := false
		for _, kept := range child {
			if kept == entry {
				found = true
			}
		}
		if !found {
			t.Fatalf("unrelated environment entry removed: %q", entry)
		}
	}
	for _, tc := range []struct {
		executable, host string
		wine             bool
	}{
		{`C:\GTA\gta_sa.exe`, "windows", false},
		{`C:\SA-MP\samp.exe`, "windows", false},
		{`C:\openmp\omp-launcher.exe`, "windows", false},
		{`C:\MTA\Multi Theft Auto.exe`, "windows", true},
		{"/prefix/Multi Theft Auto.exe", "linux", false},
	} {
		if got := mtaChildEnvironment(tc.executable, tc.host, tc.wine, inherited); got != nil {
			t.Fatalf("changed environment outside native MTA: %+v", tc)
		}
	}
	if got := mtaChildEnvironment(`C:\MTA\Multi Theft Auto.exe`, "windows", false, original[:6]); got != nil {
		t.Fatal("changed a non-Steam desktop launch")
	}
}

func TestMTAChildProcessEnvironment(t *testing.T) {
	if os.Getenv("GTASA_MTA_ENV_CHILD") == "1" {
		if os.Getenv("SteamAppId") != "" || os.Getenv("SteamGameId") != "" || os.Getenv("SteamOverlayGameId") != "" || os.Getenv("SteamNoOverlayUI") != "1" {
			os.Exit(97)
		}
		if os.Getenv("GTASA_KEEP_ME") != "unchanged" {
			os.Exit(98)
		}
		os.Exit(0)
	}
	if runtime.GOOS != "windows" || winedrive.RunningUnderWine() {
		t.Skip("native Windows child-process integration")
	}
	for _, key := range []string{"STEAM_COMPAT_DATA_PATH", "STEAM_COMPAT_CLIENT_INSTALL_PATH", "PROTON_VERSION"} {
		t.Setenv(key, "")
	}
	t.Setenv("SteamAppId", "12120")
	t.Setenv("SteamGameId", "12120")
	t.Setenv("SteamOverlayGameId", "12120")
	t.Setenv("GTASA_MTA_ENV_CHILD", "1")
	t.Setenv("GTASA_KEEP_ME", "unchanged")
	directory := t.TempDir()
	executable := filepath.Join(directory, "Multi Theft Auto.exe")
	if err := fsutil.CopyFile(context.Background(), os.Args[0], executable); err != nil {
		t.Fatal(err)
	}
	command := launchCommand(context.Background(), directory, executable, "-test.run=^TestMTAChildProcessEnvironment$")
	if command.Dir != directory || len(command.Args) != 2 || command.Args[1] != "-test.run=^TestMTAChildProcessEnvironment$" {
		t.Fatal("working directory or arguments changed")
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("child environment integration: %v: %s", err, output)
	}
	if os.Getenv("SteamAppId") != "12120" || os.Getenv("SteamGameId") != "12120" {
		t.Fatal("launcher lost its own Steam identity")
	}
}
