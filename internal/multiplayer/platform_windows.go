//go:build windows

package multiplayer

import (
	"os"
	"path/filepath"
)

func PlatformEnvironment(gameDir string) (Environment, Runner, string, error) {
	drive := os.Getenv("SystemDrive")
	if drive != "" {
		drive = filepath.Clean(drive + `\`)
	}
	return Environment{GameDir: gameDir, PrefixDriveC: drive}, DirectRunner{}, "Native Windows", nil
}
