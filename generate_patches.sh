#!/bin/bash

set -e

STEAM_DIR="SA_STEAM"
GERMAN_DIR="SA_STEAM_GERMAN"
V10_DIR="SA_10US"
PATCHES_DIR="Patches"
VARIANT_DIR="$PATCHES_DIR/variants/steam_r2_german"
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
(cd "$STEAM_DIR" && find . -type f | sed 's#^./##' | grep -ivE '^readme(/|$|\.)' | sort) > "$TEMP_DIR/steam_files.txt"
(cd "$V10_DIR" && find . -type f | sed 's#^./##' | grep -ivE '^readme(/|$|\.)' | sort) > "$TEMP_DIR/v10_files.txt"

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

echo -e "${YELLOW}Step 3: Comparing Global Steam files...${NC}"
> "$TEMP_DIR/manifest_data.txt"

GLOBAL_EXE_HASH=""
TARGET_EXE_HASH=""
FOUND_EXE=false

for exe_name in "gta-sa.exe" "gta_sa.exe"; do
    if [ -f "$STEAM_DIR/$exe_name" ]; then
        md5_steam=$(md5sum "$STEAM_DIR/$exe_name" | awk '{print $1}')
        md5_v10=$(md5sum "$V10_DIR/gta_sa.exe" | awk '{print $1}')
        GLOBAL_EXE_HASH="$md5_steam"
        TARGET_EXE_HASH="$md5_v10"
        if [ "$md5_steam" != "$md5_v10" ]; then
            echo "$exe_name|$md5_steam|$md5_v10|copy" >> "$TEMP_DIR/manifest_data.txt"
            FOUND_EXE=true
        fi
    fi
done

if [ "$FOUND_EXE" = false ] && [ -f "$V10_DIR/gta_sa.exe" ]; then
    md5_v10=$(md5sum "$V10_DIR/gta_sa.exe" | awk '{print $1}')
    TARGET_EXE_HASH="$md5_v10"
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

if [ $DIFFERENT -eq 0 ] && [ ! -d "$GERMAN_DIR" ]; then
    echo -e "${GREEN}No differences found! Directories are identical.${NC}"
    rm -rf "$TEMP_DIR"
    exit 0
fi

echo -e "${YELLOW}Step 4: Processing Global differences...${NC}"
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
        
        echo -e "\r\033[K  [$PATCH_NUM/$DIFFERENT] ${GREEN}✓${NC} $rel_path (Direct Copy)"
    else
        dst_file="$V10_DIR/$rel_path"
        patch_file="$PATCHES_DIR/$rel_path.xdelta"
        mkdir -p "$(dirname "$patch_file")"

        if "$XDELTA_BIN" -e -9 -f -s "$src_file" "$dst_file" "$patch_file" 2>/dev/null; then
            size_orig=$(stat -c%s "$src_file" 2>/dev/null || echo 0)
            size_patch=$(stat -c%s "$patch_file" 2>/dev/null || echo 0)
            TOTAL_SIZE_ORIGINAL=$((TOTAL_SIZE_ORIGINAL + size_orig))
            TOTAL_SIZE_PATCHES=$((TOTAL_SIZE_PATCHES + size_patch))
            echo -e "\r\033[K  [$PATCH_NUM/$DIFFERENT] ${GREEN}✓${NC} $rel_path"
        else
            echo -e "\r\033[K  [$PATCH_NUM/$DIFFERENT] ${RED}✗${NC} $rel_path"
            FAILED=$((FAILED + 1))
            rm -f "$patch_file"
        fi
    fi
done < "$TEMP_DIR/manifest_data.txt"

GERMAN_EXE_HASH=""
GERMAN_VARIANTS_COUNT=0
> "$TEMP_DIR/german_variants.txt"

