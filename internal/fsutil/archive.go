package fsutil

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bodgit/sevenzip"
)

type Mapper func(string) string

const MaxExtractedSize int64 = 16 << 30

// Extract supports ZIP and 7z, validates even filtered members, rejects links,
// duplicate destinations and excessive expansion, and never writes partial files.
func Extract(ctx context.Context, archive, dst, kind string, mapName Mapper) error {
	seen := map[string]bool{}
	var total int64
	member := func(name string, mode os.FileMode, size uint64, open func() (io.ReadCloser, error)) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		name = strings.TrimSuffix(strings.ReplaceAll(name, `\`, "/"), "/")
		if name == "" {
			return nil
		}
		clean, err := Relative(name)
		if err != nil {
			return err
		}
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return fmt.Errorf("archive link or special file: %s", name)
		}
		if mode.IsDir() {
			return nil
		}
		if size > uint64(MaxExtractedSize) || total > MaxExtractedSize-int64(size) {
			return fmt.Errorf("archive exceeds extraction limit")
		}
		total += int64(size)
		if mapName != nil {
			clean = mapName(clean)
		}
		if clean == "" {
			return nil
		}
		target, err := Join(dst, clean)
		if err != nil {
			return err
		}
		key := strings.ToLower(target)
		if seen[key] {
			return fmt.Errorf("duplicate archive destination: %s", clean)
		}
		seen[key] = true
		r, err := open()
		if err != nil {
			return err
		}
		defer r.Close()
		return Write(ctx, target, 0644, func(w io.Writer) error {
			n, e := Copy(ctx, w, io.LimitReader(r, int64(size)+1))
			if e != nil {
				return e
			}
			if uint64(n) != size {
				return fmt.Errorf("archive size mismatch: %s", name)
			}
			return nil
		})
	}
	if kind == "7z" {
		r, e := sevenzip.OpenReader(archive)
		if e != nil {
			return e
		}
		defer r.Close()
		for _, f := range r.File {
			if e = member(f.Name, f.Mode(), f.UncompressedSize, f.Open); e != nil {
				return e
			}
		}
	} else {
		r, e := zip.OpenReader(archive)
		if e != nil {
			return e
		}
		defer r.Close()
		for _, f := range r.File {
			if e = member(f.Name, f.Mode(), f.UncompressedSize64, f.Open); e != nil {
				return e
			}
		}
	}
	return ctx.Err()
}
