// Package game reads patch manifests and identifies game files.
package game

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const LAAHash = "2b5066bd4097ac2944ce6a9cf8fe5677"

var Versions = map[string]string{
	"170b3a9108687b26da2d8901c6948a18": "v1.0 US (Hoodlum)", LAAHash: "v1.0 US (Hoodlum + LAA Patch)",
	"4e99d762f44b1d5e7652dfa7e73d6b6f": "v1.0 US (Clean)",
	"667f799c4ba8c9e1054fccaea6d4259b": "v1.0 US (Compact)", "6c6160da9b175b66cf9127c86be57bf7": "v1.0 EU",
	"49dd417760484a18017805df46b308b8": "v1.0 EU (Alt)", "9f2d711dbf1fbbcda5ff9418a2cc1ef5": "v1.01 US",
	"25405921d1c47747fd01fd0bfe0a05ae": "v1.01 EU", "d9cb35c898d3298ca904a63e10ee18d7": "NewSteam R2 (German)",
	"d9cb35c898d3298ca904a63e10ee18d9": "NewSteam R2 (German)", "5bfd4dd83989a8264de4b8e771f237fd": "NewSteam R2",
}

type PatchSource struct {
	Hash  string `json:"hash"`
	Patch string `json:"patch,omitempty"`
}

type File struct {
	Path       string        `json:"path"`
	Action     string        `json:"action"`
	SourceHash string        `json:"source_hash"`
	TargetHash string        `json:"target_hash"`
	Payload    string        `json:"payload,omitempty"`
	Sources    []PatchSource `json:"sources,omitempty"`
	// ApplicableVersions restricts files that do not exist in every source edition.
	// Empty keeps v1 and early v2 manifests compatible.
	ApplicableVersions []string `json:"applicable_versions,omitempty"`
}
type SourceVersion struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	EXEHash string `json:"exe_hash"`
}
type Manifest struct {
	Version        string          `json:"version"`
	ReleaseID      string          `json:"release_id,omitempty"`
	SourceVersion  string          `json:"source_version"`
	TargetVersion  string          `json:"target_version"`
	SourceVersions []SourceVersion `json:"source_versions,omitempty"`
	Files          []File          `json:"files"`
}

func validHash(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 16 }
func (m *Manifest) Validate() error {
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest contains no files")
	}
	if m.Version != "" && m.Version != "1.0" && m.Version != "2.0" && m.Version != "2.1" {
		return fmt.Errorf("unsupported manifest version: %s", m.Version)
	}
	versionIDs := map[string]bool{}
	for i := range m.SourceVersions {
		version := &m.SourceVersions[i]
		version.EXEHash = strings.ToLower(version.EXEHash)
		if version.ID == "" || version.Name == "" || !validHash(version.EXEHash) {
			return fmt.Errorf("invalid source version metadata")
		}
		if versionIDs[version.ID] {
			return fmt.Errorf("duplicate source version: %s", version.ID)
		}
		versionIDs[version.ID] = true
	}
	seen := map[string]bool{}
	for i := range m.Files {
		f := &m.Files[i]
		p, e := fsutil.Relative(f.Path)
		if e != nil {
			return e
		}
		f.Path = p
		if strings.EqualFold(strings.SplitN(p, "/", 2)[0], "backups") || strings.HasPrefix(strings.ToLower(p), ".downgrader") {
			return fmt.Errorf("manifest uses reserved backup path: %s", p)
		}
		key := strings.ToLower(p)
		if seen[key] {
			return fmt.Errorf("duplicate manifest path: %s", p)
		}
		seen[key] = true
		if f.Action == "" {
			f.Action = "patch"
		}
		if f.Action != "patch" && f.Action != "copy" {
			return fmt.Errorf("unsupported action: %s", f.Action)
		}
		f.SourceHash = strings.ToLower(f.SourceHash)
		f.TargetHash = strings.ToLower(f.TargetHash)
		if !validHash(f.TargetHash) || (!validHash(f.SourceHash) && !(f.Action == "copy" && f.SourceHash == "missing")) {
			return fmt.Errorf("invalid hashes for %s", p)
		}
		if f.Action == "copy" && !IsEXE(p) {
			return fmt.Errorf("copy action is only supported for GTA executables")
		}
		applicable := map[string]bool{}
		for _, id := range f.ApplicableVersions {
			if !versionIDs[id] || applicable[id] {
				return fmt.Errorf("invalid applicable source version %q for %s", id, p)
			}
			applicable[id] = true
		}
		if f.Action == "copy" {
			if f.Payload == "" {
				f.Payload = entrypoint.RealGameExecutable
			}
			f.Payload, e = fsutil.Relative(f.Payload)
			if e != nil {
				return fmt.Errorf("invalid payload for %s: %w", p, e)
			}
		}
		sourceHashes := map[string]bool{}
		for j := range f.Sources {
			source := &f.Sources[j]
			source.Hash = strings.ToLower(source.Hash)
			if !validHash(source.Hash) {
				return fmt.Errorf("invalid source hash for %s", p)
			}
			if sourceHashes[source.Hash] {
				return fmt.Errorf("duplicate source hash for %s", p)
			}
			sourceHashes[source.Hash] = true
			if source.Patch != "" {
				source.Patch, e = fsutil.Relative(source.Patch)
				if e != nil {
					return fmt.Errorf("invalid patch for %s: %w", p, e)
				}
			} else if f.Action == "patch" {
				return fmt.Errorf("source patch is missing for %s", p)
			}
		}
		// If the primary hash is not represented in sources, the fallback delta
		// cannot be assumed to apply to that source.
		if f.Action == "patch" && len(f.Sources) > 0 && !sourceHashes[f.SourceHash] && f.SourceHash != f.TargetHash {
			return fmt.Errorf("primary source hash has no delta for %s", p)
		}
	}
	return nil
}

