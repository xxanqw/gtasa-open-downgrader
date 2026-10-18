// Package mtavalidation implements the opt-in official Linux MTA serial recovery
// workflow. It never edits the game, Proton prefix, identifiers or client settings.
package mtavalidation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const GuideURL = "https://help.multitheftauto.com/sa/trouble/serial-validation/"
const Lifetime = 5 * time.Minute
const ServerPort = 22003
const ASEPort = ServerPort + 123
const helperFlag = "--mta-serial-helper"
const protocol = "GTASA-MTA-SERIAL-1\n"

type packageSpec struct {
	URL, SHA, Prefix string
	Limit            int64
}

// Exact official archives observed 2026-10-17. Rolling official URLs are only
// transport: changed bytes fail closed until these pins are deliberately reviewed.
var serverPackage = packageSpec{"https://linux.multitheftauto.com/dl/multitheftauto_linux_x64.tar.gz", "84e3614ac9b88a7e57e5abb4ee7f72e074cbd4474675a503968edb05b16b9992", "multitheftauto_linux_x64", 16 << 20}
var configPackage = packageSpec{"https://linux.multitheftauto.com/dl/baseconfig.tar.gz", "161beb7823bc77df9952339e6e7048c61a426bd186ff6313e1e50b3b61804465", "baseconfig", 128 << 10}

type Packages struct{ server, config []byte }
type Event struct {
	Stage   string `json:"stage"`
	Message string `json:"message,omitempty"`
}
type Result struct{ MasterReady, Cleaned bool }
type Progress func(Event)

func Prepare(ctx context.Context, client *http.Client, progress Progress) (*Packages, error) {
	result := &Packages{}
	for i, spec := range []packageSpec{serverPackage, configPackage} {
		if progress != nil {
			progress(Event{Stage: "download", Message: fmt.Sprintf("Downloading official helper package %d / 2", i+1)})
		}
		data, err := download(ctx, client, spec)
		if err != nil {
			return nil, err
		}
		if _, err := unpack(data, spec); err != nil {
			return nil, err
		}
		if i == 0 {
			result.server = data
		} else {
			result.config = data
		}
	}
	return result, nil
}
func trustedURL(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && (u.Hostname() == "linux.multitheftauto.com" || u.Hostname() == "mirror-cdn.multitheftauto.com")
}
func download(ctx context.Context, client *http.Client, spec packageSpec) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	copyClient.Jar = nil
	copyClient.Timeout = 3 * time.Minute
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !trustedURL(req.URL) {
			return fmt.Errorf("untrusted MTA helper redirect")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return nil, err
	}
	if !trustedURL(req.URL) {
		return nil, fmt.Errorf("untrusted MTA helper source")
	}
	response, err := copyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download official MTA helper: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official MTA helper download returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, spec.Limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > spec.Limit {
		return nil, fmt.Errorf("MTA helper archive exceeds its size limit")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != spec.SHA {
		return nil, fmt.Errorf("official MTA helper archive changed or is corrupt; it will not be executed. Check for a reviewed application update")
	}
	return data, nil
}

type archiveFile struct {
	Name      string
	Data      []byte
	Directory bool
}

// Validate the entire inventory before privileged extraction. Links, special
// entries, collisions, traversal and oversized expansion are never accepted.
func unpack(data []byte, spec packageSpec) ([]archiveFile, error) {
	if int64(len(data)) > spec.Limit || fmt.Sprintf("%x", sha256.Sum256(data)) != spec.SHA {
		return nil, fmt.Errorf("MTA helper checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: 128<<20 + 1}
	reader := tar.NewReader(expanded)
	seen := map[string]bool{}
	var files []archiveFile
	total := int64(0)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(header.Name, "/")
		clean := path.Clean(name)
		if name == "" || name != clean || strings.ContainsAny(name, "\\\x00\r\n") || path.IsAbs(name) || len(name) > 256 || !(name == spec.Prefix || strings.HasPrefix(name, spec.Prefix+"/")) || seen[name] {
			return nil, fmt.Errorf("unsafe MTA helper archive path")
		}
		seen[name] = true
		if len(seen) > 128 || header.Size < 0 || header.Size > 64<<20 || total+header.Size > 128<<20 {
			return nil, fmt.Errorf("MTA helper archive exceeds its extraction limits")
		}
		total += header.Size
		item := archiveFile{Name: strings.TrimPrefix(name, spec.Prefix+"/")}
		if name == spec.Prefix {
			if header.Typeflag != tar.TypeDir {
				return nil, fmt.Errorf("invalid helper archive root")
			}
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			item.Directory = true
		case tar.TypeReg, tar.TypeRegA:
			item.Data, err = io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			if int64(len(item.Data)) != header.Size {
				return nil, io.ErrUnexpectedEOF
			}
		default:
			return nil, fmt.Errorf("MTA helper archive contains a link or special file")
		}
		files = append(files, item)
	}
	// Drain gzip to verify its checksum too; a corrupt trailer is not successful extraction.
	if _, err := io.Copy(io.Discard, expanded); err != nil {
		return nil, err
	}
	if expanded.N == 0 {
		return nil, fmt.Errorf("MTA helper archive expansion exceeds its limit")
	}
	return files, nil
}
