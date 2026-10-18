package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// MTAInstallation identifies a detected family, not a certified core build.
type MTAInstallation struct {
	Version string
	Client  Client
}

var mtaFamilies = []string{"1.6", "1.7", "1.5", "custom"}

func mtaFamilyFromPath(path string) string {
	normalized := "/" + strings.ToLower(strings.ReplaceAll(path, `\`, "/")) + "/"
	for _, version := range []string{"1.6", "1.7", "1.5"} {
		if strings.Contains(normalized, "/mta san andreas "+version+"/") {
			return version
		}
	}
	return "custom"
}

// DetectMTAInstallations keeps family-specific Registry/default paths separate.
// Installing a development build never silently makes it the selected client.
func DetectMTAInstallations(environment Environment) []MTAInstallation {
	candidates := mtaRegistryCandidates()
	if candidates == nil {
		candidates = map[string][]string{}
	}
	if config, err := LoadConfig(environment.GameDir); err == nil {
		if value := config.Clients[ClientMTA].Executable; value != "" {
			path := filepath.Join(environment.GameDir, filepath.FromSlash(value))
			version := mtaFamilyFromPath(path)
			if family, err := MTAExecutableFamily(path); err == nil {
				version = family
			}
			candidates[version] = append([]string{path}, candidates[version]...)
		}
	}
	for _, version := range []string{"1.6", "1.7", "1.5"} {
		candidates[version] = append(candidates[version], prefixCandidates(environment.PrefixDriveC, []string{"Program Files (x86)/MTA San Andreas " + version + "/Multi Theft Auto.exe", "Program Files/MTA San Andreas " + version + "/Multi Theft Auto.exe"})...)
	}
	var result []MTAInstallation
	for _, version := range mtaFamilies {
		for _, candidate := range candidates[version] {
			if regular(candidate) {
				if family, err := MTAExecutableFamily(candidate); err == nil && family != version {
					continue
				}
				result = append(result, MTAInstallation{version, Client{ID: ClientMTA, Name: "MTA " + version, Installed: true, Executable: filepath.Clean(candidate), WorkingDir: filepath.Dir(candidate), Version: version}})
				break
			}
		}
	}
	return result
}
func FindMTAInstallation(items []MTAInstallation, version string) (Client, bool) {
	for _, item := range items {
		if item.Version == version {
			return item.Client, true
		}
	}
	return Client{ID: ClientMTA, Name: "MTA " + version, Version: version}, false
}
func SelectedMTAClient(environment Environment) Client {
	missing := Client{ID: ClientMTA, Name: "Multi Theft Auto"}
	items := DetectMTAInstallations(environment)
	choice, err := LoadMTAChoice(environment.GameDir)
	if err != nil {
		missing.Detail = "MTA version selection is invalid; open the MTA manager to repair it: " + err.Error()
		return missing
	}
	if choice != "" {
		client, ok := FindMTAInstallation(items, choice)
		if ok {
			if choice != "custom" {
				family, err := MTAExecutableFamily(client.Executable)
				if err != nil || family != choice {
					missing.Version = choice
					missing.Detail = "Selected MTA " + choice + " executable version could not be confirmed; repair or choose a client in the MTA manager."
					return missing
				}
			}
			return client
		}
		missing.Version = choice
		missing.Detail = "Selected MTA " + choice + " is unavailable. Choose an installed version in the MTA manager."
		return missing
	}
	for _, version := range []string{"1.6", "1.5", "custom"} {
		if client, ok := FindMTAInstallation(items, version); ok {
			return client
		}
	}
	if _, ok := FindMTAInstallation(items, "1.7"); ok {
		missing.Detail = "MTA 1.7 development is installed but not selected. Choose Use 1.7 in the MTA manager after reviewing its warning."
	}
	return missing
}
func MTAInstallerResult(version string, environment Environment, runErr error) error {
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() != 199 {
			return runErr
		}
	}
	client, ok := FindMTAInstallation(DetectMTAInstallations(environment), version)
	if !ok {
		if runErr != nil {
			return runErr
		}
		return fmt.Errorf("MTA installer closed without installing the requested %s client; keep its normal separate version-specific directory", version)
	}
	if err := ValidateInstaller(client.Executable); err != nil {
		return fmt.Errorf("installed MTA %s launcher is invalid: %w", version, err)
	}
	family, err := MTAExecutableFamily(client.Executable)
	if err != nil || family != version {
		return fmt.Errorf("requested MTA %s client family was not confirmed by executable VERSIONINFO", version)
	}
	return nil
}

// UninstallMTA targets the selected row, not whichever other MTA happens to be
// active. Other installed versions never count as an unsuccessful removal.
func UninstallMTA(ctx context.Context, version string, environment Environment, runner Runner) error {
	if runner == nil {
		return fmt.Errorf("MTA uninstaller execution is unavailable")
	}
	client, ok := FindMTAInstallation(DetectMTAInstallations(environment), version)
	if !ok {
		return fmt.Errorf("MTA %s is not installed", version)
	}
	uninstaller, err := UninstallerPath(client, environment)
	if err != nil {
		return err
	}
	runErr := runner.Run(ctx, uninstaller)
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, ok = FindMTAInstallation(DetectMTAInstallations(environment), version); ok {
		if runErr != nil {
			return runErr
		}
		return fmt.Errorf("MTA %s uninstaller closed before removing that version", version)
	}
	return nil
}
