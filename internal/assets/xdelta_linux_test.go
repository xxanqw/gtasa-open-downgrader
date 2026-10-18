//go:build linux && amd64 && !launcher

package assets_test

import (
	"context"
	"debug/elf"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
)

func TestXdeltaLinuxGlibcBaseline(t *testing.T) {
	binary, cleanup, err := assets.Xdelta(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	file, err := elf.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	section := file.Section(".gnu.version_r")
	if section == nil {
		return // A statically linked binary has no shared-library version requirements.
	}
	// Version names live in the string table, not in the version-need records.
	stringsSection := file.Sections[section.Link]
	data, err := stringsSection.Data()
	if err != nil {
		t.Fatal(err)
	}
	for _, requirement := range regexp.MustCompile(`GLIBC_[^\x00]+`).FindAllString(string(data), -1) {
		var major, minor int
		if strings.HasPrefix(requirement, "GLIBC_ABI_") {
			t.Errorf("xdelta3 requires %s; rebuild with scripts/build-xdelta.sh for Ubuntu 22.04", requirement)
		} else if _, err := fmt.Sscanf(requirement, "GLIBC_%d.%d", &major, &minor); err != nil {
			t.Errorf("unknown glibc requirement %q", requirement)
		} else if major > 2 || major == 2 && minor > 35 {
			t.Errorf("xdelta3 requires %s, newer than the supported glibc 2.35 baseline", requirement)
		}
	}
}
