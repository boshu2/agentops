# install-codex.ps1 - Install AgentOps into the local Codex native plugin cache on Windows.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\scripts\install-codex.ps1
#   irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex

[CmdletBinding()]
param(
  [string]$RepoRoot = $env:AGENTOPS_BUNDLE_ROOT,
  [string]$CodexHome = $env:CODEX_HOME,
  [string]$Version = $env:AGENTOPS_INSTALL_REF,
  [string]$UpdateCommand = "irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex"
)

$ErrorActionPreference = "Stop"

$PluginName = "agentops"
$MarketplaceName = "agentops-marketplace"
$PluginKey = "$PluginName@$MarketplaceName"
$InstallRef = if ([string]::IsNullOrWhiteSpace($Version)) { "main" } else { $Version }
$ArchiveUrl = if ($InstallRef -eq "main") {
  "https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/main"
} else {
  "https://codeload.github.com/boshu2/agentops/tar.gz/refs/tags/$InstallRef"
}

function Write-Info {
  param([string]$Message)
  Write-Host "[ok] $Message" -ForegroundColor Green
}

function Write-Warn {
  param([string]$Message)
  Write-Host "[warn] $Message" -ForegroundColor Yellow
}

function Fail {
  param([string]$Message)
  throw $Message
}

function Require-Path {
  param([string]$Path, [string]$Label)
  if (-not (Test-Path -LiteralPath $Path)) {
    Fail "Missing ${Label}: $Path"
  }
}

function Get-AgentOpsTempDir {
  $path = Join-Path ([System.IO.Path]::GetTempPath()) ("agentops-codex-" + [Guid]::NewGuid().ToString("N"))
  New-Item -ItemType Directory -Path $path | Out-Null
  return $path
}

function Resolve-ArchiveRoot {
  param([string]$ArchiveFile)

  $entries = @(& tar.exe -tzf $ArchiveFile)
  if ($LASTEXITCODE -ne 0 -or $entries.Count -eq 0 -or [string]::IsNullOrWhiteSpace($entries[0])) {
    Fail "Could not read AgentOps archive"
  }

  return ($entries[0] -split "/")[0]
}

function Get-BashFlavor {
  if (-not (Get-Command bash -ErrorAction SilentlyContinue)) {
    return $null
  }

  try {
    return ((& bash -lc "uname -s" 2>$null) | Select-Object -First 1)
  } catch {
    return $null
  }
}

function Convert-ToBashPath {
  param([string]$Path, [string]$BashFlavor)

  $fullPath = [System.IO.Path]::GetFullPath($Path)
  if ($fullPath -match "^([A-Za-z]):\\(.*)$") {
    $drive = $matches[1].ToLowerInvariant()
    $tail = $matches[2] -replace "\\", "/"
    if ($BashFlavor -like "Linux*") {
      return "/mnt/$drive/$tail"
    }
    return "/$drive/$tail"
  }

  return ($fullPath -replace "\\", "/")
}

function Copy-CleanDirectory {
  param([string]$Source, [string]$Destination)

  if (Test-Path -LiteralPath $Destination) {
    Remove-Item -LiteralPath $Destination -Recurse -Force
  }
  New-Item -ItemType Directory -Path (Split-Path -Parent $Destination) -Force | Out-Null
  Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
}

function Upsert-TomlKey {
  param([string]$File, [string]$Section, [string]$Key, [string]$Value)

  New-Item -ItemType Directory -Path (Split-Path -Parent $File) -Force | Out-Null
  if (-not (Test-Path -LiteralPath $File)) {
    Set-Content -LiteralPath $File -Value @($Section, "$Key = $Value") -Encoding utf8
    return
  }

  $lines = [System.Collections.Generic.List[string]]::new()
  $lines.AddRange([string[]](Get-Content -LiteralPath $File))

  $sectionIndex = -1
  for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -eq $Section) {
      $sectionIndex = $i
      break
    }
  }

  if ($sectionIndex -lt 0) {
    if ($lines.Count -gt 0 -and $lines[$lines.Count - 1] -ne "") {
      $lines.Add("")
    }
    $lines.Add($Section)
    $lines.Add("$Key = $Value")
    Set-Content -LiteralPath $File -Value $lines -Encoding utf8
    return
  }

  $insertIndex = $lines.Count
  for ($i = $sectionIndex + 1; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match "^\[") {
      $insertIndex = $i
      break
    }
    if ($lines[$i] -match "^\s*$([regex]::Escape($Key))\s*=") {
      $lines[$i] = "$Key = $Value"
      Set-Content -LiteralPath $File -Value $lines -Encoding utf8
      return
    }
  }

  $lines.Insert($insertIndex, "$Key = $Value")
  Set-Content -LiteralPath $File -Value $lines -Encoding utf8
}

