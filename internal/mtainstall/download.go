// Package mtainstall downloads reviewed original MTA client installers. Only the
// downgrader imports it; choosing or launching a client never fetches a build.
package mtainstall

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

const maxInstaller = 200 << 20

func trusted(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && (u.Hostname() == "nightly.multitheftauto.com" || u.Hostname() == "mirror-cdn.multitheftauto.com" || u.Hostname() == "multitheftauto.com")
}
func Ensure(ctx context.Context, client *http.Client, cache string, release multiplayer.MTARelease, progress func(int64, int64)) (string, error) {
	expected, ok := multiplayer.FindMTARelease(release.Version)
	if !ok || expected != release {
		return "", fmt.Errorf("unreviewed MTA release")
	}
	target, err := fsutil.Join(cache, filepath.ToSlash(filepath.Join("multiplayer", "mta", release.SHA256, release.Installer.FileName)))
	if err != nil {
		return "", err
	}
	if multiplayer.ValidateMTAInstaller(ctx, target, release) == nil {
		return target, nil
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if client == nil {
		client = http.DefaultClient
	}
	transport := *client
	transport.Jar = nil
	transport.Timeout = 20 * time.Minute
	transport.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !trusted(req.URL) {
			return fmt.Errorf("untrusted MTA installer redirect")
		}
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.Installer.URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "gtasa-open-downgrader")
	if !trusted(request.URL) {
		return "", fmt.Errorf("untrusted MTA installer source")
	}
	response, err := transport.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("official MTA installer returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxInstaller {
		return "", fmt.Errorf("MTA installer exceeds 200 MiB limit")
	}
	if _, err = fsutil.Join(cache, filepath.ToSlash(filepath.Join("multiplayer", "mta", release.SHA256, release.Installer.FileName))); err != nil {
		return "", err
	}
	err = fsutil.Write(ctx, target, 0600, func(writer io.Writer) error {
		buffer := make([]byte, 256<<10)
		reader := io.LimitReader(response.Body, maxInstaller+1)
		var total int64
		last := time.Time{}
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, e := reader.Read(buffer)
			if n > 0 {
				total += int64(n)
				if total > maxInstaller {
					return fmt.Errorf("MTA installer exceeds 200 MiB limit")
				}
				if _, err := writer.Write(buffer[:n]); err != nil {
					return err
				}
				if progress != nil && time.Since(last) > 100*time.Millisecond {
					progress(total, response.ContentLength)
					last = time.Now()
				}
			}
			if e == io.EOF {
				if response.ContentLength >= 0 && total != response.ContentLength {
					return io.ErrUnexpectedEOF
				}
				if progress != nil {
					progress(total, total)
				}
				return nil
			}
			if e != nil {
				return e
			}
		}
	})
	if err != nil {
		return "", err
	}
	if err = multiplayer.ValidateMTAInstaller(ctx, target, release); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	return target, nil
}
