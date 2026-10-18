package multiplayer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestInstallerParametersPreserveArgumentBoundaries(t *testing.T) {
	arguments := []string{`/D=C:\GTA San Andreas`, `quote"inside`, ``, `C:\trailing space\`, `a&b;$(no shell)`, `Користувач`}
	parsed, err := windows.DecomposeCommandLine("installer.exe " + installerParameters(arguments))
	if err != nil || !reflect.DeepEqual(parsed[1:], arguments) {
		t.Fatalf("arguments=%#v, %v", parsed, err)
	}
}
func TestElevatedInstallerInfoLayoutAndCancellation(t *testing.T) {
	var info shellExecuteInfo
	if runtime.GOARCH == "amd64" && (unsafe.Sizeof(info) != 112 || unsafe.Offsetof(info.Process) != 104) {
		t.Fatalf("incorrect SHELLEXECUTEINFOW layout: size %d, process offset %d", unsafe.Sizeof(info), unsafe.Offsetof(info.Process))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runElevatedInstaller(ctx, "", "missing.exe"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestInstallerProcessWaitAndExitCode(t *testing.T) {
	if os.Getenv("GTASA_INSTALLER_CHILD") == "1" {
		directory, _ := os.Getwd()
		if filepath.Clean(directory) != filepath.Clean(os.Getenv("GTASA_INSTALLER_DIRECTORY")) {
			os.Exit(99)
		}
		os.Exit(23)
	}
	t.Setenv("GTASA_INSTALLER_CHILD", "1")
	t.Setenv("GTASA_INSTALLER_DIRECTORY", filepath.Dir(os.Args[0]))
	err := (DirectRunner{}).Run(context.Background(), os.Args[0], "-test.run=^TestInstallerProcessWaitAndExitCode$")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("installer result=%v", err)
	}
}
