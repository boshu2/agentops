#!/usr/bin/env bash
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

root="$REPO_ROOT"
factory="$root/packs/agentops-factory"
executor="$root/packs/agentops-executor"
cd "$root" || exit 2
python3 -m unittest tests.python.test_gc33_thin_pack
python3 -m unittest tests.python.test_gc_maintainer_ops
bash -n "$root/scripts/gc-maintainer-ops.sh"
for script in "$root"/deploy/gc/*.sh; do bash -n "$script"; done
gc_bin="${GC_BIN:-}"
if [ -n "$gc_bin" ]; then
  [ -x "$gc_bin" ] || { printf 'GC_BIN is not executable: %s\n' "$gc_bin" >&2; exit 2; }
  "$gc_bin" lint "$factory" --json >/dev/null
  "$gc_bin" lint "$executor" --json >/dev/null
fi
