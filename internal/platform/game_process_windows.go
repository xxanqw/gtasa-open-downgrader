package platform

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
	"unsafe"
)

func RequireGameClosed() error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("check GTA processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		if name == "gta_sa.exe" || name == "gta-sa.exe" {
			return fmt.Errorf("close GTA (%s) before changing installed mods", name)
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("check GTA processes: %w", err)
	}
	return nil
}
