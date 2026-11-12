// Package logshare uploads a user-approved log and prepares a GitHub issue draft.
package logshare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
)

const BaseURL = "https://files.xxanqw.me"
const UploadURL = BaseURL + "/logs/upload"
const IssueURL = "https://github.com/xxanqw/gtasa-open-downgrader/issues/new"

type Receipt struct {
	URL         string    `json:"url"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
	SHA256      string    `json:"sha256"`
	Size        int       `json:"size_bytes"`
}

var logPath = regexp.MustCompile(`^/logs/[A-Za-z0-9_-]{24}$`)

func Upload(ctx context.Context, client *download.Client, endpoint, text string) (Receipt, error) {
	var receipt Receipt
	if len(text) == 0 || len(text) > 16<<20 {
		return receipt, fmt.Errorf("log must contain between 1 byte and 16 MiB")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	// Consent applies to this service; never forward the log to a redirect host.
	copyClient := *client
	copyHTTP := *client.HTTP
	copyHTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return fmt.Errorf("log upload redirects are not allowed") }
	copyClient.HTTP = &copyHTTP
	data, err := copyClient.Bytes(ctx, "POST", endpoint, strings.NewReader(text), "text/plain; charset=utf-8")
	if err != nil {
		return receipt, err
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return receipt, fmt.Errorf("invalid log upload response: %w", err)
	}
	base, _ := url.Parse(endpoint)
	view, err := url.Parse(receipt.URL)
	if err != nil || base == nil || view.Scheme != base.Scheme || view.Host != base.Host || view.User != nil || !logPath.MatchString(view.Path) || view.RawQuery != "" || view.Fragment != "" {
		return Receipt{}, fmt.Errorf("log service returned an unexpected share link")
	}
	if receipt.DownloadURL != receipt.URL+"/download" {
		return Receipt{}, fmt.Errorf("log service returned an unexpected download link")
	}
	digest := sha256.Sum256([]byte(text))
	if receipt.Size != len(text) || !strings.EqualFold(receipt.SHA256, hex.EncodeToString(digest[:])) {
		return Receipt{}, fmt.Errorf("log service did not confirm the uploaded size and checksum")
	}
	if receipt.ExpiresAt.Before(time.Now()) || receipt.ExpiresAt.After(time.Now().Add(31*24*time.Hour)) {
		return Receipt{}, fmt.Errorf("log service returned an unexpected expiry")
	}
	return receipt, nil
}

func IssueDraft(version, platform, link string) *url.URL {
	body := "### What happened?\n\nDescribe the problem and how to reproduce it.\n\n### Environment\n\n- App: " + version + "\n- Platform: " + platform + "\n\n### Diagnostic log\n\n"
	if link == "" {
		body += "Attach the exported application log here.\n"
	} else {
		body += link + "\n\nThis log link expires after 30 days.\n"
	}
	u, _ := url.Parse(IssueURL)
	u.RawQuery = url.Values{"body": {body}}.Encode()
	return u
}
