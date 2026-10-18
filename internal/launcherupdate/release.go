// Package launcherupdate checks first-party release metadata and verifies launcher downloads.
package launcherupdate

import (
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	"github.com/xxanqw/gtasa-open-downgrader/internal/service"
)

const AssetName = "gta-sa.exe"
const maxSize = 128 << 20

// Manifest binds a release version to one immutable download URL and checksum.
type Manifest struct {
	Schema  int    `json:"schema_version"`
	Version string `json:"version"`
	Tag     string `json:"tag_name"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Digest  string `json:"digest"`
	URL     string `json:"browser_download_url"`
}

func (m Manifest) Validate() error {
	digest, err := hex.DecodeString(strings.TrimPrefix(m.Digest, "sha256:"))
	if m.Schema != 1 || m.Name != AssetName || !launcherversion.Valid(m.Version) || m.Tag != "launcher-"+m.Version || m.Size <= 0 || m.Size > maxSize || !strings.HasPrefix(m.Digest, "sha256:") || err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("invalid Steam launcher release metadata")
	}
	expected := service.BaseURL + "/api/v1/releases/" + url.PathEscape(m.Tag) + "/download/" + AssetName
	if m.URL != expected {
		return fmt.Errorf("launcher download URL is not the version-pinned first-party endpoint")
	}
	return nil
}

func Newer(candidate, current string) bool {
	return launcherversion.Newer(candidate, current)
}

func Check(ctx context.Context, client *http.Client) (Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := request(ctx, client, service.LauncherReleaseURL)
	if err != nil {
		return Manifest{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil {
		return Manifest{}, err
	}
	if len(data) > 64<<10 {
		return Manifest{}, fmt.Errorf("launcher release metadata is too large")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("read launcher release: %w", err)
	}
	return manifest, manifest.Validate()
}

func request(ctx context.Context, client *http.Client, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gtasa-open-downgrader-launcher-updates")
	httpClient := *client
	previousRedirect := client.CheckRedirect
	httpClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != "https" || len(via) >= 10 {
			return fmt.Errorf("unsafe launcher update redirect")
		}
		if previousRedirect != nil {
			return previousRedirect(next, via)
		}
		return nil
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if response.Request != nil && response.Request.URL.Scheme != "https" {
		response.Body.Close()
		return nil, fmt.Errorf("launcher update redirected away from HTTPS")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		if response.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("no published Steam launcher release is available yet")
		}
		return nil, fmt.Errorf("launcher update service returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

func Verify(ctx context.Context, path string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != manifest.Size {
		return fmt.Errorf("launcher download size does not match release metadata")
	}
	hash := sha256.New()
	if _, err := fsutil.Copy(ctx, hash, file); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), strings.TrimPrefix(manifest.Digest, "sha256:")) {
		return fmt.Errorf("launcher download checksum mismatch")
	}
	image, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("launcher is not a Windows executable: %w", err)
	}
	defer image.Close()
	if _, ok := image.OptionalHeader.(*pe.OptionalHeader64); !ok || image.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("launcher is not a Windows amd64 executable")
	}
	version, err := launcherversion.ReadFile(path)
	if err != nil {
		return err
	}
	if version != manifest.Version {
		return fmt.Errorf("launcher build version %s does not match release %s", version, manifest.Version)
	}
	return nil
}

// Download only stages a verified executable; it never changes the game directory.
func Download(ctx context.Context, client *http.Client, cache string, manifest Manifest, progress func(int64, int64)) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	response, err := request(ctx, client, manifest.URL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.ContentLength >= 0 && response.ContentLength != manifest.Size {
		return "", fmt.Errorf("launcher download length does not match release metadata")
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(cache, ".steam-launcher-*.exe")
	if err != nil {
		return "", err
	}
	path := file.Name()
	keep := false
	defer func() {
		file.Close()
		if !keep {
			os.Remove(path)
		}
	}()
	writer := &progressWriter{file: file, total: manifest.Size, progress: progress}
	n, err := fsutil.Copy(ctx, writer, io.LimitReader(response.Body, manifest.Size+1))
	if err != nil {
		return "", err
	}
	if n != manifest.Size {
		return "", fmt.Errorf("launcher download size does not match release metadata")
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := Verify(ctx, path, manifest); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

type progressWriter struct {
	file        *os.File
	done, total int64
	progress    func(int64, int64)
}

func (w *progressWriter) Write(data []byte) (int, error) {
	n, err := w.file.Write(data)
	w.done += int64(n)
	if w.progress != nil {
		w.progress(w.done, w.total)
	}
	return n, err
}
