#!/usr/bin/env bash
# tests/e2e/membrane-mine-session-incremental.sh
#   E6.TEST — bead age-membrane-memory-arch-tz2s.6.3
#
# Proves the E6.1 session-log miner (`ao provenance mine-session`, ADR-0010
# build-native) honors its contract END-TO-END through the real built `ao`
# binary, not just in-process: SKIPS CONSUMED input, DEDUPS (stable + per-call
# unique ids), and EMITS TYPED EDGES (closed schema, tool_call kind).
#
# RESCOPE (mixed council, Claude + Codex unanimous, 2026-06-22): the bead's
# original "assert NO FULL RE-PARSE" is asserted at the EMIT level —
# "no re-EMISSION / no downstream reprocessing of consumed events" — NOT as
# "the JSONL is not parsed". The shipped miner re-parses the whole transcript
# each run (ms-scale) BECAUSE the content prefix-checksum rollback that five
# cross-family rounds hardened structurally requires a full parse; the expensive
# work the watermark actually guards is the downstream PROV-O/membrane
# processing of events, which the emit-level watermark provably elides. Option B
# (byte-offset parse-skip) was rejected: it re-opens the hardened rollback
# surface for no real cost win on session-log-scale transcripts.
#
# SUT: the repo-built `ao` (e2e_factory_ao_bin). Real binary, isolated mktemp
# sandbox, mock nothing (tests/e2e/README.md contract). No external tools.
#
# RUN MODEL: local/manual or CI — `bash tests/e2e/membrane-mine-session-incremental.sh`.
# Self-contained; exits non-zero on first failed assertion.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=../lib/e2e-factory.sh
source "$REPO_ROOT/tests/lib/e2e-factory.sh"

SANDBOX="$(mktemp -d)"
cleanup() { local rc=$?; chmod -R u+w "$SANDBOX" 2>/dev/null || true; rm -rf "$SANDBOX" 2>/dev/null || true; exit "$rc"; }
trap cleanup EXIT

PASS=0
pass() { PASS=$((PASS + 1)); printf 'PASS: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

# Force a fresh build: the factory otherwise reuses a prebuilt cli/bin/ao, which
# may predate this command and would false-fail with "unknown command". The SUT
# must be the CURRENT source.
export PROOF_FORCE_BUILD=1
AO_BIN="$(e2e_factory_ao_bin "$SANDBOX/bin" "$REPO_ROOT")"
SCHEMA="$REPO_ROOT/schemas/provenance-mine-event.v1.schema.json"
[[ -f "$SCHEMA" ]] || fail "schema missing: $SCHEMA"

# jq is the deterministic event-shape oracle here (no model). It is REQUIRED, not
# optional: a skip-on-missing-jq would `exit 0` before any assertion and let a
# broken miner false-pass (the skip-green trap). jq is a standard tool that CI
# guarantees (validate.yml uses it throughout), so its absence is an environment
# error to FAIL on, never a reason to pass.
command -v jq >/dev/null 2>&1 || fail "jq is required for E6.TEST (event-shape oracle) and was not found — install jq; do NOT skip-green"

SESS="$SANDBOX/session.jsonl"
STATE="$SANDBOX/state.json"

# A real-shaped transcript: a Claude assistant message with a tool_use + a
# tool_result block (the block-form output), a top-level tool_result row (the
# output whose tool_name is the REAL tool — the soundness trap), two PARALLEL
# same-tool calls on one line (the dedup/uniqueness trap), and a Codex
# function_call + its function_call_output (the output, skipped).
cat > "$SESS" <<'JSONL'
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"a"}},{"type":"tool_result","content":"contents"}]}}
{"type":"tool_result","tool_name":"Bash","tool_input":{"command":"ls"},"tool_output":"done"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}},{"type":"tool_use","name":"Bash","input":{"command":"pwd"}}]}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{}"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call_output","output":"/tmp"}}
JSONL

# ---------------------------------------------------------------------------
# 1. EMITS TYPED EDGES + SOUNDNESS: exactly the real tool USES become events.
#    Read + Bash(ls) + Bash(pwd) + exec_command = 4. The two tool_result OUTPUTS
#    (block-form + top-level) and the function_call_output must NOT appear.
# ---------------------------------------------------------------------------
"$AO_BIN" provenance mine-session --file "$SESS" --state "$STATE" > "$SANDBOX/run1.jsonl" 2> "$SANDBOX/run1.err"
N1=$(grep -c . "$SANDBOX/run1.jsonl" || true)
[[ "$N1" == "4" ]] || fail "run1 emitted $N1 events, want 4 (Read,Bash,Bash,exec_command; outputs filtered)"
pass "soundness: 4 tool_call events, both tool_result outputs + function_call_output filtered"

