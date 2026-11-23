package multiplayer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const (
	OpenMPPortableURL        = "https://github.com/openmultiplayer/launcher/releases/download/v1.7.0/omp-launcher.exe"
	openMPPortableSHA256     = "8a877b1062972bb28302f3354dbf4428779d593f56014e35928062becf430daa"
	openMPPortableDirectory  = "mp.open.launcher/gtasa-downgrader"
	openMPPortableExecutable = "omp-launcher.exe"
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

func OpenMPSAMPPath(root, versionID string) (string, error) {
	version, ok := FindOpenMPSAMPVersion(versionID)
	if ok {
		return filepath.Join(root, filepath.FromSlash(version.Path)), nil
	}
	return "", fmt.Errorf("unsupported open.mp SA-MP client version %q", versionID)
}

func ValidateOpenMPSAMPInstallation(environment Environment, client Client) error {
	root, err := OpenMPSAMPRoot(environment, client)
	if err != nil {
		return err
	}
	return validateOpenMPSAMPRoot(root)
}

func OpenMPSAMPRoot(environment Environment, client Client) (string, error) {
	if client.Portable {
		return fsutil.Join(client.WorkingDir, "samp")
	}
	root, err := OpenMPDataPath(environment)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "samp"), nil
}

func openMPPortablePath(environment Environment) (string, error) {
	root, err := OpenMPDataPath(environment)
	if err != nil {
		return "", err
	}
	return fsutil.Join(filepath.Dir(root), openMPPortableDirectory+"/"+openMPPortableExecutable)
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

// InstallOpenMPPortable publishes the launcher and SA-MP files together. The
// open.mp CLI reads its plugin from a fixed path under Local AppData.
func InstallOpenMPPortable(ctx context.Context, environment Environment, launcher, plugin, archive string) error {
	if environment.LocalAppData == "" {
		return fmt.Errorf("the GTA Proton prefix is unavailable")
	}
	if err := verifyOpenMPAsset(ctx, launcher, openMPPortableSHA256); err != nil {
		return fmt.Errorf("validate open.mp portable launcher: %w", err)
	}
	if err := ValidateOpenMPClient(plugin); err != nil {
		return err
	}
	if err := verifyOpenMPAsset(ctx, archive, OpenMPSAMPArchiveSHA256); err != nil {
		return fmt.Errorf("validate open.mp SA-MP archive: %w", err)
	}
	unlock, err := lockOpenMPPortable(environment.LocalAppData)
	if err != nil {
		return err
	}
	defer unlock()
	return installOpenMPPortable(ctx, environment, launcher, plugin, archive)
}

func installOpenMPPortable(ctx context.Context, environment Environment, launcher, plugin, archive string) error {
	executable, err := openMPPortablePath(environment)
	if err != nil {
		return err
	}
	target := filepath.Dir(executable)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return fmt.Errorf("prepare open.mp data directory: %w", err)
	}
	work, err := os.MkdirTemp(parent, ".openmp-staged-")
	if err != nil {
		return fmt.Errorf("stage open.mp portable installation: %w", err)
	}
	backup := filepath.Join(work, "previous")
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(work)
		}
	}()
	staged := filepath.Join(work, "portable")
	samp := filepath.Join(staged, "samp")
	if err := fsutil.Extract(ctx, archive, samp, "7z", nil); err != nil {
		return fmt.Errorf("extract open.mp SA-MP clients: %w", err)
	}
	if err := validateOpenMPSAMPRoot(samp); err != nil {
		return err
	}
	if err := fsutil.CopyFile(ctx, launcher, filepath.Join(staged, openMPPortableExecutable)); err != nil {
		return fmt.Errorf("stage open.mp portable launcher: %w", err)
	}
	destination, err := OpenMPClientPath(environment)
	if err != nil {
		return err
	}
	if err := fsutil.CopyFile(ctx, plugin, destination); err != nil {
		return fmt.Errorf("install open.mp command-line client: %w", err)
	}
	hadPrevious := true
	if err := os.Rename(target, backup); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("preserve existing open.mp portable installation: %w", err)
		}
		hadPrevious = false
	}
	if err := os.Rename(staged, target); err != nil {
		installErr := fmt.Errorf("install open.mp portable launcher: %w", err)
		if hadPrevious {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				keepBackup = true
				return errors.Join(installErr, fmt.Errorf("restore previous open.mp installation from %s: %w", backup, restoreErr))
			}
		}
		return installErr
	}
	return nil
}

func uninstallOpenMPPortable(ctx context.Context, environment Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := openMPPortablePath(environment)
	if err != nil {
		return err
	}
	environment.OpenMPProton = false
	legacy, _ := Find(Detect(environment), ClientOpenMP)
	// An older installation can still use the plugin at this shared path.
	if !legacy.Installed {
		plugin, err := OpenMPClientPath(environment)
		if err != nil {
			return err
		}
		if err := os.Remove(plugin); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove open.mp command-line client: %w", err)
		}
	}
	if err := os.RemoveAll(filepath.Dir(executable)); err != nil {
		return fmt.Errorf("remove open.mp portable installation: %w", err)
	}
	return nil
}

func verifyOpenMPAsset(ctx context.Context, path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := fsutil.Copy(ctx, hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return fmt.Errorf("SHA-256 checksum does not match the supported release")
	}
	return nil
}
