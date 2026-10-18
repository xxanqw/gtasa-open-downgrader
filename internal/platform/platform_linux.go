package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/steam"
)

func HideConsole(cmd *exec.Cmd) {}
func Candidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var paths []string
	for _, root := range steam.FindSteamRoots(home) {
		paths = append(paths, SteamGameDirs(root)...)
	}
	return paths
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
