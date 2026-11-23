// Package mtainstall downloads and verifies official MTA client installers.
// Choosing or launching an installed client never fetches a build.
package mtainstall

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

const (
	maxInstaller   = 200 << 20
	maxNightlyPage = 2 << 20
	nightlyOrigin  = "https://nightly.multitheftauto.com/"
	nightlyIndex   = nightlyOrigin + "?branch=master"
)

var nightlyClientLink = regexp.MustCompile(`\bhref="(mtasa-1\.7-untested-([0-9]+)-[0-9]{8}\.exe)"`)

func Download(ctx context.Context, client *http.Client, cache, version string, progress func(int64, int64)) (VerifiedInstaller, error) {
	release, ok := multiplayer.FindMTARelease(version)
	if !ok {
		return VerifiedInstaller{}, fmt.Errorf("unsupported MTA release %q", version)
	}
	if client == nil {
		client = http.DefaultClient
	}
	transport := *client
	transport.Jar = nil
	transport.Timeout = 20 * time.Minute
	transport.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !trusted(req.URL) || req.URL.Hostname() != via[0].URL.Hostname() {
			return fmt.Errorf("untrusted MTA download redirect")
		}
		return nil
	}

	installer := multiplayer.Installers[multiplayer.ClientMTA]
	source, name := installer.URL, installer.FileName
	if release.Experimental {
		var err error
		name, err = latestNightly(ctx, &transport)
		if err != nil {
			return VerifiedInstaller{}, fmt.Errorf("find the current MTA %s client on the official nightly site: %w", version, err)
		}
		source = nightlyOrigin + name
	}
	response, err := get(ctx, &transport, source)
	if err != nil {
		return VerifiedInstaller{}, err
	}
	defer response.Body.Close()
	if response.ContentLength > maxInstaller {
		return VerifiedInstaller{}, fmt.Errorf("MTA installer exceeds 200 MiB limit")
	}

	dir, err := fsutil.Join(cache, "multiplayer/mta")
	if err != nil {
		return VerifiedInstaller{}, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return VerifiedInstaller{}, err
	}
	file, err := os.CreateTemp(dir, ".mta-*")
	if err != nil {
		return VerifiedInstaller{}, err
	}
	defer os.Remove(file.Name())
	defer file.Close()

	checksum := sha256.New()
	transfer := &downloadProgress{Writer: io.MultiWriter(file, checksum), total: response.ContentLength, report: progress}
	size, err := fsutil.Copy(ctx, transfer, io.LimitReader(response.Body, maxInstaller+1))
	if err != nil {
		return VerifiedInstaller{}, err
	}
	if size > maxInstaller {
		return VerifiedInstaller{}, fmt.Errorf("MTA installer exceeds 200 MiB limit")
	}
	if response.ContentLength >= 0 && size != response.ContentLength {
		return VerifiedInstaller{}, io.ErrUnexpectedEOF
	}
	if progress != nil {
		progress(size, size)
	}
	signedDigest, err := verifySignature(file, size)
	if err != nil {
		return VerifiedInstaller{}, fmt.Errorf("verify MTA %s installer publisher and signed SHA-256: %w", version, err)
	}
	applog.Event(ctx, "DEBUG", "mta.installer.verified", "file", name, "signed_sha256", signedDigest)

	sum := [sha256.Size]byte(checksum.Sum(nil))
	target, err := fsutil.Join(dir, fmt.Sprintf("%x/%s", sum, name))
	if err != nil {
		return VerifiedInstaller{}, err
	}
	if err := file.Close(); err != nil {
		return VerifiedInstaller{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return VerifiedInstaller{}, err
	}
	if err := ctx.Err(); err != nil {
		return VerifiedInstaller{}, err
	}
	if err := os.Rename(file.Name(), target); err != nil {
		return VerifiedInstaller{}, err
	}
	return VerifiedInstaller{path: target, sha256: sum}, nil
}

func latestNightly(ctx context.Context, client *http.Client) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := get(ctx, client, nightlyIndex)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	page, err := io.ReadAll(io.LimitReader(response.Body, maxNightlyPage+1))
	if err != nil {
		return "", err
	}
	if len(page) > maxNightlyPage {
		return "", fmt.Errorf("MTA nightly index exceeds 2 MiB")
	}
	var latest string
	var latestBuild uint64
	for _, link := range nightlyClientLink.FindAllSubmatch(page, -1) {
		build, err := strconv.ParseUint(string(link[2]), 10, 64)
		if err == nil && (build > latestBuild || (build == latestBuild && string(link[1]) > latest)) {
			latest, latestBuild = string(link[1]), build
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no Windows 10+ MTA 1.7 development client is listed")
	}
	return latest, nil
}

func trusted(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") &&
		(u.Hostname() == "nightly.multitheftauto.com" || u.Hostname() == "mirror-cdn.multitheftauto.com")
}

func get(ctx context.Context, client *http.Client, source string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	if !trusted(request.URL) {
		return nil, fmt.Errorf("untrusted MTA download source")
	}
	request.Header.Set("User-Agent", "gtasa-open-downgrader")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("official MTA download returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

type downloadProgress struct {
	io.Writer
	total, done int64
	report      func(int64, int64)
	last        time.Time
}

func (p *downloadProgress) Write(data []byte) (int, error) {
	n, err := p.Writer.Write(data)
	p.done += int64(n)
	if p.report != nil && time.Since(p.last) > 100*time.Millisecond {
		p.report(p.done, p.total)
		p.last = time.Now()
	}
	return n, err
}
