// Package mtafonts prepares MTA's Linux font prerequisites. Only the downgrader
// imports this package; the Steam launcher does not prepare or download fonts.
package mtafonts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

const FontTermsURL = "https://corefonts.sourceforge.net/eula.htm"
const maxPackageBytes = 8 << 20

type Progress func(label string, done, total int64)
type fontPackage struct {
	name, url, sha     string
	cabOffset, cabSize int
	fonts              map[string]string
}

// Original Microsoft packages mirrored by SourceForge, also used by Winetricks.
// Never execute the downloaded EXE, rehost fonts, or modify their font tables.
var packages = []fontPackage{
	{name: "IELPKTH.CAB", url: "https://downloads.sourceforge.net/corefonts/OldFiles/IELPKTH.CAB", sha: "c1be3fb8f0042570be76ec6daa03a99142c88367c1bc810240b85827c715961a", cabOffset: 0, cabSize: 405656, fonts: map[string]string{
		"tahoma.ttf":   "8f8c9a611820737a687ab4624d687abb17b474746cb16f6108ef215bf08eb373",
		"tahomabd.ttf": "d5eff0e5e6bed9802f4c61bd1bb153bc981eb1e07c8fa2c2f5a7f723fccfd8b7"}},
	{name: "verdan32.exe", url: "https://downloads.sourceforge.net/corefonts/the%20fonts/final/verdan32.exe", sha: "c1cb61255e363166794e47664e2f21af8e3a26cb6346eb8d2ae2fa85dd5aad96", cabOffset: 67104, cabSize: 270603, fonts: map[string]string{
		"verdana.ttf": "96ed14949ca4b7392cff235b9c41d55c125382abbe0c0d3c2b9dd66897cae0cb"}},
}

