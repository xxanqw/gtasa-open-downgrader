package launcherapp

import "golang.org/x/sys/unix"

// Do not clobber a concurrent user replacement, even between preflight and rename.
func moveCompatibilityFile(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}
