package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Launcher builds a small Windows PE with the production build identity, not UI code.
func Launcher(t testing.TB, version string) string {
	t.Helper()
	root := t.TempDir()
	Write(t, root, "go.mod", "module github.com/xxanqw/gtasa-open-downgrader\n\ngo 1.25.0\n")
	Write(t, root, "internal/launcherversion/version.go", "package launcherversion\nvar Identity = \"development\"\n")
	Write(t, root, "cmd/steam-launcher/main.go", "package main\nimport (\"fmt\"; \"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion\")\nfunc main() { fmt.Println(launcherversion.Identity) }\n")
	path := filepath.Join(root, "gta-sa.exe")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", "-s -w -X github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion.Identity=GTASA_STEAM_LAUNCHER_VERSION["+version+"]", "-o", path, "./cmd/steam-launcher")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build launcher fixture: %v\n%s", err, output)
	}
	return path
}
