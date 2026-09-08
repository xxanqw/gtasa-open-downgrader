//go:build windows

package multiplayer

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func registryCandidates() map[ClientID][]string {
	result := map[ClientID][]string{}
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, keyName := range []string{`SOFTWARE\Open Multiplayer Launcher`, `SOFTWARE\WOW6432Node\Open Multiplayer Launcher`, `Software\open\Open Multiplayer Launcher`} {
			if value := registryString(root, keyName, ""); value != "" {
				result[ClientOpenMP] = append(result[ClientOpenMP], executable(value, "omp-launcher.exe"))
			}
		}
		for _, version := range []string{"1.7", "1.6", "1.5"} {
			for _, keyName := range []string{`SOFTWARE\Multi Theft Auto: San Andreas All\` + version, `SOFTWARE\WOW6432Node\Multi Theft Auto: San Andreas All\` + version} {
				if value := registryString(root, keyName, "Last Install Location"); value != "" {
					result[ClientMTA] = append(result[ClientMTA], executable(value, "Multi Theft Auto.exe"))
				}
			}
		}
	}
	return result
}

func registryString(root registry.Key, path, name string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_32KEY)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func executable(value, name string) string {
	if strings.EqualFold(filepath.Base(value), name) {
		return value
	}
	return filepath.Join(value, name)
}
