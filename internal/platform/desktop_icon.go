package platform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const DowngraderAppID = "me.xxanqw.gtasa-open-downgrader"

// RegisterDesktopIcon lets Wayland compositors resolve the window's app ID even
// when a portable binary or AppImage has not been installed in the app menu.
// A hidden fallback entry never replaces an existing desktop integration.
func RegisterDesktopIcon(icon []byte) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	data := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(data) {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		data = filepath.Join(home, ".local", "share")
	}
	executable := os.Getenv("APPIMAGE")
	if !filepath.IsAbs(executable) {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return err
		}
	}
	return registerDesktopIcon(data, executable, icon)
}

func registerDesktopIcon(data, executable string, icon []byte) error {
	// Desktop values cannot contain physical newlines. Exec also has its own
	// escaping rules; it is not a shell command.
	if strings.ContainsAny(data+executable, "\r\n") {
		return fmt.Errorf("desktop integration paths contain a newline")
	}
	iconPath := filepath.Join(data, "icons", "hicolor", "256x256", "apps", DowngraderAppID+".png")
	existing, err := os.ReadFile(iconPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(existing, icon) {
		if err := os.MkdirAll(filepath.Dir(iconPath), 0755); err != nil {
			return err
		}
		if err := fsutil.Write(context.Background(), iconPath, 0644, func(w io.Writer) error { _, err := w.Write(icon); return err }); err != nil {
			return err
		}
	}
	desktop := filepath.Join(data, "applications", DowngraderAppID+".desktop")
	if err := os.MkdirAll(filepath.Dir(desktop), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(desktop, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Exec quoting is processed after desktop-string escaping, requiring four
	// backslashes for a literal backslash and two before reserved characters.
	execValue := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "$", `\\$`, "`", "\\\\`", "%", "%%").Replace(executable)
	iconValue := strings.ReplaceAll(iconPath, `\`, `\\`)
	_, writeErr := fmt.Fprintf(file, "[Desktop Entry]\nType=Application\nName=GTA SA Open Downgrader\nExec=\"%s\"\nIcon=%s\nStartupWMClass=%s\nNoDisplay=true\nCategories=Game;\n", execValue, iconValue, DowngraderAppID)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(desktop)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}
