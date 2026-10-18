// Package inieditor provides bounded INI discovery, structural validation and
// guarded, backed-up saves. It does not validate plugin-specific value schemas.
package inieditor

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxBytes = 256 * 1024

var scopes = map[string][]string{
	"ModLoader":         {"modloader/modloader.ini", "modloader/.profiles/default.ini"},
	"SilentPatch":       {"modloader/SilentPatch", "SilentPatchSA.ini", "scripts/SilentPatchSA.ini"},
	"Widescreen Fixes":  {"modloader/WidescreenFix", "GTASA.WidescreenFix.ini", "scripts/GTASA.WidescreenFix.ini"},
	"SkyGFX":            {"modloader/SkyGFX", "skygfx.ini", "scripts/skygfx.ini"},
	"GInput":            {"modloader/GInput", "GInputSA.ini", "scripts/GInputSA.ini"},
	"Project 2DFX":      {"modloader/Project2DFX", "SA.Project2DFX.ini", "scripts/SA.Project2DFX.ini"},
	mods.Framerate:      {"modloader/FramerateVigilante", "FramerateVigilante.ini", "scripts/FramerateVigilante.ini"},
	mods.ExtendedFonts:  {"modloader/ExtendedLanguageLoader"},
	mods.NadalaoWeapons: {"modloader/NadalaoWeaponIcons"},
}

