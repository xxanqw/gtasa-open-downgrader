package multiplayer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const (
	OpenMPSAMPArchiveURL     = "https://assets.open.mp/samp_clients.7z"
	OpenMPSAMPArchiveSHA256  = "0f9d3b9729788aec3e01e44fd904d4b1cf339866e400f992addea7cec046d1c2"
	DefaultOpenMPSAMPVersion = "037R5"
)

type OpenMPSAMPVersion struct {
	ID   string
	Name string
	Path string
}

var OpenMPSAMPVersions = []OpenMPSAMPVersion{
	{ID: "037R5", Name: "0.3.7-R5 (recommended)", Path: "0.3.7-R5/samp.dll"},
	{ID: "03DL", Name: "0.3.DL", Path: "0.3.DL/samp.dll"},
	{ID: "037R4", Name: "0.3.7-R4", Path: "0.3.7-R4/samp.dll"},
	{ID: "037R31", Name: "0.3.7-R3-1", Path: "0.3.7-R3-1/samp.dll"},
	{ID: "037R3", Name: "0.3.7-R3", Path: "0.3.7-R3/samp.dll"},
	{ID: "037R2", Name: "0.3.7-R2", Path: "0.3.7-R2/samp.dll"},
	{ID: "037R1", Name: "0.3.7-R1", Path: "0.3.7-R1/samp.dll"},
}

var OpenMPSharedFiles = []string{
	"bass.dll", "gtaweap3.ttf", "mouse.png", "samp.saa", "sampaux3.ttf", "sampgui.png",
	"SAMP/blanktex.txd", "SAMP/CUSTOM.ide", "SAMP/custom.img", "SAMP/samaps.txd",
	"SAMP/SAMP.ide", "SAMP/SAMP.img", "SAMP/SAMP.ipl", "SAMP/SAMPCOL.img",
}

func FindOpenMPSAMPVersion(id string) (OpenMPSAMPVersion, bool) {
	if id == "" {
		id = DefaultOpenMPSAMPVersion
	}
	for _, version := range OpenMPSAMPVersions {
		if version.ID == id {
			return version, true
		}
	}
	return OpenMPSAMPVersion{}, false
}

func OpenMPDataPath(environment Environment) (string, error) {
	root := environment.LocalAppData
	if root == "" {
		var err error
		root, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("locate Local AppData: %w", err)
		}
	}
	if root == "" {
		return "", fmt.Errorf("Local AppData is unavailable")
	}
	return filepath.Join(root, "mp.open.launcher"), nil
}

func OpenMPSAMPPath(environment Environment, versionID string) (string, error) {
	root, err := OpenMPDataPath(environment)
	if err != nil {
		return "", err
	}
	version, ok := FindOpenMPSAMPVersion(versionID)
	if ok {
		return filepath.Join(root, "samp", filepath.FromSlash(version.Path)), nil
	}
	return "", fmt.Errorf("unsupported open.mp SA-MP client version %q", versionID)
}

func ValidateOpenMPSAMPInstallation(environment Environment) error {
	root, err := OpenMPDataPath(environment)
	if err != nil {
		return err
	}
	return validateOpenMPSAMPRoot(filepath.Join(root, "samp"))
}

func validateOpenMPSAMPRoot(root string) error {
	for _, version := range OpenMPSAMPVersions {
		path := filepath.Join(root, filepath.FromSlash(version.Path))
		if err := ValidateInstaller(path); err != nil {
			return fmt.Errorf("open.mp SA-MP %s is invalid: %w", version.Name, err)
		}
	}
	for _, relative := range OpenMPSharedFiles {
		path := filepath.Join(root, "shared", filepath.FromSlash(relative))
		if info, statErr := os.Stat(path); statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("open.mp managed SA-MP file is missing: %s", relative)
		}
	}
	return nil
}

func InstallOpenMPSAMPArchive(ctx context.Context, archive string, environment Environment) error {
	digest, err := fileSHA256(ctx, archive)
	if err != nil {
		return fmt.Errorf("validate open.mp SA-MP archive: %w", err)
	}
	if digest != OpenMPSAMPArchiveSHA256 {
		return fmt.Errorf("open.mp SA-MP archive checksum does not match the supported release")
	}
	dataRoot, err := OpenMPDataPath(environment)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataRoot, 0700); err != nil {
		return fmt.Errorf("prepare open.mp data directory: %w", err)
	}
	staged, err := os.MkdirTemp(dataRoot, ".samp-staged-")
	if err != nil {
		return fmt.Errorf("stage open.mp SA-MP clients: %w", err)
	}
	defer os.RemoveAll(staged)
	if err := fsutil.Extract(ctx, archive, staged, "7z", nil); err != nil {
		return fmt.Errorf("extract open.mp SA-MP clients: %w", err)
	}
	if err := validateOpenMPSAMPRoot(staged); err != nil {
		return err
	}
	target := filepath.Join(dataRoot, "samp")
	backup := filepath.Join(dataRoot, ".samp-backup")
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("preserve existing open.mp SA-MP clients: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("install open.mp SA-MP clients: %w", err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

func fileSHA256(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := hash.Write(buffer[:n]); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
