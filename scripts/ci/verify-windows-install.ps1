# verify-windows-install.ps1 - Assertions for the windows-installers CI leg.
#
# Independently re-checks what scripts/install-codex.ps1 and scripts/install-ao.ps1
# just did, the same way scripts/install-codex-plugin.sh's selftest_codex_plugin
# (age-txfnl) re-checks the bash installer: prove the three numbers `ao doctor`
# reconciles are equal — the on-disk skills-codex/*/SKILL.md count, the manifest
# count AS DOCTOR COMPUTES IT (package_count when present, else len(skills[]);
# deliberate doctor parity, NOT strict package_count enforcement — a legacy
# manifest without package_count that is otherwise consistent is doctor-green
# and must be verifier-green too), and the recorded install-metadata
# skill_count. package_count and skills[] are NOT required to agree: package_count
# inventories every installable dir (incl. the 4 compatibility pointer twins)
# while skills[] lists only canonical implementation rows, so on the real bundle
# they are legitimately 66 vs 62 (see scripts/validate-codex-generated-manifest.sh
# and installer-selftest.bats, which enforce disk==package_count, never
# package_count==len(skills[])). The historical bug this guards against was a
# 66-vs-62 drift between a disk count and a stale manifest — caught by the
# manifest-count-vs-disk assertion, not by any package_count/skills[] equality.
#
# This script deliberately lives OUTSIDE install-codex.ps1 rather than adding an
# in-script selftest: it re-derives every number from the files the installer
# left on disk, so it can't share a bug with the installer's own counting logic.
# Faithful in-script selftest parity with the bash installer (closing the FU7
# deferred scope fully) is left as follow-up; see the CI leg's PR description.
#
# Usage:
#   pwsh -File scripts/ci/verify-windows-install.ps1 `
#     -CodexHome <path to sandbox .codex> `
#     -AoExePath <path to installed ao.exe>
#
# Exits non-zero (throws) on the first failed assertion, printing a `[fail]`
# line naming the mismatch, so a broken installer fails CI before a Windows
# user finds it.

[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$CodexHome,
  [Parameter(Mandatory = $true)][string]$AoExePath
)

$ErrorActionPreference = "Stop"

function Write-Ok {
  param([string]$Message)
  Write-Host "[ok] $Message" -ForegroundColor Green
}

function Fail {
  param([string]$Message)
  Write-Host "[fail] $Message" -ForegroundColor Red
  throw $Message
}

function Get-ManifestSkillCount {
  param([string]$ManifestPath)

  if (-not (Test-Path -LiteralPath $ManifestPath)) {
    Fail "Codex skill manifest not found: $ManifestPath"
  }

  $manifest = Get-Content -LiteralPath $ManifestPath -Raw | ConvertFrom-Json
  $hasCount = ($manifest.PSObject.Properties.Name -contains "package_count" -and $manifest.package_count -gt 0)
  $hasSkills = ($manifest.PSObject.Properties.Name -contains "skills")
  # package_count inventories every installable generated skill directory.
  # The manifest validator keeps it equal to the canonical skills[] inventory;
  # no compatibility-pointer directories are installed.
  if ($hasCount) {
    return [int]$manifest.package_count
  }
  if ($hasSkills) {
    # Doctor-parity fallback: doctor counts len(skills[]) when package_count is
    # absent, so a legacy-but-consistent manifest passes here exactly as it
    # passes doctor. This is a contract choice, not a missed check.
    return @($manifest.skills).Count
  }
  Fail "Manifest has neither a usable package_count nor a skills[] array: $ManifestPath"
}

function Get-JsonIntField {
  param([string]$JsonPath, [string]$Field)

  if (-not (Test-Path -LiteralPath $JsonPath)) {
    Fail "Install metadata not found: $JsonPath"
  }
  $doc = Get-Content -LiteralPath $JsonPath -Raw | ConvertFrom-Json
  if (-not ($doc.PSObject.Properties.Name -contains $Field)) {
    Fail "Install metadata $JsonPath has no '$Field' field"
  }
  return [int]$doc.$Field
}

