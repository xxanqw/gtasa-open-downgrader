package mods

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const TrackerPath = "modloader/.downgrader"

type Tracker struct {
	Schema    int
	Installed []string
	Versions  map[string]Release
	Extra     map[string]json.RawMessage
	Exists    bool
}

func (t Tracker) Legacy() bool { return t.Exists && t.Schema < 2 }
func (t Tracker) Has(name string) bool {
	for _, n := range t.Installed {
		if n == name {
			return true
		}
	}
	return false
}
func LoadTracker(root string) (Tracker, error) {
	t := Tracker{Versions: map[string]Release{}, Extra: map[string]json.RawMessage{}}
	p, e := fsutil.Join(root, TrackerPath)
	if e != nil {
		return t, e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return t, nil
	}
	if e != nil {
		return t, e
	}
	t.Exists = true
	if e = json.Unmarshal(b, &t.Extra); e != nil || t.Extra == nil {
		return t, fmt.Errorf("mod tracker is invalid; keep a copy and repair it before installing mods")
	}
	_ = json.Unmarshal(t.Extra["schema_version"], &t.Schema)
	_ = json.Unmarshal(t.Extra["installed_mods"], &t.Installed)
	_ = json.Unmarshal(t.Extra["mod_versions"], &t.Versions)
	if t.Versions == nil {
		t.Versions = map[string]Release{}
	}
	return t, nil
}
func (t *Tracker) Save(ctx context.Context, root, version string) error {
	if t.Extra == nil {
		t.Extra = map[string]json.RawMessage{}
	}
	sort.Strings(t.Installed)
	for k, v := range map[string]any{"schema_version": t.Schema, "installed_mods": t.Installed, "mod_versions": t.Versions, "version": version} {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		t.Extra[k] = b
	}
	b, e := json.MarshalIndent(t.Extra, "", "  ")
	if e != nil {
		return e
	}
	p, e := fsutil.Join(root, TrackerPath)
	if e != nil {
		return e
	}
	return fsutil.Write(ctx, p, 0644, func(w io.Writer) error { _, e := w.Write(b); return e })
}

// Selection is ordered so loaders are installed before dependent packages.
func Selection(selected []string, t Tracker) []string {
	wanted := map[string]bool{}
	for _, n := range selected {
		if Supported(n) {
			wanted[n] = true
		}
	}
	if t.Legacy() {
		for _, n := range t.Installed {
			if Supported(n) {
				wanted[n] = true
			}
		}
	}
	for n := range wanted {
		if n != "ASI Loader" && n != "ModLoader" {
			wanted["ModLoader"] = true
			wanted["ASI Loader"] = true
		}
	}
	if wanted["ModLoader"] {
		wanted["ASI Loader"] = true
	}
	var result []string
	for _, s := range Catalog {
		if wanted[s.Name] {
			result = append(result, s.Name)
		}
	}
	return result
}
