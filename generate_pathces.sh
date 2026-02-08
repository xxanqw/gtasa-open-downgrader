#!/bin/bash

set -e

STEAM_DIR="SA_STEAM"
V10_DIR="SA_10US"
PATCHES_DIR="Patches"
TEMP_DIR=".temp_patch_gen"
XDELTA_BIN="downgrader/bin/xdelta3_linux"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

if [ ! -f "$XDELTA_BIN" ]; then
    if ! command -v xdelta3 &> /dev/null; then
        echo -e "${RED}Error: xdelta3 is not installed or not found at $XDELTA_BIN${NC}"
        echo "Install it with: sudo apt install xdelta3"
        exit 1
    else
        XDELTA_BIN="xdelta3"
    fi
fi
chmod +x "$XDELTA_BIN" 2>/dev/null || true

if [ ! -d "$STEAM_DIR" ]; then
    echo -e "${RED}Error: Steam directory '$STEAM_DIR' not found${NC}"
    exit 1
fi

if [ ! -d "$V10_DIR" ]; then
    echo -e "${RED}Error: v1.0 US directory '$V10_DIR' not found${NC}"
    exit 1
fi

mkdir -p "$PATCHES_DIR"
mkdir -p "$TEMP_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GTA SA Patch Generator${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}Step 1: Scanning directories...${NC}"
(cd "$STEAM_DIR" && find . -type f | sed 's#^./##' | sort) > "$TEMP_DIR/steam_files.txt"
(cd "$V10_DIR" && find . -type f | sed 's#^./##' | sort) > "$TEMP_DIR/v10_files.txt"

STEAM_COUNT=$(wc -l < "$TEMP_DIR/steam_files.txt")
V10_COUNT=$(wc -l < "$TEMP_DIR/v10_files.txt")

echo -e "  Steam files: ${GREEN}$STEAM_COUNT${NC}"
echo -e "  v1.0 files:  ${GREEN}$V10_COUNT${NC}"
echo ""

echo -e "${YELLOW}Step 2: Finding common files...${NC}"
comm -12 "$TEMP_DIR/steam_files.txt" "$TEMP_DIR/v10_files.txt" > "$TEMP_DIR/common_files.txt"
COMMON_COUNT=$(wc -l < "$TEMP_DIR/common_files.txt")
echo -e "  Common files: ${GREEN}$COMMON_COUNT${NC}"
echo ""

echo -e "${YELLOW}Step 3: Comparing file hashes...${NC}"
> "$TEMP_DIR/manifest_data.txt"

FOUND_EXE=false
for exe_name in "gta-sa.exe" "gta_sa.exe"; do
    if [ -f "$STEAM_DIR/$exe_name" ]; then
        md5_steam=$(md5sum "$STEAM_DIR/$exe_name" | awk '{print $1}')
        md5_v10=$(md5sum "$V10_DIR/gta_sa.exe" | awk '{print $1}')
        if [ "$md5_steam" != "$md5_v10" ]; then
            echo "$exe_name|$md5_steam|$md5_v10|copy" >> "$TEMP_DIR/manifest_data.txt"
            FOUND_EXE=true
        fi
    fi
done

if [ "$FOUND_EXE" = false ] && [ -f "$V10_DIR/gta_sa.exe" ]; then
    md5_v10=$(md5sum "$V10_DIR/gta_sa.exe" | awk '{print $1}')
    echo "gta_sa.exe|MISSING|$md5_v10|copy" >> "$TEMP_DIR/manifest_data.txt"
fi

DIFFERENT=$(wc -l < "$TEMP_DIR/manifest_data.txt" || echo 0)
IDENTICAL=0
PROGRESS=0

while IFS= read -r rel_path; do
    if [[ "$rel_path" == "gta_sa.exe" || "$rel_path" == "gta-sa.exe" ]]; then continue; fi
    
    PROGRESS=$((PROGRESS + 1))
    if [ $((PROGRESS % 100)) -eq 0 ]; then
        echo -ne "  Progress: $PROGRESS / $COMMON_COUNT\r"
    fi

    md5_steam=$(md5sum "$STEAM_DIR/$rel_path" 2>/dev/null | awk '{print $1}')
    md5_v10=$(md5sum "$V10_DIR/$rel_path" 2>/dev/null | awk '{print $1}')

    if [ "$md5_steam" != "$md5_v10" ]; then
        echo "$rel_path|$md5_steam|$md5_v10|patch" >> "$TEMP_DIR/manifest_data.txt"
        DIFFERENT=$((DIFFERENT + 1))
    else
        IDENTICAL=$((IDENTICAL + 1))
    fi
done < "$TEMP_DIR/common_files.txt"

echo -e "  Progress: $COMMON_COUNT / $COMMON_COUNT"
echo -e "  ${GREEN}Identical: $IDENTICAL${NC}"
echo -e "  ${YELLOW}Different: $DIFFERENT${NC}"
echo ""

if [ $DIFFERENT -eq 0 ]; then
    echo -e "${GREEN}No differences found! Directories are identical.${NC}"
    rm -rf "$TEMP_DIR"
    exit 0
fi

echo -e "${YELLOW}Step 4: Processing differences...${NC}"
echo ""

PATCH_NUM=0
FAILED=0
TOTAL_SIZE_ORIGINAL=0
TOTAL_SIZE_PATCHES=0

