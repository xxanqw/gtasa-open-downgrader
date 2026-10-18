package winedrive

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FindHelper uses the matching release component, never an installed game
// executable or an old launcher that might ignore the flag and auto-play.
func FindHelper() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(executable)
	var candidates []string
	if root := os.Getenv("APPDIR"); root != "" {
		candidates = append(candidates, filepath.Join(root, "usr", "libexec", "gta-sa.exe"))
	}
	candidates = append(candidates, filepath.Join(dir, "gta-sa.exe"), filepath.Join(dir, "libexec", "gta-sa.exe"), filepath.Join(filepath.Dir(dir), "libexec", "gta-sa.exe"))
	for _, candidate := range candidates {
		file, e := os.Open(candidate)
		if e != nil {
			continue
		}
		st, e := file.Stat()
		if e != nil || !st.Mode().IsRegular() || st.Size() > 64<<20 {
			file.Close()
			continue
		}
		data, e := io.ReadAll(io.LimitReader(file, 64<<20+1))
		file.Close()
		if e == nil && len(data) > 2 && bytes.Equal(data[:2], []byte("MZ")) && bytes.Contains(data, []byte(HelperFlag)) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Wine drive preparation needs the matching gta-sa.exe component beside this build (or in AppImage libexec); update both files, not an old installed launcher")
}
