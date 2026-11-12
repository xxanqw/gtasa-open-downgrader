package ui

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func startDiagnostics(j *applog.Journal, game, patches string) {
	cache, err := os.UserCacheDir()
	if err == nil {
		path := filepath.Join(cache, "gtasa-open-downgrader", "logs", "current.log")
		if err = j.Store(path); err == nil {
			j.Event("INFO", "log.storage", "path", path, "segments", 3, "segment_mib", 4)
		}
	}
	if err != nil {
		j.Event("WARN", "log.storage.unavailable", "error", err)
	}
	j.Event("INFO", "session.start", "id", rand.Text(), "version", updater.Version, "launcher", launcherversion.Version(), "os", runtime.GOOS, "arch", runtime.GOARCH, "go", runtime.Version(), "cpus", runtime.NumCPU(), "gomaxprocs", runtime.GOMAXPROCS(0), "offline", assets.Offline, "pid", os.Getpid())
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range build.Deps {
			if dependency.Path == "fyne.io/fyne/v2" || dependency.Path == "github.com/go-gl/glfw/v3.4/glfw" {
				j.Event("DEBUG", "build.dependency", "module", dependency.Path, "version", dependency.Version)
			}
		}
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision", "vcs.time", "vcs.modified", "CGO_ENABLED", "-tags":
				j.Event("DEBUG", "build.setting", "key", setting.Key, "value", setting.Value)
			}
		}
	}
	// Explicit allowlist: never dump the full environment or command-line args.
	for _, key := range []string{"XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "WAYLAND_DISPLAY", "DISPLAY", "FYNE_SCALE", "GDK_BACKEND", "QT_QPA_PLATFORM", "APPIMAGE"} {
		if value := os.Getenv(key); value != "" {
			j.Event("DEBUG", "desktop.environment", "key", key, "value", value)
		}
	}
	executable, _ := os.Executable()
	working, _ := os.Getwd()
	j.Event("DEBUG", "session.paths", "executable", executable, "working_directory", working, "game_argument", game, "patches_argument", patches)
}
