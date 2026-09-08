// Package steam discovers Steam installations, libraries, applications, and
// Steam Play compatibility tools without changing Steam configuration.
package steam

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrSteamNotFound   = errors.New("Steam installation not found")
	ErrAppNotInstalled = errors.New("Steam application not installed")
	ErrPrefixMissing   = errors.New("Proton prefix not found")
	ErrToolMissing     = errors.New("Steam compatibility tool not installed")
	ErrUnsupportedOS   = errors.New("Steam Proton integration is only available on Linux")
)

type CompatibilityTool struct {
	Name             string
	DisplayName      string
	Root             string
	ProtonExecutable string
	Priority         int
}

type ProtonContext struct {
	AppID            uint32
	SteamRoot        string
	LibraryRoot      string
	GameDir          string
	CompatDataPath   string
	ToolName         string
	ToolDisplayName  string
	ToolRoot         string
	ProtonExecutable string
	Selection        string
}

// Detector permits deterministic fixture-based detection. Roots should point
// to Steam client installations, not individual library folders.
type Detector struct {
	Roots []string
}

func NewDetector(roots ...string) *Detector {
	return &Detector{Roots: append([]string(nil), roots...)}
}

func DetectProtonContext(appID uint32) (*ProtonContext, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("detect Steam Proton context on %s: %w", runtime.GOOS, ErrUnsupportedOS)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("detect Steam Proton context: user home: %w", err)
	}
	return NewDetector(FindSteamRoots(home)...).DetectProtonContext(appID)
}

// FindSteamRoots detects regular and Flatpak Steam installations below home.
// Symlink aliases such as .steam/root and .steam/steam are de-duplicated.
func FindSteamRoots(home string) []string {
	candidates := []string{
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "debian-installation"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
	}
	var roots []string
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if !isDir(filepath.Join(candidate, "steamapps")) {
			continue
		}
		key := canonicalPath(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		roots = append(roots, filepath.Clean(candidate))
	}
	return roots
}

func LibraryRoots(steamRoot string) ([]string, error) {
	root := filepath.Clean(steamRoot)
	if !isDir(filepath.Join(root, "steamapps")) {
		return nil, fmt.Errorf("Steam root %q has no steamapps directory", root)
	}
	libraries := []string{root}
	path := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return libraries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Steam library configuration %q: %w", path, err)
	}
	object, err := parseVDF(data)
	if err != nil {
		return nil, fmt.Errorf("parse Steam library configuration %q: %w", path, err)
	}
	folders, ok := object.findObject("libraryfolders")
	if !ok {
		return nil, fmt.Errorf("parse Steam library configuration %q: libraryfolders object is missing", path)
	}
	for _, entry := range folders {
		libraryPath := entry.value
		if entry.children != nil {
			libraryPath, _ = entry.children.text("path")
		}
		if libraryPath == "" {
			continue
		}
		libraries = append(libraries, filepath.Clean(libraryPath))
	}
	return uniquePaths(libraries), nil
}

func (d *Detector) DetectProtonContext(appID uint32) (*ProtonContext, error) {
	if appID == 0 {
		return nil, fmt.Errorf("detect Steam application: invalid AppID 0")
	}
	if len(d.Roots) == 0 {
		return nil, ErrSteamNotFound
	}
	var searched []string
	for _, root := range uniquePaths(d.Roots) {
		if !isDir(filepath.Join(root, "steamapps")) {
			continue
		}
		libraries, err := LibraryRoots(root)
		if err != nil {
			return nil, err
		}
		library, game, manifest, err := findApplication(libraries, appID)
		searched = append(searched, manifest...)
		if errors.Is(err, ErrAppNotInstalled) {
			continue
		}
		if err != nil {
			return nil, err
		}
		prefix := filepath.Join(library, "steamapps", "compatdata", strconv.FormatUint(uint64(appID), 10))
		if !isDir(prefix) {
			return nil, fmt.Errorf("%w for AppID %d: %q", ErrPrefixMissing, appID, prefix)
		}
		tools, err := discoverCompatibilityTools(root, libraries)
		if err != nil {
			return nil, err
		}
		mapping, err := readCompatToolMapping(root)
		if err != nil {
			return nil, err
		}
		tool, selection, err := selectCompatibilityTool(appID, mapping, tools)
		if err != nil {
			return nil, err
		}
		return &ProtonContext{
			AppID: appID, SteamRoot: root, LibraryRoot: library, GameDir: game,
			CompatDataPath: prefix, ToolName: tool.Name, ToolDisplayName: tool.DisplayName,
			ToolRoot: tool.Root, ProtonExecutable: tool.ProtonExecutable, Selection: selection,
		}, nil
	}
	if len(searched) == 0 {
		return nil, fmt.Errorf("%w: no valid Steam roots among %q", ErrSteamNotFound, d.Roots)
	}
	return nil, fmt.Errorf("%w: AppID %d; appmanifest_%d.acf was not found in %d Steam libraries", ErrAppNotInstalled, appID, appID, len(searched))
}

