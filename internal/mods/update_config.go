package mods

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const maxMergeConfigSize = 4 << 20

// prepareUpdateFile changes only the staged copy. An existing config with an
// unfamiliar format stays in the game directory; new config files are installed.
func prepareUpdateFile(ctx context.Context, current, staged string) (bool, error) {
	oldInfo, err := os.Stat(current)
	if err != nil {
		return false, err
	}
	newInfo, err := os.Stat(staged)
	if err != nil {
		return false, err
	}
	if !oldInfo.Mode().IsRegular() || !newInfo.Mode().IsRegular() {
		return false, fmt.Errorf("mod destination is not a regular file: %s", current)
	}
	switch strings.ToLower(filepath.Ext(current)) {
	case ".ini":
		if oldInfo.Size() > maxMergeConfigSize || newInfo.Size() > maxMergeConfigSize {
			return false, nil
		}
		old, err := os.ReadFile(current)
		if err != nil {
			return false, err
		}
		newer, err := os.ReadFile(staged)
		if err != nil {
			return false, err
		}
		merged, supported := mergeINI(old, newer)
		if !supported || bytes.Equal(merged, old) {
			return false, nil
		}
		if err = fsutil.Write(ctx, staged, oldInfo.Mode(), func(w io.Writer) error {
			_, err := w.Write(merged)
			return err
		}); err != nil {
			return false, err
		}
		return true, nil
	case ".cfg", ".conf", ".json", ".toml", ".yaml", ".yml", ".xml":
		return false, nil
	}
	equal, err := sameFile(ctx, current, staged)
	return !equal, err
}

func sameFile(ctx context.Context, a, b string) (bool, error) {
	one, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer one.Close()
	two, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer two.Close()
	first, err := one.Stat()
	if err != nil {
		return false, err
	}
	second, err := two.Stat()
	if err != nil {
		return false, err
	}
	if !first.Mode().IsRegular() || !second.Mode().IsRegular() {
		return false, fmt.Errorf("mod destination is not a regular file: %s", a)
	}
	if first.Size() != second.Size() {
		return false, nil
	}
	left, right := make([]byte, 64*1024), make([]byte, 64*1024)
	for {
		if err = ctx.Err(); err != nil {
			return false, err
		}
		n, leftErr := one.Read(left)
		m, rightErr := two.Read(right)
		if n != m || !bytes.Equal(left[:n], right[:m]) {
			return false, nil
		}
		if leftErr == io.EOF && rightErr == io.EOF {
			return true, nil
		}
		if leftErr != nil || rightErr != nil {
			return false, fmt.Errorf("compare mod files: %v, %v", leftErr, rightErr)
		}
	}
}

type iniSection struct {
	name string
	keys map[string]bool
	add  []string
}

// mergeINI retains every existing line and value. It only inserts keys that
// exist in the new package but nowhere in the corresponding old section.
func mergeINI(old, newer []byte) ([]byte, bool) {
	if !utf8.Valid(old) || !utf8.Valid(newer) || bytes.IndexByte(old, 0) >= 0 || bytes.IndexByte(newer, 0) >= 0 {
		return nil, false
	}
	newline := "\n"
	if bytes.Contains(old, []byte("\r\n")) {
		newline = "\r\n"
	}
	oldText := strings.ReplaceAll(string(old), "\r\n", "\n")
	newText := strings.ReplaceAll(string(newer), "\r\n", "\n")
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	oldEnded := strings.HasSuffix(oldText, "\n")
	if oldEnded {
		oldLines = oldLines[:len(oldLines)-1]
	}
	sections := map[string]*iniSection{"": {keys: map[string]bool{}}}
	seenSections := map[string]bool{"": true}
	current := ""
	for _, line := range oldLines {
		if name, ok := iniHeader(line); ok {
			current = strings.ToLower(name)
			seenSections[current] = true
			if sections[current] == nil {
				sections[current] = &iniSection{name: name, keys: map[string]bool{}}
			}
		} else if key, ok := iniKey(line); ok {
			sections[current].keys[strings.ToLower(key)] = true
		}
	}
	order := []string{""}
	current = ""
	for _, line := range newLines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if name, ok := iniHeader(line); ok {
			current = strings.ToLower(name)
			if sections[current] == nil {
				sections[current] = &iniSection{name: name, keys: map[string]bool{}}
				order = append(order, current)
			}
			continue
		}
		key, ok := iniKey(line)
		if !ok {
			return nil, false
		}
		section := sections[current]
		lower := strings.ToLower(key)
		if !section.keys[lower] {
			section.add = append(section.add, line)
			section.keys[lower] = true
		}
	}
	var out []string
	appendMissing := func(section string) {
		out = append(out, sections[section].add...)
		sections[section].add = nil
	}
	current = ""
	for _, line := range oldLines {
		if name, ok := iniHeader(line); ok {
			appendMissing(current)
			current = strings.ToLower(name)
		}
		out = append(out, line)
	}
	appendMissing(current)
	for _, section := range order {
		if seenSections[section] || len(sections[section].add) == 0 {
			continue
		}
		out = append(out, "["+sections[section].name+"]")
		appendMissing(section)
	}
	result := strings.Join(out, newline)
	if oldEnded || len(out) > len(oldLines) {
		result += newline
	}
	return []byte(result), true
}

func iniHeader(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if !strings.HasPrefix(line, "[") {
		return "", false
	}
	close := strings.IndexByte(line, ']')
	if close < 2 || strings.TrimSpace(line[1:close]) == "" {
		return "", false
	}
	rest := strings.TrimSpace(line[close+1:])
	if rest != "" && !strings.HasPrefix(rest, ";") && !strings.HasPrefix(rest, "#") {
		return "", false
	}
	return strings.TrimSpace(line[1:close]), true
}

func iniKey(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
		return "", false
	}
	equal, colon := strings.IndexByte(line, '='), strings.IndexByte(line, ':')
	if colon >= 0 && (equal < 0 || colon < equal) {
		equal = colon
	}
	if equal < 1 {
		return "", false
	}
	key := strings.TrimSpace(line[:equal])
	return key, key != ""
}
