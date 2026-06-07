#!/usr/bin/env bash
# close-guard.sh — AfterTool guard enforcing Flywheel Rules 2 & 5
# (evidence-gated close, persist before done).
#
# Wired into ~/.gemini/settings.json as an AfterTool/run_shell_command hook. It
# inspects a `br close` / `bd close` and rejects the close if the bead has no
# captured evidence linked. This is a POLICY STUB: wired and safe (no-op) until
# you implement reading the live tracker's evidence field. Implement the marked
# section against your tracker so it ENFORCES rather than no-ops.
#
# Exit 0 = allow, non-zero = block the close.
set -euo pipefail

cmd="${*:-}"
if [ -z "$cmd" ] && [ ! -t 0 ]; then cmd="$(cat 2>/dev/null || true)"; fi

# Only act on a close; everything else passes through.
case "$cmd" in
  *"br close"*|*"bd close"*) : ;;
  *) exit 0 ;;
esac

# Extract the bead id (best-effort: first token after 'close').
bead="$(printf '%s\n' "$cmd" | sed -nE 's/.*(br|bd) close[[:space:]]+([A-Za-z0-9_-]+).*/\2/p' | head -1)"
[ -n "$bead" ] || exit 0

# --- IMPLEMENT: confirm the bead carries captured evidence before allowing close
# Example shape (uncomment + adapt to your tracker):
#   ev="$(br show "$bead" --json 2>/dev/null | jq -r '.evidence // empty' || true)"
#   [ -n "$ev" ] || { echo "close-guard: bead $bead has no captured evidence (Rule 2)" >&2; exit 1; }
#   [ -f ".agents/ratchet/$bead.md" ] || { echo "close-guard: bead $bead not persisted (Rule 5)" >&2; exit 1; }
# -----------------------------------------------------------------------------

# Stub default: allow (no-op) until the evidence read above is implemented.
exit 0