func findApplication(libraries []string, appID uint32) (string, string, []string, error) {
	name := fmt.Sprintf("appmanifest_%d.acf", appID)
	var searched []string
	for _, library := range libraries {
		manifest := filepath.Join(library, "steamapps", name)
		searched = append(searched, manifest)
		data, err := os.ReadFile(manifest)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", searched, fmt.Errorf("read Steam app manifest %q: %w", manifest, err)
		}
		object, err := parseVDF(data)
		if err != nil {
			return "", "", searched, fmt.Errorf("parse Steam app manifest %q: %w", manifest, err)
		}
		state, ok := object.findObject("AppState")
		if !ok {
			return "", "", searched, fmt.Errorf("parse Steam app manifest %q: AppState object is missing", manifest)
		}
		manifestID, _ := state.text("appid")
		if manifestID != strconv.FormatUint(uint64(appID), 10) {
			return "", "", searched, fmt.Errorf("Steam app manifest %q contains AppID %q, expected %d", manifest, manifestID, appID)
		}
		installDir, _ := state.text("installdir")
		if installDir == "" || filepath.Base(installDir) != installDir {
			return "", "", searched, fmt.Errorf("Steam app manifest %q has invalid installdir %q", manifest, installDir)
		}
		game := filepath.Join(library, "steamapps", "common", installDir)
		if !isDir(game) {
			return "", "", searched, fmt.Errorf("Steam AppID %d game directory from %q is missing: %q", appID, manifest, game)
		}
		return library, game, searched, nil
	}
	return "", "", searched, ErrAppNotInstalled
}

func (p *ProtonContext) Environment(base []string) []string {
	values := map[string]string{
		"STEAM_COMPAT_DATA_PATH":           p.CompatDataPath,
		"STEAM_COMPAT_CLIENT_INSTALL_PATH": p.SteamRoot,
		"STEAM_COMPAT_APP_ID":              strconv.FormatUint(uint64(p.AppID), 10),
		"SteamAppId":                       strconv.FormatUint(uint64(p.AppID), 10),
		"SteamGameId":                      strconv.FormatUint(uint64(p.AppID), 10),
	}
	result := make([]string, 0, len(base)+len(values))
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, replaced := values[key]; !replaced {
			result = append(result, item)
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func (p *ProtonContext) Command(ctx context.Context, exe string, args ...string) (*exec.Cmd, error) {
	if ctx == nil {
		return nil, fmt.Errorf("create Proton command: nil context")
	}
	if err := requireExecutable(p.ProtonExecutable, "Proton executable"); err != nil {
		return nil, err
	}
	if !isDir(p.CompatDataPath) {
		return nil, fmt.Errorf("%w for AppID %d: %q", ErrPrefixMissing, p.AppID, p.CompatDataPath)
	}
	if err := requireFile(exe, "Windows executable"); err != nil {
		return nil, err
	}
	arguments := append([]string{"run", exe}, args...)
	command := exec.CommandContext(ctx, p.ProtonExecutable, arguments...)
	command.Env = p.Environment(os.Environ())
	command.Dir = filepath.Dir(exe)
	return command, nil
}

func (p *ProtonContext) Run(ctx context.Context, exe string, args ...string) error {
	command, err := p.Command(ctx, exe, args...)
	if err != nil {
		return err
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %q with %s: %w", exe, p.ToolDisplayName, err)
	}
	return nil
}

// WindowsPath translates a host path through the drive mappings in this
// Proton prefix. The longest matching mapping wins, which normally resolves a
// Steam library through Proton's Z: drive.
func (p *ProtonContext) WindowsPath(hostPath string) (string, error) {
	hostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(hostPath); resolveErr == nil {
		hostPath = resolved
	}
	dosDevices := filepath.Join(p.CompatDataPath, "pfx", "dosdevices")
	entries, err := os.ReadDir(dosDevices)
	if err != nil {
		return "", fmt.Errorf("read Proton drive mappings: %w", err)
	}
	bestRoot, bestDrive := "", ""
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if len(name) != 2 || name[1] != ':' || name[0] < 'a' || name[0] > 'z' {
			continue
		}
		root, resolveErr := filepath.EvalSymlinks(filepath.Join(dosDevices, entry.Name()))
		if resolveErr != nil {
			continue
		}
		root, resolveErr = filepath.Abs(root)
		if resolveErr != nil || !within(root, hostPath) || len(root) < len(bestRoot) {
			continue
		}
		bestRoot, bestDrive = root, strings.ToUpper(name[:1])
	}
	if bestDrive == "" {
		return "", fmt.Errorf("path %q is not available through the Proton prefix", hostPath)
	}
	relative, err := filepath.Rel(bestRoot, hostPath)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return bestDrive + `:\`, nil
	}
	return bestDrive + `:\` + strings.ReplaceAll(relative, string(filepath.Separator), `\`), nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// SetRegistryString uses the prefix's Windows reg.exe through the selected
// Proton build. It is intentionally limited to software keys used for client
// installer discovery.
func (p *ProtonContext) SetRegistryString(ctx context.Context, key, name, value string) error {
	upper := strings.ToUpper(key)
	if (!strings.HasPrefix(upper, `HKCU\SOFTWARE\`) && !strings.HasPrefix(upper, `HKLM\SOFTWARE\`)) || name == "" || strings.ContainsAny(key+name+value, "\x00\r\n") {
		return fmt.Errorf("invalid Windows Registry value")
	}
	if err := requireExecutable(p.ProtonExecutable, "Proton executable"); err != nil {
		return err
	}
	arguments := []string{"run", "reg.exe", "ADD", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f"}
	command := exec.CommandContext(ctx, p.ProtonExecutable, arguments...)
	command.Env = p.Environment(os.Environ())
	command.Dir = p.GameDir
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("set Proton Registry value %q: %w: %s", key, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func requireFile(path, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", description, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q is not a regular file", description, path)
	}
	return nil
}

func requireExecutable(path, description string) error {
	if err := requireFile(path, description); err != nil {
		return err
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("%s %q is not executable", description, path)
	}
	return nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

func uniquePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		key := canonicalPath(path)
		if path == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, filepath.Clean(path))
	}
	return result
}
