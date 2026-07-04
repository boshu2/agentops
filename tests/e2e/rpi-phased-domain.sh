#!/usr/bin/env bash
# tests/e2e/rpi-phased-domain.sh — RETIRED TOMBSTONE (age-ipbr, 2026-07-04).
#
# This was the F3.T2 e2e for bead soc-58nt.3.7. It exercised the `ao rpi phased
# --domain` / `--scaffold-domain` CLI surface end-to-end. That entire `ao rpi`
# command surface was REMOVED in f61c5f0e7 (ADR-0009 / age-tlj6 teardown — the
# out-of-session RPI loop was replaced by the operating loop + NTM/Agent Mail
# substrate). The test's subject no longer exists, so the real assertions were
# retired: `ao rpi phased ...` now exits 1 ('unknown command "rpi"'), which is
# what left the doctrine-proof job's F3 step red on every main push since
# 2026-06-17.
#
# WHY THE FILE STILL EXISTS (not deleted): skills/rpi/references/rpi.feature
# tags four scenarios `@covered-by:tests/e2e/rpi-phased-domain.sh`. The
# scenario->test linkage gate (scripts/check-scenario-test-linkage.sh, run in the
# skill-gates job) FAILS on a dangling @covered-by path. Fully deleting this file
# therefore requires the skills/ lane to first drop those @covered-by tags and
# allowlist rpi.feature (or retarget them at the operating-loop e2e). Until that
# skills-side retarget lands, this tombstone keeps the path resolvable so the
# linkage gate stays green while the dead `ao rpi` assertions no longer run.
#
# The F3 invocation was removed from .github/workflows/validate.yml in the same
# change. This stub is intentionally a no-op that exits 0.
set -euo pipefail

echo "RETIRED: tests/e2e/rpi-phased-domain.sh — the 'ao rpi' surface was removed in f61c5f0e7 (ADR-0009). No-op tombstone; see file header. (age-ipbr)"
exit 0
