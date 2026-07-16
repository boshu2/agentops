---
compiler_targets: premortem
detectability: advisory
escape_bead_id: age-harvest-rfd-nested-schema
escape_confirmed_sha: a3b9bcb
escape_domain: membrane-eval
escape_missed: deterministic oracle (score.sh) FAILs: the cross-family membrane ACKed a real false-done — a genuine membrane miss
escape_refuted_sha: 563b7e2
escape_run_id: harvest-qwen-codex-2026-06-22
id: escape-age-harvest-rfd-nested-schema-harvest-qwen-codex-2026-06-22-a3b9bcb-563b7e2
severity: significant
source: escape
source_skill: membrane
status: active
title: Escape: age-harvest-rfd-nested-schema confirmed then refuted
type: finding
---
# Pre-Mortem Check (escape-age-harvest-rfd-nested-schema-harvest-qwen-codex-2026-06-22-a3b9bcb-563b7e2)

Bead `age-harvest-rfd-nested-schema` was CONFIRMED by the membrane at attempt 1 (`a3b9bcb`) but a later attempt-3 review REFUTED it (`563b7e2`). The membrane let a false-done through.

**Domain:** membrane-eval — look out for this class of miss when working here.

**What was missed:** deterministic oracle (score.sh) FAILs: the cross-family membrane ACKed a real false-done — a genuine membrane miss

**Detection question:** before CONFIRMING a unit like this, has its acceptance been re-verified by a fresh-context refuter that does NOT trust the prior CONFIRMED verdict — re-running the deterministic acceptance check on the claimed head, not the verdict text?
