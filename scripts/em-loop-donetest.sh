#!/usr/bin/env bash
# em-loop-donetest.sh — the terminal e2e done-test for the membrane's self-improvement
# loop (epic age-membrane-memory-arch-tz2s; EM.3 fixture + EM.TEST e2e, .2.3/.2.6).
#
# Proves the HEADLINE product claim is TRUE on the SHIPPED ao binary, not just in a
# unit lane: a false CONFIRMED that a later REFUTED overturns (an ESCAPE) carrying a
# mechanical detector becomes a deterministic gate-block of its own re-introduction.
# Runs the REAL organs over an ISOLATED temp project + temp ledger, so it is
# repeatable and never pollutes the real repo / _beads / provenance ledger.
#
# The mechanical sequence (a skeptic can watch this once):
#   1. FIXTURE  — seed a synthetic mechanical escape (CONFIRMED@1 then REFUTED@2 with
#                 a detector_pattern + path_globs) THROUGH the real `ao yield emit`
#                 CLI — production ledger shape, round-tripped by the real writer
#                 (EM.3, fixture fidelity; not a hand-built struct).
#   2. SHADOW   — deterministic replay against stored positives and explicit negative
#                 controls emits a WARN-only shadow constraint.
#   3. MEASURE  — a shadow hit WARNs; activation without cited precision is rejected.
#   4. ACTIVATE — precision-backed shadow evidence permits explicit blocking activation.
#   5. BLOCK    — re-introduce the forbidden pattern on a matching path; constraints.enforce
#                 now FAILS deterministically (the windshield half of the EM claim).
#   6. SAFE     — with no violation, `ao gate check` reports constraints.enforce PASS.
#   7. CITE     — the derived finding (.agents/findings/<id>.md) exists and cites the
#                 escape bead (the input a next in-domain pre-mortem would LOAD).
#                 SCOPE: this asserts the gate-block (ENFORCED) half + that the cite
#                 artifact is written; the pre-mortem actually loading it is EM.4 (.2.4).
#
# Each step ASSERTS; the first failure prints FAIL and exits 1 (no silent pass).
# Exit: 0 = the loop closed · 1 = a step failed · 2 = usage / missing organ.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/em-loop-donetest.XXXXXX")" || { echo "FAIL: mktemp" >&2; exit 2; }
AO="${AGENTOPS_AO_BIN:-$WORK/ao}"
# shellcheck disable=SC2329  # invoked indirectly via the EXIT trap below.
cleanup() { rm -rf "$WORK" 2>/dev/null; }
trap cleanup EXIT

fail() { echo "✗ FAIL [$1]: $2" >&2; exit 1; }
ok()   { echo "✓ $1"; }

# Build the SHIPPED artifact (a real compiled binary, not `go run`) unless one was
# provided. EM.5 (installed-binary parity) is the reason this must be the binary.
if [ ! -x "$AO" ]; then
  ( cd "$REPO_ROOT/cli" && go build -o "$WORK/ao" ./cmd/ao ) || fail build "go build ao"
  AO="$WORK/ao"
fi
command -v python3 >/dev/null 2>&1 || { echo "FAIL: need python3" >&2; exit 2; }

PROJ="$WORK/proj"
mkdir -p "$PROJ/cli/internal/demopkg"
( cd "$PROJ" && git init -q . && git config user.email t@t && git config user.name t \
  && printf 'package demopkg\n' > cli/internal/demopkg/safe.go \
  && git add -A && git commit -qm base ) || fail setup "init temp project"
cd "$PROJ" || fail setup "cd proj"

RUN="em-loop-e2e"
gv() { "$AO" yield emit gate-verdict --bead age-demo --run "$RUN" --json "$1" >/dev/null 2>&1; }

# 1. FIXTURE — the escape, via the real CLI (production shape).
gv '{"disposition":"CONFIRMED","head_sha":"aaaaaaa1","attempt":1,"pawl_verdict_ref":{"bead_id":"age-demo","head_sha":"aaaaaaa1"},"author_context_id":"c","author_family":"claude","difficulty":1}' \
  || fail fixture "emit CONFIRMED@1"
