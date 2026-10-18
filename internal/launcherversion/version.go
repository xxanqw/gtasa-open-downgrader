// Package launcherversion identifies launcher builds without executing them.
package launcherversion

import (
	"bytes"
	"debug/buildinfo"
	_ "embed"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const prefix = "GTASA_STEAM_LAUNCHER_VERSION["
const Program = "github.com/xxanqw/gtasa-open-downgrader/cmd/steam-launcher"

// Identity is stamped by the release scripts and retained by Version. A marker
// is used because Go omits linker flags from build info when -trimpath is set.
var Identity = "development"

//go:embed VERSION
var defaultVersion string

func Version() string {
	if Identity == "development" {
		return strings.TrimSpace(defaultVersion)
	}
	return strings.TrimSuffix(strings.TrimPrefix(Identity, prefix), "]")
}

// Launcher versions are independent, monotonically increasing release numbers.
func Valid(version string) bool {
	return len(version) <= 20 && regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(version)
}

func Newer(candidate, current string) bool {
	return Valid(candidate) && Valid(current) && (len(candidate) > len(current) || len(candidate) == len(current) && candidate > current)
}

func ReadFile(path string) (string, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read launcher build identity: %w", err)
	}
	if info.Path != Program {
		return "", fmt.Errorf("executable is not the Steam launcher")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 128<<20 {
		return "", fmt.Errorf("invalid launcher binary size/type")
	}
	pattern := regexp.MustCompile(`GTASA_STEAM_LAUNCHER_VERSION\[([0-9A-Za-z.+-]{1,100})\]`)
	buffer := make([]byte, 64<<10)
	var carry []byte
	version := ""
	for {
		n, err := file.Read(buffer)
		data := append(carry, buffer[:n]...)
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			candidate := string(match[1])
			if !Valid(candidate) {
				return "", fmt.Errorf("invalid launcher release version")
			}
			if version != "" && version != candidate {
				return "", fmt.Errorf("ambiguous launcher release version")
			}
			version = candidate
		}
		carry = bytes.Clone(data[max(0, len(data)-256):])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	if version == "" {
		return "", fmt.Errorf("launcher has no release version; rebuild with the release scripts")
	}
	return version, nil
}
