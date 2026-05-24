[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$PluginName,

    [Parameter(Mandatory = $true)]
    [string]$Version,

    [string]$BuildCommand = 'make build-all-platforms',
    [string]$ReleaseTitle,
    [string]$ReleaseNotes = 'Your changelog here',
    [switch]$SkipTagCreation,
    [switch]$SkipPush
)

$ErrorActionPreference = 'Stop'

$normalizedVersion = $Version.TrimStart('v')
$tag = "$PluginName-v$normalizedVersion"

if (-not $ReleaseTitle) {
    $ReleaseTitle = "$PluginName v$normalizedVersion"
}

Write-Host "==> Building release assets"
Invoke-Expression $BuildCommand

$assets = @(
    "$PluginName-linux-amd64",
    "$PluginName-linux-arm64",
    "$PluginName-darwin-amd64",
    "$PluginName-darwin-arm64",
    "$PluginName-windows-amd64.exe",
    "$PluginName-windows-arm64.exe"
)

foreach ($asset in $assets) {
    if (-not (Test-Path $asset)) {
        throw "Missing build artifact: $asset"
    }
}

$checksumLines = @()
$releaseAssets = @()

foreach ($asset in $assets) {
    Write-Host "==> Generating checksum for $asset"
    $hash = (Get-FileHash $asset -Algorithm SHA256).Hash.ToLower()
    Set-Content -Path "$asset.sha256" -Value $hash
    $checksumLines += "$hash  $asset"
    $releaseAssets += $asset
    $releaseAssets += "$asset.sha256"
}

Set-Content -Path 'checksums.txt' -Value $checksumLines
$releaseAssets += 'checksums.txt'

if (-not $SkipTagCreation) {
    Write-Host "==> Creating git tag $tag"
    git tag -a $tag -m "Release $PluginName v$normalizedVersion"
}

if (-not $SkipPush) {
    Write-Host "==> Pushing tag $tag"
    git push origin $tag
}

Write-Host "==> Creating GitHub release $tag"
$releaseArgs = @('release', 'create', $tag) + $releaseAssets + @('--title', $ReleaseTitle, '--notes', $ReleaseNotes)
& gh @releaseArgs

Write-Host '==> Done. Wait for the registry sync, then verify plugins.json.'
