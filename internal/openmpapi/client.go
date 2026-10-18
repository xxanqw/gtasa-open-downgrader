// Package openmpapi reads the official open.mp cached server index. It never
// queries individual servers, downloads artwork, or sends connection credentials.
package openmpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Matches the official launcher's getCachedList endpoint and core/ru schema:
// https://github.com/openmultiplayer/launcher/blob/master/src/api/apis.ts
const Endpoint = "https://api.open.mp/servers/full"
const MaxResponseBytes = 8 << 20
const MaxServers = 20000

type Core struct {
	Address  string `json:"ip"`
	Name     string `json:"hn"`
	Players  int    `json:"pc"`
	Capacity int    `json:"pm"`
	Mode     string `json:"gm"`
	Language string `json:"la"`
	Password bool   `json:"pa"`
	Version  string `json:"vn"`
	OpenMP   bool   `json:"omp"`
	Partner  bool   `json:"pr"`
}
type Server struct {
	Core    Core              `json:"core"`
	Rules   map[string]string `json:"ru"`
	Updated string            `json:"lastUpdated,omitempty"`
}
type Client struct{ HTTP *http.Client }

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" || req.URL.Host != "api.open.mp" {
			return fmt.Errorf("unexpected server-index redirect")
		}
		return nil
	}}}
}
func (c *Client) List(ctx context.Context) ([]Server, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gtasa-steam-launcher (https://github.com/xxanqw/gtasa-open-downgrader)")
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open.mp server index: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open.mp server index returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > MaxResponseBytes {
		return nil, fmt.Errorf("open.mp server index is too large")
	}
	return Decode(io.LimitReader(response.Body, MaxResponseBytes+1))
}

// Decode also validates locally cached data; all server-supplied fields remain
// bounded plain text, and only literal IPv4 addresses can populate join targets.
func Decode(reader io.Reader) ([]Server, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxResponseBytes {
		return nil, fmt.Errorf("open.mp server index is too large")
	}
	var servers []Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("invalid open.mp server index: %w", err)
	}
	if servers == nil || len(servers) > MaxServers {
		return nil, fmt.Errorf("invalid open.mp server count")
	}
	result := make([]Server, 0, len(servers))
	seen := make(map[string]bool)
	for _, server := range servers {
		host, port, err := net.SplitHostPort(server.Core.Address)
		ip := net.ParseIP(host)
		number, portErr := strconv.Atoi(port)
		if err != nil || ip == nil || ip.To4() == nil || portErr != nil || number < 1 || number > 65535 || server.Core.Players < 0 || server.Core.Capacity < 0 || server.Core.Players > 100000 || server.Core.Capacity > 100000 {
			continue
		}
		server.Core.Address = net.JoinHostPort(ip.To4().String(), strconv.Itoa(number))
		if seen[server.Core.Address] {
			continue
		}
		seen[server.Core.Address] = true
		server.Core.Name = plain(server.Core.Name, 256)
		if server.Core.Name == "" {
			server.Core.Name = server.Core.Address
		}
		server.Core.Mode = plain(server.Core.Mode, 128)
		server.Core.Language = plain(server.Core.Language, 128)
		server.Core.Version = plain(server.Core.Version, 128)
		rules := make(map[string]string)
		// These are display-only. Never interpret arbitrary rules as URLs or flags.
		for _, key := range []string{"allowed_clients", "mapname", "worldtime", "weather", "lagcomp", "artwork", "weburl", "version"} {
			if value := plain(server.Rules[key], 256); value != "" {
				rules[key] = value
			}
		}
		server.Rules = rules
		server.Updated = plain(server.Updated, 64)
		result = append(result, server)
	}
	if len(servers) > 0 && len(result) == 0 {
		return nil, fmt.Errorf("open.mp server index contains no valid addresses")
	}
	return result, nil
}
func plain(text string, limit int) string {
	var b strings.Builder
	count := 0
	for _, r := range text {
		if unicode.IsControl(r) {
			continue
		}
		if count >= limit {
			break
		}
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}
func Encode(servers []Server) string          { data, _ := json.Marshal(servers); return string(data) }
func FromCache(data string) ([]Server, error) { return Decode(bytes.NewBufferString(data)) }
