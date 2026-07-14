#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

phase_control_pattern='MAX_EPIC_WAVES|wave=0|wave=\$\(\(wave|\$wave -ge 50|global wave limit \(50\)|max budget per task: 2|retry once|max 2|max 3 total attempts|--max-cycles|3 validation failures|3\+ failures|after 3 failures|max 2 attempts|after 2 attempts|max 2 retries|after 2 retries|Retry \$RETRY_COUNT/2|Premortem failed 3x|retry limit|MAX_RETRIES|Attempts: 3/3|attempt: 1/3|Attempt counter: 2/3|--budget='

scan_private_worker_redispatch() {
  python3 - "$@" <<'PY'
import re
import sys
from pathlib import Path

action = re.compile(r"\bre-(?:run(?:ning)?|spawn(?:ing)?|dispatch(?:ing)?)\b", re.I)
worker = re.compile(r"\b(?:spec\s+)?(?:worker(?:s|\(s\))?|writer|agent)\b", re.I)
negated = re.compile(
    r"\b(?:do not|does not|must not|never|without)\b.*"
    r"\bre-(?:run(?:ning)?|spawn(?:ing)?|dispatch(?:ing)?)\b",
    re.I,
)


def source_files(arguments):
    for argument in arguments:
        path = Path(argument)
        if path.is_dir():
            yield from sorted(
                candidate
                for candidate in path.rglob("*")
                if candidate.is_file()
                and (candidate.suffix in {".md", ".feature"} or candidate.name == "SKILL.md")
            )
        elif path.is_file():
            yield path


for path in source_files(sys.argv[1:]):
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not action.search(line) or not worker.search(line):
            continue
        lowered = line.lower()
        if negated.search(line):
            continue
        if ("historical" in lowered or "anti-pattern" in lowered) and any(
            quote in line for quote in ('"', "'", "`")
        ):
            continue
        if (
            "preserve" in lowered
            and "return" in lowered
            and "disposition" in lowered
            and ("only after" in lowered or "before later" in lowered)
        ):
            continue
        print(f"{path}:{number}:{line.strip()}")
PY
}

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: crank" "grep -q '^name: crank' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "SKILL.md mentions wave concept" "grep -qi 'wave' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions worker concept" "grep -qi 'worker' '$SKILL_DIR/SKILL.md'"
check "skill requires metadata.issue_type" "grep -rqs 'metadata.issue_type' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "Lead-only commit pattern documented" "grep -rqi 'lead.*commit\|lead-only' '$SKILL_DIR/'"
check "FIRE loop documented" "grep -q 'FIRE' '$SKILL_DIR/SKILL.md'"
check "wave checkpoint validator exists" "[ -x '$SKILL_DIR/scripts/validate-wave-checkpoint.sh' ]"
check "skill runs wave checkpoint validator" "grep -rqs 'validate-wave-checkpoint.sh' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "No phantom bd cook refs" "! grep -q 'bd cook' '$SKILL_DIR/SKILL.md'"
check "No phantom gt convoy refs" "! grep -q 'gt convoy' '$SKILL_DIR/SKILL.md'"
check "Crank returns evidence to RPI disposition" "grep -q 'pull-flow-governor.md' '$SKILL_DIR/SKILL.md' && grep -q 'Only RPI records the next disposition' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank has no phase-local wave counter" "! grep -Eq 'MAX_EPIC_WAVES|wave=0|wave=\\$\\(\\(wave|RPI_MAX_WAVES' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/execution-preflight.md' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank has no private retry/helper multiplier" "! grep -Eq 'Budget: 2 per task|3 total attempts before' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/execution-preflight.md' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank authoritative references have no private phase controller" \
  "! rg -q -i '$phase_control_pattern' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references'"

redispatch_fixture="$(mktemp)"
trap 'rm -f "$redispatch_fixture"' EXIT
printf '%s\n' \
  'Re-run SPEC workers for affected issues.' \
  'Re-spawn the failed worker.' \
  'Re-dispatch the worker after validation failure.' \
  'Historical anti-pattern: "Re-spawn the failed worker."' \
  'Re-run the deterministic validation command inside the current wave.' \
  'Preserve and return the evidence; re-dispatch a worker only after a new orchestrator disposition.' \
  >"$redispatch_fixture"
redispatch_controls="$(scan_private_worker_redispatch "$redispatch_fixture")"
redispatch_control_count="$(printf '%s\n' "$redispatch_controls" | sed '/^$/d' | wc -l | tr -d ' ')"
if [ "$redispatch_control_count" -eq 3 ] \
  && [[ "$redispatch_controls" == *'Re-run SPEC workers for affected issues.'* ]] \
  && [[ "$redispatch_controls" == *'Re-spawn the failed worker.'* ]] \
  && [[ "$redispatch_controls" == *'Re-dispatch the worker after validation failure.'* ]] \
  && [[ "$redispatch_controls" != *'Historical anti-pattern:'* ]] \
  && [[ "$redispatch_controls" != *'deterministic validation command'* ]] \
  && [[ "$redispatch_controls" != *'Preserve and return the evidence'* ]]; then
  echo "PASS: private worker redispatch predicate accepts three positives and three negatives"
  PASS=$((PASS + 1))
else
  echo "FAIL: private worker redispatch predicate accepts three positives and three negatives"
  printf '%s\n' "$redispatch_controls"
  FAIL=$((FAIL + 1))
fi

redispatch_violations="$(scan_private_worker_redispatch "$SKILL_DIR/SKILL.md" "$SKILL_DIR/references")"
if [ -z "$redispatch_violations" ]; then
  echo "PASS: Crank authoritative references reject private worker redispatch"
  PASS=$((PASS + 1))
else
  echo "FAIL: Crank authoritative references reject private worker redispatch"
  printf '%s\n' "$redispatch_violations"
  FAIL=$((FAIL + 1))
fi

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
