package steam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type compatMapping struct {
	Name     string
	Priority int
}

func readCompatToolMapping(steamRoot string) (map[string]compatMapping, error) {
	path := filepath.Join(steamRoot, "config", "config.vdf")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]compatMapping{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Steam compatibility configuration %q: %w", path, err)
	}
	object, err := parseVDF(data)
	if err != nil {
		return nil, fmt.Errorf("parse Steam compatibility configuration %q: %w", path, err)
	}
	mappingObject, ok := object.findObject("CompatToolMapping")
	if !ok {
		return map[string]compatMapping{}, nil
	}
	mappings := make(map[string]compatMapping, len(mappingObject))
	for _, entry := range mappingObject {
		if entry.children == nil {
			continue
		}
		name, _ := entry.children.text("name")
		priorityText, _ := entry.children.text("priority")
		priority, _ := strconv.Atoi(priorityText)
		mappings[entry.key] = compatMapping{Name: name, Priority: priority}
	}
	return mappings, nil
}

func discoverCompatibilityTools(steamRoot string, libraries []string) ([]CompatibilityTool, error) {
	var directories []string
	for _, library := range libraries {
		common := filepath.Join(library, "steamapps", "common")
		entries, err := os.ReadDir(common)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("enumerate Steam applications in %q: %w", common, err)
		}
		for _, entry := range entries {
			if directoryEntryIsDir(common, entry) {
				directories = append(directories, filepath.Join(common, entry.Name()))
			}
		}
	}
	custom := filepath.Join(steamRoot, "compatibilitytools.d")
	// Valve also permits compatibilitytool.vdf directly in this directory,
	// with install_path redirecting to a relative or absolute tool location.
	directories = append(directories, custom)
	entries, err := os.ReadDir(custom)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("enumerate custom Steam compatibility tools in %q: %w", custom, err)
	}
	for _, entry := range entries {
		if directoryEntryIsDir(custom, entry) {
			directories = append(directories, filepath.Join(custom, entry.Name()))
		}
	}

	var tools []CompatibilityTool
	for _, directory := range uniquePaths(directories) {
		metadata := filepath.Join(directory, "compatibilitytool.vdf")
		data, err := os.ReadFile(metadata)
		if errors.Is(err, os.ErrNotExist) {
			proton := filepath.Join(directory, "proton")
			if err := requireExecutable(proton, "Proton executable"); err == nil {
				name := filepath.Base(directory)
				tools = append(tools, CompatibilityTool{Name: name, DisplayName: name, Root: directory, ProtonExecutable: proton})
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read compatibility tool metadata %q: %w", metadata, err)
		}
		parsed, err := parseCompatibilityTool(metadata, directory, data)
		if err != nil {
			return nil, err
		}
		tools = append(tools, parsed...)
	}
	return deduplicateTools(tools), nil
}

func parseCompatibilityTool(metadata, directory string, data []byte) ([]CompatibilityTool, error) {
	object, err := parseVDF(data)
	if err != nil {
		return nil, fmt.Errorf("parse compatibility tool metadata %q: %w", metadata, err)
	}
	compatibilityTools, ok := object.findObject("compatibilitytools")
	if !ok {
		return nil, fmt.Errorf("parse compatibility tool metadata %q: compatibilitytools object is missing", metadata)
	}
	definitions, ok := compatibilityTools.object("compat_tools")
	if !ok {
		return nil, fmt.Errorf("parse compatibility tool metadata %q: compat_tools object is missing", metadata)
	}
	var tools []CompatibilityTool
	for _, definition := range definitions {
		if definition.children == nil {
			continue
		}
		installPath, _ := definition.children.text("install_path")
		if installPath == "" {
			installPath = "."
		}
		root := resolveToolRoot(directory, installPath)
		displayName, _ := definition.children.text("display_name")
		if displayName == "" {
			displayName = definition.key
		}
		priorityText, _ := definition.children.text("priority")
		priority, _ := strconv.Atoi(priorityText)
		proton := filepath.Join(root, "proton")
		if err := requireExecutable(proton, "Proton executable"); err != nil {
			return nil, fmt.Errorf("compatibility tool %q from %q: %w", definition.key, metadata, err)
		}
		tools = append(tools, CompatibilityTool{
			Name: definition.key, DisplayName: displayName, Root: root,
			ProtonExecutable: proton, Priority: priority,
		})
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("parse compatibility tool metadata %q: no compatibility tools are defined", metadata)
	}
	return tools, nil
}

