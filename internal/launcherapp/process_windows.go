//go:build windows

package launcherapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const gameProcessStartGrace = 8 * time.Second

func snapshotGameProcesses() (gameProcessSet, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	processes := gameProcessSet{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return processes, nil
		}
		return nil, err
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "gta_sa.exe") {
			processes[entry.ProcessID] = struct{}{}
		}
		entry.Size = uint32(unsafe.Sizeof(windows.ProcessEntry32{}))
		err = windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return processes, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func waitForSpawnedGame(ctx context.Context, before gameProcessSet, launcherDone <-chan struct{}) (bool, error) {
	var deadline *time.Timer
	var deadlineChannel <-chan time.Time
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		if deadline != nil {
			deadline.Stop()
		}
	}()

	for {
		current, err := snapshotGameProcesses()
		if err != nil {
			return false, err
		}
		for processID := range current {
			if _, existed := before[processID]; existed {
				continue
			}
			return true, waitForProcessExit(ctx, processID)
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-launcherDone:
			// Wine may create gta_sa.exe shortly after the bootstrap exits.
			// Keep watching for a bounded grace period without delaying cleanup
			// when GTA was already observed while the bootstrap was running.
			launcherDone = nil
			deadline = time.NewTimer(gameProcessStartGrace)
			deadlineChannel = deadline.C
		case <-deadlineChannel:
			return false, nil
		case <-ticker.C:
		}
	}
}

func waitForProcessExit(ctx context.Context, processID uint32) error {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, processID)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		// The process exited between the snapshot and opening its handle.
		return nil
	}
	if err != nil {
		return fmt.Errorf("open gta_sa.exe process %d: %w", processID, err)
	}
	defer windows.CloseHandle(process)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		event, err := windows.WaitForSingleObject(process, 250)
		if err != nil {
			return err
		}
		switch event {
		case windows.WAIT_OBJECT_0:
			return nil
		case uint32(windows.WAIT_TIMEOUT):
			continue
		default:
			return fmt.Errorf("unexpected wait result 0x%x for gta_sa.exe process %d", event, processID)
		}
	}
}
