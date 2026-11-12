package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopIconRegistrationPreservesExistingIntegration(t *testing.T) {
	data := t.TempDir()
	executable := `/home/player/My Games/100% GTA.AppImage`
	icon := []byte("icon")
	if err := registerDesktopIcon(data, executable, icon); err != nil {
		t.Fatal(err)
	}
	desktopPath := filepath.Join(data, "applications", DowngraderAppID+".desktop")
	desktop, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`Exec="/home/player/My Games/100%% GTA.AppImage"`, "NoDisplay=true", "StartupWMClass=" + DowngraderAppID} {
		if !strings.Contains(string(desktop), want) {
			t.Fatalf("missing %q in %s", want, desktop)
		}
	}
	custom := []byte("[Desktop Entry]\nName=Custom integration\n")
	if err := os.WriteFile(desktopPath, custom, 0644); err != nil {
		t.Fatal(err)
	}
	if err := registerDesktopIcon(data, executable, []byte("updated icon")); err != nil {
		t.Fatal(err)
	}
	preserved, _ := os.ReadFile(desktopPath)
	if string(preserved) != string(custom) {
		t.Fatal("existing integration overwritten")
	}
	installed, err := os.ReadFile(filepath.Join(data, "icons", "hicolor", "256x256", "apps", DowngraderAppID+".png"))
	if err != nil || string(installed) != "updated icon" {
		t.Fatal("icon not updated", err)
	}
}

func TestDesktopRegistrationRejectsInjectedLines(t *testing.T) {
	if err := registerDesktopIcon(t.TempDir(), "/tmp/game\nHidden=true", []byte("icon")); err == nil {
		t.Fatal("accepted newline in executable")
	}
}
