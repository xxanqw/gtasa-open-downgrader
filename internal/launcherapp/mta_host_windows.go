package launcherapp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
	"golang.org/x/sys/windows"
)

const mtaHostArgument = "--downgrader-mta-host"
const mtaHostExecutable = "gtasa-mta-host.exe"

// MTA deliberately closes all gta-sa.exe processes during startup. A neutral
// helper survives that cleanup, keeps Steam's child-process tracking alive,
// and owns compatibility-file restoration after the client actually exits.
func StartNativeMTAHost(ctx context.Context, gameDir string) (bool, error) {
	if winedrive.RunningUnderWine() || RunningUnderProton() {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	executable, err := os.Executable()
	if err != nil {
		return true, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return true, err
	}
	host, err := stageMTAHost(ctx, executable, filepath.Join(cache, "gtasa-open-downgrader", "mta-host"))
	if err != nil {
		return true, err
	}
	root, err := filepath.Abs(gameDir)
	if err != nil {
		return true, err
	}
	command := exec.Command(host, mtaHostArgument, root, strconv.Itoa(os.Getpid()))
	command.Dir = root
	if err := command.Start(); err != nil {
		return true, fmt.Errorf("start MTA lifetime helper: %w", err)
	}
	return true, command.Process.Release()
}

func stageMTAHost(ctx context.Context, executable, cache string) (string, error) {
	digest, err := entrypoint.Hash(ctx, executable)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(cache, digest)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	host := filepath.Join(directory, mtaHostExecutable)
	if current, err := entrypoint.Hash(ctx, host); err == nil && current == digest {
		return host, nil
	}
	if err := fsutil.CopyFile(ctx, executable, host); err != nil {
		return "", err
	}
	actual, err := entrypoint.Hash(ctx, host)
	if err != nil {
		return "", err
	}
	if actual != digest {
		return "", fmt.Errorf("MTA helper checksum changed while staging")
	}
	return host, nil
}

func HandleMTAHost(arguments []string) (bool, error) {
	if len(arguments) == 0 || arguments[0] != mtaHostArgument {
		return false, nil
	}
	if len(arguments) != 3 {
		return true, fmt.Errorf("invalid MTA helper arguments")
	}
	executable, err := os.Executable()
	if err != nil {
		return true, err
	}
	if !strings.EqualFold(filepath.Base(executable), mtaHostExecutable) {
		return true, fmt.Errorf("MTA helper must run under its neutral executable name")
	}
	if winedrive.RunningUnderWine() || RunningUnderProton() {
		return true, fmt.Errorf("native MTA helper is not used under Wine/Proton")
	}
	root := filepath.Clean(arguments[1])
	if !filepath.IsAbs(root) {
		return true, fmt.Errorf("MTA helper requires an absolute game directory")
	}
	parent, err := strconv.ParseUint(arguments[2], 10, 32)
	if err != nil || parent == 0 || parent == uint64(os.Getpid()) {
		return true, fmt.Errorf("invalid MTA helper parent")
	}
	if _, err := entrypoint.GamePath(root); err != nil {
		return true, err
	}
	key := sha256.Sum256([]byte(strings.ToLower(root)))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`Local\GTASAOpenDowngraderMTA-%x`, key))
	if err != nil {
		return true, err
	}
	mutex, err := windows.CreateMutex(nil, false, name)
	if mutex != 0 {
		defer windows.CloseHandle(mutex)
	}
	if err != nil {
		return true, fmt.Errorf("another MTA session may already be active: %w", err)
	}
	if err := waitMTAParent(uint32(parent)); err != nil {
		return true, err
	}
	running, err := mtaProcessIDs()
	if err != nil {
		return true, err
	}
	if len(running) != 0 {
		return true, fmt.Errorf("close the existing MTA client first")
	}
	return true, New(root).Multiplayer(context.Background(), multiplayer.ClientMTA)
}

func waitMTAParent(pid uint32) error {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("wait for Steam chooser: %w", err)
	}
	defer windows.CloseHandle(process)
	state, err := windows.WaitForSingleObject(process, uint32((30*time.Second)/time.Millisecond))
	if err != nil {
		return err
	}
	if state != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("Steam chooser did not close before MTA startup")
	}
	return nil
}

func ShowMTAHostError(err error) {
	text := "MTA launch failed: " + err.Error()
	if cache, e := os.UserCacheDir(); e == nil {
		directory := filepath.Join(cache, "gtasa-open-downgrader")
		if os.MkdirAll(directory, 0700) == nil {
			os.WriteFile(filepath.Join(directory, "mta-host-error.log"), []byte(text), 0600)
		}
	}
	message, _ := windows.UTF16PtrFromString(text)
	title := windows.StringToUTF16Ptr("Steam launcher — MTA")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