while IFS='|' read -r rel_path steam_hash v10_hash action; do
    PATCH_NUM=$((PATCH_NUM + 1))

    src_file="$STEAM_DIR/$rel_path"
    
    echo -ne "  [$PATCH_NUM/$DIFFERENT] Processing: $rel_path"

    if [ "$action" == "copy" ]; then
        mkdir -p "$(dirname "$PATCHES_DIR/gta_sa.exe")"
        cp "$V10_DIR/gta_sa.exe" "$PATCHES_DIR/gta_sa.exe"
        
        size_orig=$(stat -c%s "$src_file" 2>/dev/null || echo 0)
        size_patch=$(stat -c%s "$PATCHES_DIR/gta_sa.exe" 2>/dev/null || echo 0)
        TOTAL_SIZE_ORIGINAL=$((TOTAL_SIZE_ORIGINAL + size_orig))
        TOTAL_SIZE_PATCHES=$((TOTAL_SIZE_PATCHES + size_patch))
        
        echo -e "\r  [$PATCH_NUM/$DIFFERENT] ${GREEN}✓${NC} $rel_path (Direct Copy)"
    else
        dst_file="$V10_DIR/$rel_path"
        patch_file="$PATCHES_DIR/$rel_path.xdelta"
        mkdir -p "$(dirname "$patch_file")"

        if "$XDELTA_BIN" -e -9 -s "$src_file" "$dst_file" "$patch_file" 2>/dev/null; then
            size_orig=$(stat -c%s "$src_file" 2>/dev/null || echo 0)
            size_patch=$(stat -c%s "$patch_file" 2>/dev/null || echo 0)
            TOTAL_SIZE_ORIGINAL=$((TOTAL_SIZE_ORIGINAL + size_orig))
            TOTAL_SIZE_PATCHES=$((TOTAL_SIZE_PATCHES + size_patch))
            echo -e "\r  [$PATCH_NUM/$DIFFERENT] ${GREEN}✓${NC} $rel_path"
        else
            echo -e "\r  [$PATCH_NUM/$DIFFERENT] ${RED}✗${NC} $rel_path"
            FAILED=$((FAILED + 1))
            rm -f "$patch_file"
        fi
    fi
done < "$TEMP_DIR/manifest_data.txt"

echo ""

echo -e "${YELLOW}Step 5: Generating manifest.json...${NC}"

cat > "$PATCHES_DIR/manifest.json" << EOF
{
  "version": "1.0",
  "generated": "$(date -Iseconds)",
  "source_version": "steam",
  "target_version": "1.0_us",
  "statistics": {
    "total_files": $COMMON_COUNT,
    "identical": $IDENTICAL,
    "different": $DIFFERENT,
    "patches_generated": $((DIFFERENT - FAILED)),
    "failed": $FAILED,
    "original_size_mb": $(echo "scale=2; $TOTAL_SIZE_ORIGINAL / 1048576" | bc),
    "patches_size_mb": $(echo "scale=2; $TOTAL_SIZE_PATCHES / 1048576" | bc)
  },
  "files": [
EOF

first=true
while IFS='|' read -r rel_path steam_hash v10_hash action; do
    VALID=false
    if [ "$action" == "copy" ] && [ -f "$PATCHES_DIR/gta_sa.exe" ]; then VALID=true; fi
    if [ "$action" == "patch" ] && [ -f "$PATCHES_DIR/$rel_path.xdelta" ]; then VALID=true; fi

    if [ "$VALID" == "true" ]; then
        if [ "$first" = true ]; then
            first=false
        else
            echo "," >> "$PATCHES_DIR/manifest.json"
        fi
        cat >> "$PATCHES_DIR/manifest.json" << EOF
    {
      "path": "$rel_path",
      "action": "$action",
      "source_hash": "$steam_hash",
      "target_hash": "$v10_hash"
    }
EOF
    fi
done < "$TEMP_DIR/manifest_data.txt"

cat >> "$PATCHES_DIR/manifest.json" << EOF
  ]
}
EOF

echo -e "  ${GREEN}✓${NC} $PATCHES_DIR/manifest.json"
echo ""

rm -rf "$TEMP_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Patches generated: ${GREEN}$((DIFFERENT - FAILED))${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "Failed patches:    ${RED}$FAILED${NC}"
fi
echo -e "Output directory:  ${GREEN}$PATCHES_DIR${NC}"
echo ""

ORIG_MB=$(echo "scale=2; $TOTAL_SIZE_ORIGINAL / 1048576" | bc)
PATCH_MB=$(echo "scale=2; $TOTAL_SIZE_PATCHES / 1048576" | bc)
if [ "$TOTAL_SIZE_ORIGINAL" -gt 0 ]; then
    COMPRESSION=$(echo "scale=1; 100 * $TOTAL_SIZE_PATCHES / $TOTAL_SIZE_ORIGINAL" | bc)
else
    COMPRESSION=0
fi

echo -e "Original files size: ${YELLOW}${ORIG_MB} MB${NC}"
echo -e "Patches total size:  ${GREEN}${PATCH_MB} MB${NC}"
echo -e "Compression ratio:   ${GREEN}${COMPRESSION}%${NC}"
echo ""

echo -e "${GREEN}Done!${NC}"