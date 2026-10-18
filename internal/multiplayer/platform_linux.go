//go:build linux

package multiplayer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
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
	windowsGame, err := steamContext.GameDirectoryMappingPath(steamContext.GameDir)
	if err != nil {
		return Environment{GameDir: steamContext.GameDir, PrefixDriveC: driveC}, nil, steamContext.ToolDisplayName, err
	}
	steamContext.DriveGuard = platform.RequireGameClosed
	steamContext.GameDriveExpected = windowsGame
	environment := Environment{
		GameDir:           steamContext.GameDir,
		WindowsGameDir:    windowsGame,
		PrefixDriveC:      driveC,
		LocalAppData:      filepath.Join(driveC, "users", "steamuser", "AppData", "Local"),
		CompatibilityTool: strings.Join([]string{steamContext.ToolDisplayName, steamContext.ToolName, filepath.Base(steamContext.ToolRoot)}, " | "),
	}
	prepare := func(ctx context.Context, id ClientID, environment Environment) error {
		if id == ClientMTA {
			missing, err := MissingMTAFonts(environment)
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return fmt.Errorf("MTA needs fonts in its Proton prefix: %s; use MTA Install / Repair in Multiplayer to prepare them automatically", strings.Join(missing, ", "))
			}
		}
		executable := strings.TrimRight(environment.WindowsGameDir, `\`) + `\gta_sa.exe`
		if err := steamContext.SetRegistryString(ctx, `HKLM\SOFTWARE\Rockstar Games\GTA San Andreas\Installation`, "ExePath", executable); err != nil {
			return err
		}
		if id == ClientMTA {
			return steamContext.SetRegistryString(ctx, `HKLM\SOFTWARE\Multi Theft Auto: San Andreas All\Common`, "GTA:SA Path", environment.WindowsGameDir)
		}
		return nil
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
