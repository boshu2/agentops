#!/usr/bin/env bash
# Thin compatibility wrapper over `ao gc` — the Gas City maintainer operations
# ship in the Go CLI (ADR-0016: skill logic ships in Go via ao; shell stays thin
# glue). New callers should invoke `ao gc prepare|check|recover-affinity`
# directly; this wrapper keeps older runbooks working from a repo checkout.
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
set -euo pipefail

die() {
  printf 'gc-maintainer-ops: %s\n' "$*" >&2
  exit 1
}

repo_root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

command_name="${1:-}"
case "$command_name" in
  prepare|check|recover-affinity) ;;
  -h|--help|"")
    cat <<'EOF'
Usage:
  scripts/gc-maintainer-ops.sh prepare --city PATH --rig PATH [options]
  scripts/gc-maintainer-ops.sh check --city PATH --rig PATH [options]
  scripts/gc-maintainer-ops.sh recover-affinity --city PATH --rig PATH [--apply]

Thin wrapper over `ao gc`; see `ao gc --help` for the full option list.
--ao-bin PATH selects the ao binary (default: ao on PATH).
EOF
    exit 0
    ;;
  *) die "unknown command: $command_name" ;;
esac
shift

ao_bin=""
args=("$command_name")
while [ "$#" -gt 0 ]; do
  case "$1" in
    --ao-bin) ao_bin="${2:?--ao-bin requires a path}"; shift 2 ;;
    *) args+=("$1"); shift ;;
  esac
done

if [ -z "$ao_bin" ]; then
  ao_bin="$(command -v ao || true)"
fi
[ -n "$ao_bin" ] && [ -x "$ao_bin" ] ||
  die "ao binary not found; install the AgentOps CLI or pass --ao-bin"

# Preserve the historical wrapper semantic: skills always come from this
# checkout. recover-affinity takes no skills flag.
case "$command_name" in
  prepare|check) args+=(--skills-source "$repo_root/skills") ;;
esac

exec "$ao_bin" gc "${args[@]}"
