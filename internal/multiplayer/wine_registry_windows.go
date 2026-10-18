//go:build windows

package multiplayer

import (
	"fmt"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
	"golang.org/x/sys/windows/registry"
)

// Update only installer discovery paths for this selected game. This avoids a
// stale G: Registry path after a protected physical drive requires H: instead.
// No serial, disk identity, account, locale or client-preference values change.
func ConfigureWineGamePath(id ClientID, gamePath string) error {
	if !winedrive.RunningUnderWine() {
		return nil
	}
	if len(gamePath) < 4 || gamePath[1] != ':' || gamePath[2] != '\\' || strings.ContainsAny(gamePath, "\x00\r\n") {
		return fmt.Errorf("invalid Wine GTA directory")
	}
	set := func(key, name, value string) error {
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, key, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, value)
	}
	if err := set(`SOFTWARE\Rockstar Games\GTA San Andreas\Installation`, "ExePath", strings.TrimRight(gamePath, `\`)+`\gta_sa.exe`); err != nil {
		return fmt.Errorf("register mapped GTA executable: %w", err)
	}
	if id == ClientMTA {
		if err := set(`SOFTWARE\Multi Theft Auto: San Andreas All\Common`, "GTA:SA Path", gamePath); err != nil {
			return fmt.Errorf("register mapped MTA GTA folder: %w", err)
		}
	}
	return nil
}
