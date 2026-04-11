# install-ao.ps1 - Install the ao CLI release binary on Windows.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\scripts\install-ao.ps1
#   irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex

[CmdletBinding()]
param(
  [string]$Version = "latest",
  [string]$InstallDir = (Join-Path $HOME "bin"),
  [switch]$NoPathUpdate
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

function Write-Info {
  param([string]$Message)
  Write-Host "[agentops] $Message"
}

function Fail {
  param([string]$Message)
  throw "[agentops] $Message"
}

function Get-AOArch {
  $arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
  switch -Regex ($arch) {
    "AMD64|x86_64" { return "amd64" }
    "ARM64|AARCH64" { return "arm64" }
    default { Fail "Unsupported Windows architecture: $arch" }
  }
}

function Resolve-Release {
  param([string]$RequestedVersion)

  if ($RequestedVersion -eq "latest") {
    return Invoke-RestMethod -Uri "https://api.github.com/repos/boshu2/agentops/releases/latest" -Headers @{ "Accept" = "application/vnd.github+json" }
  }

  $tag = [Uri]::EscapeDataString($RequestedVersion)
  return Invoke-RestMethod -Uri "https://api.github.com/repos/boshu2/agentops/releases/tags/$tag" -Headers @{ "Accept" = "application/vnd.github+json" }
}

function Get-ReleaseAsset {
  param(
    [object]$Release,
    [string]$Name
  )

  $asset = $Release.assets | Where-Object { $_.name -eq $Name } | Select-Object -First 1
  if (-not $asset) {
    $names = ($Release.assets | ForEach-Object { $_.name }) -join ", "
    Fail "Release $($Release.tag_name) does not include $Name. Available assets: $names"
  }
  return $asset
}

function Get-ExpectedChecksum {
  param(
    [string]$ChecksumPath,
    [string]$AssetName
  )

  foreach ($line in Get-Content -Path $ChecksumPath) {
    $trimmed = $line.Trim()
    if (-not $trimmed) {
      continue
    }

    $parts = $trimmed -split "\s+", 2
    if ($parts.Count -eq 2 -and $parts[1] -ceq $AssetName) {
      return $parts[0].ToLowerInvariant()
    }
  }

  Fail "checksums.txt does not contain an entry for $AssetName"
}

function Add-UserPath {
  param([string]$PathToAdd)

  $resolved = [System.IO.Path]::GetFullPath($PathToAdd)
  $current = [Environment]::GetEnvironmentVariable("Path", "User")
  $parts = @()
  if ($current) {
    $parts = $current -split ";" | Where-Object { $_ }
  }
  $alreadyPresent = $false
  foreach ($part in $parts) {
    if ($part.TrimEnd("\") -ieq $resolved.TrimEnd("\")) {
      $alreadyPresent = $true
      break
    }
  }

  if (-not $alreadyPresent) {
    $newPath = if ($current) { "$current;$resolved" } else { $resolved }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Info "Added $resolved to your user PATH"
  }

  $envParts = $env:Path -split ";" | Where-Object { $_ }
  $inCurrentProcess = $false
  foreach ($part in $envParts) {
    if ($part.TrimEnd("\") -ieq $resolved.TrimEnd("\")) {
      $inCurrentProcess = $true
      break
    }
  }
  if (-not $inCurrentProcess) {
    $env:Path = "$env:Path;$resolved"
  }
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("agentops-ao-" + [Guid]::NewGuid().ToString("N"))

try {
  New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

  $release = Resolve-Release $Version
  $arch = Get-AOArch
  $assetName = "ao-windows-$arch.tar.gz"
  $archiveAsset = Get-ReleaseAsset $release $assetName
  $checksumsAsset = Get-ReleaseAsset $release "checksums.txt"

  $archivePath = Join-Path $tempDir $assetName
  $checksumsPath = Join-Path $tempDir "checksums.txt"

  Write-Info "Downloading $assetName from AgentOps $($release.tag_name)"
  Invoke-WebRequest -Uri $archiveAsset.browser_download_url -OutFile $archivePath -TimeoutSec 120
  Invoke-WebRequest -Uri $checksumsAsset.browser_download_url -OutFile $checksumsPath -TimeoutSec 30

  $expected = Get-ExpectedChecksum $checksumsPath $assetName
  $actual = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash.ToLowerInvariant()
  if ($actual -ne $expected) {
    Fail "Checksum mismatch for $assetName. Expected $expected, got $actual"
  }
  Write-Info "Verified SHA256 checksum"

  $tar = Get-Command tar.exe -ErrorAction SilentlyContinue
  if (-not $tar) {
    Fail "tar.exe was not found on PATH. Install a current Windows build or extract $assetName manually."
  }

  $stageDir = Join-Path $tempDir "stage"
  New-Item -ItemType Directory -Force -Path $stageDir | Out-Null
  & $tar.Source -xzf $archivePath -C $stageDir ao.exe
  $extractedBinary = Join-Path $stageDir "ao.exe"
  if (-not (Test-Path -LiteralPath $extractedBinary)) {
    Fail "ao.exe not found after extraction"
  }
  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  Copy-Item -LiteralPath $extractedBinary -Destination (Join-Path $InstallDir "ao.exe") -Force

  if (-not $NoPathUpdate) {
    Add-UserPath $InstallDir
  }

  $aoPath = Join-Path $InstallDir "ao.exe"
  Write-Info "Installed $aoPath"
  & $aoPath version
  Write-Host "Open a new terminal if 'ao' is not immediately available on PATH."
}
finally {
  if (Test-Path -LiteralPath $tempDir) {
    Remove-Item -LiteralPath $tempDir -Recurse -Force
  }
}
