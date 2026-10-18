package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// SHELLEXECUTEINFOW, including the process handle requested by NOCLOSEPROCESS.
type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            windows.Handle
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance                          windows.Handle
	IDList                            uintptr
	Class                             *uint16
	ClassKey                          windows.Handle
	HotKey                            uint32
	Icon                              windows.Handle
	Process                           windows.Handle
}

func installerParameters(arguments []string) string {
	escaped := make([]string, len(arguments))
	for i, argument := range arguments {
		escaped[i] = windows.EscapeArg(argument)
	}
	return strings.Join(escaped, " ")
}

func runInstaller(ctx context.Context, directory, executable string, arguments ...string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	err := command.Run()
	if !errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) {
		return err
	}
	return runElevatedInstaller(ctx, directory, executable, arguments...)
}

func runElevatedInstaller(ctx context.Context, directory, executable string, arguments ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return err
	}
	parameters, err := windows.UTF16PtrFromString(installerParameters(arguments))
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// S_FALSE (1) also owns a matching CoUninitialize call.
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil && err != syscall.Errno(1) {
		return fmt.Errorf("initialize installer elevation: %w", err)
	}
	defer windows.CoUninitialize()
	info := shellExecuteInfo{
		Mask: 0x40 | 0x100 | 0x400, // NOCLOSEPROCESS | NOASYNC | FLAG_NO_UI
		Verb: windows.StringToUTF16Ptr("runas"), File: file, Parameters: parameters, Directory: dir, Show: 1,
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, err := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return fmt.Errorf("installer administrator approval was cancelled: %w", err)
		}
		return fmt.Errorf("request installer administrator approval: %w", err)
	}
	if info.Process == 0 {
		return fmt.Errorf("elevated installer did not provide a process handle")
	}
	defer windows.CloseHandle(info.Process)
	// Never kill an elevated third-party installer halfway through installation.
	// Cancellation takes effect after the user finishes or closes its window.
	for {
		state, err := windows.WaitForSingleObject(info.Process, 200)
		if err != nil {
			return fmt.Errorf("wait for elevated installer: %w", err)
		}
		if state == windows.WAIT_OBJECT_0 {
			break
		}
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.Process, &code); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("installer exited with code %d", code)
	}
	return nil
}
