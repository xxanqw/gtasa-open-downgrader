package multiplayer

import (
	"os"
	"regexp"
	"strconv"
)

const MTASerialGuideURL = "https://help.multitheftauto.com/sa/trouble/serial-validation/"
const MTALinuxGuideURL = "https://wiki.multitheftauto.com/wiki/Client_on_Linux_Manual"
const MTAHTTPWarning = "GE-Proton <11-8 may have HTTP problems during longer MTA downloads. Updating is recommended."

var geProtonVersion = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:ge[-_ ]?proton|proton[-_ ]?ge)[-_ v]*(\d+)[.-](\d+)(?:$|[^0-9])`)
var legacyGEProtonVersion = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])proton[-_ v]*(\d+)\.(\d+)(?:rc\d+)?[-_]ge(?:[-_]|$)`)

// MTAProtonWarning is advisory, never a compatibility gate. Unknown/custom and
// Valve builds are not guessed to be old GE releases. Compare numbers, not text.
func MTAProtonWarning(tool string) string {
	match := geProtonVersion.FindStringSubmatch(tool)
	if match == nil {
		match = legacyGEProtonVersion.FindStringSubmatch(tool)
	}
	if match == nil {
		return ""
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return ""
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return ""
	}
	if major < 11 || major == 11 && minor < 8 {
		return MTAHTTPWarning
	}
	return ""
}

// RunningProtonTool also works inside the Windows launcher under Proton. The
// tool-path variables describe the running build, not a stale saved preference.
func RunningProtonTool() string {
	for _, key := range []string{"GTASA_PROTON_TOOL", "PROTON_VERSION", "STEAM_COMPAT_TOOL_PATHS", "WINELOADER", "WINEDLLPATH"} {
		value := os.Getenv(key)
		if geProtonVersion.MatchString(value) || legacyGEProtonVersion.MatchString(value) {
			return value
		}
	}
	return ""
}
