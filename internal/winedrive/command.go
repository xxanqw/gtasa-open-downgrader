package winedrive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// HandleHelper runs before Fyne or remembered launch choices. It cannot open
// the chooser or play a game, even when a mode is remembered in that folder.
func HandleHelper(args []string) (bool, error) {
	if len(args) == 0 || args[0] != HelperFlag {
		return false, nil
	}
	if len(args) != 2 {
		return true, fmt.Errorf("game-drive helper requires exactly one GTA directory")
	}
	if !RunningUnderWine() {
		return true, fmt.Errorf("game-drive helper requires the selected Wine/Proton prefix")
	}
	game, err := filepath.Abs(args[1])
	if err != nil {
		return true, err
	}
	file, err := os.Open(filepath.Join(game, "gta_sa.exe"))
	if err != nil {
		return true, fmt.Errorf("selected GTA executable is missing: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return true, fmt.Errorf("selected GTA executable is not an ordinary file")
	}
	var mz [2]byte
	_, err = io.ReadFull(file, mz[:])
	file.Close()
	if err != nil || string(mz[:]) != "MZ" {
		return true, fmt.Errorf("selected GTA executable is invalid")
	}
	result, err := EnsureWineGameDrive(game)
	if err == nil {
		fmt.Fprintln(os.Stdout, result)
	}
	return true, err
}
