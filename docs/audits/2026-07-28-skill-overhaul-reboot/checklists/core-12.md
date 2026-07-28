# Core-12 Skill Overhaul Checklist

Skills covered (audit order): **rpi · plan · implement · validate · learn · scope · reality-check · research · premortem · postmortem · council · idea-genie**

> Distilled from `/tmp/agentops-opus5-verified-skill-audit-core-12-v8.md` (read-only audit of the tree at branch `codex/skill-overhaul-20260724`, HEAD `0088c6e3`).
> **Every item below is a HYPOTHESIS to verify against the live tree.** The `file:line` cites are the AUDIT's claims, not confirmed facts — re-check before acting; the tree has moved on since.
> Tags: `[defect]` = wrong/broken today · `[enhance]` = contract/clarity improvement · `[possibly-landed]` = may already be fixed on main by PRs #995/#996 — verify first.
>
> **Cross-cutting (doctrine docs, not a skill) — [defect]:** `AGENTS.md:55` and `docs/architecture/operating-loop.md:10,12,64,70` still speak v1/v2 vocabulary (`verdict.v2`, `subject-manifest.v1`, `validate.py`) while the executable loop emits `verdict.v3` / `subject-manifest.v2` / `scope-index.v1` / `rpi-report.v2` via `validate_v3.py`. Both docs are outranked by live behavior. Rewrite to v3 with a short legacy note — vocabulary only; do not change what either grants.

## rpi
- [defect] `scripts/validate.sh:9-10` — two inert `! grep -Fq` negations ("Continuation envelope", "repair revision per wave"); under `set -euo pipefail` a `!`-inverted command never triggers exit, so the guard is a silent no-op. Fix to `if grep -Fq …; then exit 1; fi`. Witness: append the forbidden phrase to a scratch SKILL.md copy — the validator still exits 0.

## plan
- [defect] `scripts/validate.sh:8` — inert `! grep -Fq 'plan-packet.v1'` negation (same no-op class as rpi).
- [defect] `scripts/validate.sh:9` runs `mint_intent.py --help` with no `PYTHONDONTWRITEBYTECODE`/`-B`, unlike its rpi/implement/validate siblings; since `mint_intent.py` importlib-loads `kernel_v3.py`, a cold-cache run writes `kernel_v3.cpython-314.pyc` into ANOTHER skill's `__pycache__`. Fix: prefix `PYTHONDONTWRITEBYTECODE=1` (gitignored output; hygiene/asymmetry, not a tracked-byte change).
- [enhance] No CLI tests — `--expected-digest` mismatch refusal and the `argparse main()` path (`mint_intent.py:41-68`) are untested. Add `skills/plan/tests/`.
- [enhance] `SKILL.md:11-12` `produces: scope-index.v1` names a capability whose executable lives in `validate` (`freeze-scope`), not plan; `grep -rn 'freeze-scope' skills/plan/` is empty. Clarify ownership.

## implement
- [defect] `scripts/validate.sh:8-9` — two inert `! grep` negations (no-op). Note: the validator does not actually grep for commit/push/claim/close/retry at all.
- [enhance] `scripts/freeze_candidate.py:13-30` `find_kernel()` — if the sibling kernel is missing it walks every ancestor for `<ancestor>/agents/validator/skills/validate/scripts/kernel_v3.py` and executes whatever it finds with no identity/digest check. Harden or remove the ancestor-walk fallback (executes unverified code; run_once.py and mint_intent.py have no such fallback).
- [enhance] `consumes: scope-index.v1` has no implement-owned reader.
- [enhance] No adapter-level tests (`skills/implement/tests/`).

