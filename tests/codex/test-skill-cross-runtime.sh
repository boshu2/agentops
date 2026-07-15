#!/usr/bin/env bash
# Prove one canonical skill tree links consistently across runtime homes.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ao-cross-runtime.XXXXXX")"
AO="$TMP_ROOT/ao"
trap 'rm -rf "$TMP_ROOT"' EXIT

(cd "$REPO_ROOT/cli" && go build -o "$AO" ./cmd/ao)
mkdir -p "$TMP_ROOT/home"/{.agents,.claude,.codex,.gemini}

first=$(cd "$REPO_ROOT" && HOME="$TMP_ROOT/home" "$AO" skills link --json)
second=$(cd "$REPO_ROOT" && HOME="$TMP_ROOT/home" "$AO" skills link --json)

jq -e 'length == 4 and all(.[]; (.linked | length) == 50 and (.conflicts | length) == 0)' >/dev/null <<<"$first"
jq -e 'length == 4 and all(.[]; (.linked | length) == 0 and (.present | length) == 50 and (.conflicts | length) == 0)' >/dev/null <<<"$second"

for runtime in .agents .claude .codex .gemini; do
  test "$(find "$TMP_ROOT/home/$runtime/skills" -type l | wc -l | tr -d ' ')" -eq 50
  test "$(readlink "$TMP_ROOT/home/$runtime/skills/rpi")" = "$REPO_ROOT/skills/rpi"
done

echo "PASS: 50 canonical skills are live-linked and idempotent across four runtime homes"
