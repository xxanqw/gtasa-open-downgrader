package ui

import "github.com/xxanqw/gtasa-open-downgrader/internal/mods"

func installedModLabel(version string) string {
	if version == "" {
		version = "version unknown"
	}
	return "Installed · " + version
}

// Release checks add information to the locally installed state; a failed check
// or a newer upstream release must never make an installed mod look absent.
func modUpdateLabel(installed, latest mods.Release, err error) string {
	label := installedModLabel(installed.Version)
	switch {
	case err != nil:
		return label + " · Check unavailable"
	case installed.Fingerprint == "" || latest.Fingerprint == "":
		if installed.Version != "" {
			return label + " · Version untracked"
		}
	case installed.Fingerprint != latest.Fingerprint:
		return label + " · Update available: " + latest.Version
	}
	return label
}
