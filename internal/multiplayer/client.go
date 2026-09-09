// Package multiplayer detects and installs supported GTA San Andreas clients.
package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ClientID string

const (
	ClientSAMP   ClientID = "samp"
	ClientOpenMP ClientID = "openmp"
	ClientMTA    ClientID = "mta"

	OpenMPClientURL = "https://assets.open.mp/omp-client.dll"
)

type OpenMPConnection struct {
	Host          string
	Port          string
	Nickname      string
	Password      string
	ClientVersion string
}

type Client struct {
	ID         ClientID
	Name       string
	Installed  bool
	Executable string
	WorkingDir string
	Detail     string
}

type Environment struct {
	GameDir        string
	WindowsGameDir string
	PrefixDriveC   string
	LocalAppData   string
}

func (connection OpenMPConnection) Arguments(gameDir string) ([]string, error) {
	host := strings.TrimSpace(connection.Host)
	if host == "" {
		return nil, fmt.Errorf("enter a server address")
	}
	if len(host) > 253 || strings.ContainsAny(host, " \t\r\n") {
		return nil, fmt.Errorf("server address is not valid")
	}
	port, err := strconv.Atoi(strings.TrimSpace(connection.Port))
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be a number from 1 to 65535")
	}
	nickname := strings.TrimSpace(connection.Nickname)
	if nickname == "" {
		return nil, fmt.Errorf("enter a nickname")
	}
	if len(nickname) > 24 || strings.ContainsAny(nickname, "\r\n") {
		return nil, fmt.Errorf("nickname must be at most 24 characters")
	}
	if strings.TrimSpace(gameDir) == "" {
		return nil, fmt.Errorf("GTA directory is missing")
	}
	if _, ok := FindOpenMPSAMPVersion(connection.ClientVersion); !ok {
		return nil, fmt.Errorf("unsupported open.mp SA-MP client version %q", connection.ClientVersion)
	}
	arguments := []string{"-h", host, "-p", strconv.Itoa(port), "-n", nickname, "-g", gameDir}
	if connection.Password != "" {
		if strings.ContainsAny(connection.Password, "\x00\r\n") {
			return nil, fmt.Errorf("server password contains invalid characters")
		}
		arguments = append(arguments, "-P", connection.Password)
	}
	return arguments, nil
}

func OpenMPClientPath(environment Environment) (string, error) {
	root, err := OpenMPDataPath(environment)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "omp", "omp-client.dll"), nil
}

func ValidateOpenMPClient(path string) error {
	if err := ValidateInstaller(path); err != nil {
		return fmt.Errorf("open.mp command-line client is invalid: %w", err)
	}
	return nil
}

// LaunchSupport reports client restrictions that cannot be solved by changing
// the GTA installation. Current MTA production clients terminate in their
// integrity initialization under Wine/Proton (upstream issue #4783).
func LaunchSupport(id ClientID, hostOS string, underProton bool) (bool, string) {
	if id == ClientMTA && (hostOS == "linux" || underProton) {
		return false, "MTA is currently unavailable on Linux because its integrity system terminates under Wine/Proton"
	}
	return true, ""
}

type Installer struct {
	Client       ClientID
	Name         string
	URL          string
	OfficialPage string
	FileName     string
	Manual       bool
}

var Installers = map[ClientID]Installer{
	ClientSAMP: {
		Client: ClientSAMP, Name: "SA-MP", Manual: true,
		OfficialPage: "https://www.sa-mp.mp/downloads/", FileName: "sa-mp-installer.exe",
	},
	ClientOpenMP: {
		Client: ClientOpenMP, Name: "open.mp",
		URL:          "https://github.com/openmultiplayer/launcher/releases/latest/download/omp-launcher-setup.exe",
		OfficialPage: "https://open.mp/downloads/launcher", FileName: "omp-launcher-setup.exe",
	},
	ClientMTA: {
		Client: ClientMTA, Name: "Multi Theft Auto",
		URL:          "https://mirror-cdn.multitheftauto.com/mtasa/main/mtasa-1.6.exe",
		OfficialPage: "https://multitheftauto.com/", FileName: "mtasa-1.6.exe",
	},
}

func Detect(environment Environment) []Client {
	configured := map[ClientID][]string{}
	if config, err := LoadConfig(environment.GameDir); err == nil {
		for id, client := range config.Clients {
			if client.Executable != "" {
				configured[id] = append(configured[id], filepath.Join(environment.GameDir, filepath.FromSlash(client.Executable)))
			}
		}
	}
	for id, values := range registryCandidates() {
		configured[id] = append(configured[id], values...)
	}
	return detect(environment, configured)
}

