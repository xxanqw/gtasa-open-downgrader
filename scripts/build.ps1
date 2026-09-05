param([ValidateSet('online', 'offline')][string]$Variant = 'online')
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if ((go env GOARCH) -ne 'amd64') { throw 'Bundled xdelta3 requires amd64.' }
$Tags = 'release'
$Name = 'gtasa-open-downgrader-windows'
if ($Variant -eq 'offline') {
    go run ./cmd/patch-assets -verify
    if ($LASTEXITCODE -ne 0) { throw 'Offline patch validation failed.' }
    $Tags += ',offline'
    $Name += '-offline'
}
$Version = if ($env:VERSION) { $env:VERSION } else { 'v0.2.2' }
$Resource = 'cmd/downgrader/resources_windows_amd64.syso'
$Windres = if ($env:WINDRES) { $env:WINDRES } else { 'windres' }
try {
    & $Windres -i scripts/windows.rc -O coff -o $Resource
    if ($LASTEXITCODE -ne 0) { throw 'Windows resource generation failed.' }
    New-Item -ItemType Directory -Force dist | Out-Null
    $env:CGO_ENABLED = '1'
    go build -trimpath -tags $Tags -ldflags "-s -w -H=windowsgui -X github.com/xxanqw/gtasa-open-downgrader/internal/updater.Version=$Version" -o "dist/$Name.exe" ./cmd/downgrader
    if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
} finally {
    Remove-Item -ErrorAction SilentlyContinue $Resource
}
