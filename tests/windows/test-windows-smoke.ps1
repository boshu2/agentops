# test-windows-smoke.ps1 - Native Windows smoke tests for AgentOps installers and ao.

[CmdletBinding()]
param(
  [string]$RepoRoot
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
  $scriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
  $RepoRoot = (Resolve-Path (Join-Path $scriptRoot "..\..")).Path
}

function Write-Step {
  param([string]$Message)
  Write-Host "==> $Message"
}

function Test-PowerShellSyntax {
  param([string]$Path)

  $errors = $null
  $null = [System.Management.Automation.PSParser]::Tokenize((Get-Content -Raw -LiteralPath $Path), [ref]$errors)
  if ($errors) {
    $errors | ForEach-Object { Write-Error $_ }
    throw "PowerShell syntax check failed: $Path"
  }
}

function Invoke-GoTest {
  param([string[]]$TestArgs)

  Push-Location (Join-Path $RepoRoot "cli")
  try {
    & go test @TestArgs
  }
  finally {
    Pop-Location
  }
  if ($LASTEXITCODE -ne 0) {
    throw "go test failed: $($TestArgs -join ' ')"
  }
}

Write-Step "Checking PowerShell installer syntax"
Test-PowerShellSyntax (Join-Path $RepoRoot "scripts\install-ao.ps1")
Test-PowerShellSyntax (Join-Path $RepoRoot "scripts\install-codex.ps1")

Write-Step "Installing ao release binary into a temp directory"
$aoInstallDir = Join-Path ([System.IO.Path]::GetTempPath()) ("agentops-ao-smoke-" + [Guid]::NewGuid().ToString("N"))
try {
  & powershell -ExecutionPolicy Bypass -File (Join-Path $RepoRoot "scripts\install-ao.ps1") -InstallDir $aoInstallDir -NoPathUpdate
  if ($LASTEXITCODE -ne 0) {
    throw "install-ao.ps1 failed"
  }
  $releaseAO = Join-Path $aoInstallDir "ao.exe"
  & $releaseAO version
  if ($LASTEXITCODE -ne 0) {
    throw "installed ao.exe version smoke failed"
  }
}
finally {
  if (Test-Path -LiteralPath $aoInstallDir) {
    Remove-Item -LiteralPath $aoInstallDir -Recurse -Force
  }
}

Write-Step "Installing Codex plugin into a temp CODEX_HOME"
$codexHome = Join-Path ([System.IO.Path]::GetTempPath()) ("agentops-codex-smoke-" + [Guid]::NewGuid().ToString("N"))
try {
  & powershell -ExecutionPolicy Bypass -File (Join-Path $RepoRoot "scripts\install-codex.ps1") -RepoRoot $RepoRoot -CodexHome $codexHome
  if ($LASTEXITCODE -ne 0) {
    throw "install-codex.ps1 failed"
  }
  $pluginRoot = Join-Path $codexHome "plugins\cache\agentops-marketplace\agentops\local"
  $skillsRoot = Join-Path $pluginRoot "skills-codex"
  $metadata = Join-Path $codexHome ".agentops-codex-install.json"
  if (-not (Test-Path -LiteralPath $skillsRoot)) {
    throw "Codex skills root missing after install: $skillsRoot"
  }
  if (-not (Test-Path -LiteralPath $metadata)) {
    throw "Codex install metadata missing after install: $metadata"
  }
}
finally {
  if (Test-Path -LiteralPath $codexHome) {
    Remove-Item -LiteralPath $codexHome -Recurse -Force
  }
}

Write-Step "Building local ao and checking Windows doctor hints"
$builtAO = Join-Path ([System.IO.Path]::GetTempPath()) ("ao-windows-smoke-" + [Guid]::NewGuid().ToString("N") + ".exe")
try {
  Push-Location (Join-Path $RepoRoot "cli")
  try {
    & go build -o $builtAO .\cmd\ao
  }
  finally {
    Pop-Location
  }
  if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
  }

  $doctorJSON = & $builtAO doctor --json
  if ($LASTEXITCODE -ne 0) {
    throw "ao doctor --json failed"
  }
  $doctorText = ($doctorJSON -join "`n")
  if ($doctorText -notmatch "install-codex\.ps1") {
    throw "doctor output did not include the Windows Codex installer hint"
  }
  if ($doctorText -notmatch "Windows release|WSL/Homebrew") {
    throw "doctor output did not include Windows dependency guidance"
  }
}
finally {
  if (Test-Path -LiteralPath $builtAO) {
    Remove-Item -LiteralPath $builtAO -Force
  }
}

Write-Step "Running focused Windows-sensitive Go tests"
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/quality")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./cmd/ao", "-run", "^(TestBatchForge_appendForgedRecord|TestAppendForgedRecord|TestBatchForgeSkipsAlreadyForged|TestLoadAndFilterTranscripts_RespectsForgedIndex|TestCanonicalArtifactPath|TestCobraDemoConceptsCommand|TestCobraDemoQuickCommand|TestCobraShowConcepts)$")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/storage", "-run", "^TestWithLockedFile_")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/rpi", "-run", "^TestAcquireMergeLock")

Write-Host "Windows smoke tests passed"
