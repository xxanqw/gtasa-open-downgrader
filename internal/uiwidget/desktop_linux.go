//go:build linux

package uiwidget

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ConfigureDesktopCursors must run before creating the Fyne application. GLFW's
// Wayland backend reads XCURSOR_THEME/SIZE, not KDE's kcminputrc, and otherwise
// defaults to a different theme and 16px. Supply missing process-local defaults;
// never overwrite explicit environment values or change desktop configuration.
// This does not force a window backend or affect the privileged helper CLI.
func ConfigureDesktopCursors() {
	kde := false
	for _, name := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if strings.EqualFold(name, "KDE") {
			kde = true
		}
	}
	if !kde {
		return
	}
	_, themeSet := os.LookupEnv("XCURSOR_THEME")
	_, sizeSet := os.LookupEnv("XCURSOR_SIZE")
	if themeSet && sizeSet {
		return
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(config) {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		config = filepath.Join(home, ".config")
	}
	theme, size := kdeCursorSettings(filepath.Join(config, "kcminputrc"))
	if !themeSet && theme != "" {
		_ = os.Setenv("XCURSOR_THEME", theme)
	}
	if !sizeSet && size != "" {
		_ = os.Setenv("XCURSOR_SIZE", size)
	}
}

func kdeCursorSettings(path string) (string, string) {
	// Reading user-managed symlinks (e.g. dotfiles) is fine; special files and
	// oversized input are not. O_NONBLOCK prevents a FIFO from hanging startup.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return "", ""
	}
	data, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil || len(data) > 64<<10 || !utf8.Valid(data) {
		return "", ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	mouse := false
	theme, size := "", ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			mouse = end > 0 && line[1:end] == "Mouse"
			continue
		}
		if !mouse {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Immutable KConfig keys may carry [$i]. Do not evaluate [$e] shell/env
		// expansion, localized variants, or any other config directives.
		key = strings.TrimSuffix(key, "[$i]")
		switch key {
		case "cursorTheme":
			theme = ""
			if len(value) > 0 && len(value) <= 255 && value != "." && value != ".." && !strings.ContainsAny(value, "/\\") && !strings.ContainsFunc(value, unicode.IsControl) {
				theme = value
			}
		case "cursorSize":
			size = ""
			if n, e := strconv.Atoi(value); e == nil && n > 0 && n <= 512 {
				size = strconv.Itoa(n)
			}
		}
	}
	if scanner.Err() != nil {
		return "", ""
	}
	return theme, size
}
