package launcherapp

import (
	"context"
	"testing"
)

func TestDesktopMTATargetIsNativeWindowsOnly(t *testing.T) {
	for _, tc := range []struct {
		executable, host string
		wine, want       bool
	}{
		{`C:\MTA\Multi Theft Auto.exe`, "windows", false, true},
		{`C:\MTA\MULTI THEFT AUTO.EXE`, "windows", false, true},
		{`C:\MTA\Multi Theft Auto.exe`, "windows", true, false},
		{"/prefix/Multi Theft Auto.exe", "linux", false, false},
		{`C:\GTA\gta_sa.exe`, "windows", false, false},
		{`C:\SA-MP\samp.exe`, "windows", false, false},
		{`C:\openmp\omp-launcher.exe`, "windows", false, false},
	} {
		if got := desktopMTATarget(tc.executable, tc.host, tc.wine); got != tc.want {
			t.Fatalf("target %+v = %t", tc, got)
		}
	}
	command := launchCommand(context.Background(), "working-dir", "gta_sa.exe", "argument")
	if command.Env != nil || command.Dir != "working-dir" || len(command.Args) != 2 || command.Args[1] != "argument" {
		t.Fatal("ordinary launches lost default inheritance, working directory or arguments")
	}
}
