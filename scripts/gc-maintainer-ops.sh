#!/usr/bin/env bash
# Prepare and qualify the stock Gas City maintainer pack without owning a pack.
set -euo pipefail

readonly MAINTAINER_COMMIT="3b3b89f2011e06d84459aa7bea1552382f13930a"
readonly WORKFLOW_SOURCE="https://github.com/gastownhall/gascity-packs/tree/main/gascity"
readonly ROLES_SOURCE="${WORKFLOW_SOURCE}/roles"
readonly MANAGED_MARKER="managed-by: agentops gc-maintainer-ops"
readonly REQUIRED_SKILLS="using-gc plan implement test validate"

die() {
  printf 'gc-maintainer-ops: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  scripts/gc-maintainer-ops.sh prepare --city PATH --rig PATH [options]
  scripts/gc-maintainer-ops.sh check --city PATH --rig PATH [options]
  scripts/gc-maintainer-ops.sh recover-affinity --city PATH --rig PATH [--apply] [options]

Options:
  --gc-bin PATH    Gas City 1.4 binary (default: command -v gc)
  --ao-bin PATH    AgentOps CLI used to link skills (default: command -v ao)
  --pack-dir PATH  Resolved official gascity pack root (normally auto-detected)
  --apply          Apply recover-affinity; its default is read-only

prepare verifies the official workflow and rig-role pins, snapshots upstream
validation assets unchanged under the rig's ignored .gc directory, installs
AgentOps-owned check wrappers, and links AgentOps skills into city/rig Codex
sinks. check is read-only. recover-affinity only considers ready formula beads
with gc.session_affinity=require and never re-slings work.
EOF
}

canonical() {
  local path="$1"
  if command -v realpath >/dev/null 2>&1; then
    realpath "$path"
    return
  fi
  (
    cd -P "$(dirname "$path")" 2>/dev/null
    printf '%s/%s\n' "$PWD" "$(basename "$path")"
  )
}

quote_sh() {
  local value="$1"
  value="${value//\'/\'\\\'\'}"
  printf "'%s'" "$value"
}

json_or_die() {
  local description="$1"
  shift
  local output
  output="$("$@")" || die "$description"
  jq -e . >/dev/null 2>&1 <<<"$output" || die "$description returned malformed JSON"
  printf '%s' "$output"
}

command_name="${1:-}"
case "$command_name" in
  prepare|check|recover-affinity) shift ;;
  -h|--help|"") usage; exit 0 ;;
  *) die "unknown command: $command_name" ;;
esac

city=""
rig=""
gc_bin=""
ao_bin=""
pack_dir=""
apply=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --city) city="${2:?--city requires a path}"; shift 2 ;;
    --rig) rig="${2:?--rig requires a path}"; shift 2 ;;
    --gc-bin) gc_bin="${2:?--gc-bin requires a path}"; shift 2 ;;
    --ao-bin) ao_bin="${2:?--ao-bin requires a path}"; shift 2 ;;
    --pack-dir) pack_dir="${2:?--pack-dir requires a path}"; shift 2 ;;
    --apply) apply=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[ -n "$city" ] && [ -n "$rig" ] || die "--city and --rig are required"
[ -d "$city" ] || die "city directory does not exist: $city"
[ -d "$rig" ] || die "rig directory does not exist: $rig"
city="$(canonical "$city")"
rig="$(canonical "$rig")"
repo_root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "$gc_bin" ]; then
  gc_bin="$(command -v gc || true)"
fi
[ -n "$gc_bin" ] && [ -x "$gc_bin" ] || die "gc binary is not executable"
gc_bin="$(canonical "$gc_bin")"

if [ -z "$ao_bin" ]; then
  ao_bin="$(command -v ao || true)"
fi
if [ "$command_name" = "prepare" ]; then
  [ -n "$ao_bin" ] && [ -x "$ao_bin" ] || die "ao binary is required for prepare"
  ao_bin="$(canonical "$ao_bin")"
fi
command -v jq >/dev/null 2>&1 || die "jq is required"

rigs_json="$(json_or_die "cannot list city rigs" "$gc_bin" --city "$city" rig list --json)"
rig_name=""
rig_match_count=0
while IFS=$'\t' read -r candidate_name candidate_path; do
  [ -n "$candidate_name" ] && [ -n "$candidate_path" ] || continue
  candidate_path="$(canonical "$candidate_path" 2>/dev/null || true)"
  if [ "$candidate_path" = "$rig" ]; then
    rig_name="$candidate_name"
    rig_match_count=$((rig_match_count + 1))
  fi
done < <(
  jq -r '.rigs[]? | select((.hq // false) == false) | [.name, .path] | @tsv' \
    <<<"$rigs_json"
)
[ "$rig_match_count" -eq 1 ] || die "rig path is not an exact non-HQ rig in this city: $rig"

