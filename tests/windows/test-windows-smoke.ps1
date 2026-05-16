# test-windows-smoke.ps1 - Native Windows smoke tests for AgentOps installers and ao.
#
# Stabilization (soc-92ka, 2026-05-03):
#   - Set-StrictMode -Version Latest enforced (catches uninitialized vars,
#     out-of-bounds indexing, unset properties at runtime instead of producing
#     silent empty strings that flake downstream).
#   - All path construction uses Join-Path or
#     [System.IO.Path]::DirectorySeparatorChar; no hard-coded forward slashes
#     and no embedded backslash separators in path segments.
#   - Nested PowerShell invocations re-use the current $PSHOME pwsh (or
#     fall back to powershell.exe) to avoid execution-policy / host drift
#     between Windows PowerShell 5.1 and PowerShell 7+.
#
# POSIX-symlink-dependent checks: SKIPPED on Windows.
#   This script intentionally does NOT exercise checks that require POSIX
#   symlink semantics. Windows GitHub runners do not create symlinks during
#   `actions/checkout@v6` by default, and the repo policy bans symlinks
#   anyway (see CLAUDE.md "No symlinks. Ever."). Any future check that
#   relies on a POSIX symlink (e.g. `Test-Path -PathType SymbolicLink`,
#   `(Get-Item).LinkType -eq 'SymbolicLink'`, `readlink`-style traversal,
#   or symlink-aware copy/sync) MUST be guarded by `if (-not $IsWindows)`
#   or omitted from this script. The canonical symlink audit lives in
#   the Linux-only `plugin-load-test` job, not here.

[CmdletBinding()]
param(
  [string]$RepoRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# ---------------------------------------------------------------------------
# Path helpers
# ---------------------------------------------------------------------------

# Resolve a chain of segments into an absolute path using Join-Path so the
# host's [System.IO.Path]::DirectorySeparatorChar is always honored. Accepts
# any number of trailing segments.
function Join-PathSegments {
  param(
    [Parameter(Mandatory = $true)][string]$Base,
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Segments
  )
  $current = $Base
  foreach ($segment in $Segments) {
    if ([string]::IsNullOrEmpty($segment)) { continue }
    $current = Join-Path -Path $current -ChildPath $segment
  }
  return $current
}

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
  $scriptRoot = if ($PSScriptRoot) {
    $PSScriptRoot
  } else {
    Split-Path -Parent $MyInvocation.MyCommand.Path
  }
  $repoRootCandidate = Join-PathSegments -Base $scriptRoot -Segments '..', '..'
  $RepoRoot = (Resolve-Path -LiteralPath $repoRootCandidate).Path
}

# Pin the PowerShell host used for nested installer runs. Prefer the same
# host this script is running under (`pwsh` on CI via `shell: pwsh`);
# fall back to Windows PowerShell only if pwsh is unavailable. Mixing
# hosts has been a recurring flake source on Windows runners.
$nestedPSHost = $null
$pshExe = Get-Command -Name 'pwsh' -ErrorAction SilentlyContinue
if ($pshExe) {
  $nestedPSHost = $pshExe.Source
} else {
  $winPS = Get-Command -Name 'powershell' -ErrorAction SilentlyContinue
  if ($winPS) {
    $nestedPSHost = $winPS.Source
  } else {
    throw "Neither pwsh nor powershell is available on PATH"
  }
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

  $cliDir = Join-PathSegments -Base $RepoRoot -Segments 'cli'
  Push-Location -LiteralPath $cliDir
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

function Invoke-NestedScript {
  param(
    [Parameter(Mandatory = $true)][string]$ScriptPath,
    [string[]]$ScriptArgs
  )
  $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ScriptPath)
  if ($ScriptArgs) {
    $argList += $ScriptArgs
  }
  & $nestedPSHost @argList
}

function Remove-IfExists {
  param([string]$Path)
  if ($Path -and (Test-Path -LiteralPath $Path)) {
    Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction SilentlyContinue
  }
}

# ---------------------------------------------------------------------------
# 1. PowerShell installer syntax
# ---------------------------------------------------------------------------

Write-Step "Checking PowerShell installer syntax"
Test-PowerShellSyntax (Join-PathSegments -Base $RepoRoot -Segments 'scripts', 'install-ao.ps1')
Test-PowerShellSyntax (Join-PathSegments -Base $RepoRoot -Segments 'scripts', 'install-codex.ps1')

# ---------------------------------------------------------------------------
# 2. Install ao release binary into a temp directory
# ---------------------------------------------------------------------------

