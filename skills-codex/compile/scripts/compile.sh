#!/usr/bin/env bash
# skills-codex/compile/scripts/compile.sh — resilient wrapper.
#
# In a source checkout this delegates to skills/compile/scripts/compile.sh.
# In an installed bundle that copy is not present, so we progressively fall
# back to the co-located real script, and finally to `ao compile` which
# carries its own embedded copy of the compiler and handles runtime
# preflight + batching itself.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

candidates=(
  "$SCRIPT_DIR/../../../skills/compile/scripts/compile.sh"   # repo checkout (skills-codex sibling of skills)
  "$SCRIPT_DIR/../../../../skills/compile/scripts/compile.sh" # legacy path (one level deeper)
  "$SCRIPT_DIR/compile-impl.sh"                               # co-located impl (installed bundle can ship this)
)

for candidate in "${candidates[@]}"; do
  if [[ -f "$candidate" ]]; then
    exec bash "$candidate" "$@"
  fi
done

if command -v ao >/dev/null 2>&1; then
  # Translate common flags through to `ao compile`. ao compile embeds the
  # real compile.sh in the binary, preflights the runtime, and batches large
  # corpora, so it is a superset of invoking the script directly.
  echo "[compile-wrapper] delegating to: ao compile $*" >&2
  exec ao compile "$@"
fi

echo "ERROR: no compile implementation found." >&2
echo "Tried:" >&2
for candidate in "${candidates[@]}"; do
  echo "  $candidate" >&2
done
echo "And 'ao' is not on PATH." >&2
echo "Install the ao CLI: bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)" >&2
exit 1
