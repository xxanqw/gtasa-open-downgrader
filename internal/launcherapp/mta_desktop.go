package launcherapp

import (
	"errors"
	"path"
	"strings"
)

// A dispatched client may still start GTA after the dispatching process exits.
// Do not restore game files when its lifetime cannot be established safely.
var errMTAExitUnknown = errors.New("MTA exit could not be confirmed; compatibility files remain paused. Close MTA and GTA, then start the launcher again to recover them")

func desktopMTATarget(executable, hostOS string, underWine bool) bool {
	return hostOS == "windows" && !underWine && strings.EqualFold(path.Base(strings.ReplaceAll(executable, `\`, "/")), "Multi Theft Auto.exe")
}
