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
#   2. COMPILE  — `ao membrane derive-checks` turns the escape into a DRAFT constraint
#                 in .agents/constraints/index.json (the index that was empty forever).
#   3. ACTIVATE — `ao constraint activate` flips draft->active (the human-in-the-loop
#                 review seam; a draft gates NOTHING — asserted, or the test false-greens).
#   4. SAFE     — with no violation, `ao gate check` reports constraints.enforce PASS.
#   5. BLOCK    — re-introduce the forbidden pattern on a matching path; constraints.enforce
#                 now FAILS deterministically (the windshield half of the EM claim).
#   6. CITE     — the derived finding (.agents/findings/<id>.md) exists and cites the
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

# 2. COMPILE — derive-checks -> a draft constraint in index.json.
"$AO" membrane derive-checks --run "$RUN" >/dev/null 2>&1 || fail compile "derive-checks"
IDX="$PROJ/.agents/constraints/index.json"
[ -s "$IDX" ] || fail compile "index.json not written"
CID="$(python3 -c "import json;d=json.load(open('$IDX'));cs=d['constraints'];print(cs[0]['id'] if cs else '')" 2>/dev/null)"
[ -n "$CID" ] || fail compile "no constraint compiled from the escape (the cut wire is cut)"
STATUS="$(python3 -c "import json;print(json.load(open('$IDX'))['constraints'][0]['status'])" 2>/dev/null)"
[ "$STATUS" = "draft" ] || fail compile "derived constraint must be draft, got '$STATUS'"
ok "2. COMPILE: escape -> draft constraint $CID in index.json"

# constraint_verdict prints PASS or FAIL for the constraints.enforce check. It
# CAPTURES the gate output (|| true) because `ao gate check` legitimately exits
# non-zero on the bare temp project (go.build etc. fail) — piping straight into
# grep under `set -o pipefail` would mask the real constraints.enforce result.
constraint_verdict() {
  local out
  out="$("$AO" gate check --full 2>&1 || true)"
  printf '%s\n' "$out" | grep -oE "(PASS|FAIL)[[:space:]]+constraints\.enforce" | awk '{print $1}' | head -1
}

BAD='cli/internal/demopkg/bad.go'
WRITE_BAD() { printf 'package demopkg\nfunc bad() { ForbiddenCall() }\n' > "$BAD"; }

# 3. ACTIVATE — first prove the ACTIVATE boundary matters: the SAME violation that
# a DRAFT does not block must block once ACTIVE (drafts gate nothing — the
# false-green trap this whole repo's fixture-fidelity ratchet keeps reproducing).
WRITE_BAD
[ "$(constraint_verdict)" = "PASS" ] \
  || fail draft-guard "a DRAFT constraint must NOT block a violation (drafts gate nothing)"
"$AO" constraint activate "$CID" >/dev/null 2>&1 || fail activate "ao constraint activate"
ok "3. ACTIVATE: draft->active — and a draft was confirmed to gate nothing (activate guard)"

# 4. BLOCK — the SAME violation now FAILs constraints.enforce once the rule is active.
[ "$(constraint_verdict)" = "FAIL" ] \
  || fail block "re-introducing the escaped class must FAIL constraints.enforce once active (loop did not close)"
ok "4. BLOCK: re-introduced ForbiddenCall() -> constraints.enforce FAIL (deterministic gate-block)"

# 5. SAFE — remove the violation; the active constraint PASSes a clean tree (no false positive).
rm -f "$BAD"
[ "$(constraint_verdict)" = "PASS" ] \
  || fail safe "active constraint must PASS a clean tree (no false positive)"
ok "5. SAFE: violation removed -> constraints.enforce PASS (active rule, clean tree)"

# 6. CITE — the derived finding exists and references the escape bead.
FINDING="$(find "$PROJ/.agents/findings" -maxdepth 1 -name '*.md' 2>/dev/null | head -1)"
[ -n "$FINDING" ] || fail cite "no derived finding written"
grep -q "age-demo" "$FINDING" || fail cite "derived finding does not cite the escape bead"
ok "6. CITE: derived finding written + cites the escape"

# 7. LOAD — the COGNITION half (EM.4 / .2.4). The DETERMINISTIC loader the next
# in-domain pre-mortem calls (`ao lookup --query <domain>`, per skills/pre-mortem
# SKILL.md) must SURFACE the escape-derived check by domain, CITING the escape —
# proving the membrane's memory is mechanically RETRIEVABLE, not just written to
# disk. Together with step 5's gate-BLOCK this is the FULL EM-parent claim
# ("the next in-domain pre-mortem auto-loads + cites + blocks").
LOOKED="$("$AO" lookup --query demo 2>&1 || true)"
printf '%s' "$LOOKED" | grep -q "age-demo" \
  || fail load "ao lookup --query <domain> must surface the escape-derived finding (the pre-mortem's load path)"
printf '%s' "$LOOKED" | grep -qiE "Escape:|false-done" \
  || fail load "the loaded entry must be the derived escape finding (cites the escape)"
ok "7. LOAD: ao lookup --query demo surfaces the derived check citing the escape (cognition half — EM.4)"

# 8. TRAVEL — the loop fires for EVERYONE, not just this box (EM.2.9). Publish the
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
ok "8. TRAVEL: published constraint enforces in a clean checkout with no .agents/ — the learning travels (EM.2.9)"

echo
echo "✓✓ EM LOOP CLOSED on the shipped ao binary: escape -> mechanical constraint -> active -> blocks re-introduction; the derived check is retrievable by the next in-domain pre-mortem (load+cite+block); and a published constraint TRAVELS to a clean CI checkout. The membrane self-improved — for everyone."
exit 0
