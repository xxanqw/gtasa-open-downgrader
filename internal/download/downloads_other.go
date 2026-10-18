//go:build !windows

package download

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func DownloadsDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if config, err := os.UserConfigDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(config, "user-dirs.dirs")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if value, ok := strings.CutPrefix(strings.TrimSpace(line), "XDG_DOWNLOAD_DIR="); ok {
					directory, err := strconv.Unquote(strings.TrimSpace(value))
					if err != nil {
						continue
					}
					if directory == "$HOME" {
						directory = home
					} else if strings.HasPrefix(directory, "$HOME/") {
						directory = filepath.Join(home, directory[len("$HOME/"):])
					}
					if filepath.IsAbs(directory) && !strings.ContainsAny(directory, "$`\x00") {
						return filepath.Clean(directory), nil
					}
				}
			}
		}
	}
	return filepath.Join(home, "Downloads"), nil
}
