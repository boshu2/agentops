#!/usr/bin/env bash
# discriminator.sh (standards-go-conventions) — BEHAVIORAL check.
#
# Behavior of interest: does the produced Go follow the repo standards the
# skill carries — BOTH (a) error wrapping with fmt.Errorf + %w, and (b) a
# table-driven test ([]struct test cases or t.Run subtests)? Either alone is
# ABSENT: the standards travel together. Checks the emitted CODE, not prose.
#
# Exit: 0 = PRESENT, 1 = ABSENT, 2 = infra.
set -uo pipefail
tx="${1:?usage: discriminator.sh <transcript>}"
[[ -s "$tx" ]] || { echo "DEGRADED: empty/missing transcript"; exit 2; }
python3 - "$tx" <<'PY'
import sys, re
text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
wraps = re.search(r'fmt\.Errorf\([^)]*%w', text)
table = re.search(r'\[\]struct\s*{', text) or re.search(r't\.Run\(', text)
if wraps and table:
    print("PRESENT: %w wrapping + table-driven test both present")
    sys.exit(0)
missing = []
if not wraps: missing.append("no fmt.Errorf %w wrapping")
if not table: missing.append("no table-driven test")
print("ABSENT: " + "; ".join(missing))
sys.exit(1)
PY
exit $?
