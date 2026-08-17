#!/usr/bin/env bash
# Bounded PreToolUse policy dispatcher over the package-owned registry.
set -uo pipefail

MAX_INPUT_BYTES=65536
MAX_REGISTRY_BYTES=65536
MAX_TELEMETRY_BYTES=$((1024 * 1024))
MATCH_TIMEOUT_SECONDS=1

script_dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$script_dir/policies.json" ]]; then
  registry="$script_dir/policies.json"
else
  registry="$script_dir/../policies/policies.json"
fi

# Runtime overrides previously let any process persist an arbitrary command
# policy. Only the package-owned/installed adjacent registry is executable.
[[ -f "$registry" && ! -L "$registry" ]] || exit 0
registry_bytes="$(wc -c < "$registry" | tr -d ' ')"
[[ "$registry_bytes" -le "$MAX_REGISTRY_BYTES" ]] || {
  echo "policy dispatcher: registry exceeds safety bound" >&2
  exit 2
}
command -v jq >/dev/null 2>&1 || exit 0
TIMEOUT_BIN=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null 2>&1; then TIMEOUT_BIN="$(command -v "$candidate")"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || {
  echo "policy dispatcher: bounded matcher unavailable" >&2
  exit 2
}

input="$(head -c $((MAX_INPUT_BYTES + 1)))"
if [[ ${#input} -gt $MAX_INPUT_BYTES ]]; then
  echo "policy dispatcher: hook input exceeds $MAX_INPUT_BYTES bytes" >&2
  exit 2
fi
tool="$(printf '%s' "$input" | jq -r '.tool_name // ""' 2>/dev/null)" || exit 0
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null)" || exit 0
fpath="$(printf '%s' "$input" | jq -r '.tool_input.file_path // ""' 2>/dev/null)" || exit 0
sid="$(printf '%s' "$input" | jq -r '.session_id // "nosession"' 2>/dev/null)" || sid=nosession
[[ -n "$tool" ]] || exit 0

hash_value() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "$1" | sha256sum | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1
  else
    return 1
  fi
}

state_dir() {
  local base="${TMPDIR:-/tmp}/aop-policy-dispatch-v2"
  mkdir -m 700 -p "$base" 2>/dev/null || return 1
  find "$base" -type f -mmin +60 -delete 2>/dev/null || true
  printf '%s\n' "$base"
}

sentinel_for() {
  local key digest base
  key="$sid:$1:$2"
  digest="$(hash_value "$key")" || return 1
  base="$(state_dir)" || return 1
  printf '%s/%s\n' "$base" "$digest"
}

emit_telemetry() {
  # Telemetry is opt-in and fixed beneath an explicit existing root. No caller
  # may select an arbitrary filename, and the retained file has a hard cap.
  local pid="$1" mode="$2" decision="$3" value="$4"
  local root="${AOP_TELEMETRY_ROOT:-}" h line file
  [[ "$root" == /* && -d "$root" && ! -L "$root" ]] || return 0
  file="$root/guardrail-telemetry.jsonl"
  [[ ! -L "$file" ]] || return 0
  [[ ! -e "$file" || "$(wc -c < "$file" | tr -d ' ')" -lt $MAX_TELEMETRY_BYTES ]] || return 0
  h="$(hash_value "$value")" || return 0
  line="$(jq -nc --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg token_class "$pid" --arg path_sha256 "$h" --arg mode "$mode" \
    --arg decision "$decision" \
    '{ts:$ts,token_class:$token_class,path_sha256:$path_sha256,mode:$mode,decision:$decision}')" \
    || return 0
  printf '%s\n' "$line" >> "$file" 2>/dev/null || true
}

waived() {
  # One-process waiver only. Persistent waiver files are intentionally not read.
  local waiver="${AOP_WAIVE:-}"
  [[ ${#waiver} -le 1024 ]] || return 1
  case ",$waiver," in *",$1,"*) return 0 ;; esac
  return 1
}

deny_id=""; deny_msg=""; deny_val=""
route_id=""; route_msg=""; route_val=""
matcher_count=0
while IFS=$'\x1f' read -r pid pmode pfield ppattern pmsg; do
  [[ -n "$pid" ]] || continue
  matcher_count=$((matcher_count + 1))
  [[ "$matcher_count" -le 64 ]] || {
    echo "policy dispatcher: matcher count exceeds safety bound" >&2
    exit 2
  }
  case "$pfield" in command) val="$cmd" ;; file_path) val="$fpath" ;; *) continue ;; esac
  [[ -n "$val" ]] || continue
  "$TIMEOUT_BIN" "$MATCH_TIMEOUT_SECONDS" grep -qE "$ppattern" <<<"$val" 2>/dev/null
  match_rc=$?
  [[ "$match_rc" -eq 0 ]] || continue
  if waived "$pid"; then
    emit_telemetry "$pid" "$pmode" waived "$val"
    continue
  fi
  case "$pmode" in
    deny) [[ -n "$deny_id" ]] || { deny_id="$pid"; deny_msg="$pmsg"; deny_val="$val"; } ;;
    route) [[ -n "$route_id" ]] || { route_id="$pid"; route_msg="$pmsg"; route_val="$val"; } ;;
    audit) emit_telemetry "$pid" audit audit "$val" ;;
  esac
done < <(jq -r --arg tool "$tool" '
  .policies[] | . as $p | .matchers[] | select(.tools | index($tool))
  | [$p.id, $p.mode, .field, .pattern, ($p.route_message // "")] | join("")
' "$registry" 2>/dev/null)

if [[ -n "$deny_id" ]]; then
  emit_telemetry "$deny_id" deny deny "$deny_val"
  sentinel="$(sentinel_for "$deny_id" deny 2>/dev/null || true)"
  if [[ -n "$sentinel" && -f "$sentinel" ]]; then
    printf '⛔ policy %s: blocked (reason shown earlier this session).\n' "$deny_id" >&2
  else
    [[ -z "$sentinel" ]] || : > "$sentinel" 2>/dev/null || true
    printf '⛔ policy %s\n%s\n' "$deny_id" "$deny_msg" >&2
  fi
  exit 2
fi

if [[ -n "$route_id" ]]; then
  # Permission rewrites are opt-in and one-shot per policy/session for one hour.
  # Subsequent matching calls fail closed rather than persistently rewriting.
  [[ "${AOP_ENABLE_PERMISSION_ROUTING:-0}" == 1 ]] || {
    printf '⛔ policy %s: permission routing is not explicitly enabled.\n' "$route_id" >&2
    exit 2
  }
  sentinel="$(sentinel_for "$route_id" route 2>/dev/null || true)"
  [[ -n "$sentinel" && ! -f "$sentinel" ]] || {
    printf '⛔ policy %s: bounded permission route already consumed.\n' "$route_id" >&2
    exit 2
  }
  : > "$sentinel" 2>/dev/null || {
    echo "policy dispatcher: could not record bounded permission route" >&2
    exit 2
  }
  emit_telemetry "$route_id" route ask "$route_val"
  jq -nc --arg reason "policy ${route_id}: ${route_msg}" \
    '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"ask",permissionDecisionReason:$reason}}'
  exit 0
fi

exit 0
