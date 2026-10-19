package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func gameProcessName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "gta_sa.exe" || name == "gta-sa.exe" || name == "gtasa-mta-host.exe"
}

// RequireGameClosed checks both supported filenames, including Wine's comm name.
// Failure to enumerate processes does not authorize changing game assets.
func RequireGameClosed() error { return requireGameClosedIn("/proc") }

// procfs may report ESRCH, not just ENOENT, if a process exits during a read.
func processExited(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

func requireGameClosedIn(proc string) error {
	entries, err := os.ReadDir(proc)
	if err != nil {
		return fmt.Errorf("check GTA processes: %w", err)
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil || !entry.IsDir() {
			continue
		}
		name, err := os.ReadFile(filepath.Join(proc, entry.Name(), "comm"))
		if processExited(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("check GTA process %s: %w", entry.Name(), err)
		}
		if gameProcessName(string(name)) {
			return fmt.Errorf("close GTA (%s) before changing installed mods", strings.TrimSpace(string(name)))
		}
	}
	return nil
}
