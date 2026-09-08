//go:build linux

package multiplayer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/steam"
)

const GTAAppID = 12120

func PlatformEnvironment(gameDir string) (Environment, Runner, string, error) {
	steamContext, err := steam.DetectProtonContext(GTAAppID)
	if err != nil {
		return Environment{GameDir: gameDir}, nil, "", err
	}
	if gameDir != "" && !sameDirectory(steamContext.GameDir, gameDir) {
		return Environment{GameDir: gameDir}, nil, "", fmt.Errorf("selected game folder does not match Steam AppID %d (%s)", GTAAppID, steamContext.GameDir)
	}
	driveC := filepath.Join(steamContext.CompatDataPath, "pfx", "drive_c")
	windowsGame, err := steamContext.EnsureDriveMapping(steamContext.GameDir)
	if err != nil {
		return Environment{GameDir: steamContext.GameDir, PrefixDriveC: driveC}, nil, steamContext.ToolDisplayName, err
	}
	environment := Environment{GameDir: steamContext.GameDir, WindowsGameDir: windowsGame, PrefixDriveC: driveC}
	prepare := func(ctx context.Context, _ ClientID, environment Environment) error {
		executable := strings.TrimRight(environment.WindowsGameDir, `\`) + `\gta_sa.exe`
		if err := steamContext.SetRegistryString(ctx, `HKLM\SOFTWARE\Rockstar Games\GTA San Andreas\Installation`, "ExePath", executable); err != nil {
			return err
		}
		return steamContext.SetRegistryString(ctx, `HKLM\SOFTWARE\Multi Theft Auto: San Andreas All\Common`, "GTA:SA Path", environment.WindowsGameDir)
	}
	return environment, ProtonRunner{Context: steamContext, PrepareFunc: prepare}, steamContext.ToolDisplayName, nil
}

func sameDirectory(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	leftAbs, _ := filepath.Abs(left)
	rightAbs, _ := filepath.Abs(right)
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
