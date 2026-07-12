#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
SCHEMA="$REPO_ROOT/schemas/pawl-verdict.v1.schema.json"
TMP="$(mktemp)"
trap 'rm -f -- "$TMP"' EXIT

awk '/Terminal artifact example/{seen=1} seen && /^\{/{f=1} f{print} f && /^\}$/{exit}' \
  "$SKILL_DIR/SKILL.md" >"$TMP"

python3 -m jsonschema -i "$TMP" "$SCHEMA" >/dev/null
jq -e '
  .disposition == "CONFIRMED"
  and all(.refuters[]; .verdict == "CONFIRMED")
  and all(.refuters[]; .evidence | test("evidence-round-[0-9]+/lane-[0-9]+\\.json$"))
  and ([.. | objects | has("nonce_echo")] | all(. == false))
' "$TMP" >/dev/null
