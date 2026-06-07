#!/usr/bin/env bash
# init-ledger.sh — scaffold a GAUNTLET-LEDGER.md for a Rust crate gauntlet run.
# Usage: init-ledger.sh [ledger-path]   (default: GAUNTLET-LEDGER.md)
# Idempotent: refuses to clobber an existing ledger.
set -euo pipefail

LEDGER="${1:-GAUNTLET-LEDGER.md}"

if [[ -e "$LEDGER" ]]; then
  echo "Ledger already exists at $LEDGER — leaving it untouched." >&2
  exit 0
fi

CRATE="$(awk -F'"' '/^name *=/{print $2; exit}' Cargo.toml 2>/dev/null || echo unknown)"
TOOLCHAIN="$(rustc -Vv 2>/dev/null | head -1 || echo 'rustc not found')"
DATE="$(date +%Y-%m-%d)"

cat > "$LEDGER" <<EOF
# Gauntlet Ledger — ${CRATE}

Toolchain: ${TOOLCHAIN}
Features: <fill in enabled features>
Snapshot taken: ${DATE}

## Gate matrix
| Gate   | Status  | Last run | Notes |
| ------ | ------- | -------- | ----- |
| fmt    | UNKNOWN | -        |       |
| build  | UNKNOWN | -        |       |
| clippy | UNKNOWN | -        |       |
| test   | UNKNOWN | -        |       |
| miri   | UNKNOWN | -        |       |
| fuzz   | UNKNOWN | -        |       |
| bench  | UNKNOWN | -        |       |

## Negative-evidence log (never retry these)
| Gate | Approach tried | Why it failed | Date |
| ---- | -------------- | ------------- | ---- |
EOF

echo "Initialized $LEDGER for crate '$CRATE'."