func resolve(root, rel string) (string, error) {
	clean, err := fsutil.Relative(rel)
	if err != nil {
		return "", err
	}
	parts := strings.Split(clean, "/")
	current := root
	for i, part := range parts {
		entries, e := os.ReadDir(current)
		if e != nil {
			return "", e
		}
		found := ""
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				if found != "" {
					return "", fmt.Errorf("ambiguous case-insensitive path: %s", rel)
				}
				found = entry.Name()
			}
		}
		if found == "" {
			return "", os.ErrNotExist
		}
		parts[i] = found
		current, e = fsutil.Join(root, strings.Join(parts[:i+1], "/"))
		if e != nil {
			return "", e
		}
	}
	return strings.Join(parts, "/"), nil
}
func Supports(mod string) bool { return len(scopes[mod]) > 0 }
func Discover(root, mod string) ([]string, error) {
	var paths []string
	seen := map[string]bool{}
	for _, scope := range scopes[mod] {
		rel, err := resolve(root, scope)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		full, err := fsutil.Join(root, rel)
		if err != nil {
			return nil, err
		}
		visited := 0
		err = filepath.WalkDir(full, func(p string, d os.DirEntry, e error) error {
			visited++
			if visited > 4096 {
				return fmt.Errorf("configuration discovery exceeds 4096-entry limit")
			}
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("configuration path contains a symbolic link: %s", p)
			}
			if d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ".ini") {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("configuration is not an ordinary file: %s", p)
			}
			candidate, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			candidate = filepath.ToSlash(candidate)
			if !seen[candidate] {
				paths = append(paths, candidate)
				seen[candidate] = true
			}
			if len(paths) > 64 {
				return fmt.Errorf("too many INI files in this package")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

type Document struct {
	Root, Mod, Path, Text, LineEnding string
	BOM                               bool
	Original                          []byte
	Mode                              os.FileMode
	Info                              os.FileInfo
}

func Load(root, mod, rel string) (*Document, error) {
	files, err := Discover(root, mod)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, p := range files {
		allowed = allowed || p == rel
	}
	if !allowed {
		return nil, fmt.Errorf("INI is outside this mod's configuration scope")
	}
	full, err := fsutil.Join(root, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > MaxBytes {
		return nil, fmt.Errorf("INI must be an ordinary file no larger than %d KiB", MaxBytes/1024)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("INI exceeds size limit")
	}
	original := append([]byte(nil), data...)
	bom := bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if bom {
		data = data[3:]
	}
	if !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
		return nil, fmt.Errorf("unsupported INI encoding: only UTF-8/ASCII text is editable; legacy encodings are not converted")
	}
	ending := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		ending = "\r\n"
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.Contains(text, "\r") || (ending == "\r\n" && strings.Contains(strings.ReplaceAll(string(data), "\r\n", ""), "\n")) {
		return nil, fmt.Errorf("mixed or unsupported line endings; original file left unchanged")
	}
	if strings.Count(text, "\n") > 6000 {
		return nil, fmt.Errorf("INI exceeds 6000-line editor limit")
	}
	return &Document{Root: root, Mod: mod, Path: rel, Text: text, LineEnding: ending, BOM: bom, Original: original, Mode: st.Mode(), Info: st}, nil
}

type Diagnostic struct {
	Line    int
	Warning bool
	Message string
}

func Validate(text string) []Diagnostic {
	var out []Diagnostic
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return []Diagnostic{{1, false, "Invalid UTF-8 or NUL character"}}
	}
	if len(text) > MaxBytes || strings.Count(text, "\n") > 6000 {
		return []Diagnostic{{1, false, "Editor size/line limit exceeded"}}
	}
	section := ""
	sections := map[string]bool{}
	keys := map[string]bool{}
	for i, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, ";") || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "[") {
			close := strings.IndexByte(t, ']')
			if close < 2 {
				out = append(out, Diagnostic{i + 1, false, "Missing closing bracket or empty section name"})
				continue
			}
			tail := strings.TrimSpace(t[close+1:])
			name := strings.TrimSpace(t[1:close])
			if name == "" || strings.ContainsAny(name, "[]") || (tail != "" && !strings.HasPrefix(tail, ";") && !strings.HasPrefix(tail, "#")) {
				out = append(out, Diagnostic{i + 1, false, "Malformed section header"})
				continue
			}
			section = strings.ToLower(name)
			if sections[section] {
				out = append(out, Diagnostic{i + 1, true, "Repeated section; plugin behavior may differ"})
			}
			sections[section] = true
			continue
		}
		key, _, ok := strings.Cut(t, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, "[]") {
			out = append(out, Diagnostic{i + 1, false, "Expected key=value assignment"})
			continue
		}
		id := section + "\x00" + strings.ToLower(key)
		if keys[id] {
			out = append(out, Diagnostic{i + 1, true, "Duplicate key; plugin behavior may differ"})
		}
		keys[id] = true
	}
	return out
}
func (d *Document) verify() (string, error) {
	full, err := fsutil.Join(d.Root, d.Path)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(full)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() || !os.SameFile(d.Info, st) || st.Mode() != d.Mode || st.Size() > MaxBytes {
		return "", fmt.Errorf("INI was replaced or its permissions changed; reload before saving")
	}
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	opened, err := f.Stat()
	if err != nil {
		f.Close()
		return "", err
	}
	if !os.SameFile(d.Info, opened) || !opened.Mode().IsRegular() {
		f.Close()
		return "", fmt.Errorf("INI was replaced while reading; reload before saving")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if !bytes.Equal(data, d.Original) {
		return "", fmt.Errorf("INI changed outside the editor; reload before saving")
	}
	return full, nil
}

// Save never manufactures missing files or silently overwrites an external
// replacement. Callers provide a fresh running-game and UI-state guard.
func (d *Document) Save(text string, guard func() error) (string, error) {
	if guard == nil {
		return "", fmt.Errorf("configuration save requires a closed-game guard")
	}
	if err := guard(); err != nil {
		return "", err
	}
	for _, issue := range Validate(text) {
		if !issue.Warning {
			return "", fmt.Errorf("line %d: %s", issue.Line, issue.Message)
		}
	}
	data := []byte(strings.ReplaceAll(text, "\n", d.LineEnding))
	if d.BOM {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("encoded INI exceeds size limit")
	}
	full, err := d.verify()
	if err != nil {
		return "", err
	}
	if bytes.Equal(data, d.Original) {
		return "", nil
	}
	sum := sha256.Sum256(d.Original)
	backupRel := fmt.Sprintf(".downgrader/config-editor/backups/%d-%x.bin", time.Now().UnixNano(), sum[:8])
	backup, err := fsutil.Join(d.Root, backupRel)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(backup), 0700); err != nil {
		return "", err
	}
	backup, err = fsutil.Join(d.Root, backupRel)
	if err != nil {
		return "", err
	}
	b, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, err = b.Write(d.Original)
	if err == nil {
		err = b.Sync()
	}
	closeErr := b.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(full), ".config-editor-*.bin")
	if err != nil {
		return "", err
	}
	stage := f.Name()
	defer os.Remove(stage)
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(d.Mode.Perm())
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr = f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	// Recheck after staging and backup, immediately before atomic replacement.
	if err = guard(); err != nil {
		return "", err
	}
	if _, err = d.verify(); err != nil {
		return "", err
	}
	files, err := Discover(d.Root, d.Mod)
	if err != nil {
		return "", err
	}
	allowed := false
	for _, p := range files {
		allowed = allowed || p == d.Path
	}
	if !allowed {
		return "", fmt.Errorf("configuration scope changed")
	}
	if err = guard(); err != nil {
		return "", err
	}
	if _, err = d.verify(); err != nil {
		return "", err
	}
	if err = os.Rename(stage, full); err != nil {
		return "", err
	}
	d.Original = append([]byte(nil), data...)
	d.Text = text
	d.Info, err = os.Stat(full)
	if err != nil {
		return backupRel, err
	}
	return backupRel, nil
}