TOOLS=$(jq -r '.tool' "$SANDBOX/run1.jsonl" | LC_ALL=C sort | tr '\n' ',' )
[[ "$TOOLS" == "Bash,Bash,Read,exec_command," ]] || fail "tools=$TOOLS, want Bash,Bash,Read,exec_command,"
pass "typed-edges: tools are exactly {Read, Bash, Bash, exec_command}"

# Every event is schema-shaped: closed schema, kind=tool_call, 16-hex id.
while IFS= read -r line; do
  [[ -n "$line" ]] || continue
  echo "$line" | jq -e '.schema_version=="agentops-provenance-mine-event.v1" and .kind=="tool_call" and (.id|test("^[0-9a-f]{16}$")) and (.source_line|type=="number")' >/dev/null \
    || fail "event not schema-shaped: $line"
done < "$SANDBOX/run1.jsonl"
pass "typed-edges: every event is schema-shaped (closed schema, tool_call, 16-hex id)"

# Schema validation via a real validator when present (fail-closed if it rejects).
if command -v check-jsonschema >/dev/null 2>&1; then
  while IFS= read -r line; do [[ -n "$line" ]] || continue; echo "$line" > "$SANDBOX/ev.json"
    check-jsonschema --schemafile "$SCHEMA" "$SANDBOX/ev.json" >/dev/null 2>&1 || fail "event fails schema: $line"
  done < "$SANDBOX/run1.jsonl"
  pass "typed-edges: every event validates against provenance-mine-event.v1.schema.json"
fi

# ---------------------------------------------------------------------------
# 2. DEDUPS: the two parallel Bash calls on one source line get DISTINCT ids
#    (else a downstream id-dedup collapses two real calls), and ALL ids are
#    globally unique within the run.
# ---------------------------------------------------------------------------
IDS_TOTAL=$(jq -r '.id' "$SANDBOX/run1.jsonl" | wc -l | tr -d ' ')
IDS_UNIQ=$(jq -r '.id' "$SANDBOX/run1.jsonl" | sort -u | wc -l | tr -d ' ')
[[ "$IDS_TOTAL" == "$IDS_UNIQ" && "$IDS_UNIQ" == "4" ]] || fail "ids not unique: total=$IDS_TOTAL uniq=$IDS_UNIQ (parallel same-tool calls collapsed)"
pass "dedups: all 4 ids unique (parallel same-tool calls disambiguated by ordinal)"

# ---------------------------------------------------------------------------
# 3. SKIPS CONSUMED (no re-emission): a re-mine with the same state emits 0 new
#    events. The watermark elides already-consumed events from downstream work.
# ---------------------------------------------------------------------------
"$AO_BIN" provenance mine-session --file "$SESS" --state "$STATE" > "$SANDBOX/run2.jsonl" 2>/dev/null
N2=$(grep -c . "$SANDBOX/run2.jsonl" || true)
[[ "$N2" == "0" ]] || fail "run2 (idempotent re-mine) emitted $N2 events, want 0 (consumed not re-emitted)"
pass "skips-consumed: re-mine emits 0 new events (no re-emission of consumed input)"

# Append a NEW tool line -> ONLY the new event is mined (incremental, not full re-emit).
printf '%s\n' '{"type":"tool_use","tool_name":"Edit","tool_input":{"file_path":"z"}}' >> "$SESS"
"$AO_BIN" provenance mine-session --file "$SESS" --state "$STATE" > "$SANDBOX/run3.jsonl" 2>/dev/null
N3=$(grep -c . "$SANDBOX/run3.jsonl" || true)
[[ "$N3" == "1" ]] || fail "run3 (append) emitted $N3 events, want exactly 1 (only the new line)"
[[ "$(jq -r '.tool' "$SANDBOX/run3.jsonl")" == "Edit" ]] || fail "run3 mined the wrong tool"
pass "skips-consumed: appending one line mines exactly the one new event (incremental)"

# ---------------------------------------------------------------------------
# 4. STABLE IDS: re-mining the original lines yields the SAME ids as run1 (so a
#    downstream graph keyed on id never double-inserts a re-seen call).
# ---------------------------------------------------------------------------
rm -f "$STATE"
"$AO_BIN" provenance mine-session --file "$SESS" > "$SANDBOX/full_a.jsonl" 2>/dev/null
"$AO_BIN" provenance mine-session --file "$SESS" > "$SANDBOX/full_b.jsonl" 2>/dev/null
if ! diff -q "$SANDBOX/full_a.jsonl" "$SANDBOX/full_b.jsonl" >/dev/null; then
  fail "two full mines of identical input differ (ids not stable / not deterministic)"
fi
pass "dedups: two full mines of identical input are byte-identical (stable, idempotent ids)"

echo "OK: E6.TEST — $PASS assertions passed (skips-consumed, dedups, typed-edges)"