function Remove-TomlKey {
  param([string]$File, [string]$Section, [string]$Key)

  if (-not (Test-Path -LiteralPath $File)) {
    return
  }

  $lines = [System.Collections.Generic.List[string]]::new()
  $lines.AddRange([string[]](Get-Content -LiteralPath $File))

  $sectionIndex = -1
  for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -eq $Section) {
      $sectionIndex = $i
      break
    }
  }

  if ($sectionIndex -lt 0) {
    return
  }

  for ($i = $sectionIndex + 1; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match "^\[") {
      break
    }
    if ($lines[$i] -match "^\s*$([regex]::Escape($Key))\s*=") {
      $lines.RemoveAt($i)
      $i--
    }
  }

  Set-Content -LiteralPath $File -Value $lines -Encoding utf8
}

function Archive-SkillRoot {
  param([string]$Root, [string]$SkillsSource, [bool]$ManagedRoot)

  if (-not (Test-Path -LiteralPath $Root)) {
    return $null
  }

  $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $backupDir = Join-Path (Split-Path -Parent $Root) ("skills.backup.$timestamp")
  $moved = 0

  foreach ($skill in Get-ChildItem -LiteralPath $SkillsSource -Directory) {
    $target = Join-Path $Root $skill.Name
    if (-not (Test-Path -LiteralPath $target)) {
      continue
    }
    if (-not $ManagedRoot -and -not (Test-Path -LiteralPath (Join-Path $target ".agentops-generated.json"))) {
      continue
    }
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
    Move-Item -LiteralPath $target -Destination (Join-Path $backupDir $skill.Name) -Force
    $moved += 1
  }

  foreach ($name in @(".agentops-manifest.json", ".agentops-codex-state.json")) {
    $path = Join-Path $Root $name
    if (Test-Path -LiteralPath $path) {
      New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
      Move-Item -LiteralPath $path -Destination (Join-Path $backupDir $name) -Force
      $moved += 1
    }
  }

  if ($moved -gt 0) {
    return $backupDir
  }
  return $null
}

