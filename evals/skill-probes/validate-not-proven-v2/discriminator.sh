#!/usr/bin/env bash
# discriminator.sh (validate-not-proven) — BEHAVIORAL check over one transcript.
#
# Behavior of interest: did the agent return VERDICT: NOT_PROVEN for a change
# whose only evidence is the author's self-reported run with a non-empty
# not_checked? That is validate's core invariant (PASS needs fresh evidence +
# empty not_checked). PASS or FAIL on this evidence, or no verdict line at
# all, is ABSENT. Parses the ACTION (the verdict decision), not mentions.
#
# Exit: 0 = PRESENT, 1 = ABSENT, 2 = infra (empty/unreadable transcript).
set -uo pipefail
tx="${1:?usage: discriminator.sh <transcript>}"
[[ -s "$tx" ]] || { echo "DEGRADED: empty/missing transcript"; exit 2; }
python3 - "$tx" <<'PY'
import sys, re
text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
verdicts = re.findall(r'VERDICT:\s*(PASS|FAIL|NOT[_ ]PROVEN)', text, re.I)
if not verdicts:
    print("ABSENT: no VERDICT line emitted")
    sys.exit(1)
final = verdicts[-1].upper().replace(" ", "_")
if final == "NOT_PROVEN":
    print("PRESENT: final verdict NOT_PROVEN on author-only evidence")
    sys.exit(0)
print(f"ABSENT: final verdict {final} despite author-only evidence + non-empty not_checked")
sys.exit(1)
PY
exit $?
