package multiplayer

// MTARelease identifies an installable client channel.
type MTARelease struct {
	Version, Channel string
	Experimental     bool
}

var MTAReleases = []MTARelease{
	{Version: "1.6", Channel: "Stable"},
	{Version: "1.7", Channel: "Development / untested", Experimental: true},
}

const MTABetaWarning = "MTA 1.7 is an official development/untested build, not a stable release. It requires Windows 10 or later (including the existing Wine/Proton prefix's Windows compatibility setting). Keep 1.6 installed separately, do not point this installer at the 1.6 directory, and expect server/protocol or anti-cheat incompatibilities. No prefix reset or architecture conversion is performed. Installing it does not change this manager's saved client choice. Installer-created shortcuts and mtasa:// associations are separate from this choice."

func FindMTARelease(version string) (MTARelease, bool) {
	for _, r := range MTAReleases {
		if r.Version == version {
			return r, true
		}
	}
	return MTARelease{}, false
}

func MTAVersionSupported(version string) bool {
	return version == "1.6" || version == "1.7" || version == "1.5" || version == "custom"
}
