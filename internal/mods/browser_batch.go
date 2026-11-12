package mods

import "context"

// BrowserArchive describes an archive, including auxiliary packages that would
// otherwise be discovered only after installing part of the selection.
type BrowserArchive struct {
	Name, URL, Checksum string
}

// Preparation returns a session downloader and cleanup for temporary archives.
// The installer calls cleanup even when installation subsequently fails.
type PrepareBrowserDownloadsFunc func(context.Context, []BrowserArchive) (BrowserDownloadFunc, func(), error)

func browserArchives(name string, rel Release) []BrowserArchive {
	if rel.FileID == "" {
		return nil
	}
	checksum := rel.Checksum
	if spec, ok := interfacePacks[name]; ok {
		checksum = spec.checksum
	}
	archives := []BrowserArchive{{Name: name, URL: rel.URL, Checksum: checksum}}
	if name == NadalaoWeapons {
		archives = append(archives, BrowserArchive{Name: name + " helper", URL: weaponHelperURL, Checksum: weaponHelperSHA})
	}
	return archives
}

func (i Installer) prepareBrowser(ctx context.Context, resolver *Resolver, archives []BrowserArchive) (func(), error) {
	if len(archives) == 0 {
		return func() {}, nil
	}
	downloader, cleanup, err := i.PrepareBrowserDownloads(ctx, archives)
	if err != nil {
		return nil, err
	}
	resolver.BrowserDownload = downloader
	if cleanup == nil {
		cleanup = func() {}
	}
	return cleanup, nil
}
