package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func HideConsole(cmd *exec.Cmd) {}
func Candidates() []string {
	home, _ := os.UserHomeDir()
	var paths []string
	for _, rel := range []string{".local/share/Steam", ".steam/steam", ".steam/root", ".var/app/com.valvesoftware.Steam/data/Steam"} {
		paths = append(paths, SteamLibraries(filepath.Join(home, rel))...)
	}
	return append(paths, "/run/media/mmcblk0p1/steamapps/common/Grand Theft Auto San Andreas")
}
func UserData(game string) string {
	p := filepath.Join(filepath.Dir(filepath.Dir(game)), "compatdata", "12120", "pfx", "drive_c", "users", "steamuser", "Documents", "GTA San Andreas User Files")
	if st, e := os.Stat(p); e == nil && st.IsDir() {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "GTA San Andreas User Files")
}
func FixRegistry(context.Context, string) error {
	return fmt.Errorf("registry repair is available on Windows")
}
func Shortcut(context.Context, string) error {
	return fmt.Errorf("desktop shortcuts are available on Windows")
}
func ProtonApps(ctx context.Context) (string, error) {
	b, e := exec.CommandContext(ctx, "protontricks", "-l").CombinedOutput()
	return string(b), e
}
func ProtonInstall(ctx context.Context, id, file string) error {
	if !regexp.MustCompile(`^\d+$`).MatchString(id) {
		return fmt.Errorf("invalid Steam app ID")
	}
	if !strings.HasSuffix(strings.ToLower(file), ".exe") {
		return fmt.Errorf("select a Windows executable")
	}
	return exec.CommandContext(ctx, "protontricks-launch", "--appid", id, file).Run()
}
func WaitProcess(ctx context.Context, pid int) error {
	for {
		if e := syscall.Kill(pid, 0); e == syscall.ESRCH {
			return nil
		} else if e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
