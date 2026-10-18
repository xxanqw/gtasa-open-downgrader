// Package winedrive maintains a prefix-local GTA-only drive view. It never
// changes other Wine drives, creates a prefix, edits hardware IDs or copies GTA.
package winedrive

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ChildName = "Grand Theft Auto San Andreas"
const HelperFlag = "--gtasa-prepare-game-drive-v1"
const owner = "GTASA-OPEN-DOWNGRADER-GAME-DRIVE-1\n"

func ViewPath(driveC string) string {
	return filepath.Join(driveC, ".gtasa-open-downgrader", "game-drive")
}
func GamePath(letter byte) string { return string(letter) + `:\` + ChildName }
func SameDirectory(a, b string) bool {
	x, e := os.Stat(a)
	y, f := os.Stat(b)
	return e == nil && f == nil && x.IsDir() && y.IsDir() && os.SameFile(x, y)
}

// EnsureView owns only its marked control directory. Unexpected entries and
// replaced game links are refused, not deleted. Native and Wine startup share
// the same on-disk directory and marker, including external Steam libraries.
func EnsureView(driveC, game string) (string, error) {
	game, err := filepath.Abs(game)
	if err != nil {
		return "", err
	}
	if st, e := os.Stat(game); e != nil || !st.IsDir() {
		return "", fmt.Errorf("GTA drive target is not an existing directory")
	}
	if st, e := os.Stat(driveC); e != nil || !st.IsDir() {
		return "", fmt.Errorf("existing Wine/Proton drive_c is missing; the prefix will not be created")
	}
	control := filepath.Join(driveC, ".gtasa-open-downgrader")
	view := ViewPath(driveC)
	if rel, e := filepath.Rel(game, control); e == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)) {
		return "", fmt.Errorf("GTA directory cannot contain its own filtered drive view")
	}
	marker := filepath.Join(control, "owner")
	if err = os.Mkdir(control, 0700); err == nil {
		file, e := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return "", e
		}
		_, e = file.WriteString(owner)
		e = errors.Join(e, file.Close())
		if e != nil {
			return "", e
		}
	} else if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if st, e := os.Lstat(control); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("unsafe game-drive control directory")
	}
	st, err := os.Lstat(marker)
	if err != nil || !st.Mode().IsRegular() || st.Size() != int64(len(owner)) {
		return "", fmt.Errorf("game-drive control directory is not owned by this application")
	}
	file, err := os.Open(marker)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(len(owner))+1))
	file.Close()
	if err != nil || !bytes.Equal(data, []byte(owner)) {
		return "", fmt.Errorf("invalid game-drive ownership marker")
	}
	entries, err := os.ReadDir(control)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name() != "owner" && entry.Name() != "game-drive" {
			return "", fmt.Errorf("unexpected file in game-drive control directory; nothing was removed")
		}
	}
	if err = os.Mkdir(view, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if st, e := os.Lstat(view); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("unsafe filtered game-drive directory")
	}
	entries, err = os.ReadDir(view)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name() != ChildName {
			return "", fmt.Errorf("filtered game drive contains unexpected files; nothing was removed")
		}
	}
	child := filepath.Join(view, ChildName)
	if SameDirectory(child, game) {
		return view, nil
	}
	if _, e := os.Lstat(child); e == nil {
		return "", fmt.Errorf("filtered GTA link points elsewhere; it will not be overwritten")
	} else if !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	if err = createGameLink(game, child); err != nil {
		if SameDirectory(child, game) {
			return view, nil
		}
		return "", fmt.Errorf("create filtered GTA link: %w", err)
	}
	if !SameDirectory(child, game) {
		return "", fmt.Errorf("filtered GTA link did not resolve to the selected game")
	}
	return view, nil
}
