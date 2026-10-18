// Package mods resolves official packages and preserves the Python tracker format.
package mods

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/service"
)

const (
	Framerate          = "Framerate Vigilante (60fps fix)"
	HDFonts            = "HD Fonts" // retired tracker/uninstall ID
	ExtendedFonts      = "Extended Fonts (ELL)"
	HDRadarMap         = "HD Radar Map (Original HD)"
	legacySVGRadarMap  = "HD Radar Map (SVG)"
	OriginalHQIcons    = "Original HQ Radar Icons"
	AtmosphereHUD      = "Atmosphere HUD Textures"
	NadalaoWeapons     = "nadalao Weapon Icons"
	SanLtdRussian      = "SanLtd Russian" // retired tracker/uninstall ID
	legacySALtdRussian = "SALtd Russian"
)

type Source struct {
	Name, Repository, Endpoint, Asset, Prefix, Version, URL, Page string
	DateVersion                                                   bool
}

var Catalog = []Source{
	{Name: "ASI Loader", Version: "1.3", URL: "https://silent.rockstarvision.com/uploads/silents_asi_loader_13.zip", Page: "https://silentsblog.com/mods/gta-sa/#asiloader"},
	{Name: "ModLoader", Repository: "thelink2012/modloader", Endpoint: "releases/latest", Asset: "modloader.zip"},
	{Name: "SilentPatch", Repository: "CookiePLMonster/SilentPatch", Endpoint: "releases/latest", Asset: "SilentPatchSA.zip"},
	{Name: "Widescreen Fixes", Repository: "ThirteenAG/WidescreenFixesPack", Endpoint: "releases/tags/gtasa", Asset: "GTASA.WidescreenFix.zip", DateVersion: true},
	{Name: "SkyGFX", Repository: "aap/skygfx", Endpoint: "releases/latest", Prefix: "skygfx_sa_"},
	{Name: "Frontend Mods", Repository: "ThirteenAG/WidescreenFixesPack", Endpoint: "releases/tags/gtasa", Asset: "GTASA.WidescreenFrontend.zip", DateVersion: true},
	{Name: ExtendedFonts, Version: "0.1.0 / ELL 0.4.2 Beta", URL: extendedFontsURL},
	{Name: HDRadarMap, Version: "2021-07-20 / Yojo2 original HQ 512px", URL: "https://sharemods.com/zfcud8b4sc5g/Atmosphere_Interface_Pack.7z.html", Page: "https://www.mixmods.com.br/2021/01/atmosphere-interface-pack-interface-hd/"},
	{Name: OriginalHQIcons, Version: "2021-01-25", URL: "https://sharemods.com/e9q9mnaxppyv/SA_Original_Radar_Icons_HQ.7z.html", Page: "https://www.mixmods.com.br/2021/11/sa-sade-original-hq-icons-icones-originais-hd/"},
	{Name: AtmosphereHUD, Version: "2021-07-20 (selected textures)", URL: "https://sharemods.com/zfcud8b4sc5g/Atmosphere_Interface_Pack.7z.html", Page: "https://www.mixmods.com.br/2021/01/atmosphere-interface-pack-interface-hd/"},
	{Name: NadalaoWeapons, Version: "2022-02-16 / helper 1.0", URL: "https://sharemods.com/tysasyrhqebt/nadalao_Weapon_Icons_-_icones_fieis_corrigidos.7z.html", Page: "https://www.mixmods.com.br/2022/02/icones-de-armas-fieis-desenhos-corrigidos/"},
	{Name: Framerate, Version: "2026-08-16", URL: "https://sharemods.com/0cxm88ppiv38/Framerate_Vigilante.zip.html", Page: "https://www.mixmods.com.br/2022/08/iii-vc-sa-framerate-vigilante/"},
	{Name: "GInput", Version: "2016-02-09", URL: "https://silent.rockstarvision.com/uploads/GInputSA.zip", Page: "https://silentsblog.com/mods/gta-sa/#ginput"},
	{Name: "Project 2DFX", Repository: "ThirteenAG/III.VC.SA.IV.Project2DFX", Endpoint: "releases/tags/gtasa", Asset: "SA.Project2DFX.zip", DateVersion: true},
}

type Release struct {
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Checksum    string `json:"archive_sha256,omitempty"`
	SourceURL   string `json:"source_url"`
	InstalledAt string `json:"installed_at,omitempty"`
	Style       string `json:"style,omitempty"`
	URL         string `json:"-"`
	FileID      string `json:"-"`
	ArchiveType string `json:"-"`
}
type githubRelease struct {
	Tag    string `json:"tag_name"`
	Page   string `json:"html_url"`
	Assets []struct {
		ID      int64  `json:"id"`
		Size    int64  `json:"size"`
		Name    string `json:"name"`
		Updated string `json:"updated_at"`
		URL     string `json:"browser_download_url"`
	} `json:"assets"`
}
type BrowserDownloadFunc func(context.Context, string, string, string, download.Progress) error

