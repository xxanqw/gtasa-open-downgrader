package steam

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestUnavailableSelectedProtonExplainsCustomBuildWithoutFallback(t *testing.T) {
	for _, app := range []string{"12120", "0"} {
		t.Run(app, func(t *testing.T) {
			fixture := newSteamFixture(t)
			fixture.addTool(t, "compatibilitytools.d/GE-Proton", "GE-Proton", "GE-Proton", 300)
			fixture.configure(t, app, "proton_experimental")
			configPath := fixture.root + "/config/config.vdf"
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			context, err := NewDetector(fixture.root).DetectProtonContext(testAppID)
			if context != nil || !errors.Is(err, ErrToolMissing) {
				t.Fatalf("unavailable selected tool silently replaced: context=%+v error=%v", context, err)
			}
			for _, want := range []string{"custom Proton", "GE-Proton", "Proton-GE", "proton_experimental", "restart Steam", "Properties > Compatibility", "refresh Multiplayer"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("missing guidance %q: %v", want, err)
				}
			}
			for _, unhelpful := range []string{"explicitly selects", "not installed"} {
				if strings.Contains(err.Error(), unhelpful) {
					t.Errorf("misleading message %q: %v", unhelpful, err)
				}
			}
			after, readErr := os.ReadFile(configPath)
			if readErr != nil || string(after) != string(before) {
				t.Fatal("Steam configuration was modified")
			}
		})
	}
}