Write-Host "== Verifying install-codex.ps1 output =="

$PluginRoot = Join-Path $CodexHome "plugins\cache\agentops-marketplace\agentops\local"
$SkillsDst = Join-Path $PluginRoot "skills-codex"
$ManifestPath = Join-Path $SkillsDst ".agentops-manifest.json"
$InstallMeta = Join-Path $CodexHome ".agentops-codex-install.json"
$ConfigFile = Join-Path $CodexHome "config.toml"

if (-not (Test-Path -LiteralPath $PluginRoot)) {
  Fail "Plugin cache root was not created: $PluginRoot"
}
Write-Ok "Plugin cache root exists: $PluginRoot"

if (-not (Test-Path -LiteralPath $SkillsDst)) {
  Fail "Installed skills-codex directory not found: $SkillsDst"
}

$diskCount = @(Get-ChildItem -LiteralPath $SkillsDst -Directory | Where-Object {
  Test-Path -LiteralPath (Join-Path $_.FullName "SKILL.md")
}).Count
if ($diskCount -eq 0) {
  Fail "No installed skill directories contain a SKILL.md under $SkillsDst"
}
Write-Ok "On-disk skill directory count: $diskCount"

$manifestCount = Get-ManifestSkillCount -ManifestPath $ManifestPath
if ($manifestCount -ne $diskCount) {
  Fail "Manifest skill count ($manifestCount) != on-disk skill directory count ($diskCount) that 'ao doctor' would flag"
}
Write-Ok "Manifest skill count matches disk: $manifestCount"

$metaCount = Get-JsonIntField -JsonPath $InstallMeta -Field "skill_count"
if ($metaCount -ne $diskCount) {
  Fail "Install metadata skill_count ($metaCount) != on-disk skill directory count ($diskCount)"
}
Write-Ok "Install metadata skill_count matches disk: $metaCount"

if (-not (Test-Path -LiteralPath $ConfigFile)) {
  Fail "config.toml was not written: $ConfigFile"
}
$configText = Get-Content -LiteralPath $ConfigFile -Raw
if ($configText -notmatch [regex]::Escape('[plugins."agentops@agentops-marketplace"]')) {
  Fail "config.toml is missing the plugin enable entry [plugins.`"agentops@agentops-marketplace`"]"
}
# Section-scoped: 'plugins = true' must sit inside the [features] table, not
# merely anywhere in the file (a later [other] table would swallow it).
$featuresSection = [regex]::Match($configText, '(?ms)^\[features\][ \t]*\r?$(.*?)(?=^\[|\z)')
if (-not $featuresSection.Success -or $featuresSection.Groups[1].Value -notmatch '(?m)^plugins = true[ \t]*\r?$') {
  Fail "config.toml is missing 'plugins = true' under [features]"
}
Write-Ok "config.toml has the plugin enable entries"

$sentinel = Get-ChildItem -LiteralPath $SkillsDst -Directory |
  ForEach-Object { Join-Path $_.FullName "SKILL.md" } |
  Where-Object { Test-Path -LiteralPath $_ } |
  Select-Object -First 1
if (-not $sentinel -or -not (Test-Path -LiteralPath $sentinel)) {
  Fail "No readable sentinel SKILL.md under $SkillsDst"
}
Get-Content -LiteralPath $sentinel -TotalCount 1 | Out-Null
Write-Ok "Sentinel skill readable: $sentinel"

Write-Host ""
Write-Host "== Verifying install-ao.ps1 output =="

if (-not (Test-Path -LiteralPath $AoExePath)) {
  Fail "ao.exe was not installed: $AoExePath"
}
Write-Ok "ao.exe exists: $AoExePath"

$versionOutput = & $AoExePath version 2>&1
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
  Fail "'ao version' exited $exitCode. Output: $versionOutput"
}
if ([string]::IsNullOrWhiteSpace(($versionOutput | Out-String))) {
  Fail "'ao version' produced no output"
}
Write-Ok "ao version: $(($versionOutput | Out-String).Trim())"

Write-Host ""
Write-Host "All windows installer assertions passed." -ForegroundColor Green