## validate
- [defect] The v2 writer contradicts the skill's own grep-asserted invariant: `SKILL.md:106` says "Never re-snapshot intent during storage" (and `validate.sh` greps that literal), yet `validate.py store-verdict` (`:552-558`) reads `--intent-source` from a LIVING file and calls `snapshot_intent()` before persisting `verdict.v2`. Gate v2 `store-verdict` behind an explicit legacy flag (`validate.py` is unbound — code fix needs no ceremony).
- [defect] Dangling path citation: `SKILL.md:119` points an external caller at `scripts/record_proof_transition.py` (repo-root), which does not exist; the real file is `skills/validate/scripts/record_proof_transition.py`.
- [defect] Dual path grammars: `validate.py:38-46` `normalize_rel` rewrites `\`→`/` and strips `./`, while `kernel_v3.py:180-199` rejects both outright — inconsistent path handling; document, preferably reconcile.
- [possibly-landed] ADR-0016 §3 wants the unpinned governed Python (`kernel_v3.py`, `validate_v3.py`, `record_proof_transition.py`, `check_kernel_v3_corpus.py`, plus plan's `mint_intent.py`, implement's `freeze_candidate.py`, rpi's `run_once.py`) moved to Go/out of `skills/*/scripts`, but they are digest-bound components re-hashed on every run — the rpi/validate digest binding and the Python→Go ratchet gate collide. May be resolved by #995/#996.

## learn
- [defect] `scripts/validate.sh:6` is live-false-passing: `! grep -Eiq 'receipt|plan_impact|next_action|retry|delivery|closure' SKILL.md` matches "emit a lifecycle receipt" at `SKILL.md:35`, `!` inverts, `set -e` exempts inverted commands → PASS rc 0. The one corpus case where the forbidden token is ALREADY present and the guard is ALREADY silent. Fix to `if grep …; then exit 1; fi` + reword `SKILL.md:35`.
- [defect] `SKILL.md:44-47` cites two verdict digests under `.agents/ao/verdicts/` that resolve 0 of 2 — the skill violates its own "dead citations get pruned" decay rule.
- [defect] v2 vocabulary: `consumes: [verdict.v2]` (`SKILL.md:8-9,30`) + body prose while the live writer emits `verdict.v3`.
- [enhance] No scenario coverage (`check-scenario-coverage.sh` fail 0/2).

## scope
- [enhance] The YAML output block is prose-only — add an output validator (`skills/scope/scripts/`).
- [enhance] No feature file (`skills/scope/references/`).

## reality-check
- [defect] Dangling output contract: `produces: [reality-check-report.v1]` / `output_contract: reality-check-report.v1` names a contract with zero schema hits anywhere.
- [enhance] Skill holds exactly one tracked file — no `scripts/validate.sh`. Give the named contract a schema+validator (copy `idea-genie/scripts/validate-output.sh` or `premortem/scripts/validate-output.sh`).

## research
- [enhance] `effects: []` (`SKILL.md:14,16-17`) versus a declared `Write` — missing/contradictory effects declaration (same class as postmortem below).
- [enhance] No scenario coverage (fail 0/3).

## premortem
- [defect] v2 vocabulary: `SKILL.md:86` "not `verdict.v2`" should read "no verdict of any version".
- [enhance] No scenario coverage (fail 0/2).

## postmortem
- [defect] `output_contract` points at a `.feature` file (`SKILL.md:30`).
- [defect] v2 vocabulary: `consumes: [verdict.v2]`.
- [enhance] `effects: []` (`SKILL.md:17`) while the skill produces/emits/fully specifies `postmortem-report.md` (`:10-11`, `:64-65`, `:91-95`). Either declare a write-effect scoped to the artifact dir, or make the output response-only.
- [enhance] Artifact dir `.agents/postmortem/` (`SKILL.md:91`) sits outside the ADR-0016 closed set (`ao/`, `scratch/`, `projections/`); move under `scratch/`.
- [possibly-landed] The ADR-0016 §1 enforcement detector `fm-ws-noncanonical-topdir` (an `ao doctor` check, bead `age-state-tiers-operationalize-5mzlm.7`) is absent from `cli/` and `scripts/`, so §1 is inert prose. May have been built by #995/#996.
- [enhance] No scenario coverage (fail 0/2).

## council
- [defect] Dangling output contract: `council-report.v1` (`SKILL.md`) has zero schema hits.
- [defect] v2 vocabulary (`SKILL.md:78`).
- [enhance] No `scripts/validate.sh`, no feature file — give `council-report.v1` a schema+validator (copy `idea-genie/scripts/validate-challenge.sh`).

## idea-genie
- [enhance] Artifact dir `.agents/ideas/<run-id>/` (`SKILL.md:98,104`) violates the ADR-0016 closed set; move under `scratch/` and update `tests/scripts/agentops-native-skills.bats` fixtures (embed `.agents/ideas/run-1`, `run-2`). Validator logic unchanged.
- [enhance] Add a shared context-isolation reference linked from BOTH council and idea-genie (the two duel/quorum skills overlap only on the isolation checklist; strategies stay distinct).

---

> ~5 lower-priority items dropped — the proof-epoch / verdict / packet-freeze machinery the reboot abandoned, plus no-action findings: rpi proof-bound-membership disclosure prose, rpi correlation-bound "no action", validate `test_kernel_v3.py` relocation hygiene, and the audit's rejected/superseded findings. Not covered by this audit: the `regen-all.sh` executable bit (verify separately against #995/#996).