if [ -d "$GERMAN_DIR" ]; then
    echo ""
    echo -e "${YELLOW}Step 5: Processing German variants...${NC}"
    
    for exe_name in "gta-sa.exe" "gta_sa.exe"; do
        if [ -f "$GERMAN_DIR/$exe_name" ]; then
            GERMAN_EXE_HASH=$(md5sum "$GERMAN_DIR/$exe_name" | awk '{print $1}')
            break
        fi
    done

    (cd "$GERMAN_DIR" && find . -type f | sed 's#^./##' | grep -ivE '^readme(/|$|\.)|\.json$|^gta[-_]sa\.exe$' | sort) > "$TEMP_DIR/german_files.txt"
    GERMAN_FILE_COUNT=$(wc -l < "$TEMP_DIR/german_files.txt" || echo 0)
    echo -e "  German files to inspect: ${GREEN}$GERMAN_FILE_COUNT${NC}"

    G_NUM=0
    while IFS= read -r rel_path; do
        if [ -z "$rel_path" ]; then continue; fi
        G_NUM=$((G_NUM + 1))
        src_file="$GERMAN_DIR/$rel_path"
        dst_file="$V10_DIR/$rel_path"
        patch_file="$VARIANT_DIR/$rel_path.xdelta"

        if [ ! -f "$dst_file" ]; then
            echo -e "  [German $G_NUM/$GERMAN_FILE_COUNT] ${RED}✗${NC} $rel_path (Missing in $V10_DIR)"
            continue
        fi

        german_hash=$(md5sum "$src_file" 2>/dev/null | awk '{print $1}')
        target_hash=$(md5sum "$dst_file" 2>/dev/null | awk '{print $1}')

        # 1. German file is already target 1.0 US -> no delta required
        if [ "$german_hash" == "$target_hash" ]; then
            continue
        fi

        # 2. Check if German hash is already supported by Global source
        global_file="$STEAM_DIR/$rel_path"
        global_hash=""
        if [ -f "$global_file" ]; then
            global_hash=$(md5sum "$global_file" 2>/dev/null | awk '{print $1}')
        fi

        if [ -n "$global_hash" ] && [ "$german_hash" == "$global_hash" ]; then
            continue
        fi

        echo -ne "  [German $G_NUM/$GERMAN_FILE_COUNT] Processing variant: $rel_path"
        mkdir -p "$(dirname "$patch_file")"

        if "$XDELTA_BIN" -e -9 -f -s "$src_file" "$dst_file" "$patch_file" 2>/dev/null; then
            size_orig=$(stat -c%s "$src_file" 2>/dev/null || echo 0)
            size_patch=$(stat -c%s "$patch_file" 2>/dev/null || echo 0)
            TOTAL_SIZE_ORIGINAL=$((TOTAL_SIZE_ORIGINAL + size_orig))
            TOTAL_SIZE_PATCHES=$((TOTAL_SIZE_PATCHES + size_patch))
            echo -e "\r\033[K  [German $G_NUM/$GERMAN_FILE_COUNT] ${GREEN}✓${NC} $rel_path"
            echo "$rel_path|$german_hash|$target_hash|$global_hash|variants/steam_r2_german/$rel_path.xdelta" >> "$TEMP_DIR/german_variants.txt"
            GERMAN_VARIANTS_COUNT=$((GERMAN_VARIANTS_COUNT + 1))
        else
            echo -e "\r\033[K  [German $G_NUM/$GERMAN_FILE_COUNT] ${RED}✗${NC} $rel_path"
            FAILED=$((FAILED + 1))
            rm -f "$patch_file"
        fi
    done < "$TEMP_DIR/german_files.txt"
    echo -e "  German variants generated: ${GREEN}$GERMAN_VARIANTS_COUNT${NC}"
fi

echo ""
echo -e "${YELLOW}Step 6: Generating manifest.json (v2.0)...${NC}"

PYTHON_CMD="python3"
if command -v uv &> /dev/null; then
    PYTHON_CMD="uv run python"
fi

$PYTHON_CMD - << 'EOF' "$TEMP_DIR" "$PATCHES_DIR" "$COMMON_COUNT" "$IDENTICAL" "$FAILED" "$TOTAL_SIZE_ORIGINAL" "$TOTAL_SIZE_PATCHES" "$GLOBAL_EXE_HASH" "$GERMAN_EXE_HASH" "$TARGET_EXE_HASH"
import sys
import os
import json
from datetime import datetime, timezone

temp_dir = sys.argv[1]
patches_dir = sys.argv[2]
common_count = int(sys.argv[3])
identical_count = int(sys.argv[4])
failed_count = int(sys.argv[5])
total_size_orig = int(sys.argv[6])
total_size_patches = int(sys.argv[7])
global_exe_hash = sys.argv[8] if len(sys.argv) > 8 else ""
german_exe_hash = sys.argv[9] if len(sys.argv) > 9 else ""
target_exe_hash = sys.argv[10] if len(sys.argv) > 10 else ""

source_versions = []
if global_exe_hash:
    source_versions.append({
        "id": "steam_r2_global",
        "name": "NewSteam R2",
        "exe_hash": global_exe_hash
    })
if german_exe_hash:
    source_versions.append({
        "id": "steam_r2_german",
        "name": "NewSteam R2 (German)",
        "exe_hash": german_exe_hash
    })

