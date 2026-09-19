$ErrorActionPreference = "Stop"

$SteamDir = "SA_STEAM"
$GermanDir = "SA_STEAM_GERMAN"
$V10Dir = "SA_10US"
$PatchesDir = "Patches"
$VariantDir = Join-Path $PatchesDir "variants\steam_r2_german"
$TempDir = ".temp_patch_gen"
$XdeltaBin = "downgrader\bin\xdelta3.exe"

if (-not (Test-Path $XdeltaBin)) {
    $XdeltaBin = "xdelta3.exe"
    try {
        xdelta3 -V | Out-Null
    } catch {
        Write-Host "Error: xdelta3 is not installed or not found in $XdeltaBin" -ForegroundColor Red
        exit 1
    }
}

if (-not (Test-Path $SteamDir)) {
    Write-Host "Error: Steam directory '$SteamDir' not found" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $V10Dir)) {
    Write-Host "Error: v1.0 US directory '$V10Dir' not found" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $PatchesDir)) { New-Item -ItemType Directory -Path $PatchesDir | Out-Null }
if (-not (Test-Path $TempDir)) { New-Item -ItemType Directory -Path $TempDir | Out-Null }

Write-Host "========================================" -ForegroundColor Blue
Write-Host "GTA SA Patch Generator (Windows)" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

Write-Host "Step 1: Scanning directories..." -ForegroundColor Yellow
$SteamFiles = Get-ChildItem -Path $SteamDir -Recurse -File | ForEach-Object { 
    $_.FullName.Substring((Get-Item $SteamDir).FullName.Length + 1) 
} | Where-Object { $_ -notmatch '^(?i)readme(/|\\|$|\.)' }
$V10Files = Get-ChildItem -Path $V10Dir -Recurse -File | ForEach-Object { 
    $_.FullName.Substring((Get-Item $V10Dir).FullName.Length + 1) 
} | Where-Object { $_ -notmatch '^(?i)readme(/|\\|$|\.)' }

Write-Host "  Steam files: $($SteamFiles.Count)" -ForegroundColor Green
Write-Host "  v1.0 files:  $($V10Files.Count)" -ForegroundColor Green
Write-Host ""

Write-Host "Step 2: Finding common files..." -ForegroundColor Yellow
$CommonFiles = Compare-Object $SteamFiles $V10Files -IncludeEqual -ExcludeDifferent | Select-Object -ExpandProperty InputObject
Write-Host "  Common files: $($CommonFiles.Count)" -ForegroundColor Green
Write-Host ""

Write-Host "Step 3: Comparing Global Steam files..." -ForegroundColor Yellow
$ManifestData = New-Object System.Collections.Generic.List[PSObject]

function Get-FileHashMD5($path) {
    if (-not (Test-Path $path)) { return $null }
    return (Get-FileHash -Path $path -Algorithm MD5).Hash.ToLower()
}

$GlobalExeHash = $null
$TargetExeHash = $null
$FoundExe = $false

foreach ($exeName in @("gta-sa.exe", "gta_sa.exe")) {
    $steamExePath = Join-Path $SteamDir $exeName
    $v10ExePath = Join-Path $V10Dir "gta_sa.exe"
    
    if (Test-Path $steamExePath) {
        $hashSteam = Get-FileHashMD5 $steamExePath
        $hashV10 = Get-FileHashMD5 $v10ExePath
        $GlobalExeHash = $hashSteam
        $TargetExeHash = $hashV10
        if ($hashSteam -ne $hashV10) {
            $ManifestData.Add([PSCustomObject]@{
                path = $exeName
                source_hash = $hashSteam
                target_hash = $hashV10
                action = "copy"
            })
            $FoundExe = $true
        }
    }
}

if (-not $FoundExe -and (Test-Path (Join-Path $V10Dir "gta_sa.exe"))) {
    $hashV10 = Get-FileHashMD5 (Join-Path $V10Dir "gta_sa.exe")
    $TargetExeHash = $hashV10
    $ManifestData.Add([PSCustomObject]@{
        path = "gta_sa.exe"
        source_hash = "MISSING"
        target_hash = $hashV10
        action = "copy"
    })
}

$IdenticalCount = 0
$Progress = 0

