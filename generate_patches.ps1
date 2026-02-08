$ErrorActionPreference = "Stop"

$SteamDir = "SA_STEAM"
$V10Dir = "SA_10US"
$PatchesDir = "Patches"
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
}
$V10Files = Get-ChildItem -Path $V10Dir -Recurse -File | ForEach-Object { 
    $_.FullName.Substring((Get-Item $V10Dir).FullName.Length + 1) 
}

Write-Host "  Steam files: $($SteamFiles.Count)" -ForegroundColor Green
Write-Host "  v1.0 files:  $($V10Files.Count)" -ForegroundColor Green
Write-Host ""

Write-Host "Step 2: Finding common files..." -ForegroundColor Yellow
$CommonFiles = Compare-Object $SteamFiles $V10Files -IncludeEqual -ExcludeDifferent | Select-Object -ExpandProperty InputObject
Write-Host "  Common files: $($CommonFiles.Count)" -ForegroundColor Green
Write-Host ""

Write-Host "Step 3: Comparing file hashes..." -ForegroundColor Yellow
$ManifestData = New-Object System.Collections.Generic.List[PSObject]

function Get-FileHashMD5($path) {
    if (-not (Test-Path $path)) { return $null }
    return (Get-FileHash -Path $path -Algorithm MD5).Hash.ToLower()
}

$FoundExe = $false
foreach ($exeName in @("gta-sa.exe", "gta_sa.exe")) {
    $steamExePath = Join-Path $SteamDir $exeName
    $v10ExePath = Join-Path $V10Dir "gta_sa.exe"
    
    if (Test-Path $steamExePath) {
        $hashSteam = Get-FileHashMD5 $steamExePath
        $hashV10 = Get-FileHashMD5 $v10ExePath
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

if ($ManifestData.Count -eq 0) {
    Write-Host "No differences found! Directories are identical." -ForegroundColor Green
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 0
}

Write-Host "Step 4: Processing differences..." -ForegroundColor Yellow
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

        $argList = @("-d", "-e", "-9", "-s", "`"$srcFile`"", "`"$dstFile`"", "`"$patchFile`"")
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

Write-Host ""

Write-Host "Step 5: Generating manifest.json..." -ForegroundColor Yellow

$Manifest = [PSCustomObject]@{
    version = "1.0"
    generated = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssK")
    source_version = "steam"
    target_version = "1.0_us"
    statistics = @{
        total_files = $CommonFiles.Count
        identical = $IdenticalCount
        different = $ManifestData.Count
        patches_generated = $ManifestData.Count - $FailedCount
        failed = $FailedCount
        original_size_mb = [Math]::Round($TotalSizeOriginal / 1MB, 2)
        patches_size_mb = [Math]::Round($TotalSizePatches / 1MB, 2)
    }
    files = New-Object System.Collections.Generic.List[PSObject]
}

foreach ($item in $ManifestData) {
    $relPath = $item.path
    $action = $item.action
    $valid = $false
    if ($action -eq "copy" -and (Test-Path (Join-Path $PatchesDir "gta_sa.exe"))) { $valid = $true }
    if ($action -eq "patch" -and (Test-Path (Join-Path $PatchesDir "$relPath.xdelta"))) { $valid = $true }

    if ($valid) {
        $Manifest.files.Add([PSCustomObject]@{
            path = $relPath
            action = $action
            source_hash = $item.source_hash
            target_hash = $item.target_hash
        })
    }
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