manifest_entries = {}
manifest_data_file = os.path.join(temp_dir, "manifest_data.txt")
if os.path.exists(manifest_data_file):
    with open(manifest_data_file, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rel_path, steam_hash, v10_hash, action = line.split("|")
            if action == "copy":
                if os.path.isfile(os.path.join(patches_dir, "gta_sa.exe")):
                    sources = []
                    if steam_hash and steam_hash != "MISSING":
                        sources.append({"hash": steam_hash})
                    if german_exe_hash and german_exe_hash != steam_hash and german_exe_hash != v10_hash:
                        sources.append({"hash": german_exe_hash})
                    manifest_entries[rel_path] = {
                        "path": rel_path,
                        "action": "copy",
                        "source_hash": steam_hash,
                        "target_hash": v10_hash,
                        "payload": "gta_sa.exe",
                        "sources": sources
                    }
            elif action == "patch":
                patch_file = os.path.join(patches_dir, f"{rel_path}.xdelta")
                if os.path.isfile(patch_file):
                    manifest_entries[rel_path] = {
                        "path": rel_path,
                        "action": "patch",
                        "source_hash": steam_hash,
                        "target_hash": v10_hash,
                        "sources": [
                            {
                                "hash": steam_hash,
                                "patch": f"{rel_path}.xdelta"
                            }
                        ]
                    }

german_variants_file = os.path.join(temp_dir, "german_variants.txt")
if os.path.exists(german_variants_file):
    with open(german_variants_file, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rel_path, german_hash, target_hash, global_hash, patch_rel = line.split("|")
            patch_full = os.path.join(patches_dir, patch_rel)
            if not os.path.isfile(patch_full):
                continue
            
            if rel_path in manifest_entries:
                entry = manifest_entries[rel_path]
                if not any(s.get("hash") == german_hash for s in entry["sources"]):
                    entry["sources"].append({
                        "hash": german_hash,
                        "patch": patch_rel
                    })
            else:
                source_hash = global_hash if global_hash else target_hash
                manifest_entries[rel_path] = {
                    "path": rel_path,
                    "action": "patch",
                    "source_hash": source_hash,
                    "target_hash": target_hash,
                    "sources": [
                        {
                            "hash": german_hash,
                            "patch": patch_rel
                        }
                    ]
                }

file_list = list(manifest_entries.values())
file_list.sort(key=lambda x: (0 if x["path"] in ("gta-sa.exe", "gta_sa.exe") else 1, x["path"]))

total_patches = sum(len([s for s in f.get("sources", []) if "patch" in s]) for f in file_list)

manifest = {
    "version": "2.0",
    "generated": datetime.now(timezone.utc).isoformat(),
    "source_version": "steam",
    "target_version": "1.0_us",
    "source_versions": source_versions,
    "statistics": {
        "total_files": common_count,
        "identical": identical_count,
        "different": len(file_list),
        "patches_generated": total_patches,
        "failed": failed_count,
        "original_size_mb": round(total_size_orig / 1048576, 2),
        "patches_size_mb": round(total_size_patches / 1048576, 2)
    },
    "files": file_list
}

with open(os.path.join(patches_dir, "manifest.json"), "w") as f:
    json.dump(manifest, f, indent=2)
EOF

echo -e "  ${GREEN}✓${NC} $PATCHES_DIR/manifest.json"
echo ""

rm -rf "$TEMP_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Patches generated: ${GREEN}$((DIFFERENT - FAILED + GERMAN_VARIANTS_COUNT))${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "Failed patches:    ${RED}$FAILED${NC}"
fi
echo -e "Output directory:  ${GREEN}$PATCHES_DIR${NC}"
echo ""

ORIG_MB=$(awk "BEGIN {printf \"%.2f\", $TOTAL_SIZE_ORIGINAL / 1048576}")
PATCH_MB=$(awk "BEGIN {printf \"%.2f\", $TOTAL_SIZE_PATCHES / 1048576}")
if [ "$TOTAL_SIZE_ORIGINAL" -gt 0 ]; then
    COMPRESSION=$(awk "BEGIN {printf \"%.1f\", 100 * $TOTAL_SIZE_PATCHES / $TOTAL_SIZE_ORIGINAL}")
else
    COMPRESSION=0
fi

echo -e "Original files size: ${YELLOW}${ORIG_MB} MB${NC}"
echo -e "Patches total size:  ${GREEN}${PATCH_MB} MB${NC}"
echo -e "Compression ratio:   ${GREEN}${COMPRESSION}%${NC}"
echo ""

echo -e "${GREEN}Done!${NC}"