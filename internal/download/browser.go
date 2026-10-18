package download

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const maxBrowserArchive = 512 << 20

type fileStamp struct {
	size     int64
	modified time.Time
}
type observedDownload struct {
	stamp fileStamp
	since time.Time
}

// DownloadWatch observes only a requested archive and browser duplicate names.
// Existing files, symlinks, partial downloads, and unrelated downloads are ignored.
type DownloadWatch struct {
	directory string
	names     *regexp.Regexp
	baseline  map[string]fileStamp
	observed  map[string]observedDownload
	rejected  map[string]fileStamp
}

func WatchDownloads(directory, filename string) (*DownloadWatch, error) {
	if filepath.Base(filename) != filename || filename == "." || filename == "" {
		return nil, fmt.Errorf("invalid browser archive filename")
	}
	ext := filepath.Ext(filename)
	if !strings.EqualFold(ext, ".zip") && !strings.EqualFold(ext, ".7z") {
		return nil, fmt.Errorf("browser download must be a ZIP or 7z archive")
	}
	info, err := os.Stat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("browser download location is not a directory")
	}
	w := &DownloadWatch{directory: directory, names: regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(strings.TrimSuffix(filename, ext)) + `(?: ?\([0-9]+\)|\.[0-9]+)?` + regexp.QuoteMeta(ext) + `$`), baseline: map[string]fileStamp{}, observed: map[string]observedDownload{}, rejected: map[string]fileStamp{}}
	files, err := w.files()
	if err != nil {
		return nil, err
	}
	w.baseline = files
	return w, nil
}

func (w *DownloadWatch) files() (map[string]fileStamp, error) {
	entries, err := os.ReadDir(w.directory)
	if err != nil {
		return nil, err
	}
	files := map[string]fileStamp{}
	for _, entry := range entries {
		if !w.names.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if info.Mode().IsRegular() {
			files[entry.Name()] = fileStamp{info.Size(), info.ModTime()}
		}
	}
	return files, nil
}

func (w *DownloadWatch) Wait(ctx context.Context, destination, checksum string, notify func(string)) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		files, err := w.files()
		if err != nil {
			return fmt.Errorf("watch browser downloads: %w", err)
		}
		for name, stamp := range files {
			if stamp.size <= 0 || stamp == w.baseline[name] || stamp == w.rejected[name] {
				continue
			}
			source := filepath.Join(w.directory, name)
			partial := false
			for _, suffix := range []string{".crdownload", ".part", ".tmp", ".download"} {
				if _, err := os.Lstat(source + suffix); !os.IsNotExist(err) {
					partial = true
					break
				}
			}
			if partial {
				delete(w.observed, name)
				continue
			}
			previous, ok := w.observed[name]
			if !ok || previous.stamp != stamp {
				w.observed[name] = observedDownload{stamp, time.Now()}
				continue
			}
			if time.Since(previous.since) < 2*time.Second {
				continue
			}
			if err := CopyBrowserArchive(ctx, source, destination, checksum); err == nil {
				return nil
			} else {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				var fileError *os.PathError
				if errors.As(err, &fileError) {
					// Browsers and Windows antivirus can briefly retain a file lock.
					delete(w.observed, name)
					continue
				}
				w.rejected[name] = stamp
				if notify != nil {
					notify(fmt.Sprintf("Ignored %s: %v. Download the matching official archive or cancel.", name, err))
				}
			}
		}
		// A disappeared candidate must become stable again if it returns.
		for name := range w.observed {
			if _, ok := files[name]; !ok {
				delete(w.observed, name)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// CopyBrowserArchive stages a private, verified copy. User downloads are never
// moved/deleted, and a changing, substituted, or invalid archive is not committed.
func CopyBrowserArchive(ctx context.Context, source, destination, checksum string) error {
	if checksum != "" {
		digest, err := hex.DecodeString(checksum)
		if err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("invalid expected archive checksum")
		}
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBrowserArchive {
		return fmt.Errorf("download is not a regular archive within 512 MiB")
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return fmt.Errorf("download changed while opening")
	}
	var header [6]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return fmt.Errorf("incomplete archive: %w", err)
	}
	isZip := bytes.Equal(header[:4], []byte{'P', 'K', 3, 4}) || bytes.Equal(header[:4], []byte{'P', 'K', 5, 6})
	sevenZip := bytes.Equal(header[:], []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c})
	if !isZip && !sevenZip {
		return fmt.Errorf("download is not a ZIP or 7z archive")
	}
	if isZip {
		if _, err := zip.NewReader(file, opened.Size()); err != nil {
			return fmt.Errorf("incomplete ZIP archive: %w", err)
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return fsutil.Write(ctx, destination, 0600, func(out io.Writer) error {
		hash := sha256.New()
		n, err := fsutil.Copy(ctx, io.MultiWriter(out, hash), io.LimitReader(file, maxBrowserArchive+1))
		if err != nil {
			return err
		}
		final, err := file.Stat()
		if err != nil {
			return err
		}
		current, err := os.Lstat(source)
		if err != nil {
			return err
		}
		if n != opened.Size() || final.Size() != opened.Size() || !final.ModTime().Equal(opened.ModTime()) || !os.SameFile(current, opened) || !current.Mode().IsRegular() {
			return fmt.Errorf("download changed while copying; wait for completion")
		}
		if checksum != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), checksum) {
			return fmt.Errorf("download does not match the reviewed package checksum")
		}
		return nil
	})
}
