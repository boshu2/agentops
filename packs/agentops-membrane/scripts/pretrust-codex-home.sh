#!/usr/bin/env bash
# pretrust-codex-home.sh — seed a city-scoped, pre-trusted CODEX_HOME.
#
# Residual gap 2 (RESIDUAL-GAPS.md): a fresh codex verifier session can wedge
# at startup on the user-global ~/.codex/hooks.json TRUST MODAL — the lane
# blocks awaiting an interactive click and never produces its lane JSON (the
# gate then correctly DEGRADES every round; fitness-run gap B). This is a
# codex-CLI startup gate, outside the pack's runtime reach — the durable
# mitigation is a SETUP step: give city codex sessions their own clean,
# pre-trusted CODEX_HOME so the modal never fires and the operator's global
# codex config stays untouched.
#
# Usage: pretrust-codex-home.sh <city-dir>
#   Creates <city>/.gc/codex-home with a trusted (empty) hooks.json and prints
#   the exact city.toml stanza that wires it into the codex provider (TOML
#   cannot expand variables, so the literal path is printed for copy-paste;
#   install-gc-city.sh applies it automatically).
set -euo pipefail

CITY="${1:?usage: pretrust-codex-home.sh <city-dir>}"
CITY="$(cd "$CITY" && pwd)"
HOME_DIR="$CITY/.gc/codex-home"

mkdir -p "$HOME_DIR"
[ -s "$HOME_DIR/hooks.json" ] || printf '{}\n' > "$HOME_DIR/hooks.json"

echo "pretrust-codex-home: seeded $HOME_DIR (hooks.json trusted-empty)"
echo
echo "Wire it into the city's codex provider (city.toml) — idempotent to re-add:"
echo
echo '  [providers.codex]'
echo '  base = "builtin:codex"'
echo "  env = { CODEX_HOME = \"$HOME_DIR\" }"
echo
echo "agy note: the equivalent agy/antigravity trust prompt has no file seed —"
echo "run the provider once interactively and accept it (one-time, per host)."