foreach ($relPath in $CommonFiles) {
    if ($relPath -eq "gta_sa.exe" -or $relPath -eq "gta-sa.exe") { continue }
    
    $Progress++
    if ($Progress % 100 -eq 0) {
        Write-Host -NoNewline "  Progress: $Progress / $($CommonFiles.Count)`r"
    }

    $hashSteam = Get-FileHashMD5 (Join-Path $SteamDir $relPath)
    $hashV10 = Get-FileHashMD5 (Join-Path $V10Dir $relPath)

    if ($hashSteam -ne $hashV10) {
        $ManifestData.Add([PSCustomObject]@{
            path = $relPath
            source_hash = $hashSteam
            target_hash = $hashV10
            action = "patch"
        })
    } else {
        $IdenticalCount++
    }
}

Write-Host "  Progress: $($CommonFiles.Count) / $($CommonFiles.Count)"
Write-Host "  Identical: $IdenticalCount" -ForegroundColor Green
Write-Host "  Different: $($ManifestData.Count)" -ForegroundColor Yellow
Write-Host ""

if ($ManifestData.Count -eq 0 -and -not (Test-Path $GermanDir)) {
    Write-Host "No differences found! Directories are identical." -ForegroundColor Green
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 0
}

Write-Host "Step 4: Processing Global differences..." -ForegroundColor Yellow
Write-Host ""

$PatchNum = 0
$FailedCount = 0
$TotalSizeOriginal = 0
$TotalSizePatches = 0

foreach ($item in $ManifestData) {
    $PatchNum++
    $relPath = $item.path
    $action = $item.action
    
    Write-Host -NoNewline "  [$PatchNum/$($ManifestData.Count)] Processing: $relPath"

    if ($action -eq "copy") {
        $v10Exe = Join-Path $V10Dir "gta_sa.exe"
        $patchExe = Join-Path $PatchesDir "gta_sa.exe"
        $destDir = Split-Path $patchExe
        if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir | Out-Null }
        
        Copy-Item $v10Exe $patchExe -Force
        
        $steamExePath = Join-Path $SteamDir $relPath
        if (Test-Path $steamExePath) { $TotalSizeOriginal += (Get-Item $steamExePath).Length }
        $TotalSizePatches += (Get-Item $patchExe).Length
        
        Write-Host "`r  [$PatchNum/$($ManifestData.Count)] ✓ $relPath (Direct Copy)   " -ForegroundColor Green
    } else {
        $srcFile = Join-Path $SteamDir $relPath
        $dstFile = Join-Path $V10Dir $relPath
        $patchFile = Join-Path $PatchesDir "$relPath.xdelta"
        $destDir = Split-Path $patchFile
        if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir | Out-Null }

        $argList = @("-d", "-e", "-9", "-f", "-s", "`"$srcFile`"", "`"$dstFile`"", "`"$patchFile`"")
        $proc = Start-Process -FilePath $XdeltaBin -ArgumentList $argList -NoNewWindow -Wait -PassThru -ErrorAction SilentlyContinue
        
        if ($proc -and $proc.ExitCode -eq 0) {
            $TotalSizeOriginal += (Get-Item $srcFile).Length
            $TotalSizePatches += (Get-Item $patchFile).Length
            Write-Host "`r  [$PatchNum/$($ManifestData.Count)] ✓ $relPath   " -ForegroundColor Green
        } else {
            Write-Host "`r  [$PatchNum/$($ManifestData.Count)] ✗ $relPath   " -ForegroundColor Red
            $FailedCount++
            if (Test-Path $patchFile) { Remove-Item $patchFile }
        }
    }
}

$GermanExeHash = $null
$GermanVariants = New-Object System.Collections.Generic.List[PSObject]
$GermanVariantsCount = 0

