package launcherapp

import "golang.org/x/sys/windows"

func moveCompatibilityFile(from, to string) error {
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// No MOVEFILE_REPLACE_EXISTING: concurrent replacements must be preserved.
	return windows.MoveFileEx(source, target, windows.MOVEFILE_WRITE_THROUGH)
}
