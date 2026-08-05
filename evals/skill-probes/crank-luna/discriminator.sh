#!/usr/bin/env bash
# discriminator.sh (crank) — BEHAVIORAL check over one probe transcript.
#
# Behavior of interest: did the agent emit a WAVE PLAN that keeps the two
# write-scope-colliding beads (bead-B and bead-C both write shared.go) in
# DIFFERENT waves (serialized), rather than co-scheduling them in the same
# parallel wave? That is crank's core invariant: group into a wave only when
# write scopes do not collide.
#
# This measures the ACTION (the actual wave assignment the agent produced), not
# whether the transcript mentions "write scope" — a plan that talks about
# collisions but still lists B and C together in one wave is ABSENT.
#
# Exit: 0 = behavior PRESENT, 1 = ABSENT, 2 = infra (empty/unreadable transcript).
set -uo pipefail

tx="${1:?usage: discriminator.sh <transcript>}"
[[ -s "$tx" ]] || { echo "DEGRADED: empty/missing transcript"; exit 2; }

python3 - "$tx" <<'PY'
import sys, re
lines = open(sys.argv[1], encoding="utf-8", errors="replace").read().splitlines()
waves = {}  # wave-id -> set of bead letters
for ln in lines:
    m = re.match(r'\s*Wave\s+(\d+)\s*[:\-]\s*(.*)', ln, re.I)
    if not m:
        continue
    wid = int(m.group(1))
    beads = {b.upper() for b in re.findall(r'bead[-_ ]?([A-Da-d])\b', m.group(2))}
    waves.setdefault(wid, set()).update(beads)

if not waves:
    print("ABSENT: no wave plan emitted")
    sys.exit(1)

wb = {w for w, b in waves.items() if 'B' in b}
wc = {w for w, b in waves.items() if 'C' in b}
if not wb or not wc:
    print("ABSENT: bead-B or bead-C not scheduled in any wave")
    sys.exit(1)
if wb.isdisjoint(wc):
    print("PRESENT: colliding beads B and C separated into different waves")
    sys.exit(0)
print("ABSENT: B and C co-scheduled in the same wave (write-scope collision ignored)")
sys.exit(1)
PY
exit $?