if (Test-Path $GermanDir) {
    Write-Host ""
    Write-Host "Step 5: Processing German variants..." -ForegroundColor Yellow
    
    foreach ($exeName in @("gta-sa.exe", "gta_sa.exe")) {
        $germanExePath = Join-Path $GermanDir $exeName
        if (Test-Path $germanExePath) {
            $GermanExeHash = Get-FileHashMD5 $germanExePath
            break
        }
    }

    $GermanFiles = Get-ChildItem -Path $GermanDir -Recurse -File | ForEach-Object { 
        $_.FullName.Substring((Get-Item $GermanDir).FullName.Length + 1) 
    } | Where-Object { $_ -notmatch '^(?i)readme(/|\\|$|\.)' -and $_ -notmatch '(?i)\.json$' -and $_ -notmatch '^(?i)gta[-_]sa\.exe$' }

    Write-Host "  German files to inspect: $($GermanFiles.Count)" -ForegroundColor Green

    $gNum = 0
    foreach ($relPath in $GermanFiles) {
        $gNum++
        $srcFile = Join-Path $GermanDir $relPath
        $dstFile = Join-Path $V10Dir $relPath
        $patchFile = Join-Path $VariantDir "$relPath.xdelta"

        if (-not (Test-Path $dstFile)) {
            Write-Host "  [German $gNum/$($GermanFiles.Count)] Target file not found: $relPath" -ForegroundColor Red
            continue
        }

        $hashGerman = Get-FileHashMD5 $srcFile
        $hashV10 = Get-FileHashMD5 $dstFile

        # 1. German file is already target 1.0 US -> no delta required
        if ($hashGerman -eq $hashV10) {
            continue
        }

        # 2. Check if German hash is already supported by Global source
        $globalFile = Join-Path $SteamDir $relPath
        $hashGlobal = if (Test-Path $globalFile) { Get-FileHashMD5 $globalFile } else { $null }

        if ($hashGlobal -and ($hashGerman -eq $hashGlobal)) {
            continue
        }

        Write-Host -NoNewline "  [German $gNum/$($GermanFiles.Count)] Processing variant: $relPath"
        $destDir = Split-Path $patchFile
        if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir | Out-Null }

        $argList = @("-d", "-e", "-9", "-f", "-s", "`"$srcFile`"", "`"$dstFile`"", "`"$patchFile`"")
        $proc = Start-Process -FilePath $XdeltaBin -ArgumentList $argList -NoNewWindow -Wait -PassThru -ErrorAction SilentlyContinue

        if ($proc -and $proc.ExitCode -eq 0) {
            $TotalSizeOriginal += (Get-Item $srcFile).Length
            $TotalSizePatches += (Get-Item $patchFile).Length
            Write-Host "`r  [German $gNum/$($GermanFiles.Count)] ✓ $relPath   " -ForegroundColor Green
            $variantRel = ("variants/steam_r2_german/" + ($relPath -replace '\\', '/')) + ".xdelta"
            $GermanVariants.Add([PSCustomObject]@{
                path = ($relPath -replace '\\', '/')
                german_hash = $hashGerman
                target_hash = $hashV10
                global_hash = $hashGlobal
                patch_rel = $variantRel
            })
            $GermanVariantsCount++
        } else {
            Write-Host "`r  [German $gNum/$($GermanFiles.Count)] ✗ $relPath   " -ForegroundColor Red
            $FailedCount++
            if (Test-Path $patchFile) { Remove-Item $patchFile }
        }
    }
    Write-Host "  German variants generated: $GermanVariantsCount" -ForegroundColor Green
}

Write-Host ""
Write-Host "Step 6: Generating manifest.json (v2.0)..." -ForegroundColor Yellow

$SourceVersions = New-Object System.Collections.Generic.List[PSObject]
if ($GlobalExeHash) {
    $SourceVersions.Add([PSCustomObject]@{
        id = "steam_r2_global"
        name = "NewSteam R2"
        exe_hash = $GlobalExeHash
    })
}
if ($GermanExeHash) {
    $SourceVersions.Add([PSCustomObject]@{
        id = "steam_r2_german"
        name = "NewSteam R2 (German)"
        exe_hash = $GermanExeHash
    })
}

$ManifestFiles = [System.Collections.Generic.Dictionary[string, PSObject]]::new()

