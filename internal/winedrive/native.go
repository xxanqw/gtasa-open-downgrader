//go:build !windows

package winedrive

import "os"

func createGameLink(target, link string) error   { return os.Symlink(target, link) }
func RunningUnderWine() bool                     { return false }
func EnsureWineGameDrive(string) (string, error) { return "", nil }