func detect(environment Environment, registry map[ClientID][]string) []Client {
	clients := []Client{
		{ID: ClientSAMP, Name: "SA-MP"},
		{ID: ClientOpenMP, Name: "open.mp"},
		{ID: ClientMTA, Name: "Multi Theft Auto"},
	}
	for i := range clients {
		var candidates []string
		switch clients[i].ID {
		case ClientSAMP:
			path := filepath.Join(environment.GameDir, "samp.exe")
			if regular(path) && regular(filepath.Join(environment.GameDir, "samp.dll")) {
				candidates = append(candidates, path)
			}
		case ClientOpenMP:
			candidates = append(candidates, filepath.Join(environment.GameDir, "omp-launcher.exe"))
			candidates = append(candidates, prefixCandidates(environment.PrefixDriveC, []string{
				`Program Files (x86)/Open Multiplayer Launcher/omp-launcher.exe`,
				`Program Files/Open Multiplayer Launcher/omp-launcher.exe`,
				`users/steamuser/AppData/Local/Open Multiplayer Launcher/omp-launcher.exe`,
			})...)
		case ClientMTA:
			for _, version := range []string{"1.7", "1.6", "1.5"} {
				candidates = append(candidates, prefixCandidates(environment.PrefixDriveC, []string{
					"Program Files (x86)/MTA San Andreas " + version + "/Multi Theft Auto.exe",
					"Program Files/MTA San Andreas " + version + "/Multi Theft Auto.exe",
				})...)
			}
		}
		candidates = append(registry[clients[i].ID], candidates...)
		for _, candidate := range candidates {
			if regular(candidate) {
				clients[i].Installed = true
				clients[i].Executable = filepath.Clean(candidate)
				clients[i].WorkingDir = filepath.Dir(candidate)
				break
			}
		}
	}
	return clients
}

func prefixCandidates(root string, relative []string) []string {
	if root == "" {
		return nil
	}
	result := make([]string, 0, len(relative))
	for _, value := range relative {
		result = append(result, filepath.Join(root, filepath.FromSlash(value)))
	}
	return result
}

func regular(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func Find(clients []Client, id ClientID) (Client, bool) {
	for _, client := range clients {
		if client.ID == id {
			return client, true
		}
	}
	return Client{}, false
}

func ValidateExecutable(client Client) error {
	if !client.Installed || client.Executable == "" {
		return fmt.Errorf("%s is not installed", client.Name)
	}
	if !regular(client.Executable) {
		return fmt.Errorf("%s executable is missing", client.Name)
	}
	return nil
}

// InstallerResult handles the MTA installer's UAC handoff under Proton. A
// completed install can return 199 to the host even though the client was
// written successfully. Only accept that result after detecting and validating
// the installed client; every other non-zero result remains an error.
func InstallerResult(id ClientID, environment Environment, runErr error) error {
	if runErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if id != ClientMTA || !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 199 {
		return runErr
	}
	client, ok := Find(Detect(environment), id)
	if !ok || ValidateExecutable(client) != nil {
		return runErr
	}
	return nil
}

type CommandExecutor interface {
	Execute(context.Context, string, string, ...string) error
}

type CommandExecutorFunc func(context.Context, string, string, ...string) error

func (f CommandExecutorFunc) Execute(ctx context.Context, directory, executable string, arguments ...string) error {
	return f(ctx, directory, executable, arguments...)
}

type DirectRunner struct{ Executor CommandExecutor }

func (runner DirectRunner) Prepare(_ context.Context, id ClientID, environment Environment) ([]string, error) {
	if id == ClientSAMP && environment.WindowsGameDir != "" {
		return []string{"/D=" + environment.WindowsGameDir}, nil
	}
	return nil, nil
}

func (runner DirectRunner) Run(ctx context.Context, executable string, arguments ...string) error {
	if runner.Executor == nil {
		runner.Executor = CommandExecutorFunc(func(ctx context.Context, directory, executable string, arguments ...string) error {
			command := exec.CommandContext(ctx, executable, arguments...)
			command.Dir = directory
			return command.Run()
		})
	}
	if !regular(executable) {
		return fmt.Errorf("installer %q is missing or is not a regular file", executable)
	}
	return runner.Executor.Execute(ctx, filepath.Dir(executable), executable, arguments...)
}

type ProtonContext interface {
	Run(context.Context, string, ...string) error
}

type PrepareFunc func(context.Context, ClientID, Environment) error

type ProtonRunner struct {
	Context     ProtonContext
	PrepareFunc PrepareFunc
}

func (runner ProtonRunner) Prepare(ctx context.Context, id ClientID, environment Environment) ([]string, error) {
	if runner.PrepareFunc != nil {
		if err := runner.PrepareFunc(ctx, id, environment); err != nil {
			return nil, err
		}
	}
	if id == ClientSAMP {
		if environment.WindowsGameDir == "" {
			return nil, fmt.Errorf("GTA directory is not mapped into the Proton prefix")
		}
		return []string{"/D=" + environment.WindowsGameDir}, nil
	}
	return nil, nil
}

func (runner ProtonRunner) Run(ctx context.Context, executable string, arguments ...string) error {
	if runner.Context == nil {
		return errors.New("Proton context is not configured")
	}
	if !regular(executable) {
		return fmt.Errorf("installer %q is missing or is not a regular file", executable)
	}
	return runner.Context.Run(ctx, executable, arguments...)
}

type Runner interface {
	Prepare(context.Context, ClientID, Environment) ([]string, error)
	Run(context.Context, string, ...string) error
}

func ValidateInstaller(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var signature [2]byte
	if _, err = file.Read(signature[:]); err != nil {
		return fmt.Errorf("read installer header: %w", err)
	}
	if string(signature[:]) != "MZ" {
		return fmt.Errorf("downloaded file is not a Windows executable")
	}
	return nil
}

func ValidateDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("installer URL must use HTTPS")
	}
	return nil
}