func (f File) AppliesTo(version string) bool {
	if len(f.ApplicableVersions) == 0 {
		return true
	}
	for _, id := range f.ApplicableVersions {
		if id == version {
			return true
		}
	}
	return false
}

// SourceEdition identifies an installation before any executable is replaced.
// A previous downgrade can be identified by the originals kept in backups.
func (m Manifest) SourceEdition(ctx context.Context, root string) (string, error) {
	if len(m.SourceVersions) == 0 {
		return "", nil
	}
	names := []string{entrypoint.SteamExecutable, entrypoint.RealGameExecutable}
	if entrypoint.MetadataPresent(root) {
		names = []string{entrypoint.RealGameExecutable}
	}
	// Only the live Steam name is reserved for the launcher. The ordinary
	// downgrade backup still contains original GTA, under either spelling.
	candidates := append(names, "backups/"+entrypoint.SteamExecutable, "backups/"+entrypoint.RealGameExecutable)
	match := func(rel string) (string, error) {
		p, err := fsutil.Join(root, rel)
		if err != nil {
			return "", err
		}
		hash, err := fsutil.Hash(ctx, p)
		if os.IsNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		for _, version := range m.SourceVersions {
			if hash == version.EXEHash {
				return version.ID, nil
			}
		}
		return "", nil
	}
	for _, rel := range candidates {
		if edition, err := match(rel); edition != "" || err != nil {
			return edition, err
		}
	}
	// Some installations only retained the original through launcher setup.
	// Use that backup only with valid metadata and its recorded SHA-256 intact.
	if config, err := entrypoint.Load(root); err == nil && config.BackupExecutable != "" && config.BackupSHA256 != "" {
		p, err := fsutil.Join(root, config.BackupExecutable)
		if err != nil {
			return "", err
		}
		digest, err := entrypoint.Hash(ctx, p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && strings.EqualFold(digest, config.BackupSHA256) {
			return match(config.BackupExecutable)
		}
	}
	return "", nil
}

func (f File) MatchesSource(hash string) bool {
	if hash != "" && hash == f.SourceHash {
		return true
	}
	for _, source := range f.Sources {
		if hash != "" && hash == source.Hash {
			return true
		}
	}
	return false
}

// PatchAsset selects the delta matching the installed source. For modified
// files, retain the v1 force-attempt behavior by falling back to the first
// declared delta; the output hash still must match before replacement.
func (f File) PatchAsset(hash string) string {
	for _, source := range f.Sources {
		if source.Hash == hash && source.Patch != "" {
			return source.Patch
		}
	}
	for _, source := range f.Sources {
		if source.Patch != "" {
			return source.Patch
		}
	}
	return f.Path + ".xdelta"
}

func (f File) PatchAssets() []string {
	if f.Action == "copy" {
		return []string{f.Payload}
	}
	if len(f.Sources) == 0 {
		return []string{f.Path + ".xdelta"}
	}
	assets := make([]string, 0, len(f.Sources))
	seen := map[string]bool{}
	for _, source := range f.Sources {
		if source.Patch != "" && !seen[source.Patch] {
			assets = append(assets, source.Patch)
			seen[source.Patch] = true
		}
	}
	return assets
}
func Load(path string) (Manifest, error) {
	var m Manifest
	b, e := os.ReadFile(path)
	if e == nil {
		e = json.Unmarshal(b, &m)
	}
	if e == nil {
		e = m.Validate()
	}
	return m, e
}
func IsEXE(p string) bool {
	return p == entrypoint.RealGameExecutable || p == entrypoint.SteamExecutable
}
func OtherEXE(p string) string {
	if p == entrypoint.RealGameExecutable {
		return entrypoint.SteamExecutable
	}
	return entrypoint.RealGameExecutable
}
func Source(root, rel string) (string, error) {
	if IsEXE(rel) && entrypoint.MetadataPresent(root) {
		return fsutil.Join(root, entrypoint.RealGameExecutable)
	}
	p, e := fsutil.Join(root, rel)
	if e != nil {
		return "", e
	}
	if _, err := os.Stat(p); os.IsNotExist(err) && IsEXE(rel) {
		alt, e := fsutil.Join(root, OtherEXE(rel))
		if e != nil {
			return "", e
		}
		if _, e = os.Stat(alt); e == nil {
			return alt, nil
		}
	}
	return p, nil
}
func HasEXE(p string) bool {
	names := []string{entrypoint.RealGameExecutable, entrypoint.SteamExecutable}
	if entrypoint.MetadataPresent(p) {
		names = names[:1]
	}
	for _, n := range names {
		st, e := os.Stat(filepath.Join(p, n))
		if e == nil && st.Mode().IsRegular() {
			return true
		}
	}
	return false
}

type Row struct{ Path, Status, NeedsPatch, CurrentHash, TargetHash string }
type Scan struct {
	Version  string
	ReadOnly bool
	Rows     []Row
}

func Inspect(ctx context.Context, root string, m Manifest, progress func(int, int)) (Scan, error) {
	result := Scan{Version: "Unknown"}
	st, e := os.Stat(root)
	if e != nil {
		return result, e
	}
	if !st.IsDir() {
		return result, fmt.Errorf("select a game directory")
	}
	probe, e := os.CreateTemp(root, ".downgrader-write-test-*")
	if e != nil {
		result.ReadOnly = true
	} else {
		probe.Close()
		os.Remove(probe.Name())
	}
	// Prefer Steam's gta-sa.exe when both spellings exist. A stale or already
	// downgraded gta_sa.exe must not hide a supported Global/German Steam source.
	executables := []string{entrypoint.SteamExecutable, entrypoint.RealGameExecutable}
	if entrypoint.MetadataPresent(root) {
		executables = []string{entrypoint.RealGameExecutable}
	}
	for _, exe := range executables {
		p, e := fsutil.Join(root, exe)
		if e != nil {
			return result, e
		}
		h, e := fsutil.Hash(ctx, p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return result, e
		}
		if v, ok := Versions[h]; ok {
			result.Version = v
			break
		}
		result.Version = "Custom/Unknown (" + h[:8] + ")"
	}
	edition, e := m.SourceEdition(ctx, root)
	if e != nil {
		return result, e
	}
	for i, f := range m.Files {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		p, e := Source(root, f.Path)
		if e != nil {
			return result, e
		}
		h, e := fsutil.Hash(ctx, p)
		if e != nil && !os.IsNotExist(e) {
			return result, e
		}
		row := Row{Path: f.Path, CurrentHash: h, TargetHash: f.TargetHash, Status: "Modified", NeedsPatch: "Yes (Force)"}
		switch {
		case edition != "" && !f.AppliesTo(edition):
			row.Status, row.NeedsPatch = "Not in this edition", "No"
		case h == f.TargetHash || IsEXE(f.Path) && h == LAAHash:
			// Exact target evidence does not depend on an original executable
			// still being available. Unknown-edition changes remain blocked.
			row.Status = "Already Downgraded"
			row.NeedsPatch = "No"
			if h == LAAHash {
				row.Status += " (LAA)"
			}
		case len(f.ApplicableVersions) > 0 && edition == "":
			row.Status, row.NeedsPatch = "Source edition unknown", "N/A"
		case h == "":
			row.Status = "Missing"
			row.NeedsPatch = "N/A"
			row.CurrentHash = "N/A"
		case f.MatchesSource(h):
			row.Status = "Original (Needs Patch)"
			row.NeedsPatch = "Yes"
		}
		result.Rows = append(result.Rows, row)
		if progress != nil {
			progress(i+1, len(m.Files))
		}
	}
	return result, ctx.Err()
}