type Resolver struct {
	BrowserDownload BrowserDownloadFunc
	Client          *download.Client
	GitHubAPI       string
	cache           map[string]githubRelease
}

func NewResolver(c *download.Client) *Resolver {
	if c == nil {
		c = download.New()
	}
	return &Resolver{Client: c, GitHubAPI: service.GitHubMetadataURL, cache: map[string]githubRelease{}}
}
func Supported(name string) bool {
	name = canonicalModName(name)
	for _, s := range Catalog {
		if s.Name == name {
			return true
		}
	}
	return false
}
func (r *Resolver) Resolve(ctx context.Context, name string) (Release, error) {
	name = canonicalModName(name)
	var s Source
	for _, candidate := range Catalog {
		if candidate.Name == name {
			s = candidate
			break
		}
	}
	if s.Name == "" {
		return Release{}, fmt.Errorf("unknown mod %q", name)
	}
	if s.Repository != "" {
		raw := r.GitHubAPI + "/" + s.Repository + "/" + s.Endpoint
		release, ok := r.cache[raw]
		if !ok {
			if e := r.Client.JSON(ctx, raw, &release); e != nil {
				return Release{}, e
			}
			r.cache[raw] = release
		}
		for _, a := range release.Assets {
			if a.Name == s.Asset || s.Prefix != "" && strings.HasPrefix(strings.ToLower(a.Name), s.Prefix) && strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
				version := release.Tag
				if s.DateVersion && len(a.Updated) >= 10 {
					version = a.Updated[:10]
				}
				if version == "" {
					version = "unknown"
				}
				return Release{Version: version, Fingerprint: fmt.Sprintf("github:%d:%s:%d", a.ID, a.Updated, a.Size), SourceURL: release.Page, URL: a.URL, ArchiveType: "zip"}, nil
			}
		}
		return Release{}, fmt.Errorf("official %s release has no matching package", name)
	}
	if name == ExtendedFonts {
		return Release{Version: s.Version, Fingerprint: "sha256:" + extendedFontsSHA + ":" + extendedLoaderSHA,
			URL: s.URL, SourceURL: s.URL, ArchiveType: "zip"}, nil
	}
	if spec, ok := interfacePacks[name]; ok {
		fingerprint := "sha256:" + spec.checksum
		if name == NadalaoWeapons {
			fingerprint += ":" + weaponHelperSHA
		}
		if name == HDRadarMap {
			fingerprint += ":original-hq-radar-512-v1"
		}
		rel := Release{Version: s.Version, Fingerprint: fingerprint, Checksum: spec.checksum, URL: s.URL, SourceURL: s.Page, ArchiveType: "7z"}
		if match := shareLink.FindStringSubmatch(s.URL); match != nil {
			rel.FileID = match[1]
		}
		return rel, nil
	}
	if name == Framerate {
		return r.framerate(ctx, s)
	}
	if name == SanLtdRussian {
		return r.saltdRussian(ctx, s)
	}
	parts := []string{"static", s.URL, s.Version}
	short, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, e := r.Client.Request(short, "HEAD", s.URL, nil, "")
	if e == nil {
		resp.Body.Close()
		parts = append(parts, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), resp.Header.Get("Content-Length"))
	}
	if ctx.Err() != nil {
		return Release{}, ctx.Err()
	}
	return Release{Version: s.Version, Fingerprint: strings.Join(parts, ":"), URL: s.URL, SourceURL: s.Page, ArchiveType: "zip"}, nil
}

var saltdCDN = regexp.MustCompile(`^dl[0-9]+\.gtavicecity\.ru$`)