foreach ($item in $ManifestData) {
    $relPath = ($item.path -replace '\\', '/')
    $action = $item.action
    if ($action -eq "copy" -and (Test-Path (Join-Path $PatchesDir "gta_sa.exe"))) {
        $sources = New-Object System.Collections.Generic.List[PSObject]
        if ($item.source_hash -and $item.source_hash -ne "MISSING") {
            $sources.Add([PSCustomObject]@{ hash = $item.source_hash })
        }
        if ($GermanExeHash -and $GermanExeHash -ne $item.source_hash -and $GermanExeHash -ne $item.target_hash) {
            $sources.Add([PSCustomObject]@{ hash = $GermanExeHash })
        }
        $ManifestFiles[$relPath] = [PSCustomObject]@{
            path = $relPath
            action = "copy"
            source_hash = $item.source_hash
            target_hash = $item.target_hash
            payload = "gta_sa.exe"
            sources = $sources
        }
    } elseif ($action -eq "patch" -and (Test-Path (Join-Path $PatchesDir "$($item.path).xdelta"))) {
        $sources = New-Object System.Collections.Generic.List[PSObject]
        $sources.Add([PSCustomObject]@{
            hash = $item.source_hash
            patch = "$relPath.xdelta"
        })
        $ManifestFiles[$relPath] = [PSCustomObject]@{
            path = $relPath
            action = "patch"
            source_hash = $item.source_hash
            target_hash = $item.target_hash
            sources = $sources
        }
    }
}

foreach ($v in $GermanVariants) {
    $relPath = $v.path
    $patchFull = Join-Path $PatchesDir ($v.patch_rel -replace '/', '\')
    if (-not (Test-Path $patchFull)) { continue }

    if ($ManifestFiles.ContainsKey($relPath)) {
        $entry = $ManifestFiles[$relPath]
        $alreadyHas = $false
        foreach ($s in $entry.sources) {
            if ($s.hash -eq $v.german_hash) { $alreadyHas = $true; break }
        }
        if (-not $alreadyHas) {
            $entry.sources.Add([PSCustomObject]@{
                hash = $v.german_hash
                patch = $v.patch_rel
            })
        }
    } else {
        $sourceHash = if ($v.global_hash) { $v.global_hash } else { $v.target_hash }
        $sources = New-Object System.Collections.Generic.List[PSObject]
        $sources.Add([PSCustomObject]@{
            hash = $v.german_hash
            patch = $v.patch_rel
        })
        $ManifestFiles[$relPath] = [PSCustomObject]@{
            path = $relPath
            action = "patch"
            source_hash = $sourceHash
            target_hash = $v.target_hash
            sources = $sources
        }
    }
}

$SortedFiles = $ManifestFiles.Values | Sort-Object { if ($_.path -eq "gta-sa.exe" -or $_.path -eq "gta_sa.exe") { 0 } else { 1 } }, path

$TotalPatchesCount = 0
foreach ($f in $SortedFiles) {
    if ($f.sources) {
        foreach ($s in $f.sources) {
            if ($s.patch) { $TotalPatchesCount++ }
        }
    }
}

$Manifest = [PSCustomObject]@{
    version = "2.0"
    generated = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssK")
    source_version = "steam"
    target_version = "1.0_us"
    source_versions = $SourceVersions
    statistics = @{
        total_files = $CommonFiles.Count
        identical = $IdenticalCount
        different = $SortedFiles.Count
        patches_generated = $TotalPatchesCount
        failed = $FailedCount
        original_size_mb = [Math]::Round($TotalSizeOriginal / 1MB, 2)
        patches_size_mb = [Math]::Round($TotalSizePatches / 1MB, 2)
    }
    files = $SortedFiles
}

$Manifest | ConvertTo-Json -Depth 10 | Set-Content (Join-Path $PatchesDir "manifest.json")

Write-Host "  ✓ $PatchesDir/manifest.json" -ForegroundColor Green
Write-Host ""

Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "========================================" -ForegroundColor Blue
Write-Host "Summary" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host "Patches generated: $($Manifest.statistics.patches_generated)" -ForegroundColor Green
if ($FailedCount -gt 0) {
    Write-Host "Failed patches:    $FailedCount" -ForegroundColor Red
}
Write-Host "Output directory:  $PatchesDir" -ForegroundColor Green
Write-Host ""

$Compression = 0
if ($TotalSizeOriginal -gt 0) {
    $Compression = [Math]::Round(100 * $TotalSizePatches / $TotalSizeOriginal, 1)
}

Write-Host "Original files size: $($Manifest.statistics.original_size_mb) MB" -ForegroundColor Yellow
Write-Host "Patches total size:  $($Manifest.statistics.patches_size_mb) MB" -ForegroundColor Green
Write-Host "Compression ratio:   $Compression%" -ForegroundColor Green
Write-Host ""

Write-Host "Done!" -ForegroundColor Green