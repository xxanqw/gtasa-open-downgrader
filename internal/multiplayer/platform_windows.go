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
	localAppData, _ := os.UserCacheDir()
	return Environment{GameDir: gameDir, WindowsGameDir: gameDir, PrefixDriveC: drive, LocalAppData: localAppData}, DirectRunner{}, "Native Windows", nil
}
