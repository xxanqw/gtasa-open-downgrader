//go:build windows

package launcherapp

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessWatcherHelper(t *testing.T) {
	if os.Getenv("DOWNGRADER_GTA_PROCESS_HELPER") != "1" {
		return
	}
	time.Sleep(750 * time.Millisecond)
	os.Exit(0)
}

func TestWaitForSpawnedGameTracksGTAProcess(t *testing.T) {
	before, err := snapshotGameProcesses()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(t.TempDir(), "gta_sa.exe")
	if err := copyExecutable(executable, helper); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(helper, "-test.run=^TestProcessWatcherHelper$")
	command.Env = append(os.Environ(), "DOWNGRADER_GTA_PROCESS_HELPER=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	launcherDone := make(chan struct{})
	close(launcherDone)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started, waitErr := waitForSpawnedGame(ctx, before, launcherDone)
	commandErr := command.Wait()
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if !started {
		t.Fatal("gta_sa.exe process was not detected")
	}
}

func copyExecutable(source, destination string) error {
	from, err := os.Open(source)
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(to, from)
	closeErr := to.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
