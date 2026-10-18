package launcherapp

import (
	"path"
	"strings"
)

// Keep Steam's launch context on the parent chooser and all other clients. MTA
// started from Explorer works without these inherited GTA launch identifiers.
// Wine/Proton must retain their Steam environment to use the selected prefix.
func mtaChildEnvironment(executable, hostOS string, underWine bool, inherited []string) []string {
	if hostOS != "windows" || underWine || !strings.EqualFold(path.Base(strings.ReplaceAll(executable, `\`, "/")), "Multi Theft Auto.exe") {
		return nil // Preserve exec.Cmd's normal inheritance behavior.
	}
	steamLaunch := false
	for _, entry := range inherited {
		key, value, ok := strings.Cut(entry, "=")
		if ok && value != "" && steamLaunchKey(strings.ToLower(key)) {
			steamLaunch = true
			break
		}
	}
	if !steamLaunch {
		return nil
	}
	environment := make([]string, 0, len(inherited)+2)
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToLower(key)
		if steamLaunchKey(key) || key == "steamnooverlayui" || key == "steamnooverlayuidrawing" {
			continue
		}
		environment = append(environment, entry)
	}
	// Overlay suppression is a request, not a guarantee that Steam's injection
	// hooks cannot load. Steam's per-game Overlay setting remains the fallback.
	return append(environment, "SteamNoOverlayUI=1", "SteamNoOverlayUIDrawing=1")
}

func steamLaunchKey(key string) bool {
	switch key {
	case "steamappid", "steamgameid", "steamclientlaunch", "steamenv", "steamclientdll", "steamclientdll64":
		return true
	}
	return strings.HasPrefix(key, "steamoverlay")
}
