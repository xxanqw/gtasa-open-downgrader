package download

import "golang.org/x/sys/windows"

func DownloadsDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
}
