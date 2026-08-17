#!/usr/bin/env bash
# Bounded local/fleet CASS search. Remote destinations must be present in the
# built-in allowlist or an explicit, bounded, non-symlink hosts file.
set -uo pipefail
shopt -s nullglob

usage() {
  echo "usage: $0 [--hosts-file /absolute/non-symlink/file] QUERY [host ...]" >&2
  exit 2
}

HOSTS_FILE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --hosts-file) [[ $# -ge 2 ]] || usage; HOSTS_FILE="$2"; shift 2 ;;
    --hosts-file=*) HOSTS_FILE="${1#*=}"; shift ;;
    --) shift; break ;;
    -*) usage ;;
    *) break ;;
  esac
done
[[ $# -ge 1 ]] || usage
QUERY="$1"
shift
HOSTS=("$@")
[[ ${#HOSTS[@]} -gt 0 ]] || HOSTS=(css csd ts1 ts2)

PER_HOST_TIMEOUT="${CASS_FANOUT_TIMEOUT:-30}"
MAX_RESULT_BYTES=$((1024 * 1024))
MAX_HOSTS=16
MAX_QUERY_BYTES=4096
[[ "$PER_HOST_TIMEOUT" =~ ^[0-9]+$ && "$PER_HOST_TIMEOUT" -ge 1 && "$PER_HOST_TIMEOUT" -le 60 ]] \
  || { echo "error: CASS_FANOUT_TIMEOUT must be an integer in [1,60]" >&2; exit 2; }
[[ ${#HOSTS[@]} -le $MAX_HOSTS ]] \
  || { echo "error: host count exceeds $MAX_HOSTS" >&2; exit 2; }
[[ ${#QUERY} -le $MAX_QUERY_BYTES && "$QUERY" != *$'\n'* ]] \
  || { echo "error: query must be one line and at most $MAX_QUERY_BYTES bytes" >&2; exit 2; }

TIMEOUT_BIN=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null 2>&1; then TIMEOUT_BIN="$(command -v "$candidate")"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || { echo "error: timeout or gtimeout is required" >&2; exit 2; }
for tool in jq ssh cass; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: '$tool' not on PATH" >&2; exit 2; }
done

ALLOWED_HOSTS=(css csd ts1 ts2)
if [[ -n "$HOSTS_FILE" ]]; then
  [[ "$HOSTS_FILE" == /* && -f "$HOSTS_FILE" && ! -L "$HOSTS_FILE" ]] \
    || { echo "error: --hosts-file must be an absolute regular non-symlink file" >&2; exit 2; }
  host_file_size="$(wc -c < "$HOSTS_FILE" | tr -d ' ')"
  [[ "$host_file_size" -le 4096 ]] \
    || { echo "error: --hosts-file exceeds 4096 bytes" >&2; exit 2; }
  while IFS= read -r allowed || [[ -n "$allowed" ]]; do
    [[ -z "$allowed" || "$allowed" == \#* ]] && continue
    [[ "$allowed" =~ ^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$ ]] \
      || { echo "error: invalid host entry in --hosts-file" >&2; exit 2; }
    ALLOWED_HOSTS+=("$allowed")
    [[ ${#ALLOWED_HOSTS[@]} -le $MAX_HOSTS ]] \
      || { echo "error: allowed host count exceeds $MAX_HOSTS" >&2; exit 2; }
  done < "$HOSTS_FILE"
fi

for host in "${HOSTS[@]}"; do
  [[ "$host" =~ ^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$ ]] \
    || { echo "error: invalid host token" >&2; exit 2; }
  approved=0
  for allowed in "${ALLOWED_HOSTS[@]}"; do
    [[ "$host" == "$allowed" ]] && approved=1 && break
  done
  [[ "$approved" -eq 1 ]] \
    || { echo "error: host is not explicitly approved: $host" >&2; exit 2; }
done

FANOUT_DIR="$(mktemp -d -t cass-fanout-XXXXXX)" || exit 2
cleanup() { rm -rf -- "$FANOUT_DIR"; }
trap cleanup EXIT

bounded_json() {
  # head closes the pipe at the byte cap, preventing command substitution from
  # retaining an unbounded child response. pipefail turns an oversize SIGPIPE
  # into a failed search rather than accepting a truncated JSON document.
  head -c $((MAX_RESULT_BYTES + 1))
}

local_search() {
  local raw
  raw=$("$TIMEOUT_BIN" "$PER_HOST_TIMEOUT" cass search "$QUERY" --json --fields summary --limit 20 \
    2>/dev/null | bounded_json) || raw=""
  [[ ${#raw} -le $MAX_RESULT_BYTES ]] || raw=""
  if [[ -z "$raw" ]]; then
    echo '[]' > "$FANOUT_DIR/0.json"
    return
  fi
  printf '%s' "$raw" | jq '[(.hits // [])[] | . + {origin_host: "local"}]' \
    > "$FANOUT_DIR/0.json" || echo '[]' > "$FANOUT_DIR/0.json"
}

remote_search() {
  local index="$1" host="$2" raw rc
  # shellcheck disable=SC2016
  raw=$("$TIMEOUT_BIN" "$PER_HOST_TIMEOUT" ssh -o ConnectTimeout=5 -o BatchMode=yes "$host" \
    'IFS= read -r q && cass search "$q" --json --fields summary --limit 20' \
    <<<"$QUERY" 2>/dev/null | bounded_json)
  rc=$?
  [[ "$rc" -eq 0 && ${#raw} -le $MAX_RESULT_BYTES ]] || raw=""
  if [[ -z "$raw" ]]; then
    echo '[]' > "$FANOUT_DIR/$index.json"
    printf '  ! %s: remote search failed or exceeded a bound\n' "$host" >&2
    return
  fi
  printf '%s' "$raw" | jq --arg h "$host" '[(.hits // [])[] | . + {origin_host: $h}]' \
    > "$FANOUT_DIR/$index.json" || echo '[]' > "$FANOUT_DIR/$index.json"
}

echo '→ local' >&2
local_search &
index=1
for host in "${HOSTS[@]}"; do
  printf '→ %s\n' "$host" >&2
  remote_search "$index" "$host" &
  index=$((index + 1))
done
wait

files=("$FANOUT_DIR"/*.json)
[[ ${#files[@]} -gt 0 ]] || { echo '[]'; exit 0; }
jq -s '
  (add // [])
  | map(select(type == "object"))
  | unique_by((.source_path // "") + ":" + ((.line_number // 0)|tostring))
  | sort_by(-(.score // 0)) | .[0:50]
  | map({
      host: (.origin_host // "?"), agent: (.agent // ""),
      line: (.line_number // null), score: (.score // null),
      title: ((.title // "") | .[0:100]),
      source: ((.source_path // "") | split("/") | last)
    })
' "${files[@]}"