Write-Step "Installing ao release binary into a temp directory"
$aoInstallDir = Join-PathSegments -Base ([System.IO.Path]::GetTempPath()) -Segments ("agentops-ao-smoke-" + [Guid]::NewGuid().ToString("N"))
try {
  $installAoScript = Join-PathSegments -Base $RepoRoot -Segments 'scripts', 'install-ao.ps1'
  Invoke-NestedScript -ScriptPath $installAoScript -ScriptArgs @('-InstallDir', $aoInstallDir, '-NoPathUpdate')
  if ($LASTEXITCODE -ne 0) {
    throw "install-ao.ps1 failed"
  }
  $releaseAO = Join-PathSegments -Base $aoInstallDir -Segments 'ao.exe'
  & $releaseAO version
  if ($LASTEXITCODE -ne 0) {
    throw "installed ao.exe version smoke failed"
  }
}
finally {
  Remove-IfExists -Path $aoInstallDir
}

# ---------------------------------------------------------------------------
# 3. Install Codex plugin into a temp CODEX_HOME
# ---------------------------------------------------------------------------

Write-Step "Installing Codex plugin into a temp CODEX_HOME"
$codexHome = Join-PathSegments -Base ([System.IO.Path]::GetTempPath()) -Segments ("agentops-codex-smoke-" + [Guid]::NewGuid().ToString("N"))
try {
  $installCodexScript = Join-PathSegments -Base $RepoRoot -Segments 'scripts', 'install-codex.ps1'
  Invoke-NestedScript -ScriptPath $installCodexScript -ScriptArgs @('-RepoRoot', $RepoRoot, '-CodexHome', $codexHome)
  if ($LASTEXITCODE -ne 0) {
    throw "install-codex.ps1 failed"
  }
  $pluginRoot = Join-PathSegments -Base $codexHome -Segments 'plugins', 'cache', 'agentops-marketplace', 'agentops', 'local'
  $skillsRoot = Join-PathSegments -Base $pluginRoot -Segments 'skills-codex'
  $metadata = Join-PathSegments -Base $codexHome -Segments '.agentops-codex-install.json'
  if (-not (Test-Path -LiteralPath $skillsRoot)) {
    throw "Codex skills root missing after install: $skillsRoot"
  }
  if (-not (Test-Path -LiteralPath $metadata)) {
    throw "Codex install metadata missing after install: $metadata"
  }
}
finally {
  Remove-IfExists -Path $codexHome
}

# ---------------------------------------------------------------------------
# 4. Build local ao and check Windows doctor hints
# ---------------------------------------------------------------------------

Write-Step "Building local ao and checking Windows doctor hints"
$builtAO = Join-PathSegments -Base ([System.IO.Path]::GetTempPath()) -Segments ("ao-windows-smoke-" + [Guid]::NewGuid().ToString("N") + ".exe")
try {
  $cliDir = Join-PathSegments -Base $RepoRoot -Segments 'cli'
  $aoCmdPkg = Join-PathSegments -Base '.' -Segments 'cmd', 'ao'
  Push-Location -LiteralPath $cliDir
  try {
    & go build -o $builtAO $aoCmdPkg
  }
  finally {
    Pop-Location
  }
  if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
  }

  # The Windows install hints live in the legacy check table, which the plain
  # `ao doctor` still emits — `ao doctor --json` is now the engine Report and
  # carries findings, not those hints. `ao doctor` exits 0 (healthy) or 1
  # (findings present); both are valid diagnostic outcomes, only a higher code
  # is a real failure.
  $doctorText = (& $builtAO doctor 2>&1) -join "`n"
  if ($LASTEXITCODE -gt 1) {
    throw "ao doctor failed (exit $LASTEXITCODE)"
  }
  if ($doctorText -notmatch 'install-codex\.ps1') {
    throw "doctor output did not include the Windows Codex installer hint"
  }
  if ($doctorText -notmatch 'Windows release|WSL/Homebrew') {
    throw "doctor output did not include Windows dependency guidance"
  }
}
finally {
  Remove-IfExists -Path $builtAO
}

# ---------------------------------------------------------------------------
# 5. Focused Windows-sensitive Go tests
# ---------------------------------------------------------------------------

Write-Step "Running focused Windows-sensitive Go tests"
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/quality")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./cmd/ao", "-run", "^(TestBatchForge_appendForgedRecord|TestAppendForgedRecord|TestBatchForgeSkipsAlreadyForged|TestLoadAndFilterTranscripts_RespectsForgedIndex|TestCanonicalArtifactPath|TestCobraDemoConceptsCommand|TestCobraDemoQuickCommand|TestCobraShowConcepts)$")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/storage", "-run", "^TestWithLockedFile_")
Invoke-GoTest -TestArgs @("-timeout", "3m", "./internal/rpi", "-run", "^TestAcquireMergeLock")
Invoke-GoTest -TestArgs @("-timeout", "5m", "./internal/overnight")

Write-Host "Windows smoke tests passed"
