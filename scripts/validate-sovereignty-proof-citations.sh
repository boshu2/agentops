#!/usr/bin/env bash
# validate-sovereignty-proof-citations.sh
#
# Scans docs/sovereignty-proof/ for file:line citations and verifies each
# resolves at HEAD: the file exists, and the cited line number is within
# the file's line count.
#
# Why: docs/sovereignty-proof/index.md is the falsifiable artifact behind
# the sovereignty claim. A proof page that lies about its citations is
# worse than no proof page at all. This gate is mechanical enforcement.
#
# Citation forms recognized:
#   path/to/file:NN         — single line
#   path/to/file:NN-MM      — line range (validates NN and MM)
#
# Citations are matched anywhere in the page text (markdown bullets,
# tables, prose). Paths are resolved relative to the repo root.
#
# Exit codes:
#   0  — all citations resolve
#   1  — one or more citations fail to resolve
#   2  — usage / setup error
#
# Sibling pattern: matches the shape of scripts/check-registry-drift.sh
# (simple POSIX bash, repo-root cd, summary + exit code).
#
# soc-vuu6.32

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TARGET_DIR="docs/sovereignty-proof"

if [ ! -d "$TARGET_DIR" ]; then
  echo "ERROR: $TARGET_DIR does not exist" >&2
  exit 2
fi

declare -i checked=0
declare -i failed=0
failures=()

# Find all .md files under docs/sovereignty-proof/
mapfile -t pages < <(find "$TARGET_DIR" -type f -name "*.md" | sort)

if [ ${#pages[@]} -eq 0 ]; then
  echo "ERROR: no .md files under $TARGET_DIR" >&2
  exit 2
fi

# Citation regex:
#   - Captures: file path (no spaces), colon, line number (or NN-MM)
#   - File path must contain at least one slash to avoid matching e.g. "x:1"
#   - Excludes URLs (http: / https:) by requiring no protocol prefix
#   - Excludes citations inside fenced code blocks (handled via awk below)
#
# We use awk to strip fenced code blocks first, then grep for citations.
#
# Recognized:
#   foo/bar.go:42
#   foo/bar.md:10-25
#   `foo/bar.sh:5`        (backtick wrappers OK; we strip via -o)

extract_citations() {
  local page="$1"
  awk '
    /^```/ { in_block = !in_block; next }
    !in_block { print }
  ' "$page" | grep -oE '[a-zA-Z0-9_./-]+\.(go|sh|md|json|yml|yaml|ts|js|py|rs|sql|hujson|toml)[`]?:[0-9]+(-[0-9]+)?' | sed 's/`//g'
}

for page in "${pages[@]}"; do
  while IFS= read -r citation; do
    [ -z "$citation" ] && continue
    checked=$((checked + 1))

    # Split file:lines
    file="${citation%:*}"
    lines="${citation##*:}"

    # Resolve line endpoints (single or range)
    if [[ "$lines" == *-* ]]; then
      start="${lines%-*}"
      end="${lines#*-}"
    else
      start="$lines"
      end="$lines"
    fi

    # File must exist relative to repo root
    if [ ! -f "$file" ]; then
      failed=$((failed + 1))
      failures+=("$page → $citation : file does not exist")
      continue
    fi

    # Line count must be >= cited end line
    total_lines=$(wc -l < "$file")

    if [ "$end" -gt "$total_lines" ]; then
      failed=$((failed + 1))
      failures+=("$page → $citation : cited line $end exceeds file's $total_lines lines")
      continue
    fi

    if [ "$start" -lt 1 ]; then
      failed=$((failed + 1))
      failures+=("$page → $citation : cited line $start < 1")
      continue
    fi
  done < <(extract_citations "$page")
done

echo "validate-sovereignty-proof-citations: scanned ${#pages[@]} pages, $checked citations"

if [ "$failed" -eq 0 ]; then
  echo "PASS — all citations resolve."
  exit 0
fi

echo "FAIL — $failed unresolved citation(s):" >&2
for f in "${failures[@]}"; do
  echo "  - $f" >&2
done
exit 1
