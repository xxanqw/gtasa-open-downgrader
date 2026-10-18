//go:build windows

package winedrive

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Wine's mount-manager extensions are the same API used by winecfg, not a
// namespace-only SUBST (which can report success without a usable Wine drive).
// ABI: wine/include/ddk/mountmgr.h, struct mountmgr_unix_drive (32 bytes).
const (
	queryUnixDrive    = 0x6d4084
	defineUnixDrive   = 0x6dc080
	defineShellFolder = 0x6dc088
)

var wineVersion = windows.NewLazySystemDLL("ntdll.dll").NewProc("wine_get_version")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var wineUnixName = kernel32.NewProc("wine_get_unix_file_name")
var processHeap = kernel32.NewProc("GetProcessHeap")
var heapFree = kernel32.NewProc("HeapFree")
var ansiLength = kernel32.NewProc("lstrlenA")
var ansiCopy = kernel32.NewProc("lstrcpynA")

func RunningUnderWine() bool { return wineVersion.Find() == nil }

type wineDrive struct {
	letter        byte
	kind          uint32
	mount, device string
	present       bool
}

func mountManager() (windows.Handle, error) {
	name, _ := windows.UTF16PtrFromString(`\\.\MountPointManager`)
	return windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}
func ioctl(h windows.Handle, code uint32, input, output []byte) error {
	var in, out *byte
	if len(input) > 0 {
		in = &input[0]
	}
	if len(output) > 0 {
		out = &output[0]
	}
	var n uint32
	return windows.DeviceIoControl(h, code, in, uint32(len(input)), out, uint32(len(output)), &n, nil)
}
func queryDrive(h windows.Handle, letter byte) (wineDrive, error) {
	d := wineDrive{letter: letter}
	in := make([]byte, 32)
	queryLetter := letter
	if queryLetter >= 'A' && queryLetter <= 'Z' {
		queryLetter += 'a' - 'A'
	}
	binary.LittleEndian.PutUint16(in[24:], uint16(queryLetter))
	out := make([]byte, 32768)
	if err := ioctl(h, queryUnixDrive, in, out); err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) || errors.Is(err, windows.ERROR_INVALID_DRIVE) || errors.Is(err, windows.ERROR_NO_SUCH_DEVICE) {
			return d, nil
		}
		return d, fmt.Errorf("query Wine drive %c: %w", letter, err)
	}
	size := int(binary.LittleEndian.Uint32(out))
	if size < 32 || size > len(out) {
		return d, fmt.Errorf("unsupported Wine drive-query response")
	}
	text := func(at int) (string, error) {
		offset := int(binary.LittleEndian.Uint16(out[at:]))
		if offset == 0 {
			return "", nil
		}
		if offset < 32 || offset >= size {
			return "", fmt.Errorf("invalid Wine drive offset")
		}
		s := out[offset:size]
		for i, c := range s {
			if c == 0 {
				return string(s[:i]), nil
			}
		}
		return "", fmt.Errorf("unterminated Wine drive path")
	}
	var err error
	d.mount, err = text(26)
	if err != nil {
		return d, err
	}
	d.device, err = text(28)
	if err != nil {
		return d, err
	}
	d.kind = binary.LittleEndian.Uint32(out[4:])
	d.present = true
	return d, nil
}
func unixPath(h windows.Handle, winpath string) (string, error) {
	absolute, err := filepath.Abs(winpath)
	if err != nil {
		return "", err
	}
	if len(absolute) < 3 || absolute[1] != ':' || absolute[2] != '\\' {
		return "", fmt.Errorf("Wine path must have a mapped drive letter")
	}
	// Wine exposes relative prefix links such as c: -> ../drive_c. Resolve
	// through its own path converter, not WINEPREFIX guesses or a required Z:.
	if err := wineUnixName.Find(); err != nil {
		return "", fmt.Errorf("Wine path-conversion API is unavailable: %w", err)
	}
	name, err := windows.UTF16PtrFromString(absolute[:3])
	if err != nil {
		return "", err
	}
	pointer, _, callErr := wineUnixName.Call(uintptr(unsafe.Pointer(name)))
	if pointer == 0 {
		return "", fmt.Errorf("resolve Wine directory: %w", callErr)
	}
	heap, _, _ := processHeap.Call()
	defer heapFree.Call(heap, 0, pointer)
	length, _, _ := ansiLength.Call(pointer)
	if length == 0 || length > 128<<10 {
		return "", fmt.Errorf("Wine Unix path exceeds its bounds")
	}
	buffer := make([]byte, length+1)
	copied, _, copyErr := ansiCopy.Call(uintptr(unsafe.Pointer(&buffer[0])), pointer, length+1)
	if copied == 0 {
		return "", fmt.Errorf("copy Wine Unix path: %w", copyErr)
	}
	resolved := string(buffer[:length])
	if !strings.HasPrefix(resolved, "/") {
		return "", fmt.Errorf("Wine path converter did not return an absolute Unix path")
	}
	return path.Join(resolved, strings.ReplaceAll(absolute[3:], `\`, "/")), nil
}
func createGameLink(target, link string) error {
	if !RunningUnderWine() {
		return fmt.Errorf("filtered game drive is only needed under Wine")
	}
	h, err := mountManager()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	unix, err := unixPath(h, target)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(link)
	if err != nil {
		return err
	}
	nt, err := windows.UTF16FromString(`\??\` + absolute)
	if err != nil {
		return err
	}
	nt = nt[:len(nt)-1]
	folder := unsafe.Slice((*byte)(unsafe.Pointer(&nt[0])), len(nt)*2)
	data := make([]byte, 16+len(folder)+len(unix)+1)
	binary.LittleEndian.PutUint32(data[4:], 16)
	binary.LittleEndian.PutUint32(data[8:], uint32(len(folder)))
	binary.LittleEndian.PutUint32(data[12:], uint32(16+len(folder)))
	copy(data[16:], folder)
	copy(data[16+len(folder):], unix)
	// No create_backup and no replacement: the common layer requires absence.
	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filtered GTA link already exists")
	}
	if exists, e := prefixEntryExists(link); e != nil {
		return e
	} else if exists {
		return fmt.Errorf("filtered GTA link already exists (possibly a dangling Unix link)")
	}
	return ioctl(h, defineShellFolder, data, nil)
}

// Wine's Win32 directory enumeration can omit dangling Unix symlinks. An
// exclusive, delete-on-close probe distinguishes those from truly absent names
// without unlinking an existing entry or leaving a persistent reservation.
func prefixEntryExists(name string) (bool, error) {
	pointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false, err
	}
	file, err := windows.CreateFile(pointer, windows.GENERIC_READ|windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err == nil {
		return false, windows.CloseHandle(file)
	}
	if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return true, nil
	}
	return false, err
}
func chooseDrive(h windows.Handle, game, view string) (wineDrive, error) {
	viewUnix, err := unixPath(h, view)
	if err != nil {
		return wineDrive{}, err
	}
	gameUnix, err := unixPath(h, game)
	if err != nil {
		return wineDrive{}, err
	}
	cRoot, err := unixPath(h, wineDriveC())
	if err != nil {
		return wineDrive{}, err
	}
	deviceRoot := `\\?\unix` + path.Join(path.Dir(cRoot), "dosdevices")
	entries, err := os.ReadDir(deviceRoot)
	if err != nil {
		return wineDrive{}, fmt.Errorf("inspect current Wine prefix drive reservations: %w", err)
	}
	reserved := map[string]bool{}
	for _, entry := range entries {
		reserved[strings.ToLower(entry.Name())] = true
	}
	var free *wineDrive
	for letter := byte('G'); letter <= 'Y'; letter++ {
		d, err := queryDrive(h, letter)
		if err != nil {
			return d, err
		}
		name := strings.ToLower(string(letter)) + ":"
		for _, entry := range []string{name, name + ":"} {
			if !reserved[entry] {
				exists, e := prefixEntryExists(deviceRoot + "/" + entry)
				if e != nil {
					return d, fmt.Errorf("inspect Wine drive reservation %s: %w", entry, e)
				}
				reserved[entry] = exists
			}
		}
		if reserved[name+":"] {
			continue
		}
		diskRoot := deviceRoot + "/" + name
		ownedDisk := SameDirectory(diskRoot, view) || SameDirectory(diskRoot, game) || SameDirectory(diskRoot, filepath.Dir(game))
		if reserved[name] && !ownedDisk {
			continue
		} // Includes broken links, absent from mountmgr's live table.
		if ownedDisk && d.device == "" {
			return d, nil
		}
		if d.present {
			if d.device != "" {
				continue
			} // Physical reservations can overwrite directory links at Wine startup.
			root := string(letter) + `:\`
			// Recognize only this view, or the exact old game/parent layout. Never
			// steal a different game, user drive, removable mount or broken mapping.
			if SameDirectory(root, view) || SameDirectory(root, game) || SameDirectory(root, filepath.Dir(game)) || d.mount == viewUnix || d.mount == gameUnix || d.mount == path.Dir(gameUnix) {
				return d, nil
			}
			continue
		}
		if free == nil {
			copy := d
			free = &copy
		}
	}
	if free == nil {
		return wineDrive{}, fmt.Errorf("no free Wine drive letter G: through Y:; existing drives were retained")
	}
	return *free, nil
}
func wineDriveC() string {
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	return drive + `\`
}
func PlanWineGameDrive(game string) (string, error) {
	if !RunningUnderWine() {
		return game, nil
	}
	if err := RequireWineGameClosed(); err != nil {
		return "", err
	}
	h, err := mountManager()
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	d, err := chooseDrive(h, game, ViewPath(wineDriveC()))
	if err != nil {
		return "", err
	}
	return GamePath(d.letter), nil
}
func EnsureWineGameDrive(game string) (string, error) {
	if !RunningUnderWine() {
		return game, nil
	}
	if err := RequireWineGameClosed(); err != nil {
		return "", err
	}
	view, err := EnsureView(wineDriveC(), game)
	if err != nil {
		return "", err
	}
	h, err := mountManager()
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	d, err := chooseDrive(h, game, view)
	if err != nil {
		return "", err
	}
	result := GamePath(d.letter)
	// Reapply through mountmgr even if the on-disk link is present: active Wine
	// sessions cache devices. A fixed directory mapping works outside Steam too.
	mount, err := unixPath(h, view)
	if err != nil {
		return "", err
	}
	current, err := queryDrive(h, d.letter)
	if err != nil {
		return "", err
	}
	if current != d {
		return "", fmt.Errorf("Wine drive changed during preparation; retry without changing winecfg")
	}
	data := make([]byte, 32+len(mount)+1)
	binary.LittleEndian.PutUint32(data, uint32(len(data)))
	binary.LittleEndian.PutUint32(data[4:], windows.DRIVE_FIXED)
	binary.LittleEndian.PutUint16(data[24:], uint16(d.letter))
	binary.LittleEndian.PutUint16(data[26:], 32)
	copy(data[32:], mount)
	// Recheck physical reservations just before activation, including raw
	// entries not exposed in Wine's cached table. Never set labels or serials.
	cRoot, err := unixPath(h, wineDriveC())
	if err != nil {
		return "", err
	}
	rawName := `\\?\unix` + path.Join(path.Dir(cRoot), "dosdevices", strings.ToLower(string(d.letter))+"::")
	rawExists, err := prefixEntryExists(rawName)
	if err != nil {
		return "", err
	}
	if d.device != "" || rawExists {
		return "", fmt.Errorf("a physical Wine drive reservation appeared; it was retained")
	}

	if err = ioctl(h, defineUnixDrive, data, nil); err != nil {
		return "", fmt.Errorf("activate Wine game drive: %w", err)
	}
	if !SameDirectory(result, game) {
		return "", fmt.Errorf("Wine game drive did not resolve to the selected GTA folder")
	}
	return result, nil
}
func RequireWineGameClosed() error {
	if !RunningUnderWine() {
		return nil
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("inspect Wine game processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		if entry.ProcessID != uint32(os.Getpid()) {
			name := windows.UTF16ToString(entry.ExeFile[:])
			if strings.EqualFold(name, "gta_sa.exe") || strings.EqualFold(name, "gta-sa.exe") {
				return fmt.Errorf("close other GTA/launcher processes before preparing the Wine game drive")
			}
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		err = windows.Process32Next(snapshot, &entry)
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return fmt.Errorf("enumerate Wine game processes: %w", err)
}
