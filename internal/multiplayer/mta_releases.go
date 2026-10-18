package multiplayer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// MTARelease identifies a reviewed client installer, never a server package.
// The 1.7 artifact is explicitly an untested development build, not stable.
type MTARelease struct {
	Version, Channel, Build, SHA256 string
	Installer                       Installer
	Experimental                    bool
}

var MTAReleases = []MTARelease{
	{Version: "1.6", Channel: "Stable", Build: "1.6.0.24149", SHA256: "c8a2963b0d3541670567f3660fee8b0d6bb3fc93ca55ff4a13d1d7d4982608c8", Installer: Installers[ClientMTA]},
	{Version: "1.7", Channel: "Development / untested", Build: "26966 · 2026-10-09", Experimental: true, SHA256: "d058b3b8031ec66a551c077b929c9412965acf931a53d4f03f6a3234857bac34", Installer: Installer{Client: ClientMTA, Name: "MTA 1.7 development", URL: "https://nightly.multitheftauto.com/mtasa-1.7-untested-26966-20261009.exe", OfficialPage: "https://nightly.multitheftauto.com/?branch=master", FileName: "mtasa-1.7-untested-26966-20261009.exe"}},
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
func ValidateMTAInstaller(ctx context.Context, path string, release MTARelease) error {
	expected, ok := FindMTARelease(release.Version)
	if !ok || expected != release {
		return fmt.Errorf("unrecognized MTA release")
	}
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() > 200<<20 {
		return fmt.Errorf("MTA installer must be an ordinary file no larger than 200 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 128<<10)
	var total int64
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, e := file.Read(buffer)
		if n > 0 {
			total += int64(n)
			if total > 200<<20 {
				return fmt.Errorf("MTA installer exceeds 200 MiB limit")
			}
			hash.Write(buffer[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != release.SHA256 {
		return fmt.Errorf("MTA %s installer checksum changed or is corrupt; it will not run. Use a reviewed application update", release.Version)
	}
	return ValidateInstaller(path)
}
