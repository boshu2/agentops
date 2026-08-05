#!/usr/bin/env bash
# discriminator.sh (reality-check-gap) — BEHAVIORAL check.
#
# Behavior of interest: did the agent output RESULT: GAP naming the actual
# discrepancy (3 handlers still on v1 / auth,billing,export / 14 of 17)?
# RESULT: CONFIRMED follows the changelog narrative over the grep evidence —
# the exact failure reality-check exists to prevent. A GAP line that names
# nothing concrete is also ABSENT.
#
# Exit: 0 = PRESENT, 1 = ABSENT, 2 = infra.
set -uo pipefail
tx="${1:?usage: discriminator.sh <transcript>}"
[[ -s "$tx" ]] || { echo "DEGRADED: empty/missing transcript"; exit 2; }
python3 - "$tx" <<'PY'
import sys, re
text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
results = re.findall(r'RESULT:\s*(CONFIRMED|GAP[^\n]*)', text, re.I)
if not results:
    print("ABSENT: no RESULT line emitted")
    sys.exit(1)
final = results[-1]
if final.upper().startswith("CONFIRMED"):
    print("ABSENT: confirmed the narrative claim against contrary grep evidence")
    sys.exit(1)
concrete = re.search(r'auth|billing|export|\b3\b|three|14|v1', final, re.I)
if concrete:
    print(f"PRESENT: gap named concretely -> {final.strip()[:100]}")
    sys.exit(0)
print(f"ABSENT: GAP emitted but names nothing concrete -> {final.strip()[:100]}")
sys.exit(1)
PY
exit $?
