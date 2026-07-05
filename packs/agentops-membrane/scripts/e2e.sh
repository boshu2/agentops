#!/usr/bin/env bash
# e2e.sh — the "membrane smoke" an operator or CI runs against a city that
# imports agentops-membrane. Structural + doctor-level ONLY: it does NOT run a
# live agent drill (that is age-gc-mvp-w2-nuiw.4). It answers one question:
# "is the verification membrane installed and sound in this city, right now?"
#
# It:
#   1. runs `gc doctor --json` and asserts the two membrane doctor checks —
#      law0-print-args (LAW 0 structural) and membrane-health (close door
#      installed + cross-family posture) — both report status=ok. It filters to
#      those two checks by name suffix so unrelated city doctor noise (a
#      throwaway --no-start city has many advisory findings) never masks or
#      falsely fails the membrane smoke;
#   2. runs `gc lint` on the pack (structural: pack.toml + prompt templates
#      parse) — the "does the membrane formula/pack compile" dry check.
#
# Exit 0 = membrane sound. Exit non-zero = broken membrane install (missing/
# non-exec close door, absent trinity, or <2 provider families). Designed to be
# run headless: `bash scripts/e2e.sh` from anywhere inside the city, or with an
# explicit --city.
#
# Usage: e2e.sh [--city <path>] [--pack <path>] [--quiet]
set -u

CITY=""; PACK=""; QUIET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --city)  CITY="$2"; shift 2 ;;
    --pack)  PACK="$2"; shift 2 ;;
    --quiet) QUIET=1; shift ;;
    *) echo "e2e: unknown arg: $1" >&2; exit 2 ;;
  esac
done

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -n "$PACK" ] || PACK="${GC_PACK_DIR:-$(cd "$HERE/.." && pwd)}"
[ -n "$CITY" ] || CITY="${GC_CITY_PATH:-$PWD}"

GC_BIN="${GC_BIN:-$(command -v gc || echo gc)}"
say() { [ "$QUIET" -eq 1 ] || echo "$@"; }
fail() { echo "e2e FAIL: $*" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq not found (required)"

say "== membrane e2e smoke =="
say "city: $CITY"
say "pack: $PACK"

# --- 1. gc doctor: the two membrane checks must be green ---------------------
say "-- gc doctor (membrane checks) --"
if ! report="$( (cd "$CITY" && "$GC_BIN" doctor --json) 2>/dev/null)"; then
  # non-zero exit only means SOME blocking check failed; we still parse the
  # report and judge the membrane checks specifically. Empty report = real fail.
  [ -n "$report" ] || fail "'gc doctor --json' produced no report in $CITY"
fi

check_status() {  # $1 = check-name suffix
  printf '%s' "$report" | jq -r --arg s "$1" '
    (.results // [])
    | map(select(.name | endswith($s)))
    | if length == 0 then "MISSING"
      else (map(.status) | if all(. == "ok") then "ok" else (map(select(. != "ok"))[0]) end)
      end'
}

rc=0
for suffix in law0-print-args membrane-health; do
  st="$(check_status "$suffix")"
  case "$st" in
    ok)      say "  ok:      $suffix" ;;
    MISSING) echo "  MISSING: $suffix (pack doctor check not discovered — run 'gc import install')"; rc=1 ;;
    *)       echo "  $st:  $suffix"; rc=1 ;;
  esac
done
[ "$rc" -eq 0 ] || fail "a membrane doctor check is not green"

# --- 2. structural: the pack compiles (gc lint) -----------------------------
say "-- gc lint (pack compiles) --"
if lint_out="$( "$GC_BIN" lint "$PACK" 2>&1)"; then
  say "  ok:      pack.toml + prompt templates parse"
else
  echo "$lint_out" | sed 's/^/  /'
  fail "gc lint reported a broken pack"
fi

say "== membrane e2e PASS =="
exit 0
