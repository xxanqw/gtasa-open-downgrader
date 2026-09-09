//go:build windows

package multiplayer

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	frPrivate       = 0x10
	frNotEnum       = 0x20
	hwndBroadcast   = 0xffff
	wmFontChange    = 0x001d
	smtoAbortIfHung = 0x0002
)

var (
	removeFontResourceW   = windows.NewLazySystemDLL("gdi32.dll").NewProc("RemoveFontResourceW")
	removeFontResourceExW = windows.NewLazySystemDLL("gdi32.dll").NewProc("RemoveFontResourceExW")
	sendMessageTimeoutW   = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
)

func releaseSAMPFonts(paths []string) {
	changed := false
	for _, path := range paths {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			continue
		}
		for _, flags := range []uintptr{frPrivate, frPrivate | frNotEnum, frNotEnum, 0} {
			for attempts := 0; attempts < 32; attempts++ {
				removed, _, _ := removeFontResourceExW.Call(uintptr(unsafe.Pointer(name)), flags, 0)
				if removed == 0 {
					break
				}
				changed = true
			}
		}
		for attempts := 0; attempts < 32; attempts++ {
			removed, _, _ := removeFontResourceW.Call(uintptr(unsafe.Pointer(name)))
			if removed == 0 {
				break
			}
			changed = true
		}
	}
	if !changed {
		return
	}
	var result uintptr
	_, _, _ = sendMessageTimeoutW.Call(hwndBroadcast, wmFontChange, 0, 0, smtoAbortIfHung, 1000, uintptr(unsafe.Pointer(&result)))
}

func scheduleRemoval(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(name, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}
