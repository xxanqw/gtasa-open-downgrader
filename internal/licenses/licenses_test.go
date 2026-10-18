package licenses

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

func TestNoticeBundleAndMPLSourceIntegrity(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema      int                                         `json:"schema"`
		NoticesHash string                                      `json:"notices_sha256"`
		SourceHash  string                                      `json:"mpl_source_sha256"`
		Targets     map[string][]struct{ Path, Version string } `json:"targets"`
		Notices     []struct{ Title, Source, SHA256 string }    `json:"notices"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	hash := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if manifest.Schema != 1 || hash([]byte(Text)) != manifest.NoticesHash || hash(MPLSource) != manifest.SourceHash {
		t.Fatal("generated notices/source do not match the reviewed manifest")
	}
	for _, record := range manifest.Notices {
		start := strings.Index(Text, strings.ToUpper(record.Title)+"\n\n")
		if start < 0 {
			t.Fatalf("missing notice %s", record.Title)
		}
		body := Text[start+len(strings.ToUpper(record.Title))+2:]
		if end := strings.Index(body, "\n\n"+strings.Repeat("-", 48)+"\n\n"); end >= 0 {
			body = body[:end]
		}
		if hash([]byte(body)) != record.SHA256 {
			t.Fatalf("incomplete/modified notice %s", record.Title)
		}
	}
	for _, target := range []string{"linux", "windows", "launcher"} {
		modules := manifest.Targets[target]
		if len(modules) == 0 {
			t.Fatalf("missing %s production modules", target)
		}
		for _, module := range modules {
			if !strings.Contains(Text, strings.ToUpper(module.Path+" "+module.Version)+" — ") {
				t.Fatalf("%s module missing its versioned notice: %s", target, module.Path)
			}
			if strings.Contains(module.Path, "testify") || strings.Contains(module.Path, "go-difflib") {
				t.Fatalf("test-only module entered notice inventory: %s", module.Path)
			}
		}
	}
	for _, required := range []string{"Copyright (C) 2018 Fyne.io developers", "Copyright 2009 The Go Authors.",
		"THE INTER PROJECT AUTHORS", "LICENSE_DEJAVU-POWERLINE.TXT", "LICENSE_EMOJIONECOLOR.TXT",
		"HarfBuzz", "Mozilla Public License", "simplelru/LICENSE_list", "SAVE MPL SOURCE"} {
		if !strings.Contains(strings.ToUpper(Text), strings.ToUpper(required)) {
			t.Fatalf("missing %s", required)
		}
	}
	archive, err := zip.NewReader(bytes.NewReader(MPLSource), int64(len(MPLSource)))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, file := range archive.File {
		if path.IsAbs(file.Name) || strings.Contains(file.Name, "..") || !strings.HasPrefix(file.Name, "golang-lru-v2-v2.0.7/") || file.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("unsafe source member %s", file.Name)
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(file.Name, "golang-lru-v2-v2.0.7/")
		found[name] = true
		if name == "LICENSE" && !strings.Contains(string(body), "Mozilla Public License") {
			t.Fatal("source archive lost MPL license")
		}
	}
	for _, name := range []string{"LICENSE", "simplelru/LICENSE_list", "internal/list.go", "lru.go", "go.mod"} {
		if !found[name] {
			t.Fatalf("source archive missing %s", name)
		}
	}
}

func TestReleasePackagingIncludesNotices(t *testing.T) {
	for _, name := range []string{"scripts/build.sh", "scripts/build-launcher.sh", "scripts/appimage.sh"} {
		data, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "generate-notices.py --check --package dist/THIRD-PARTY-NOTICES.zip") {
			t.Fatalf("%s skips notice coverage/packaging", name)
		}
	}
	data, err := os.ReadFile("../../.github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "dist/THIRD-PARTY-NOTICES.zip") != 3 || !strings.Contains(string(data), "gta-sa.exe THIRD-PARTY-NOTICES.zip > SHA256SUMS") {
		t.Fatal("release workflow omits notice artifact or checksum")
	}
}
