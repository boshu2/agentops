#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DOCS_PATH="$REPO_ROOT/cli/docs/COMMANDS.md"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

(cd "$REPO_ROOT/cli" && env -u AGENTOPS_RPI_RUNTIME go build -o "$tmp_dir/ao" ./cmd/ao)

top_count="$(rg -c '^### `ao ' "$DOCS_PATH")"
sub_count="$(rg -c '^#### `ao ' "$DOCS_PATH")"
all_count="$(rg -c '^#{3,4} `ao ' "$DOCS_PATH")"

if [[ "$top_count" != "73" || "$sub_count" != "195" || "$all_count" != "268" ]]; then
  printf 'unexpected command heading counts: top=%s sub=%s all=%s\n' "$top_count" "$sub_count" "$all_count" >&2
  exit 1
fi

# shellcheck disable=SC2016 # literal backticks delimit generated Markdown command headings.
mapfile -t commands < <(rg '^#{3,4} `ao ' "$DOCS_PATH" | sed -E 's/^.*`([^`]+)`.*/\1/')

if [[ "${#commands[@]}" -ne 268 ]]; then
  printf 'unexpected command matrix size: %s\n' "${#commands[@]}" >&2
  exit 1
fi

for command in "${commands[@]}"; do
  args="${command#ao }"
  read -r -a argv <<<"$args"
  if ! "$tmp_dir/ao" "${argv[@]}" --help >/dev/null; then
    printf 'help failed: %s\n' "$command" >&2
    exit 1
  fi
done

printf 'cli-command-headings: top=%s sub=%s all=%s\n' "$top_count" "$sub_count" "$all_count"
printf 'cli-help-matrix-ok\n'
