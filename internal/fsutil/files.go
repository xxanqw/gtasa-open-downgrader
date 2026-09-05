// Package fsutil provides cancellable, staged file operations shared by installers.
package fsutil

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Relative rejects paths that are unsafe on either supported platform.
func Relative(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	if name == "" || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." {
			continue
		}
		if part == ".." || strings.ContainsAny(part, ":\x00") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("unsafe path %q", name)
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
			return "", fmt.Errorf("reserved path %q", name)
		}
	}
	name = path.Clean(name)
	if name == "." {
		return "", fmt.Errorf("empty path")
	}
	return name, nil
}

// Join also rejects existing symlink components. Installers never follow archive
// or game-directory links outside the directory the user selected.
func Join(root, name string) (string, error) {
	rel, err := Relative(name)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	for _, part := range strings.Split(rel, "/") {
		full = filepath.Join(full, part)
		st, e := os.Lstat(full)
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symbolic link is not allowed: %s", full)
		}
	}
	return full, nil
}

type Reader struct {
	Context context.Context
	Reader  io.Reader
}

func (r Reader) Read(p []byte) (int, error) {
	if err := r.Context.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
func Copy(ctx context.Context, w io.Writer, r io.Reader) (int64, error) {
	return io.CopyBuffer(w, Reader{ctx, r}, make([]byte, 256*1024))
}
func Hash(ctx context.Context, p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := md5.New()
	if _, e = Copy(ctx, h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Write commits a sibling temporary file only when writing and cancellation
// checks succeed. Rename replaces files on the supported Windows/Unix systems.
func Write(ctx context.Context, dst string, mode os.FileMode, write func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), ".downgrader-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	err = write(f)
	if err == nil {
		err = f.Chmod(mode.Perm())
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
func CopyFile(ctx context.Context, src, dst string) error {
	f, e := os.Open(src)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", src)
	}
	return Write(ctx, dst, st.Mode(), func(w io.Writer) error { _, e := Copy(ctx, w, f); return e })
}