// Ensure downloads only packages needed by this prefix and reuses verified
// originals from cache. Extraction uses the selected prefix's built-in
// extrac32.exe (MSZIP/LZX), not system Wine, a downloaded installer or a new tool.
func Ensure(ctx context.Context, client *http.Client, cache string, environment multiplayer.Environment, runner multiplayer.Runner, guard func() error, progress Progress) error {
	if guard == nil || runner == nil {
		return fmt.Errorf("automatic MTA fonts require the selected Proton runner and a closed-game guard")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	missing, err := multiplayer.MissingMTAFonts(environment)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	needed := map[string]bool{}
	for _, name := range missing {
		needed[name] = true
	}
	type fetched struct {
		spec fontPackage
		data []byte
	}
	var fetchedPackages []fetched
	for _, spec := range packages {
		required := false
		for name := range spec.fonts {
			required = required || needed[name]
		}
		if !required {
			continue
		}
		data, err := cachedPackage(ctx, client, cache, spec, progress)
		if err != nil {
			return err
		}
		fetchedPackages = append(fetchedPackages, fetched{spec, data})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	// Short C: paths avoid depending on a Z: mapping or exposing hidden Unix
	// cache directories to Wine. This private staging folder is never autoloaded.
	work, err := os.MkdirTemp(environment.PrefixDriveC, ".downgrader-mta-fonts-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	output := filepath.Join(work, "fonts")
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	windowsWork := `C:\` + filepath.Base(work)
	extractor, err := prefixExtractor(environment)
	if err != nil {
		return err
	}
	for i, item := range fetchedPackages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		spec := item.spec
		if spec.cabOffset < 0 || spec.cabSize < 36 || spec.cabOffset > len(item.data)-spec.cabSize {
			return fmt.Errorf("invalid pinned cabinet bounds: %s", spec.name)
		}
		cab := item.data[spec.cabOffset : spec.cabOffset+spec.cabSize]
		if !bytes.Equal(cab[:4], []byte("MSCF")) || int(binary.LittleEndian.Uint32(cab[8:12])) != spec.cabSize {
			return fmt.Errorf("invalid pinned cabinet header: %s", spec.name)
		}
		name := fmt.Sprintf("package-%d.cab", i)
		path := filepath.Join(work, name)
		if err := os.WriteFile(path, cab, 0600); err != nil {
			return err
		}
		if progress != nil {
			progress("Extracting MTA fonts", 0, 0)
		}
		if err := runner.Run(ctx, extractor, "/Y", "/E", "/L", windowsWork+`\fonts`, windowsWork+`\`+name); err != nil {
			return fmt.Errorf("extract MTA fonts with the selected Proton build: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// extrac32 can return success despite an extraction failure. The exact font
		// payload hashes, followed by family/face validation, are the completion proof.
		for font, hash := range spec.fonts {
			path, err := extractedFont(output, font)
			if err != nil {
				return err
			}
			data, err := readBounded(path, 4<<20)
			if err != nil {
				return err
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != hash {
				return fmt.Errorf("MTA font checksum mismatch: %s", font)
			}
		}
	}
	if progress != nil {
		progress("Installing MTA fonts", 0, 0)
	}
	return multiplayer.ImportMTAFonts(ctx, environment, output, guard)
}

func sourceForgeURL(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && (host == "downloads.sourceforge.net" || strings.HasSuffix(host, ".dl.sourceforge.net"))
}
func cachedPackage(ctx context.Context, client *http.Client, cache string, spec fontPackage, progress Progress) ([]byte, error) {
	path, err := fsutil.Join(cache, "mta-fonts/"+spec.sha+"/"+spec.name)
	if err != nil {
		return nil, err
	}
	if data, err := readBounded(path, maxPackageBytes); err == nil && fmt.Sprintf("%x", sha256.Sum256(data)) == spec.sha {
		return data, nil
	}
	if client == nil {
		return nil, fmt.Errorf("font downloader is unavailable")
	}
	bounded := *client
	bounded.Jar = nil
	bounded.Timeout = 3 * time.Minute
	bounded.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 8 || !sourceForgeURL(request.URL) {
			return fmt.Errorf("refusing MTA font redirect outside HTTPS SourceForge mirrors")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.url, nil)
	if err != nil {
		return nil, err
	}
	if !sourceForgeURL(req.URL) {
		return nil, fmt.Errorf("invalid MTA font source")
	}
	req.Header.Set("User-Agent", "gtasa-open-downgrader")
	req.Header.Set("Accept", "application/octet-stream")
	if progress != nil {
		progress("Downloading MTA fonts from SourceForge", 0, 0)
	}
	response, err := bounded.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", spec.name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", spec.name, response.StatusCode)
	}
	if response.ContentLength > maxPackageBytes {
		return nil, fmt.Errorf("font package exceeds 8 MiB limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPackageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPackageBytes {
		return nil, fmt.Errorf("font package exceeds 8 MiB limit")
	}
	if response.ContentLength >= 0 && int64(len(data)) != response.ContentLength {
		return nil, io.ErrUnexpectedEOF
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != spec.sha {
		return nil, fmt.Errorf("SourceForge font package checksum mismatch: %s", spec.name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := fsutil.Join(cache, "mta-fonts/"+spec.sha+"/"+spec.name); err != nil {
		return nil, err
	}
	if err := fsutil.Write(ctx, path, 0600, func(w io.Writer) error { _, err := w.Write(data); return err }); err != nil {
		return nil, err
	}
	if progress != nil {
		progress("Downloading MTA fonts from SourceForge", int64(len(data)), int64(len(data)))
	}
	return data, nil
}
func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, fmt.Errorf("font package is not an ordinary bounded file")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("font package exceeds size limit")
	}
	return data, err
}
func extractedFont(directory, name string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}
	if len(entries) > 64 {
		return "", fmt.Errorf("unexpected font cabinet inventory")
	}
	path := ""
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			if path != "" || !entry.Type().IsRegular() {
				return "", fmt.Errorf("ambiguous or non-ordinary extracted font: %s", name)
			}
			path = filepath.Join(directory, entry.Name())
		}
	}
	if path == "" {
		return "", fmt.Errorf("font cabinet did not extract %s", name)
	}
	return path, nil
}
func prefixExtractor(environment multiplayer.Environment) (string, error) {
	windows, err := extractedDirectory(environment.PrefixDriveC, "windows")
	if err != nil {
		return "", err
	}
	system, err := extractedDirectory(windows, "system32")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(system)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "extrac32.exe") {
			path := filepath.Join(system, entry.Name())
			if err := multiplayer.ValidateInstaller(path); err != nil {
				return "", err
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("selected Proton prefix is missing its built-in extrac32.exe; repair the Proton runtime")
}
func extractedDirectory(root, name string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	result := ""
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			if result != "" {
				return "", fmt.Errorf("ambiguous prefix directory: %s", name)
			}
			result = filepath.Join(root, entry.Name())
		}
	}
	if result == "" {
		return "", fmt.Errorf("selected Proton prefix is missing %s", name)
	}
	return result, nil
}
