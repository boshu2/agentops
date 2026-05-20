#!/usr/bin/env bash
# export-evidence.sh — promote a .agents/ artifact to docs/evidence/<bead-id>/
# so public docs can cite it (.agents/ is gitignored; public claims cannot
# rely on it). See docs/contracts/pmf-evidence.md.
#
# Provenance footer is appended to the promoted file: source path, SHA-256,
# UTC promotion date, bead-id. The footer makes "is this still the same
# evidence?" mechanically checkable.
#
# Usage:
#   scripts/export-evidence.sh <bead-id> <source-path> [<dest-name>]
#   scripts/export-evidence.sh soc-vuu6.33 .agents/research/ablation.md
#   scripts/export-evidence.sh soc-vuu6.33 .agents/research/ablation.md headline.md
#
# Exit codes:
#   0 — promoted (or already up-to-date)
#   1 — drift: a docs/evidence/<bead-id>/<dest> already exists with a
#       different source hash (manual review required)
#   2 — usage error
#   3 — source missing or unreadable

set -euo pipefail

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
  echo "export-evidence: bad arg count" >&2
  usage 2
fi

BEAD_ID="$1"
SRC="$2"
DEST_NAME="${3:-$(basename "$SRC")}"

# Validate bead id shape (soc-xyz123 or soc-xyz.1.2 etc).
if ! printf '%s' "$BEAD_ID" | grep -qE '^[a-z]{2,6}-[0-9a-z.]+$'; then
  echo "export-evidence: bead id '$BEAD_ID' does not match ^[a-z]{2,6}-[0-9a-z.]+$" >&2
  exit 2
fi

if [ ! -r "$SRC" ]; then
  echo "export-evidence: source not readable: $SRC" >&2
  exit 3
fi

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
DEST_DIR="$ROOT/docs/evidence/$BEAD_ID"
DEST_PATH="$DEST_DIR/$DEST_NAME"
mkdir -p "$DEST_DIR"

# Compute source SHA-256 (portable: sha256sum on linux, shasum -a 256 on mac).
src_hash() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

SRC_HASH="$(src_hash "$SRC")"

# If the destination already exists, check for drift before overwriting. We
# treat the destination's existing provenance footer (or its absence) as the
# baseline and refuse to silently replace evidence that came from a different
# source hash. Operator must remove the file and re-promote intentionally.
if [ -f "$DEST_PATH" ]; then
  existing_hash="$(grep -oE 'source-sha256: [0-9a-f]{64}' "$DEST_PATH" | awk '{print $2}' | head -1 || true)"
  if [ -n "$existing_hash" ] && [ "$existing_hash" != "$SRC_HASH" ]; then
    echo "export-evidence: DRIFT — $DEST_PATH was promoted from a different source." >&2
    echo "  existing source-sha256: $existing_hash" >&2
    echo "  current  source-sha256: $SRC_HASH" >&2
    echo "  Remove the file and re-run to confirm the new evidence is intentional." >&2
    exit 1
  fi
  if [ -n "$existing_hash" ] && [ "$existing_hash" = "$SRC_HASH" ]; then
    echo "export-evidence: $DEST_PATH already up-to-date"
    exit 0
  fi
fi

# Copy the source verbatim, then append the provenance footer. We use cp
# (preserves nothing extra) and then atomic-write via mv so partial files
# don't leave a half-promoted artifact.
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
cp "$SRC" "$TMP"
{
  # Newlines + horizontal rule + provenance comment. `printf '%s\n' '---'`
  # avoids printf treating '---' as a flag, which happens when '---' appears
  # at the start of a format string (e.g. `printf '\n---\n'`).
  printf '\n\n'
  printf '%s\n' '---'
  printf '\n%s\n' '<!--'
  printf 'provenance:\n'
  printf '  source-path: %s\n' "$SRC"
  printf '  source-sha256: %s\n' "$SRC_HASH"
  printf '  promoted-at: %s\n' "$(date -u +%FT%TZ)"
  printf '  bead-id: %s\n' "$BEAD_ID"
  printf '  promoter: scripts/export-evidence.sh\n'
  printf '%s\n' '-->'
} >> "$TMP"
mv "$TMP" "$DEST_PATH"
trap - EXIT

echo "export-evidence: wrote $DEST_PATH"
echo "  source: $SRC"
echo "  bead: $BEAD_ID"
echo "  sha256: $SRC_HASH"
