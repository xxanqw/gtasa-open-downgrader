//go:build linux

package multiplayer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/steam"
)

const GTAAppID = 12120

func PlatformEnvironment(gameDir string) (Environment, Runner, string, error) {
	context, err := steam.DetectProtonContext(GTAAppID)
	if err != nil {
		return Environment{GameDir: gameDir}, nil, "", err
	}
	if gameDir != "" && !sameDirectory(context.GameDir, gameDir) {
		return Environment{GameDir: gameDir}, nil, "", fmt.Errorf("selected game folder does not match Steam AppID %d (%s)", GTAAppID, context.GameDir)
	}
	driveC := context.CompatDataPath + "/pfx/drive_c"
	return Environment{GameDir: context.GameDir, PrefixDriveC: driveC}, ProtonRunner{Context: context}, context.ToolDisplayName, nil
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
