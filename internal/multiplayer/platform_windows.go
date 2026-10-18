//go:build windows

package multiplayer

import (
	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
	"os"
	"path/filepath"
)

func PlatformEnvironment(gameDir string) (Environment, Runner, string, error) {
	drive := os.Getenv("SystemDrive")
	if drive != "" {
		drive = filepath.Clean(drive + `\`)
	}
	localAppData, _ := os.UserCacheDir()
	environment := Environment{GameDir: gameDir, WindowsGameDir: gameDir, PrefixDriveC: drive, LocalAppData: localAppData}
	if winedrive.RunningUnderWine() {
		mapped, err := winedrive.PlanWineGameDrive(gameDir)
		if err != nil {
			return environment, nil, "Wine", err
		}
		environment.WindowsGameDir = mapped
		return environment, DirectRunner{GameDir: gameDir}, "Wine", nil
	}
	return environment, DirectRunner{}, "Native Windows", nil
}
