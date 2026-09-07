package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
)

func TestUpdateChecksPreserveInstalledState(t *testing.T) {
	for _, tc := range []struct {
		name              string
		installed, latest mods.Release
		err               error
		detail            string
	}{
		{name: "current", installed: mods.Release{Version: "v1.0", Fingerprint: "same"}, latest: mods.Release{Version: "v1.0", Fingerprint: "same"}},
		{name: "update", installed: mods.Release{Version: "v1.0", Fingerprint: "old"}, latest: mods.Release{Version: "v2.0", Fingerprint: "new"}, detail: "Update available: v2.0"},
		{name: "offline", installed: mods.Release{Version: "v1.0"}, err: errors.New("offline"), detail: "Check unavailable"},
		{name: "untracked", installed: mods.Release{Version: "v1.0"}, latest: mods.Release{Version: "v2.0", Fingerprint: "new"}, detail: "Version untracked"},
		{name: "legacy", detail: "version unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := modUpdateLabel(tc.installed, tc.latest, tc.err)
			if !strings.HasPrefix(got, "Installed · ") || !strings.Contains(got, tc.installed.Version) || !strings.Contains(got, tc.detail) {
				t.Fatalf("lost installed state or update detail: %q", got)
			}
		})
	}
}