gv '{"disposition":"REFUTED","head_sha":"bbbbbbb2","attempt":2,"pawl_verdict_ref":{"bead_id":"age-demo","head_sha":"bbbbbbb2"},"author_context_id":"c","author_family":"claude","difficulty":1,"domain":"demo","reason":"forbidden ForbiddenCall()","detector_pattern":"ForbiddenCall\\(","constraint_path_globs":"cli/internal/demopkg/**","detector_kind":"regex"}' \
  || fail fixture "emit REFUTED@2 (mechanical detector)"
ok "1. FIXTURE: synthetic mechanical escape seeded via the real ao yield emit CLI"

# 2. SHADOW — only deterministic replay against positives and negative controls
# can produce a mechanical candidate. The initial evidence intentionally has no
# precision measurement, so activation must remain impossible.
REPLAY="$WORK/replay.json"
READY="$WORK/activation-ready.json"
printf '%s\n' '{"positive_fixtures":[{"ref":"positive-1.go","content":"func a() { ForbiddenCall() }"},{"ref":"positive-2.go","content":"func b() { ForbiddenCall() }"}],"negative_controls":[{"ref":"negative-1.go","content":"func safe() {}"},{"ref":"negative-2.go","content":"// ForbiddenCall is discussed, not invoked"}]}' >"$REPLAY"
printf '%s\n' '{"positive_fixtures":[{"ref":"positive-1.go","content":"func a() { ForbiddenCall() }"},{"ref":"positive-2.go","content":"func b() { ForbiddenCall() }"}],"negative_controls":[{"ref":"negative-1.go","content":"func safe() {}"},{"ref":"negative-2.go","content":"// ForbiddenCall is discussed, not invoked"}],"precision":{"evidence_ref":".agents/evidence/constraints/forbidden-call-shadow.json","samples":40,"true_positives":39,"false_positives":1}}' >"$READY"
"$AO" membrane derive-checks --run "$RUN" --detector-evidence "$REPLAY" >/dev/null 2>&1 || fail shadow "derive-checks with replay evidence"
IDX="$PROJ/.agents/constraints/index.json"
[ -s "$IDX" ] || fail shadow "index.json not written"
CID="$(python3 -c "import json;d=json.load(open('$IDX'));cs=d['constraints'];print(cs[0]['id'] if cs else '')" 2>/dev/null)"
[ -n "$CID" ] || fail shadow "no constraint compiled from passing replay evidence"
STATUS="$(python3 -c "import json;print(json.load(open('$IDX'))['constraints'][0]['status'])" 2>/dev/null)"
[ "$STATUS" = "shadow" ] || fail shadow "derived constraint must be shadow, got '$STATUS'"
MODE="$(python3 -c "import json;print(json.load(open('$IDX'))['constraints'][0]['enforcement_mode'])" 2>/dev/null)"
[ "$MODE" = "warn" ] || fail shadow "derived shadow must be warn-only, got '$MODE'"
CHECK="$PROJ/.agents/premortem-checks/$CID.md"
[ -s "$CHECK" ] || fail shadow "canonical premortem check not written"
ok "2. SHADOW: replayed positives + negative controls -> warn-only shadow $CID"

# constraint_verdict prints PASS, WARN, or FAIL for constraints.enforce. It
# CAPTURES the gate output (|| true) because `ao gate check` legitimately exits
# non-zero on the bare temp project (go.build etc. fail) — piping straight into
# grep under `set -o pipefail` would mask the real constraints.enforce result.
constraint_verdict() {
  local out
  out="$("$AO" gate check --full 2>&1 || true)"
  printf '%s\n' "$out" | grep -oE "(PASS|WARN|FAIL)[[:space:]]+constraints\.enforce" | awk '{print $1}' | head -1
}

BAD='cli/internal/demopkg/bad.go'
WRITE_BAD() { printf 'package demopkg\nfunc bad() { ForbiddenCall() }\n' > "$BAD"; }

# 3. MEASURE — a shadow detector reports the hit but never blocks. Activation
# without precision evidence must fail and preserve shadow state.
WRITE_BAD
[ "$(constraint_verdict)" = "WARN" ] \
  || fail shadow-warn "a shadow hit must WARN without blocking"
