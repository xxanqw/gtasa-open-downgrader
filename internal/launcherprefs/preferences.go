// Package launcherprefs stores only the remembered game mode, shared across
// native Linux downgrader and Windows/Proton launcher. No credentials or servers.
package launcherprefs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const Path = ".downgrader/launcher-preferences.json"

type Choice string

const (
	None         Choice = ""
	SinglePlayer Choice = "single-player"
	SAMP         Choice = "samp"
	OpenMP       Choice = "openmp"
	MTA          Choice = "mta"
)

func (c Choice) Valid() bool {
	return c == None || c == SinglePlayer || c == SAMP || c == OpenMP || c == MTA
}
func (c Choice) Name() string {
	switch c {
	case SinglePlayer:
		return "Single Player"
	case SAMP:
		return "SA-MP"
	case OpenMP:
		return "open.mp"
	case MTA:
		return "MTA"
	}
	return "None"
}

type record struct {
	Schema int    `json:"schema_version"`
	Choice Choice `json:"remembered_choice"`
}

func location(root string) (string, error) {
	p, err := fsutil.Join(root, Path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("launcher preferences must be an ordinary file")
	}
	return p, nil
}
func Load(root string) (Choice, error) {
	p, err := location(root)
	if err != nil {
		return None, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return None, nil
	}
	if err != nil {
		return None, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return None, err
	}
	if len(data) > 4096 {
		return None, fmt.Errorf("launcher preferences exceed 4096 bytes")
	}
	var r record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return None, fmt.Errorf("invalid launcher preferences: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return None, fmt.Errorf("trailing data in launcher preferences")
	}
	if r.Schema != 1 || !r.Choice.Valid() {
		return None, fmt.Errorf("unsupported launcher preferences")
	}
	return r.Choice, nil
}
func Save(ctx context.Context, root string, choice Choice) error {
	if !choice.Valid() {
		return fmt.Errorf("unsupported remembered choice")
	}
	if _, err := Load(root); err != nil {
		return err
	}
	p, err := location(root)
	if err != nil {
		return err
	}
	return fsutil.Write(ctx, p, 0600, func(w io.Writer) error { return json.NewEncoder(w).Encode(record{1, choice}) })
}

// Reset can remove a malformed record, but never a link, directory or other file.
func Reset(root string) error {
	p, err := location(root)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