imports_json="$(json_or_die "cannot inspect installed imports" "$gc_bin" --city "$city" import status --json)"
if ! jq -e \
  --arg workflow "$WORKFLOW_SOURCE" \
  --arg roles "$ROLES_SOURCE" \
  --arg commit "$MAINTAINER_COMMIT" \
  --arg rig_import "rig:${rig_name}:gc" '
    (.ok == true)
    and any(.imports[]?;
      .source == $workflow and .pin.commit == $commit)
    and any(.imports[]?;
      .name == $rig_import and .source == $roles and .pin.commit == $commit)
  ' <<<"$imports_json" >/dev/null; then
  die "official gascity workflow and rig-role pins are not both installed at ${MAINTAINER_COMMIT}"
fi

resolve_pack_dir() {
  if [ -n "$pack_dir" ]; then
    [ -d "$pack_dir" ] || die "pack directory does not exist: $pack_dir"
    canonical "$pack_dir"
    return
  fi

  local gc_home="${GC_HOME:-${HOME:?HOME is required}/.gc}"
  local marker candidate commit
  for marker in "$gc_home"/cache/repos/*/.gc-bundled-pack-cache.toml; do
    [ -f "$marker" ] || continue
    commit="$(
      sed -n 's/^[[:space:]]*commit[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' \
        "$marker" | head -1
    )"
    [ "$commit" = "$MAINTAINER_COMMIT" ] || continue
    candidate="$(dirname "$marker")/gascity"
    [ -d "$candidate/assets/scripts/checks" ] || continue
    [ -d "$candidate/schemas" ] || continue
    canonical "$candidate"
    return
  done
  die "cannot locate the resolved official gascity ${MAINTAINER_COMMIT} pack cache; pass --pack-dir"
}

pack_dir="$(resolve_pack_dir)"
[ -d "$pack_dir/assets/scripts/checks" ] || die "maintainer pack has no assets/scripts/checks"
[ -f "$pack_dir/assets/scripts/validate_build_artifact.py" ] ||
  die "maintainer pack has no build artifact validator"
[ -d "$pack_dir/schemas" ] || die "maintainer pack has no schemas"
pack_marker="$(dirname "$pack_dir")/.gc-bundled-pack-cache.toml"
[ -f "$pack_marker" ] || die "resolved pack has no bundled cache provenance marker"
grep -Eq "^[[:space:]]*commit[[:space:]]*=[[:space:]]*\"${MAINTAINER_COMMIT}\"" \
  "$pack_marker" || die "resolved pack cache marker does not match ${MAINTAINER_COMMIT}"

select_python() {
  local candidate seen=""
  for candidate in \
    "${GC_PYTHON_BIN:-}" \
    /opt/homebrew/bin/python3 \
    /usr/bin/python3 \
    "$(command -v python3 2>/dev/null || true)"; do
    [ -n "$candidate" ] && [ -x "$candidate" ] || continue
    case " $seen " in *" $candidate "*) continue ;; esac
    seen="$seen $candidate"
    if "$candidate" -c 'import yaml' >/dev/null 2>&1; then
      canonical "$candidate"
      return
    fi
  done
  die "no existing python3 interpreter imports PyYAML; install PyYAML or set GC_PYTHON_BIN"
}

python_bin="$(select_python)"
runtime_root="$rig/.gc/agentops-maintainer-runtime"
runtime="$runtime_root/versions/$MAINTAINER_COMMIT"
checks_dir="$rig/.gc/scripts/checks"

check_wrapper_conflicts() {
  local source name destination
  for source in "$pack_dir"/assets/scripts/checks/*.sh; do
    [ -f "$source" ] || continue
    name="$(basename "$source")"
    destination="$checks_dir/$name"
    if [ -e "$destination" ] && ! grep -Fq "$MANAGED_MARKER" "$destination"; then
      die "refusing to overwrite unmanaged check: $destination"
    fi
  done
}

check_runtime() {
  [ -f "$runtime/agentops-runtime.env" ] ||
    die "maintainer runtime is not prepared at $runtime"
  grep -Fqx "commit=$MAINTAINER_COMMIT" "$runtime/agentops-runtime.env" ||
    die "maintainer runtime commit marker is invalid"
  [ -x "$runtime/bin/python3" ] || die "maintainer runtime python shim is missing"
  "$runtime/bin/python3" -c 'import yaml' >/dev/null 2>&1 ||
    die "maintainer runtime Python can no longer import PyYAML"
  diff -qr "$pack_dir/assets/scripts" "$runtime/gascity/assets/scripts" >/dev/null ||
    die "contained maintainer scripts differ from the resolved upstream pack"
  diff -qr "$pack_dir/schemas" "$runtime/gascity/schemas" >/dev/null ||
    die "contained maintainer schemas differ from the resolved upstream pack"

  local source name destination
  for source in "$pack_dir"/assets/scripts/checks/*.sh; do
    [ -f "$source" ] || continue
    name="$(basename "$source")"
    destination="$checks_dir/$name"
    [ -x "$destination" ] || die "managed check wrapper is missing: $destination"
    grep -Fq "$MANAGED_MARKER" "$destination" ||
      die "check wrapper is no longer AgentOps-managed: $destination"
  done
}

check_skill_link_conflicts() {
  local sink skill actual expected target
  for sink in "$city/.codex/skills" "$rig/.codex/skills"; do
    for skill in $REQUIRED_SKILLS; do
      target="$sink/$skill"
      if [ -e "$target" ] || [ -L "$target" ]; then
        [ -f "$target/SKILL.md" ] ||
          die "refusing conflicting AgentOps skill path: $target"
        actual="$(canonical "$target/SKILL.md")"
        expected="$(canonical "$repo_root/skills/$skill/SKILL.md")"
        [ "$actual" = "$expected" ] ||
          die "AgentOps skill does not resolve to this checkout: $target"
      fi
    done
  done
}

check_skill_links() {
  local sink skill actual expected
  for sink in "$city/.codex/skills" "$rig/.codex/skills"; do
    for skill in $REQUIRED_SKILLS; do
      [ -f "$sink/$skill/SKILL.md" ] ||
        die "AgentOps skill is not visible at $sink/$skill"
      actual="$(canonical "$sink/$skill/SKILL.md")"
      expected="$(canonical "$repo_root/skills/$skill/SKILL.md")"
      [ "$actual" = "$expected" ] ||
        die "AgentOps skill does not resolve to this checkout: $sink/$skill"
    done
  done
}

check_service_binary() {
  [ "${AGENTOPS_GC_SKIP_SERVICE_CHECK:-0}" = "1" ] && return
  [ "$(uname -s)" = "Darwin" ] || return
  local plist="${HOME:?HOME is required}/Library/LaunchAgents/com.gascity.supervisor.plist"
  [ -f "$plist" ] || return
  local program
  program="$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments:0' "$plist" 2>/dev/null || true)"
  [ -n "$program" ] || die "Gas City supervisor LaunchAgent has no program path"
  [ -x "$program" ] || die "Gas City supervisor LaunchAgent points to a missing binary: $program"
  program="$(canonical "$program")"
  [ "$program" = "$gc_bin" ] ||
    die "Gas City supervisor LaunchAgent binary differs from --gc-bin: $program"
}

check_gc_health() {
  local doctor status sessions warning_count
  doctor="$(json_or_die "gc doctor failed" "$gc_bin" --city "$city" doctor --json)"
  jq -e '(.ok == true) and ((.blocking_failed // 0) == 0) and ((.failed // 0) == 0)' \
    <<<"$doctor" >/dev/null || die "gc doctor reports failures"
  warning_count="$(jq '[.results[]? | select(.status == "warning")] | length' <<<"$doctor")"
  if [ "$warning_count" -gt 0 ]; then
    printf 'warning: gc doctor reports %s upstream/config warning(s)\n' "$warning_count" >&2
  fi

  status="$(json_or_die "gc status failed" "$gc_bin" --city "$city" status --json)"
  sessions="$(json_or_die "gc session list failed" "$gc_bin" --city "$city" session list --json)"
  if jq -e '.partial == true' <<<"$status" >/dev/null; then
    printf 'warning: gc status returned a partial snapshot; use session, pane, bead, and Doctor evidence\n' >&2
  fi
  if jq -e '
      (.health.signals // [] | index("no_agents_running")) != null
    ' <<<"$status" >/dev/null &&
    jq -e 'any(.sessions[]?; .state == "active" or .state == "creating")' \
      <<<"$sessions" >/dev/null; then
    printf 'warning: gc status says no agents while session state has a live session; roster is authoritative for liveness only\n' >&2
  fi
}

prepare_runtime() {
  check_wrapper_conflicts
  check_skill_link_conflicts

  if [ ! -d "$runtime" ]; then
    local versions tmp
    versions="$runtime_root/versions"
    mkdir -p "$versions"
    tmp="$(mktemp -d "$versions/.${MAINTAINER_COMMIT}.XXXXXX")"
    mkdir -p "$tmp/gascity/assets" "$tmp/gascity" "$tmp/bin"
    cp -R "$pack_dir/assets/scripts" "$tmp/gascity/assets/scripts"
    cp -R "$pack_dir/schemas" "$tmp/gascity/schemas"
    printf '#!/bin/sh\nexec %s "$@"\n' "$(quote_sh "$python_bin")" >"$tmp/bin/python3"
    chmod 0755 "$tmp/bin/python3"
    {
      printf 'commit=%s\n' "$MAINTAINER_COMMIT"
      printf 'source=%s\n' "$WORKFLOW_SOURCE"
      printf 'python=%s\n' "$python_bin"
    } >"$tmp/agentops-runtime.env"
    if ! mv "$tmp" "$runtime" 2>/dev/null; then
      rm -rf "$tmp"
      [ -d "$runtime" ] || die "cannot install maintainer runtime at $runtime"
    fi
  fi

  mkdir -p "$checks_dir"
  local source name destination tmp_wrapper runtime_q upstream_q
  runtime_q="$(quote_sh "$runtime")"
  for source in "$pack_dir"/assets/scripts/checks/*.sh; do
    [ -f "$source" ] || continue
    name="$(basename "$source")"
    destination="$checks_dir/$name"
    upstream_q="$(quote_sh "$runtime/gascity/assets/scripts/checks/$name")"
    tmp_wrapper="$(mktemp "$checks_dir/.${name}.XXXXXX")"
    {
      printf '#!/bin/sh\n'
      printf '# %s\n' "$MANAGED_MARKER"
      printf 'set -eu\n'
      printf 'runtime=%s\n' "$runtime_q"
      # shellcheck disable=SC2016 # Generated wrapper expands its own runtime/PATH.
      printf '%s\n' 'PATH="$runtime/bin:$PATH"' 'export PATH'
      printf 'exec %s "$@"\n' "$upstream_q"
    } >"$tmp_wrapper"
    chmod 0755 "$tmp_wrapper"
    mv "$tmp_wrapper" "$destination"
  done

  mkdir -p "$city/.codex/skills" "$rig/.codex/skills"
  (
    cd "$repo_root"
    "$ao_bin" skills link --dest "$city/.codex/skills" --json >/dev/null
    "$ao_bin" skills link --dest "$rig/.codex/skills" --json >/dev/null
  )

  check_runtime
  check_skill_links
}

recover_affinity() {
  local sessions ready stale bead assignee routed
  sessions="$(json_or_die "cannot list sessions" "$gc_bin" --city "$city" session list --json)"
  ready="$(json_or_die "cannot list ready rig work" "$gc_bin" --city "$city" --rig "$rig_name" bd ready --json)"
  stale="$(
    jq -nr \
      --argjson ready "$ready" \
      --argjson sessions "$sessions" '
      def live($assignee):
        any($sessions.sessions[]?;
          (.name == $assignee or .session_name == $assignee or .alias == $assignee)
          and (.state == "active" or .state == "creating"
            or .state == "starting" or .state == "waking"));
      $ready[]?
      | select((.assignee // "") != "")
      | select((.metadata["gc.session_affinity"] // "") == "require")
      | select((.metadata["gc.routed_to"] // "") != "")
      | select(live(.assignee) | not)
      | [.id, .assignee, (.metadata["gc.routed_to"] // "")]
      | @tsv
    '
  )"
  if [ -z "$stale" ]; then
    printf 'No stale required session-affinity assignments found.\n'
    return
  fi
  while IFS=$'\t' read -r bead assignee routed; do
    [ -n "$bead" ] || continue
    if [ "$apply" -eq 0 ]; then
      printf 'would clear %s assignee=%s routed_to=%s\n' "$bead" "$assignee" "$routed"
    else
      "$gc_bin" --city "$city" --rig "$rig_name" bd update "$bead" --assignee '' >/dev/null
      printf 'cleared %s assignee=%s routed_to=%s\n' "$bead" "$assignee" "$routed"
    fi
  done <<<"$stale"
  if [ "$apply" -eq 0 ]; then
    printf 'Dry run only; pass --apply to clear exactly these ready stale assignments.\n'
  fi
}

case "$command_name" in
  prepare)
    [ "$apply" -eq 0 ] || die "--apply is only valid with recover-affinity"
    prepare_runtime
    check_service_binary
    check_gc_health
    printf 'maintainer runtime ready: city=%s rig=%s commit=%s\n' \
      "$city" "$rig_name" "$MAINTAINER_COMMIT"
    ;;
  check)
    [ "$apply" -eq 0 ] || die "--apply is only valid with recover-affinity"
    check_runtime
    check_skill_links
    check_service_binary
    check_gc_health
    printf 'maintainer runtime ready: city=%s rig=%s commit=%s\n' \
      "$city" "$rig_name" "$MAINTAINER_COMMIT"
    ;;
  recover-affinity)
    recover_affinity
    ;;
esac
