// Package assets embeds the application artwork and platform patch engine.
package assets

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

//go:embed icon.png
var Icon []byte

//go:embed LICENSE.txt
var XdeltaLicense string

func Xdelta(ctx context.Context) (string, func(), error) {
	if runtime.GOARCH != "amd64" {
		return "", func() {}, fmt.Errorf("bundled xdelta3 supports amd64 only")
	}
	dir, e := os.MkdirTemp("", "gtasa-xdelta-*")
	if e != nil {
		return "", func() {}, e
	}
	cleanup := func() { os.RemoveAll(dir) }
	name := "xdelta3"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(dir, name)
	e = fsutil.Write(ctx, p, 0700, func(w io.Writer) error { _, e := w.Write(xdelta); return e })
	if e != nil {
		cleanup()
	}
	return p, cleanup, e
}