"$AO" constraint activate "$CID" >/dev/null 2>&1 \
  && fail activation-guard "activation without cited precision evidence succeeded"
STATUS="$(python3 -c "import json;print(json.load(open('$IDX'))['constraints'][0]['status'])" 2>/dev/null)"
[ "$STATUS" = "shadow" ] || fail activation-guard "failed activation changed status to '$STATUS'"
ok "3. MEASURE: shadow hit WARNs; missing precision cannot activate"

# 4. ACTIVATE — replace the candidate with cited shadow precision, then activate.
ACTIVATE_OUT="$("$AO" membrane derive-checks --run "$RUN" --force --detector-evidence "$READY" 2>&1)" \
  || fail activate "refresh shadow with precision evidence: $ACTIVATE_OUT"
ACTIVATE_OUT="$("$AO" constraint activate "$CID" 2>&1)" \
  || fail activate "ao constraint activate: $ACTIVATE_OUT"
ok "4. ACTIVATE: precision-backed shadow -> active blocking constraint"

# 5. BLOCK — the SAME violation now FAILs constraints.enforce once active.
[ "$(constraint_verdict)" = "FAIL" ] \
  || fail block "re-introducing the escaped class must FAIL constraints.enforce once active (loop did not close)"
ok "5. BLOCK: re-introduced ForbiddenCall() -> constraints.enforce FAIL"

# 6. SAFE — remove the violation; the active constraint PASSes a clean tree.
rm -f "$BAD"
[ "$(constraint_verdict)" = "PASS" ] \
  || fail safe "active constraint must PASS a clean tree (no false positive)"
ok "6. SAFE: violation removed -> constraints.enforce PASS"

# 7. CITE — the derived finding exists and references the escape bead.
FINDING="$(find "$PROJ/.agents/findings" -maxdepth 1 -name '*.md' 2>/dev/null | head -1)"
[ -n "$FINDING" ] || fail cite "no derived finding written"
grep -q "age-demo" "$FINDING" || fail cite "derived finding does not cite the escape bead"
ok "7. CITE: derived finding written + cites the escape"

# 8. LOAD — Premortem's deterministic compiled-prevention reader consumes the
# canonical directory directly. Prove the canonical check carries the escape
# citation instead of relying on an archived convenience command.
grep -q "age-demo" "$CHECK" \
  || fail load "canonical Premortem check must cite the escape bead"
grep -qiE "Escape:|fresh-context refuter|false-done" "$CHECK" \
  || fail load "canonical Premortem check must carry the derived prevention"
ok "8. LOAD: canonical Premortem input carries the escape citation"

# 9. TRAVEL — the loop fires for EVERYONE, not just this box (EM.2.9). Publish the
# active constraint to the TRACKED surface (sanitized — no private .agents paths),
# then HIDE .agents/ entirely to simulate a clean CI checkout / fresh clone, and
# confirm the published constraint STILL enforces the re-introduction. The membrane's
# learning travels with the repo instead of dying on one developer's machine.
"$AO" constraint publish >/dev/null 2>&1 || fail travel "ao constraint publish"
[ -s "$PROJ/docs/constraints/published.json" ] || fail travel "published surface not written"
grep -q '\.agents' "$PROJ/docs/constraints/published.json" \
  && fail travel "published surface LEAKS a private .agents path (no private evidence may travel)"
mv "$PROJ/.agents" "$PROJ/.agents.hidden"   # simulate a clean CI checkout: no local index at all
WRITE_BAD
[ "$(constraint_verdict)" = "FAIL" ] \
  || fail travel "the PUBLISHED constraint must enforce with NO local .agents/ (CI / clean clone)"
mv "$PROJ/.agents.hidden" "$PROJ/.agents"
rm -f "$BAD"
ok "9. TRAVEL: published constraint enforces with no local .agents/"

echo
echo "✓✓ EM LOOP CLOSED: escape -> replayed warn-only shadow -> measured activation -> deterministic block; the Premortem check is retrievable, and the published constraint travels."
exit 0
