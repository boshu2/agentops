#!/usr/bin/env bash
# Regenerate the two command-document heading projections from the live Cobra
# reference. The Cobra tree itself is executable truth; no handwritten command
# list is maintained here or in tests.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

check=false
[[ "${1:-}" == "--check" ]] && check=true

docs="cli/docs/COMMANDS.md"
smoke="evals/agentops-core/fixtures/cli-command-surface-smoke.sh"
matrix="evals/agentops-core/cli-command-surface-matrix.json"
for file in "$docs" "$smoke" "$matrix"; do
  [[ -f "$file" ]] || { echo "ERROR: missing $file" >&2; exit 2; }
done

top_count="$(grep -cE '^### `ao ' "$docs")"
sub_count="$(grep -cE '^#### `ao ' "$docs")"
all_count="$(grep -cE '^#{3,5} `ao ' "$docs")"
drift=0

rewrite() {
  local source="$1" generated="$2" label="$3"
  if cmp -s "$source" "$generated"; then
    return
  fi
  drift=1
  if $check; then
    echo "DRIFT: $label" >&2
  else
    cp "$generated" "$source"
    echo "updated $source"
  fi
}

tmp_smoke="$(mktemp)"
tmp_matrix="$(mktemp)"
trap 'rm -f "$tmp_smoke" "$tmp_matrix"' EXIT

sed -E \
  -e "s/\[\[ \"\$top_count\" != \"[0-9]+\" \|\| \"\$sub_count\" != \"[0-9]+\" \|\| \"\$all_count\" != \"[0-9]+\" \]\]/[[ \"\$top_count\" != \"${top_count}\" || \"\$sub_count\" != \"${sub_count}\" || \"\$all_count\" != \"${all_count}\" ]]/" \
  -e "s/-ne [0-9]+ \]\]/-ne ${all_count} ]]/" \
  "$smoke" > "$tmp_smoke"
sed -E "s/cli-command-headings: top=[0-9]+ sub=[0-9]+ all=[0-9]+/cli-command-headings: top=${top_count} sub=${sub_count} all=${all_count}/" \
  "$matrix" > "$tmp_matrix"

rewrite "$smoke" "$tmp_smoke" "$smoke heading counts"
rewrite "$matrix" "$tmp_matrix" "$matrix heading counts"

if $check && [[ $drift -ne 0 ]]; then
  echo "command-surface drift — regenerate the CLI reference and rerun this script" >&2
  exit 1
fi
echo "command surfaces in sync (top=$top_count sub=$sub_count all=$all_count)"
