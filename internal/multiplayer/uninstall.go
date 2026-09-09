package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var SAMPFontFiles = []string{"gtaweap3.ttf", "sampaux3.ttf"}

type UninstallResult struct {
	RestartRequired bool
}

func UninstallerPath(client Client, environment Environment) (string, error) {
	if !client.Installed {
		return "", fmt.Errorf("%s is not installed", client.Name)
	}
	var path string
	switch client.ID {
	case ClientSAMP:
		path = filepath.Join(environment.GameDir, "SAMPUninstall.exe")
	case ClientOpenMP:
		path = filepath.Join(client.WorkingDir, "uninstall.exe")
	case ClientMTA:
		path = filepath.Join(client.WorkingDir, "Uninstall.exe")
	default:
		return "", fmt.Errorf("unsupported multiplayer client %q", client.ID)
	}
	if err := ValidateInstaller(path); err != nil {
		return "", fmt.Errorf("%s uninstaller is unavailable: %w", client.Name, err)
	}
	return path, nil
}

// Uninstall runs the client's own uninstaller. For open.mp it also removes the
// managed command-line client data added by the Linux integration.
func Uninstall(ctx context.Context, id ClientID, environment Environment, runner Runner) (UninstallResult, error) {
	if runner == nil {
		return UninstallResult{}, fmt.Errorf("uninstaller execution is unavailable")
	}
	client, ok := Find(Detect(environment), id)
	if !ok || !client.Installed {
		return UninstallResult{}, fmt.Errorf("%s is not installed", clientName(id))
	}
	uninstaller, err := UninstallerPath(client, environment)
	if err != nil {
		return UninstallResult{}, err
	}
	fontPaths := sampFontPaths(environment.GameDir)
	if id == ClientSAMP {
		releaseSAMPFonts(fontPaths)
	}
	runErr := runner.Run(ctx, uninstaller)
	if err := ctx.Err(); err != nil {
		return UninstallResult{}, err
	}
	remaining, _ := Find(Detect(environment), id)
	if remaining.Installed {
		if runErr != nil {
			return UninstallResult{}, runErr
		}
		return UninstallResult{}, fmt.Errorf("%s uninstaller closed before removing the client", client.Name)
	}
	// Some uninstallers finish through a child process and return a non-zero
	// bootstrap status. The missing client executable is the completion proof.
	if id == ClientOpenMP {
		dataRoot, dataErr := OpenMPDataPath(environment)
		if dataErr == nil {
			if dataErr = os.RemoveAll(dataRoot); dataErr != nil {
				return UninstallResult{}, fmt.Errorf("remove open.mp managed client data: %w", dataErr)
			}
		}
	}
	result := UninstallResult{}
	if id == ClientSAMP {
		releaseSAMPFonts(fontPaths)
		result.RestartRequired, err = cleanupSAMPFonts(fontPaths, os.Remove, scheduleRemoval)
		if err != nil {
			return UninstallResult{}, err
		}
	}
	return result, nil
}

func clientName(id ClientID) string {
	if installer, ok := Installers[id]; ok {
		return installer.Name
	}
	return string(id)
}

func sampFontPaths(gameDir string) []string {
	paths := make([]string, 0, len(SAMPFontFiles))
	for _, name := range SAMPFontFiles {
		paths = append(paths, filepath.Join(gameDir, name))
	}
	return paths
}

func cleanupSAMPFonts(paths []string, remove func(string) error, schedule func(string) error) (bool, error) {
	restart := false
	var failures []error
	for _, path := range paths {
		err := remove(path)
		if err == nil || os.IsNotExist(err) {
			continue
		}
		if scheduleErr := schedule(path); scheduleErr != nil {
			failures = append(failures, fmt.Errorf("remove SA-MP font %s: %v; schedule after restart: %w", filepath.Base(path), err, scheduleErr))
			continue
		}
		restart = true
	}
	return restart, errors.Join(failures...)
}