func directoryEntryIsDir(parent string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(parent, entry.Name()))
	return err == nil && info.IsDir()
}

func resolveToolRoot(directory, installPath string) string {
	if filepath.IsAbs(installPath) {
		return filepath.Clean(installPath)
	}
	return filepath.Clean(filepath.Join(directory, installPath))
}

// A Steam mapping can refer to a Steam-internal tool that is not available to
// this app. Keep the missing-tool classification, but explain the next step
// instead of implying that Steam has not installed its own Proton build.
type unavailableCompatibilityToolError struct{ name string }

func (e unavailableCompatibilityToolError) Error() string {
	return fmt.Sprintf("Use a custom Proton build such as GE-Proton (Proton-GE). Steam's tool %q is unavailable to this app. Install the custom build, restart Steam, select it in the game's Properties > Compatibility, then refresh Multiplayer in the downgrader.", e.name)
}
func (unavailableCompatibilityToolError) Unwrap() error { return ErrToolMissing }

func selectCompatibilityTool(appID uint32, mappings map[string]compatMapping, tools []CompatibilityTool) (CompatibilityTool, string, error) {
	appKey := strconv.FormatUint(uint64(appID), 10)
	if mapping, ok := mappings[appKey]; ok && mapping.Name != "" {
		tool, found := findTool(tools, mapping.Name)
		if !found {
			return CompatibilityTool{}, "", unavailableCompatibilityToolError{name: mapping.Name}
		}
		return tool, "per-game mapping", nil
	}
	if mapping, ok := mappings["0"]; ok && mapping.Name != "" {
		tool, found := findTool(tools, mapping.Name)
		if !found {
			return CompatibilityTool{}, "", unavailableCompatibilityToolError{name: mapping.Name}
		}
		return tool, "Steam default mapping", nil
	}
	if len(tools) == 0 {
		return CompatibilityTool{}, "", fmt.Errorf("%w: no Proton installations were found", ErrToolMissing)
	}
	// Compatibility tool metadata exposes priority for Steam's selector. When
	// config.vdf has no per-game or global mapping, prefer that priority and use
	// the internal name only to keep equal-priority selection deterministic.
	tools = append([]CompatibilityTool(nil), tools...)
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Priority != tools[j].Priority {
			return tools[i].Priority > tools[j].Priority
		}
		return naturalLess(tools[j].Name, tools[i].Name)
	})
	return tools[0], "installed-tool fallback", nil
}

func findTool(tools []CompatibilityTool, selected string) (CompatibilityTool, bool) {
	for _, tool := range tools {
		if strings.EqualFold(tool.Name, selected) {
			return tool, true
		}
	}
	for _, tool := range tools {
		if strings.EqualFold(tool.DisplayName, selected) || strings.EqualFold(filepath.Base(tool.Root), selected) {
			return tool, true
		}
	}
	return CompatibilityTool{}, false
}

func deduplicateTools(tools []CompatibilityTool) []CompatibilityTool {
	var result []CompatibilityTool
	seen := map[string]bool{}
	for _, tool := range tools {
		key := strings.ToLower(tool.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, tool)
	}
	return result
}

func naturalLess(left, right string) bool {
	left, right = strings.ToLower(left), strings.ToLower(right)
	for len(left) > 0 && len(right) > 0 {
		leftDigit := left[0] >= '0' && left[0] <= '9'
		rightDigit := right[0] >= '0' && right[0] <= '9'
		if leftDigit && rightDigit {
			leftEnd, rightEnd := 0, 0
			for leftEnd < len(left) && left[leftEnd] >= '0' && left[leftEnd] <= '9' {
				leftEnd++
			}
			for rightEnd < len(right) && right[rightEnd] >= '0' && right[rightEnd] <= '9' {
				rightEnd++
			}
			leftNumber, _ := strconv.Atoi(left[:leftEnd])
			rightNumber, _ := strconv.Atoi(right[:rightEnd])
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			left, right = left[leftEnd:], right[rightEnd:]
			continue
		}
		if left[0] != right[0] {
			return left[0] < right[0]
		}
		left, right = left[1:], right[1:]
	}
	return len(left) < len(right)
}