$tempDir = $null
try {
  Write-Host "Installing AgentOps for Codex on Windows..."
  Write-Host ""

  if ([string]::IsNullOrWhiteSpace($CodexHome)) {
    $CodexHome = Join-Path $HOME ".codex"
  }
  $CodexHome = [System.IO.Path]::GetFullPath($CodexHome)

  if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    if (-not (Get-Command tar.exe -ErrorAction SilentlyContinue)) {
      Fail "Missing required command: tar.exe"
    }
    $tempDir = Get-AgentOpsTempDir
    $archiveFile = Join-Path $tempDir "agentops.tar.gz"
    Write-Info "Downloading AgentOps bundle..."
    Invoke-WebRequest -UseBasicParsing -Uri $ArchiveUrl -OutFile $archiveFile
    $archiveRoot = Resolve-ArchiveRoot -ArchiveFile $archiveFile
    & tar.exe -xzf $archiveFile -C $tempDir
    if ($LASTEXITCODE -ne 0) {
      Fail "Could not extract AgentOps archive"
    }
    $RepoRoot = Join-Path $tempDir $archiveRoot
  }

  $RepoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
  $pluginManifest = Join-Path $RepoRoot ".codex-plugin\plugin.json"
  $marketplaceFile = Join-Path $RepoRoot "plugins\marketplace.json"
  $skillsSrc = Join-Path $RepoRoot "skills-codex"
  $skillManifest = Join-Path $skillsSrc ".agentops-manifest.json"
  $pluginCacheRoot = Join-Path $CodexHome "plugins\cache\$MarketplaceName\$PluginName\local"
  $configFile = Join-Path $CodexHome "config.toml"
  $installMeta = Join-Path $CodexHome ".agentops-codex-install.json"
  $pluginStateFile = Join-Path $pluginCacheRoot ".agentops-codex-state.json"

  Require-Path $pluginManifest "Codex plugin manifest"
  Require-Path $marketplaceFile "Codex marketplace manifest"
  Require-Path $skillsSrc "Codex-native skill bundle"
  Require-Path $skillManifest "Codex skill manifest"

  Write-Info "Installing AgentOps Codex native plugin..."
  if (Test-Path -LiteralPath $pluginCacheRoot) {
    Remove-Item -LiteralPath $pluginCacheRoot -Recurse -Force
  }
  New-Item -ItemType Directory -Path $pluginCacheRoot -Force | Out-Null
  Copy-Item -LiteralPath (Join-Path $RepoRoot ".codex-plugin") -Destination (Join-Path $pluginCacheRoot ".codex-plugin") -Recurse -Force
  Copy-Item -LiteralPath $skillsSrc -Destination (Join-Path $pluginCacheRoot "skills-codex") -Recurse -Force
  foreach ($optionalFile in @(".mcp.json", ".app.json")) {
    $path = Join-Path $RepoRoot $optionalFile
    if (Test-Path -LiteralPath $path) {
      Copy-Item -LiteralPath $path -Destination (Join-Path $pluginCacheRoot $optionalFile) -Force
    }
  }

  Upsert-TomlKey $configFile "[features]" "plugins" "true"
  Upsert-TomlKey $configFile "[plugins.`"$PluginKey`"]" "enabled" "true"
  Upsert-TomlKey $configFile "[ui]" "suppress_unstable_features_warning" "true"
  Remove-TomlKey $configFile "[features]" "codex_hooks"

  $manifestHash = (Get-FileHash -LiteralPath $skillManifest -Algorithm SHA256).Hash.ToLowerInvariant()
  $installedManifest = Join-Path $pluginCacheRoot "skills-codex\.agentops-manifest.json"
  $installedHash = (Get-FileHash -LiteralPath $installedManifest -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($manifestHash -ne $installedHash) {
    Fail "Installed plugin cache manifest hash mismatch; expected $manifestHash, got $installedHash"
  }

  $skillCount = @(Get-ChildItem -LiteralPath (Join-Path $pluginCacheRoot "skills-codex") -Directory | Where-Object {
    Test-Path -LiteralPath (Join-Path $_.FullName "SKILL.md")
  }).Count

  $legacyBackup = Archive-SkillRoot -Root (Join-Path $CodexHome "skills") -SkillsSource $skillsSrc -ManagedRoot $true
  $userSkillsRoot = Join-Path (Split-Path -Parent $CodexHome) ".agents\skills"
  $managedUserRoot = (Test-Path -LiteralPath (Join-Path $userSkillsRoot ".agentops-manifest.json")) -or (Test-Path -LiteralPath (Join-Path $userSkillsRoot ".agentops-codex-state.json"))
  $userBackup = Archive-SkillRoot -Root $userSkillsRoot -SkillsSource $skillsSrc -ManagedRoot $managedUserRoot

  $installedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  $hookRuntime = "hookless-default"
  $state = [ordered]@{
    installed_at = $installedAt
    install_mode = "native-plugin"
    hook_runtime = $hookRuntime
    hooks_installed = $false
    version = $InstallRef
    manifest_hash = $manifestHash
    skill_count = $skillCount
    plugin_root = $pluginCacheRoot
  }
  $state | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $pluginStateFile -Encoding utf8

  $meta = [ordered]@{
    installed_at = $installedAt
    source = "install-codex.ps1"
    install_mode = "native-plugin"
    hook_runtime = $hookRuntime
    hooks_installed = $false
    lifecycle_commands = @("ao rpi phased", "ao codex status")
    plugin_key = $PluginKey
    version = $InstallRef
    plugin_root = $pluginCacheRoot
    manifest_hash = $manifestHash
    skill_count = $skillCount
    plugin_state_file = $pluginStateFile
    user_skills_root = $null
    update_command = $UpdateCommand
  }
  $meta | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $installMeta -Encoding utf8

  Remove-TomlKey $configFile "[features]" "hooks"
  Write-Info "Codex hooks not installed (hookless - skills + ao CLI only)"

  Write-Info "Native Codex plugin installed"
  Write-Host "  Plugin key: $PluginKey"
  Write-Host "  Plugin root: $pluginCacheRoot"
  Write-Host "  Skills available: $skillCount"
  Write-Host "  Config updated: $configFile"
  if ($legacyBackup) {
    Write-Host "  Archived overlapping ~/.codex/skills entries to: $legacyBackup"
  }
  if ($userBackup) {
    Write-Host "  Archived overlapping ~/.agents/skills entries to: $userBackup"
  }
  Write-Info "Install metadata written: $installMeta"
  Write-Host ""
  Write-Host "Restart Codex to pick up the native plugin."
} finally {
  if ($tempDir -and (Test-Path -LiteralPath $tempDir)) {
    Remove-Item -LiteralPath $tempDir -Recurse -Force
  }
}
