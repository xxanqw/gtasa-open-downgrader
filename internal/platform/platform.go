// Package platform isolates operating-system integration from the UI.
package platform

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

const LaunchOptions = `WINEDLLOVERRIDES="vorbisFile=n,b" %command%`

func Detect() string {
	for _, p := range Candidates() {
		if game.HasEXE(p) {
			return p
		}
	}
	return ""
}
func SteamLibraries(root string) []string {
	libs := []string{filepath.Join(root, "steamapps", "common", "Grand Theft Auto San Andreas")}
	b, e := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if e != nil {
		return libs
	}
	for _, m := range regexp.MustCompile(`"path"\s+"([^"]+)"`).FindAllStringSubmatch(string(b), -1) {
		libs = append(libs, filepath.Join(strings.ReplaceAll(m[1], `\\`, `\`), "steamapps", "common", "Grand Theft Auto San Andreas"))
	}
	return libs
}

// ApplyLAA validates the DOS and PE signatures before modifying characteristics.
func ApplyLAA(ctx context.Context, p string) error {
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	if len(b) < 64 || string(b[:2]) != "MZ" {
		return fmt.Errorf("not a PE executable")
	}
	offset := uint64(binary.LittleEndian.Uint32(b[60:64]))
	if offset+24 > uint64(len(b)) || string(b[offset:offset+4]) != "PE\x00\x00" {
		return fmt.Errorf("invalid PE header")
	}
	flags := binary.LittleEndian.Uint16(b[offset+22 : offset+24])
	if flags&0x20 != 0 {
		return nil
	}
	binary.LittleEndian.PutUint16(b[offset+22:offset+24], flags|0x20)
	st, e := os.Stat(p)
	if e != nil {
		return e
	}
	return fsutil.Write(ctx, p, st.Mode(), func(w io.Writer) error { _, e := w.Write(b); return e })
}
