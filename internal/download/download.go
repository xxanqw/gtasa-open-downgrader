// Package download handles bounded HTTP requests and cancellable streamed downloads.
package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

const PatchURL = "https://www.icloud.com/iclouddrive/0afGK6zDBog_0drwp6YZoDLIg#Patches"
const ICloudAPI = "https://ckdatabasews.icloud.com/database/1/com.apple.cloudkit/production/public/records/resolve"

type Progress func(done, total int64)
type Client struct {
	HTTP      *http.Client
	ICloudAPI string
}

func New() *Client {
	jar, _ := cookiejar.New(nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	httpClient := &http.Client{Transport: transport, Jar: jar, Timeout: 30 * time.Minute}
	httpClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0].URL.Scheme == "https" && request.URL.Scheme != "https" {
			return fmt.Errorf("refusing HTTPS downgrade redirect")
		}
		return nil
	}
	return &Client{HTTP: httpClient, ICloudAPI: ICloudAPI}
}
func (c *Client) Request(ctx context.Context, method, raw string, body io.Reader, contentType string) (*http.Response, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return nil, e
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported URL scheme")
	}
	req, e := http.NewRequestWithContext(ctx, method, raw, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "gtasa-open-downgrader")
	req.Header.Set("Accept", "application/vnd.github+json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: HTTP %d", u.Host, resp.StatusCode)
	}
	return resp, nil
}
func (c *Client) Bytes(ctx context.Context, method, raw string, body io.Reader, contentType string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	r, e := c.Request(ctx, method, raw, body, contentType)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	b, e := io.ReadAll(io.LimitReader(r.Body, 8<<20+1))
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("metadata response too large")
	}
	return b, e
}
func (c *Client) JSON(ctx context.Context, raw string, v any) error {
	b, e := c.Bytes(ctx, "GET", raw, nil, "")
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (c *Client) Resolve(ctx context.Context, raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return "", e
	}
	if u.Hostname() != "www.icloud.com" && u.Hostname() != "icloud.com" {
		return raw, nil
	}
	id := strings.TrimPrefix(u.Path, "/iclouddrive/")
	if id == u.Path || id == "" || strings.Contains(id, "/") {
		return "", fmt.Errorf("invalid iCloud file link")
	}
	payload, _ := json.Marshal(map[string]any{"shortGUIDs": []any{map[string]string{"value": id}}})
	b, e := c.Bytes(ctx, "POST", c.ICloudAPI, bytes.NewReader(payload), "application/json")
	if e != nil {
		return "", e
	}
	var result struct {
		Results []struct {
			RootRecord struct {
				Fields struct {
					FileContent struct {
						Value struct {
							DownloadURL string `json:"downloadURL"`
						} `json:"value"`
					} `json:"fileContent"`
				} `json:"fields"`
			} `json:"rootRecord"`
		} `json:"results"`
	}
	if e = json.Unmarshal(b, &result); e != nil {
		return "", e
	}
	if len(result.Results) == 0 || result.Results[0].RootRecord.Fields.FileContent.Value.DownloadURL == "" {
		return "", fmt.Errorf("iCloud did not return a file download URL (folders are unsupported)")
	}
	return result.Results[0].RootRecord.Fields.FileContent.Value.DownloadURL, nil
}

type counter struct {
	io.Writer
	done, total int64
	progress    Progress
	last        time.Time
}

func (w *counter) Write(b []byte) (int, error) {
	n, e := w.Writer.Write(b)
	w.done += int64(n)
	if w.progress != nil && time.Since(w.last) > 100*time.Millisecond {
		w.progress(w.done, w.total)
		w.last = time.Now()
	}
	return n, e
}
func (c *Client) File(ctx context.Context, raw, dst string, progress Progress) error {
	raw, e := c.Resolve(ctx, raw)
	if e != nil {
		return e
	}
	r, e := c.Request(ctx, "GET", raw, nil, "")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	return fsutil.Write(ctx, dst, 0644, func(w io.Writer) error {
		count := &counter{Writer: w, total: r.ContentLength, progress: progress}
		_, e := fsutil.Copy(ctx, count, r.Body)
		if e == nil && r.ContentLength >= 0 && count.done != r.ContentLength {
			e = io.ErrUnexpectedEOF
		}
		if progress != nil {
			progress(count.done, count.total)
		}
		return e
	})
}
