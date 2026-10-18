package multiplayer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"golang.org/x/image/font/sfnt"
)

var mtaFonts = []struct {
	name, family string
	bold         bool
}{
	{"tahoma.ttf", "Tahoma", false}, {"tahomabd.ttf", "Tahoma", true}, {"verdana.ttf", "Verdana", false},
}

const mtaFontMaxBytes = 4 * 1024 * 1024

// caseChild accepts the actual casing used by Wine and user-provided fonts,
// but refuses ambiguous matches rather than choosing a different file silently.
func caseChild(root, name string) (string, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return filepath.Join(root, name), nil
	}
	if err != nil {
		return "", err
	}
	result := ""
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			if result != "" {
				return "", fmt.Errorf("ambiguous font path: %s", name)
			}
			result = filepath.Join(root, entry.Name())
		}
	}
	if result == "" {
		result = filepath.Join(root, name)
	}
	return result, nil
}
func mtaFontDirectory(environment Environment) (string, error) {
	if environment.PrefixDriveC == "" {
		return "", fmt.Errorf("MTA font setup requires the detected Proton prefix")
	}
	if st, err := os.Stat(environment.PrefixDriveC); err != nil || !st.IsDir() {
		return "", fmt.Errorf("MTA Proton drive_c is missing")
	}
	windows, err := caseChild(environment.PrefixDriveC, "windows")
	if err != nil {
		return "", err
	}
	return caseChild(windows, "Fonts")
}
func readMTAFont(path, family string, bold bool) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > mtaFontMaxBytes {
		return nil, fmt.Errorf("font must be an ordinary TTF no larger than 4 MiB: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, mtaFontMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > mtaFontMaxBytes {
		return nil, fmt.Errorf("font exceeds size limit: %s", path)
	}
	font, err := sfnt.Parse(data)
	if err != nil || font.NumGlyphs() == 0 {
		return nil, fmt.Errorf("invalid TrueType font: %s", path)
	}
	var buffer sfnt.Buffer
	name, err := font.Name(&buffer, sfnt.NameIDFamily)
	if err != nil || !strings.EqualFold(strings.TrimSpace(name), family) {
		return nil, fmt.Errorf("%s must contain the %s font family, not a renamed substitute", path, family)
	}
	subfamily, err := font.Name(&buffer, sfnt.NameIDSubfamily)
	if err != nil || strings.Contains(strings.ToLower(subfamily), "bold") != bold || strings.Contains(strings.ToLower(subfamily), "italic") {
		return nil, fmt.Errorf("wrong regular/bold face: %s", path)
	}
	return data, nil
}

// MissingMTAFonts is read-only. Existing Wine font symlinks can satisfy the
// requirement; imports never follow those links to replace their targets.
func MissingMTAFonts(environment Environment) ([]string, error) {
	directory, err := mtaFontDirectory(environment)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, font := range mtaFonts {
		path, err := caseChild(directory, font.name)
		if err != nil {
			return nil, err
		}
		if _, err = readMTAFont(path, font.family, font.bold); err != nil {
			missing = append(missing, font.name)
		}
	}
	return missing, nil
}

// ImportMTAFonts copies only missing fonts from a validated staging folder. Each font is
// staged, synced and installed without replacement. Partial completion remains
// usable/retryable; no files, preferences or prefix architecture are reset.
func ImportMTAFonts(ctx context.Context, environment Environment, source string, guard func() error) error {
	if guard == nil {
		return fmt.Errorf("MTA font import requires a closed-game guard")
	}
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("MTA font staging folder is missing")
	}
	directory, err := mtaFontDirectory(environment)
	if err != nil {
		return err
	}
	type pending struct {
		name string
		data []byte
	}
	var fonts []pending
	for _, font := range mtaFonts {
		target, err := caseChild(directory, font.name)
		if err != nil {
			return err
		}
		if _, err := readMTAFont(target, font.family, font.bold); err == nil {
			continue
		}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("existing font is invalid or a different face; it will not be overwritten: %s", target)
		} else if !os.IsNotExist(err) {
			return err
		}
		path, err := caseChild(source, font.name)
		if err != nil {
			return err
		}
		data, err := readMTAFont(path, font.family, font.bold)
		if err != nil {
			return err
		}
		fonts = append(fonts, pending{font.name, data})
	}
	if len(fonts) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	relative, err := filepath.Rel(environment.PrefixDriveC, directory)
	if err != nil {
		return err
	}
	safe, err := fsutil.Join(environment.PrefixDriveC, filepath.ToSlash(relative))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(safe, 0755); err != nil {
		return err
	}
	for _, font := range fonts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		target, err := fsutil.Join(environment.PrefixDriveC, filepath.ToSlash(filepath.Join(relative, font.name)))
		if err != nil {
			return err
		}
		// Detect a concurrently installed case variant too, even on Linux.
		current, err := caseChild(directory, font.name)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(current); err == nil {
			return fmt.Errorf("font appeared during import; existing copy retained: %s", current)
		} else if !os.IsNotExist(err) {
			return err
		}
		stage, err := os.CreateTemp(directory, ".downgrader-mta-font-*.bin")
		if err != nil {
			return err
		}
		name := stage.Name()
		err = func() error {
			defer os.Remove(name)
			if _, err := stage.Write(font.data); err != nil {
				stage.Close()
				return err
			}
			if err := stage.Chmod(0644); err != nil {
				stage.Close()
				return err
			}
			if err := stage.Sync(); err != nil {
				stage.Close()
				return err
			}
			if err := stage.Close(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := guard(); err != nil {
				return err
			}
			if _, err := fsutil.Join(environment.PrefixDriveC, filepath.ToSlash(filepath.Join(relative, font.name))); err != nil {
				return err
			}
			// Link, unlike Rename, fails if a replacement appeared after inspection.
			return os.Link(name, target)
		}()
		if err != nil {
			return fmt.Errorf("import %s (already completed font copies are retained): %w", font.name, err)
		}
	}
	return nil
}
