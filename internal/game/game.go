// Package game reads the existing manifest format and identifies game files.
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
	"667f799c4ba8c9e1054fccaea6d4259b": "v1.0 US (Compact)", "6c6160da9b175b66cf9127c86be57bf7": "v1.0 EU",
	"49dd417760484a18017805df46b308b8": "v1.0 EU (Alt)", "9f2d711dbf1fbbcda5ff9418a2cc1ef5": "v1.01 US",
	"25405921d1c47747fd01fd0bfe0a05ae": "v1.01 EU", "d9cb35c898d3298ca904a63e10ee18d9": "NewSteam R2 (German)", "5bfd4dd83989a8264de4b8e771f237fd": "NewSteam R2",
}

type File struct {
	Path       string `json:"path"`
	Action     string `json:"action"`
	SourceHash string `json:"source_hash"`
	TargetHash string `json:"target_hash"`
}
type Manifest struct {
	Version       string `json:"version"`
	SourceVersion string `json:"source_version"`
	TargetVersion string `json:"target_version"`
	Files         []File `json:"files"`
}

func validHash(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 16 }
func (m *Manifest) Validate() error {
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest contains no files")
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
	}
	return nil
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
	executables := []string{entrypoint.RealGameExecutable, entrypoint.SteamExecutable}
	if entrypoint.MetadataPresent(root) {
		executables = executables[:1]
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
		case h == f.TargetHash || IsEXE(f.Path) && h == LAAHash:
			row.Status = "Already Downgraded"
			row.NeedsPatch = "No"
			if h == LAAHash {
				row.Status += " (LAA)"
			}
		case h == "":
			row.Status = "Missing"
			row.NeedsPatch = "N/A"
			row.CurrentHash = "N/A"
		case h == f.SourceHash:
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
