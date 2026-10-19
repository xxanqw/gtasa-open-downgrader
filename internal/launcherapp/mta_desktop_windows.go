package launcherapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
	"golang.org/x/sys/windows"
)

func powershellLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

// A new Shell.Application object runs in the caller. FindWindowSW obtains the
// existing Explorer desktop's automation object instead, so Explorer launches
// MTA without propagating Steam's injected CreateProcess hooks.
func desktopMTAScript(directory, executable string, arguments []string) string {
	escaped := make([]string, len(arguments))
	for i, a := range arguments {
		escaped[i] = windows.EscapeArg(a)
	}
	return "$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue';" +
		"$s=New-Object -ComObject Shell.Application;$w=$s.Windows();$h=0;" +
		"$d=$w.FindWindowSW(0,0,8,[ref]$h,1);if($null -eq $d){throw 'Windows desktop shell is unavailable'};" +
		"$d.Document.Application.ShellExecute(" + powershellLiteral(executable) + "," + powershellLiteral(strings.Join(escaped, " ")) + "," + powershellLiteral(directory) + ",'open',1)"
}

func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func requireDesktopMTAClosed() error {
	if winedrive.RunningUnderWine() || RunningUnderProton() {
		return nil
	}
	processes, err := mtaProcessIDs()
	if err != nil {
		return fmt.Errorf("inspect running MTA clients: %w", err)
	}
	if len(processes) != 0 {
		return fmt.Errorf("close MTA before restoring multiplayer compatibility files")
	}
	return nil
}

func mtaProcessIDs() (map[uint32]struct{}, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	result := map[uint32]struct{}{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return result, nil
		}
		return nil, err
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "Multi Theft Auto.exe") {
			result[entry.ProcessID] = struct{}{}
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		err = windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func waitDesktopMTA(ctx context.Context, executable string, before map[uint32]struct{}) error {
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := mtaProcessIDs()
		if err != nil {
			return fmt.Errorf("%w: enumerate client: %v", errMTAExitUnknown, err)
		}
		for pid := range current {
			if _, old := before[pid]; old {
				continue
			}
			process, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				continue
			}
			if err != nil {
				return fmt.Errorf("%w: open client: %v", errMTAExitUnknown, err)
			}
			buffer := make([]uint16, 32768)
			size := uint32(len(buffer))
			err = windows.QueryFullProcessImageName(process, 0, &buffer[0], &size)
			if err != nil {
				windows.CloseHandle(process)
				return fmt.Errorf("%w: identify client: %v", errMTAExitUnknown, err)
			}
			actual := windows.UTF16ToString(buffer[:size])
			samePath := strings.EqualFold(filepath.Clean(actual), filepath.Clean(executable))
			if !samePath {
				actualInfo, actualErr := os.Stat(actual)
				expectedInfo, expectedErr := os.Stat(executable)
				samePath = actualErr == nil && expectedErr == nil && os.SameFile(actualInfo, expectedInfo)
			}
			if !samePath {
				windows.CloseHandle(process)
				continue
			}
			err = waitMTAHandle(ctx, process)
			windows.CloseHandle(process)
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", errMTAExitUnknown, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("%w: desktop did not expose a trackable MTA process", errMTAExitUnknown)
		case <-ticker.C:
		}
	}
}

func waitMTAHandle(ctx context.Context, process windows.Handle) error {
	for {
		state, err := windows.WaitForSingleObject(process, 200)
		if err != nil {
			return fmt.Errorf("%w: wait: %v", errMTAExitUnknown, err)
		}
		if state == windows.WAIT_OBJECT_0 {
			var code uint32
			if err := windows.GetExitCodeProcess(process, &code); err != nil {
				return fmt.Errorf("read MTA result: %w", err)
			}
			if code != 0 {
				return fmt.Errorf("MTA exited with status 0x%08x", code)
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: %v", errMTAExitUnknown, err)
		}
	}
}

func executeDesktopMTA(ctx context.Context, directory, executable string, arguments ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before, err := mtaProcessIDs()
	if err != nil {
		return fmt.Errorf("inspect MTA processes: %w", err)
	}
	if len(before) != 0 {
		return fmt.Errorf("close the existing MTA client before starting another one")
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	dispatchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(dispatchCtx, filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(desktopMTAScript(directory, executable, arguments)))
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Windows desktop dispatch: %w", err)
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("%w: desktop dispatch failed: %v: %s", errMTAExitUnknown, err, strings.TrimSpace(diagnostics.String()))
	}
	return waitDesktopMTA(ctx, executable, before)
}
