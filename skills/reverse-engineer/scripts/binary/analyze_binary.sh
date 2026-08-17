#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: analyze_binary.sh <binary_path> <out_dir>" >&2
  exit 2
fi

BIN="$1"
OUT="$2"
if [[ ! -f "$BIN" || ! -x "$BIN" || -L "$BIN" ]]; then
  echo "error: binary must be an executable regular non-symlink file" >&2
  exit 2
fi
[[ ! -L "$OUT" ]] || { echo "error: output directory must not be a symlink" >&2; exit 2; }
binary_size="$(wc -c < "$BIN" | tr -d ' ')"
[[ "$binary_size" -le $((256 * 1024 * 1024)) ]] \
  || { echo "error: binary exceeds 268435456-byte analysis bound" >&2; exit 2; }
TIMEOUT_BIN=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null 2>&1; then TIMEOUT_BIN="$(command -v "$candidate")"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || { echo "error: timeout or gtimeout is required" >&2; exit 2; }
mkdir -p "$OUT"

run_bounded() {
  "$TIMEOUT_BIN" --signal=TERM --kill-after=1s 10 "$@"
}

redact_paths() {
  sed -E 's#(^|[[:space:]])/[^[:space:]]+#\1[REDACTED-PATH]#g'
}

{
  echo "# Binary Analysis (Best-Effort)"
  echo
  echo "- Target: \`$(basename "$BIN")\`"
  echo "- Generated: $(date +%F)"
  echo
  echo "## file(1)"
  echo
  if command -v file >/dev/null 2>&1; then
    run_bounded file -b "$BIN" | head -c 65536 | redact_paths || true
  else
    echo "_file not available_"
  fi
  echo
  echo "## Linked Libraries (best-effort)"
  echo
  if command -v otool >/dev/null 2>&1; then
    run_bounded otool -L "$BIN" 2>/dev/null | head -c 65536 | redact_paths || true
  elif command -v ldd >/dev/null 2>&1; then
    run_bounded ldd "$BIN" 2>/dev/null | head -c 65536 | redact_paths || true
  else
    echo "_otool/ldd not available_"
  fi
  echo
  echo "## Language Heuristics (best-effort)"
  echo
  if command -v strings >/dev/null 2>&1; then
    # Cache strings output to a temp file for multiple scans
    _STRINGS_FILE=$(mktemp)
    trap 'rm -f "$_STRINGS_FILE"' EXIT
    run_bounded strings -a "$BIN" 2>/dev/null | head -c $((4 * 1024 * 1024)) >"$_STRINGS_FILE" || true

    # Helper: search strings file with rg falling back to grep -E
    _str_match() {
      local pattern="$1"
      if command -v rg >/dev/null 2>&1; then
        rg -m 1 "$pattern" "$_STRINGS_FILE" 2>/dev/null
      else
        grep -E -m 1 "$pattern" "$_STRINGS_FILE" 2>/dev/null
      fi
    }

    # --- Go detection (broad markers for stripped binaries) ---
    GO_DETECTED=false
    GO_MARKER=""
    # Original markers (unstripped binaries)
    if _str_match 'runtime\.morestack|go\.buildid|Go build ID|type\.\*runtime\.' >/dev/null 2>&1; then
      GO_DETECTED=true; GO_MARKER="Go runtime markers"
    # Broader markers for stripped binaries (version strings, GOROOT, module paths)
    elif _str_match 'go1\.[0-9]|GOROOT|github\.com/|golang\.org/' >/dev/null 2>&1; then
      GO_DETECTED=true; GO_MARKER="Go version/module strings"
    fi

    # --- Python detection ---
    PYTHON_DETECTED=false
    if _str_match '__pycache__|\.pyc|Py_Initialize|libpython|python[0-9]\.[0-9]' >/dev/null 2>&1; then
      PYTHON_DETECTED=true
    fi

    # --- Report language ---
    if $GO_DETECTED && $PYTHON_DETECTED; then
      echo "- Likely language/runtime: Go + Python (Go binary embedding Python code)"
      echo "  - Go detection: $GO_MARKER"
    elif $GO_DETECTED; then
      echo "- Likely language/runtime: Go (heuristic: $GO_MARKER)"
    elif $PYTHON_DETECTED; then
      echo "- Likely language/runtime: Python (heuristic: Python runtime markers in strings)"
    else
      echo "- Likely language/runtime: unknown (no Go or Python markers found)"
    fi

    # --- Go details (version, module, packages) ---
    if $GO_DETECTED; then
      echo
      echo "### Go Details"
      echo
      # Go version string (e.g. "go1.23.4") — match lines that ARE the version
      _go_ver=$({
        if command -v rg >/dev/null 2>&1; then
          rg -m 1 -o '^go1\.[0-9]+\.[0-9]+$' "$_STRINGS_FILE" 2>/dev/null
        else
          grep -E -m 1 '^go1\.[0-9]+\.[0-9]+$' "$_STRINGS_FILE" 2>/dev/null
        fi
      } || true)
      if [[ -n "$_go_ver" ]]; then
        echo "- Go version: \`$_go_ver\`"
      else
        echo "- Go version: _not found (stripped)_"
      fi
      # Module path — prefer github.com/gitlab.com/golang.org paths first
      _go_mod=$({
        if command -v rg >/dev/null 2>&1; then
          rg -m 1 -o '^(github|gitlab|bitbucket)\.com/[^\s]+' "$_STRINGS_FILE" 2>/dev/null \
          || rg -m 1 -o '^golang\.org/[^\s]+' "$_STRINGS_FILE" 2>/dev/null \
          || rg -m 1 -o '^[a-z][a-z0-9.-]+\.[a-z]{2,}/[^\s]+' "$_STRINGS_FILE" 2>/dev/null
        else
          grep -E -m 1 -o '^(github|gitlab|bitbucket)\.com/[^ ]+' "$_STRINGS_FILE" 2>/dev/null \
          || grep -E -m 1 -o '^golang\.org/[^ ]+' "$_STRINGS_FILE" 2>/dev/null \
          || grep -E -m 1 -o '^[a-z][a-z0-9.-]+\.[a-z]{2,}/[^ ]+' "$_STRINGS_FILE" 2>/dev/null
        fi
      } || true)
      if [[ -n "$_go_mod" ]]; then
        echo "- Module path marker: present (raw value intentionally not retained)"
      else
        echo "- Module path: _not found_"
      fi
      # Internal package count (unique Go module-style paths)
      _go_pkgs=$({
        if command -v rg >/dev/null 2>&1; then
          rg -o '^(github|gitlab|bitbucket)\.com/[^\s]+|^golang\.org/[^\s]+' "$_STRINGS_FILE" 2>/dev/null
        else
          grep -E -o '^(github|gitlab|bitbucket)\.com/[^ ]+|^golang\.org/[^ ]+' "$_STRINGS_FILE" 2>/dev/null
        fi
      } | sort -u | wc -l || echo 0)
      echo "- Internal packages (approx): ${_go_pkgs##* }"
    fi
  else
    echo "- strings not available; cannot run heuristics"
  fi
  echo
  echo "## Embedded Archive Signatures (ZIP, best-effort)"
  echo
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$BIN" <<'PY'
import sys
from pathlib import Path

p = Path(sys.argv[1])
sig = b"PK\x03\x04"
hits = []
offset = 0
carry = b""
with p.open("rb") as handle:
    while chunk := handle.read(1024 * 1024):
        data = carry + chunk
        start = 0
        while len(hits) < 5000:
            i = data.find(sig, start)
            if i < 0:
                break
            hits.append(offset - len(carry) + i)
            start = i + 1
        carry = data[-3:]
        offset += len(chunk)

print(f"- ZIP local header occurrences: {len(hits)}")
for i in hits[:10]:
    print(f"  - offset: {i}")
if len(hits) > 10:
    print("  - ...")
PY
  else
    echo "_python3 not available_"
  fi
} >"$OUT/binary-analysis.md"

# Raw strings and disassembly are deliberately not retained. The report keeps
# bounded aggregate heuristics only, so a secret-bearing binary cannot turn
# arbitrary string lines into evidence artifacts.