func saltdDownload(body string) (string, error) {
	for _, match := range regexp.MustCompile(`(?i)href=["']([^"']+)["']`).FindAllStringSubmatch(body, -1) {
		candidate := html.UnescapeString(match[1])
		u, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if u.Scheme == "https" && saltdCDN.MatchString(strings.ToLower(u.Hostname())) && strings.HasPrefix(u.Path, "/ru/manual/22077/") {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("the SALtd manual ZIP mirror did not provide a valid download link")
}

func (r *Resolver) saltdRussian(ctx context.Context, s Source) (Release, error) {
	b, err := r.Client.Bytes(ctx, "GET", s.URL+"?ajax=true", nil, "")
	if err != nil {
		return Release{}, err
	}
	raw, err := saltdDownload(string(b))
	if err != nil {
		return Release{}, err
	}
	// The mirror rejects HEAD, so open and immediately close a GET response to
	// validate the generated URL and read stable archive metadata.
	resp, err := r.Client.Request(ctx, "GET", raw, nil, "")
	if err != nil {
		return Release{}, fmt.Errorf("validate SALtd manual ZIP: %w", err)
	}
	resp.Body.Close()
	fingerprint := strings.Join([]string{
		"gtavicecity", "22077", s.Version,
		resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), resp.Header.Get("Content-Length"),
	}, ":")
	return Release{Version: s.Version, Fingerprint: fingerprint, SourceURL: s.Page, URL: raw, ArchiveType: "zip"}, nil
}

var shareLink = regexp.MustCompile(`(?i)https?://(?:www\.)?sharemods\.com/([^/"'<>\s]+)/[^"'<>\s]+\.(zip|7z)\.html`)

func (r *Resolver) framerate(ctx context.Context, s Source) (Release, error) {
	rel := Release{Version: s.Version, URL: s.URL, SourceURL: s.Page, FileID: "0cxm88ppiv38", ArchiveType: "zip"}
	if b, e := r.Client.Bytes(ctx, "GET", s.Page, nil, ""); e == nil {
		if match := shareLink.FindStringSubmatch(string(b)); match != nil {
			rel.URL = html.UnescapeString(match[0])
			rel.FileID = match[1]
			rel.ArchiveType = strings.ToLower(match[2])
		}
	}
	uploaded, size := "unknown", "unknown"
	if b, e := r.Client.Bytes(ctx, "GET", rel.URL, nil, ""); e == nil {
		uploaded = metadata(string(b), "Uploaded on:")
		size = metadata(string(b), "Size:")
	}
	if ctx.Err() != nil {
		return Release{}, ctx.Err()
	}
	if uploaded != "unknown" {
		rel.Version = uploaded
	}
	rel.Fingerprint = "sharemods:" + rel.FileID + ":" + uploaded + ":" + size
	return rel, nil
}
func metadata(body, key string) string {
	m := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `\s*<strong>([^<]+)`).FindStringSubmatch(body)
	if m == nil {
		return "unknown"
	}
	return strings.TrimSpace(html.UnescapeString(m[1]))
}
func shareDownload(body, id string) (string, error) {
	for _, match := range regexp.MustCompile(`(?i)href=["'](https://[^"']+)["']`).FindAllStringSubmatch(body, -1) {
		candidate := html.UnescapeString(match[1])
		u, e := url.Parse(candidate)
		if e != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		r2 := host == "r2.cloudflarestorage.com" || strings.HasSuffix(host, ".r2.cloudflarestorage.com")
		ovh := host == "sharem.s3.de.io.cloud.ovh.net" && strings.HasPrefix(u.Path, "/uploads/") && strings.HasSuffix(strings.TrimRight(u.Path, "/"), "/"+id)
		if u.Scheme == "https" && (r2 || ovh) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ShareMods did not provide its official download link; download the package in your browser and use Import mod archive")
}

// Download routes protected ShareMods packages through the user's browser when
// available. Both paths retain the reviewed archive checksum verification.
func (r *Resolver) Download(ctx context.Context, rel Release, destination, checksum string, progress download.Progress) error {
	if rel.FileID != "" && r.BrowserDownload != nil {
		if err := r.BrowserDownload(ctx, rel.URL, destination, checksum, progress); err != nil {
			return err
		}
	} else {
		raw, err := r.DownloadURL(ctx, rel)
		if err != nil {
			return err
		}
		if err := r.Client.File(ctx, raw, destination, progress); err != nil {
			return err
		}
	}
	if checksum != "" {
		return checkSHA(destination, checksum)
	}
	return nil
}

func (r *Resolver) DownloadURL(ctx context.Context, rel Release) (string, error) {
	if rel.FileID == "" {
		return rel.URL, nil
	}
	if _, e := r.Client.Bytes(ctx, "GET", rel.URL, nil, ""); e != nil {
		return "", fmt.Errorf("ShareMods blocked the request; use Import mod archive: %w", e)
	}
	body := url.Values{"op": {"download2"}, "id": {rel.FileID}, "rand": {""}, "referer": {""}, "method_free": {""}, "method_premium": {""}}
	b, e := r.Client.Bytes(ctx, "POST", rel.URL, strings.NewReader(body.Encode()), "application/x-www-form-urlencoded")
	if e != nil {
		return "", e
	}
	return shareDownload(string(b), rel.FileID)
}
