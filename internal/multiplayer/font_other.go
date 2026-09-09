//go:build !windows

package multiplayer

import "fmt"

func releaseSAMPFonts([]string) {}

func scheduleRemoval(path string) error {
	return fmt.Errorf("cannot defer removal of %s on this platform", path)
}
