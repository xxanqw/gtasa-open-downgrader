package multiplayer

import (
	"encoding/binary"
	"fmt"
	"os"
)

// MTAExecutableFamily reads only bounded PE VERSIONINFO resources. MTA launchers
// contain both updater build versions (e.g. 1.26812) and the user-facing 1.7
// family in another language resource; a neutral updater revision is not 1.7.
// This identifies metadata, not authenticity or an installed core build number.
func MTAExecutableFamily(path string) (string, error) {
	before, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Size() > 64<<20 {
		return "", fmt.Errorf("MTA executable metadata exceeds its limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 || !os.SameFile(before, info) {
		return "", fmt.Errorf("MTA executable metadata exceeds its limit")
	}
	// Read only fixed bounded headers. In particular, do not load a potentially
	// enormous COFF symbol/string table while inspecting a user-owned executable.
	read := func(offset int64, size int) ([]byte, error) {
		if offset < 0 || offset+int64(size) > info.Size() {
			return nil, fmt.Errorf("MTA PE metadata is truncated")
		}
		data := make([]byte, size)
		_, err := file.ReadAt(data, offset)
		return data, err
	}
	dos, err := read(0, 64)
	if err != nil {
		return "", err
	}
	if string(dos[:2]) != "MZ" {
		return "", fmt.Errorf("MTA executable is not PE")
	}
	peOffset := int64(binary.LittleEndian.Uint32(dos[60:64]))
	header, err := read(peOffset, 24)
	if err != nil {
		return "", err
	}
	if string(header[:4]) != "PE\x00\x00" {
		return "", fmt.Errorf("MTA PE signature is invalid")
	}
	count := int(binary.LittleEndian.Uint16(header[6:8]))
	optionalSize := int(binary.LittleEndian.Uint16(header[20:22]))
	if count < 1 || count > 96 || optionalSize < 128 || optionalSize > 4096 {
		return "", fmt.Errorf("MTA PE header exceeds its bounds")
	}
	optional, err := read(peOffset+24, optionalSize)
	if err != nil {
		return "", err
	}
	directory := 96
	switch binary.LittleEndian.Uint16(optional[:2]) {
	case 0x10b:
	case 0x20b:
		directory = 112
	default:
		return "", fmt.Errorf("unsupported MTA PE format")
	}
	if directory+24 > len(optional) || binary.LittleEndian.Uint32(optional[directory-4:directory]) < 3 {
		return "", fmt.Errorf("MTA resource directory is missing")
	}
	rva := binary.LittleEndian.Uint32(optional[directory+16 : directory+20])
	if rva == 0 {
		return "", fmt.Errorf("MTA version resource is absent")
	}
	table, err := read(peOffset+24+int64(optionalSize), count*40)
	if err != nil {
		return "", err
	}
	type peSection struct{ VirtualAddress, Size, Offset uint32 }
	sections := make([]peSection, 0, count)
	for i := 0; i < count; i++ {
		entry := table[i*40 : (i+1)*40]
		sections = append(sections, peSection{binary.LittleEndian.Uint32(entry[12:16]), binary.LittleEndian.Uint32(entry[16:20]), binary.LittleEndian.Uint32(entry[20:24])})
	}
	readRVA := func(address, size uint32) ([]byte, error) {
		if size == 0 || size > 4<<20 {
			return nil, fmt.Errorf("MTA resource size is invalid")
		}
		for _, section := range sections {
			if uint64(address) >= uint64(section.VirtualAddress) && uint64(address)+uint64(size) <= uint64(section.VirtualAddress)+uint64(section.Size) {
				return read(int64(section.Offset)+int64(address-section.VirtualAddress), int(size))
			}
		}
		return nil, fmt.Errorf("MTA resource address is invalid")
	}
	// Only directory tables are needed here. Their offsets are relative to the
	// resource root; never trust offsets into unrelated sections or large blobs.
	var root []byte
	for _, section := range sections {
		if uint64(rva) >= uint64(section.VirtualAddress) && uint64(rva) < uint64(section.VirtualAddress)+uint64(section.Size) {
			size := uint64(section.VirtualAddress) + uint64(section.Size) - uint64(rva)
			if size > 4<<20 {
				return "", fmt.Errorf("MTA resource directory exceeds its limit")
			}
			root, err = readRVA(rva, uint32(size))
			break
		}
	}
	if err != nil {
		return "", err
	}
	if len(root) < 16 {
		return "", fmt.Errorf("MTA resource directory is missing")
	}
	u16 := func(data []byte, offset int) uint16 { return binary.LittleEndian.Uint16(data[offset : offset+2]) }
	u32 := func(data []byte, offset int) uint32 { return binary.LittleEndian.Uint32(data[offset : offset+4]) }
	family := ""
	resources, directories := 0, 0
	var visit func(uint32, int) error
	visit = func(offset uint32, depth int) error {
		directories++
		if directories > 512 {
			return fmt.Errorf("too many MTA resource directories")
		}
		if depth > 2 || uint64(offset)+16 > uint64(len(root)) {
			return fmt.Errorf("invalid MTA resource tree")
		}
		base := int(offset)
		count := int(u16(root, base+12)) + int(u16(root, base+14))
		if count > 128 || base+16+count*8 > len(root) {
			return fmt.Errorf("invalid MTA resource entries")
		}
		for i := 0; i < count; i++ {
			position := base + 16 + i*8
			name, next := u32(root, position), u32(root, position+4)
			if depth == 0 && name != 16 {
				continue
			}
			if next&0x80000000 != 0 {
				if err := visit(next&0x7fffffff, depth+1); err != nil {
					return err
				}
				continue
			}
			if depth != 2 || uint64(next)+16 > uint64(len(root)) {
				return fmt.Errorf("invalid MTA version leaf")
			}
			resources++
			if resources > 32 {
				return fmt.Errorf("too many MTA version resources")
			}
			size := u32(root, int(next)+4)
			if size > 64<<10 {
				return fmt.Errorf("MTA VERSIONINFO exceeds its limit")
			}
			data, err := readRVA(u32(root, int(next)), size)
			if err != nil {
				return err
			}
			if len(data) < 6 {
				continue
			}
			length := int(u16(data, 0))
			if length > len(data) || length < 6 {
				continue
			}
			data = data[:length]
			key := "VS_VERSION_INFO"
			position = 6
			valid := true
			for _, character := range key {
				if position+2 > len(data) || u16(data, position) != uint16(character) {
					valid = false
					break
				}
				position += 2
			}
			if !valid || position+2 > len(data) || u16(data, position) != 0 {
				continue
			}
			position = (position + 2 + 3) &^ 3
			if u16(data, 2) < 52 || position+52 > len(data) || u32(data, position) != 0xfeef04bd {
				continue
			}
			version := u32(data, position+8)
			major, minor := version>>16, version&0xffff
			if major == 1 && (minor == 5 || minor == 6 || minor == 7) {
				value := fmt.Sprintf("1.%d", minor)
				if family != "" && family != value {
					return fmt.Errorf("MTA version resources disagree")
				}
				family = value
			}
		}
		return nil
	}
	if err = visit(0, 0); err != nil {
		return "", err
	}
	if family == "" {
		return "", fmt.Errorf("MTA client family was not found in VERSIONINFO")
	}
	return family, nil
}
