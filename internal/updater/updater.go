// Package updater selects matching release assets and stages self-updates.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	"golang.org/x/mod/semver"
)

var Version = "v0.2.2"

const API = "https://api.github.com/repos/xxanqw/gtasa-open-downgrader/releases/latest"

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type Release struct {
	Tag    string  `json:"tag_name"`
	URL    string  `json:"html_url"`
	Assets []Asset `json:"assets"`
}

func Newer(candidate, current string) bool {
	candidate = "v" + strings.TrimPrefix(candidate, "v")
	current = "v" + strings.TrimPrefix(current, "v")
	return semver.IsValid(candidate) && semver.IsValid(current) && semver.Compare(candidate, current) > 0
}
func Select(r Release, system string, offline, appimage bool) (Asset, bool) {
	name := "gtasa-open-downgrader-" + system
	if offline {
		name += "-offline"
	}
	switch system {
	case "windows":
		name += ".exe"
	case "linux":
		if appimage {
			name += ".AppImage"
		}
	default:
		return Asset{}, false
	}
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}
func Check(ctx context.Context, c *download.Client) (Release, error) {
	var r Release
	e := c.JSON(ctx, API, &r)
	return r, e
}
func Executable() string {
	if p := os.Getenv("APPIMAGE"); p != "" {
		return p
	}
	p, _ := os.Executable()
	return p
}
func Prepare(ctx context.Context, c *download.Client, a Asset, target string, p download.Progress) (string, error) {
	f, e := os.CreateTemp(filepath.Dir(target), ".downgrader-update-*")
	if e != nil {
		return "", e
	}
	staged := f.Name()
	f.Close()
	ok := false
	defer func() {
		if !ok {
			os.Remove(staged)
		}
	}()
	if e = c.File(ctx, a.URL, staged, p); e != nil {
		return "", e
	}
	st, e := os.Stat(staged)
	if e != nil {
		return "", e
	}
	if a.Size > 0 && st.Size() != a.Size {
		return "", fmt.Errorf("update size does not match release metadata")
	}
	f, e = os.Open(staged)
	if e != nil {
		return "", e
	}
	head := make([]byte, 4)
	_, e = io.ReadFull(f, head)
	f.Close()
	if e != nil {
		return "", e
	}
	if string(head[:2]) != "MZ" && string(head) != "\x7fELF" {
		return "", fmt.Errorf("download is not an executable")
	}
	if a.Digest != "" {
		if !strings.HasPrefix(a.Digest, "sha256:") {
			return "", fmt.Errorf("unsupported release checksum")
		}
		f, e = os.Open(staged)
		if e != nil {
			return "", e
		}
		h := sha256.New()
		_, e = fsutil.Copy(ctx, h, f)
		f.Close()
		if e != nil {
			return "", e
		}
		if hex.EncodeToString(h.Sum(nil)) != strings.TrimPrefix(a.Digest, "sha256:") {
			return "", fmt.Errorf("update checksum mismatch")
		}
	}
	if e = os.Chmod(staged, 0755); e != nil {
		return "", e
	}
	ok = true
	return staged, nil
}

// StartHelper launches a copy outside the AppImage mount / Windows locked EXE.
// The caller must close its workers and exit after this succeeds.
func StartHelper(ctx context.Context, staged, target string) error {
	current, e := os.Executable()
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "gtasa-update-helper-*")
	if e != nil {
		return e
	}
	helper := filepath.Join(dir, "updater"+filepath.Ext(current))
	if e = fsutil.CopyFile(ctx, current, helper); e != nil {
		os.RemoveAll(dir)
		return e
	}
	cmd := exec.Command(helper, "--apply-update", strconv.Itoa(os.Getpid()), staged, target)
	platform.HideConsole(cmd)
	log, e := os.OpenFile(target+".update.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		os.RemoveAll(dir)
		return e
	}
	defer log.Close()
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		os.RemoveAll(dir)
		return e
	}
	return cmd.Process.Release()
}
func ApplyHelper(ctx context.Context, parent int, staged, target string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if e := platform.WaitProcess(ctx, parent); e != nil {
		return e
	}
	previous := target + ".previous"
	if _, e := os.Stat(previous); e == nil {
		return fmt.Errorf("previous executable already exists: %s", previous)
	}
	if e := os.Rename(target, previous); e != nil {
		return e
	}
	if e := os.Rename(staged, target); e != nil {
		rollback := os.Rename(previous, target)
		return fmt.Errorf("update replacement: %v; rollback: %v", e, rollback)
	}
	cmd := exec.Command(target)
	platform.HideConsole(cmd)
	if e := cmd.Start(); e != nil {
		failed := target + ".failed"
		_ = os.Rename(target, failed)
		rollback := os.Rename(previous, target)
		return fmt.Errorf("restart: %v; rollback: %v", e, rollback)
	}
	_ = cmd.Process.Release()
	return os.Remove(previous)
}
