package multiplayer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const (
	ConfigSchema = 1
	ConfigPath   = ".downgrader/multiplayer.json"
)

type ClientConfig struct {
	// Executable is relative to the selected game directory. External clients
	// such as MTA are rediscovered instead of trusting an absolute config path.
	Executable string `json:"executable,omitempty"`
}

type Config struct {
	SchemaVersion int                       `json:"schema_version"`
	Clients       map[ClientID]ClientConfig `json:"clients,omitempty"`
}

func LoadConfig(gameDir string) (Config, error) {
	path, err := fsutil.Join(gameDir, ConfigPath)
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err = json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("read multiplayer config: %w", err)
	}
	if config.SchemaVersion == 0 {
		config.SchemaVersion = ConfigSchema
	}
	if config.SchemaVersion != ConfigSchema {
		return Config{}, fmt.Errorf("unsupported multiplayer config schema %d", config.SchemaVersion)
	}
	for id, client := range config.Clients {
		if _, ok := Installers[id]; !ok {
			return Config{}, fmt.Errorf("unsupported multiplayer client %q", id)
		}
		if client.Executable == "" {
			continue
		}
		relative, err := fsutil.Relative(client.Executable)
		if err != nil || filepath.IsAbs(client.Executable) {
			return Config{}, fmt.Errorf("unsafe executable for %s", id)
		}
		client.Executable = relative
		name := filepath.Base(filepath.FromSlash(relative))
		expected := map[ClientID]string{ClientSAMP: "samp.exe", ClientOpenMP: "omp-launcher.exe", ClientMTA: "Multi Theft Auto.exe"}[id]
		if !strings.EqualFold(name, expected) {
			return Config{}, fmt.Errorf("unexpected executable name for %s", id)
		}
		config.Clients[id] = client
	}
	return config, nil
}
