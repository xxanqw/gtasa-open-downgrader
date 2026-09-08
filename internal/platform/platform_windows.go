package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
func Candidates() []string {
	var paths []string
	for _, source := range []struct {
		root       registry.Key
		key, value string
		steam      bool
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Rockstar Games\Grand Theft Auto San Andreas\Installation`, "ExePath", false},
		{registry.LOCAL_MACHINE, `SOFTWARE\Rockstar Games\Grand Theft Auto San Andreas\Installation`, "ExePath", false},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath", true},
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath", true},
	} {
		key, e := registry.OpenKey(source.root, source.key, registry.QUERY_VALUE)
		if e != nil {
			continue
		}
		v, _, e := key.GetStringValue(source.value)
		key.Close()
		if e != nil {
			continue
		}
		if source.steam {
			paths = append(paths, SteamLibraries(v)...)
		} else {
			if strings.HasSuffix(strings.ToLower(v), ".exe") {
				v = filepath.Dir(v)
			}
			paths = append(paths, v)
		}
	}
	return paths
}
func UserData(string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "GTA San Andreas User Files")
}
func FixRegistry(ctx context.Context, game string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	key, _, e := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Rockstar Games\Grand Theft Auto San Andreas\Installation`, registry.SET_VALUE)
	if e != nil {
		return fmt.Errorf("registry repair requires Administrator access: %w", e)
	}
	defer key.Close()
	return key.SetStringValue("ExePath", filepath.Join(game, "gta_sa.exe"))
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func Shortcut(ctx context.Context, game string) error {
	exe := filepath.Join(game, "gta-sa.exe")
	if _, e := os.Stat(exe); e != nil {
		exe = filepath.Join(game, "gta_sa.exe")
	}
	script := `$ErrorActionPreference='Stop';$s=(New-Object -ComObject WScript.Shell).CreateShortcut([IO.Path]::Combine([Environment]::GetFolderPath('Desktop'),'GTA San Andreas.lnk'));$s.TargetPath=` + quote(exe) + `;$s.WorkingDirectory=` + quote(game) + `;$s.Save()`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	HideConsole(cmd)
	b, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("shortcut: %w: %s", e, b)
	}
	return nil
}
func WaitProcess(ctx context.Context, pid int) error {
	handle, e := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if e != nil {
		if e == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return e
	}
	defer windows.CloseHandle(handle)
	for {
		result, e := windows.WaitForSingleObject(handle, 100)
		if e != nil {
			return e
		}
		if result == windows.WAIT_OBJECT_0 {
			return nil
		}
		if e = ctx.Err(); e != nil {
			return e
		}
	}
}
