package multiplayer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const MTAChoicePath = ".downgrader/mta-client.json"

type mtaChoice struct {
	Schema  int    `json:"schema_version"`
	Version string `json:"version"`
}

func readMTAChoice(root string) (string, []byte, os.FileInfo, error) {
	path, err := fsutil.Join(root, MTAChoicePath)
	if err != nil {
		return "", nil, nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, nil, nil
	}
	if err != nil {
		return "", nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return "", nil, nil, fmt.Errorf("MTA choice must be an ordinary file no larger than 4096 bytes")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return "", nil, nil, fmt.Errorf("MTA choice changed while reading")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", nil, nil, err
	}
	if len(data) > 4096 {
		return "", nil, nil, fmt.Errorf("MTA choice exceeds its size limit")
	}
	return path, data, info, nil
}
func decodeMTAChoice(data []byte) (string, error) {
	var choice mtaChoice
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&choice); err != nil {
		return "", fmt.Errorf("invalid MTA version choice: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", fmt.Errorf("trailing data in MTA version choice")
	}
	if choice.Schema != 1 || !MTAVersionSupported(choice.Version) {
		return "", fmt.Errorf("unsupported MTA version choice")
	}
	return choice.Version, nil
}
func LoadMTAChoice(root string) (string, error) {
	_, data, info, err := readMTAChoice(root)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", nil
	}
	return decodeMTAChoice(data)
}

// SetMTAChoice stores only a detected version, never an executable path or server.
// It can replace a malformed ordinary record explicitly, but never a link or a
// concurrently replaced record. A closed-game/current-scope guard is mandatory.
func SetMTAChoice(ctx context.Context, root, version string, environment Environment, guard func() error) error {
	if !MTAVersionSupported(version) {
		return fmt.Errorf("unsupported MTA version")
	}
	if environment.GameDir != root {
		return fmt.Errorf("MTA selection scope changed")
	}
	if guard == nil {
		return fmt.Errorf("MTA selection requires a closed-game guard")
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		client, ok := FindMTAInstallation(DetectMTAInstallations(environment), version)
		if !ok {
			return fmt.Errorf("MTA %s is no longer installed", version)
		}
		if err := ValidateInstaller(client.Executable); err != nil {
			return err
		}
		if version != "custom" {
			family, err := MTAExecutableFamily(client.Executable)
			if err != nil || family != version {
				return fmt.Errorf("installed MTA %s version could not be confirmed by executable VERSIONINFO", version)
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	_, before, identity, err := readMTAChoice(root)
	if err != nil {
		return err
	}
	path, err := fsutil.Join(root, MTAChoicePath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err = fsutil.Join(root, MTAChoicePath); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mta-client-*.bin")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	err = json.NewEncoder(file).Encode(mtaChoice{1, version})
	if err == nil {
		err = file.Chmod(0600)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	_, current, currentInfo, err := readMTAChoice(root)
	if err != nil {
		return err
	}
	if (identity == nil) != (currentInfo == nil) || identity != nil && (!os.SameFile(identity, currentInfo) || identity.Mode() != currentInfo.Mode() || !bytes.Equal(before, current)) {
		return fmt.Errorf("MTA selection changed externally; refresh before choosing a version")
	}
	if _, err = fsutil.Join(root, MTAChoicePath); err != nil {
		return err
	}
	if identity == nil {
		return os.Link(temporary, path)
	}
	return os.Rename(temporary, path)
}
