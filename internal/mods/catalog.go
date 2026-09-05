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
)

const Framerate = "Framerate Vigilante (60fps fix)"

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
	{Name: Framerate, Version: "2026-08-16", URL: "https://sharemods.com/0cxm88ppiv38/Framerate_Vigilante.zip.html", Page: "https://www.mixmods.com.br/2022/08/iii-vc-sa-framerate-vigilante/"},
	{Name: "GInput", Version: "2016-02-09", URL: "https://silent.rockstarvision.com/uploads/GInputSA.zip", Page: "https://silentsblog.com/mods/gta-sa/#ginput"},
	{Name: "Project 2DFX", Repository: "ThirteenAG/III.VC.SA.IV.Project2DFX", Endpoint: "releases/tags/gtasa", Asset: "SA.Project2DFX.zip", DateVersion: true},
}

type Release struct {
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
	SourceURL   string `json:"source_url"`
	InstalledAt string `json:"installed_at,omitempty"`
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
type Resolver struct {
	Client    *download.Client
	GitHubAPI string
	cache     map[string]githubRelease
}

func NewResolver(c *download.Client) *Resolver {
	return &Resolver{Client: c, GitHubAPI: "https://api.github.com/repos", cache: map[string]githubRelease{}}
}
func Supported(name string) bool {
	for _, s := range Catalog {
		if s.Name == name {
			return true
		}
	}
	return false
}
func (r *Resolver) Resolve(ctx context.Context, name string) (Release, error) {
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
	if name == Framerate {
		return r.framerate(ctx, s)
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
