# Loop restore: converge and crank as control flow under the verdict contract

> **Status:** r2 2026-09-03, EXECUTED on Bo's "do it" (landed with this PR; lanes A, B, C plus per-lane regen commits). r1 had one cross-family
> read (13 findings, all folded here; that read is the only plan review this
> intent gets). · **Intent source:** this document. **Provenance:** the
> 2026-09-03 finding that the 2026-07-14 single-pass cut (`482307762`) removed
> the iterate loop together with the unproven compounding claim, although
> ADR-0011 demoted only the latter; and the 2026-09-02 Train 1 run, where the
> loop was improvised by hand (8 validators, 2 stops) because the contract had
> no repair phase. **Retirement:** superseded when the lanes merge.
> **Consumer:** the lanes, one fresh validator on the final tip, one
> cross-family read of the integrated diff.
> **Constraint floor:** no new `ao` root command (`ao converge` / `ao crank`
> stay removed); no new `skills/*/scripts/**/*.py` (editing grandfathered
> `run_once.py` is allowed; its tests are exempt; the grandfather file is not
> touched); `schemas/**` byte-identical; ADR-0004 and ADR-0011 stay in force;
> **ADR-0017** records the narrow reversal of the cut's over-reach and is the
> authority for every conformance assertion this plan flips.

## The one behavior

An RPI invocation that gets `FAIL` or `NOT_PROVEN` with findings repairs and
re-validates under a convergence law bounded by the caller, and stops only
when converged, stopped by the law, or out of budget. A multi-wave intent is
executed one wave per `crank` invocation; the caller selects the wave and the
repair bound, crank forwards them and returns wave evidence. Validate is
cross-family by default when the diff touches a risky surface, with a
LAW-0-safe dispatch shape per direction. Premortem stays a single advisory
judge; Plan names the first check and Implement runs it RED — those phase
boundaries do not move.

Countermetrics: `ao gate check --full` green (with a HEAD-built binary); CI's
literal bats command green once on the final integrated tip; routing goldens
green; no description > 180 chars; the Claude catalog grows by exactly one
description (crank); `schemas/**` unchanged; the Go build bar green (a Go
file changes in L-A).

## Ground truth

- Old `converge` (`git show '482307762~1:skills/converge/SKILL.md'`): converged
  ⇔ ≥ 2 distinct non-author contexts PASS with zero FAIL, each citing the
  executed acceptance; BLOCK after 3 consecutive failing rounds; the fix step
  is the orchestrator's; judge legs never mutate. Restored: the criterion.
  Not restored: the Go command, the canary, the findings registry.
- Old `crank`: one ready wave, acceptance once, evidence returned, no retry or
  re-plan inside. Restored: that shape. Not restored: flags, lifecycle tiers,
  Sisyphus markers, eight references.
- Assertions that currently forbid the restoration and are flipped ONLY under
  ADR-0017, all owned by L-A: `scripts/check-cathedral-cut-conformance.py:57-59,991-993,1066-1073`
  (crank listed as removed; RPI must contain "Stop regardless"; loops in
  `run_once.py` rejected), `workflows/rpi.js:4-9,179-180` (single-pass),
  `skills/rpi/scripts/validate.sh:9`, `skills/rpi/tests/test_run_once.py:149`
  (stop-on-FAIL), `evals/agentops-core/rpi-behavior.json:31-52`. Public
  single-pass surfaces that must agree: `PRODUCT.md:19,56`, `docs/CI-CD.md:16`,
  `docs/agent-workflow-reference.md:10`, `cli/internal/commands/quickstart/module.go:44-56`,
  `tests/scripts/agents-operating-contract.bats:3-20`,
  `tests/scripts/legible-l1-codex-descriptions.bats:154-168` (the rpi literal).
- Dispatch legality (`AGENTS.md:74`, `skills/agent-native/references/model-dispatch.md:8,38-46`):
  Codex is the only permitted headless leg; Claude must be interactive.
- `verdict.v2` carries `findings[].id` (`schemas/verdict.v2.schema.json:44-54`);
  categories are neither stable nor present. `not_checked` means unverified
  in-scope acceptance only (`skills/validate/SKILL.md:82-99`).
- A new skill must satisfy `schemas/skill-frontmatter.v2.schema.json` (fields
  `practices`, `hexagonal_role`, `metadata.tier/dependencies/capabilities/effects/canonical_status/disposition`)
  and `python3 scripts/generate-skill-mesh.py --check`; the probe-coverage
  denominator excludes meta tier by construction, so no exclusion line.
- Routing goldens are JSON under `evals/routing-probes/goldens/` per
  `schemas/pack-quality-expectations.v1.schema.json`; the
  `tests/explicit-skill-requests` runner is archived and is not proof.

## The convergence law (rpi; crank forwards the caller's bound)

A repair round is admitted only while all hold:

1. `rounds_used < repair_rounds` (caller-declared, default 2).
2. The open finding set, keyed by the validators' stable `findings[].id`
   (union across the fresh and, when used, cross-family validators), is not
   larger than the previous round's.
3. No finding id closed in an earlier round reopens.
4. Between rounds either the subject-manifest digest changed (generated-only
   changes count when they change the digest) or, for `NOT_PROVEN`, new
   digest-bound evidence was supplied that resolves a named gap.

Converged ⇔ the fresh validator returns PASS and, when the diff touches a
risky surface, the cross-family validator also returns PASS. On any violation
of 1-4 RPI stops and reports the current status; `checked` carries one line
per round (`repair round N: k open findings`), the open findings ride in the
validation result and the interactive report, and `not_checked` keeps its
meaning. No third judge, no escalation, no auto-replan.

Risky surface (cross-family default): `cli/internal/gates/**`,
`scripts/check-*.sh`, `tests/**`, `skills/*/scripts/**`,
`skills/cc-hooks/policies/**`, `lib/**`, anything `security-gate.sh` scans.

Cross-family dispatch (LAW 0): orchestrating in Claude → judge leg is
read-only `codex exec`; orchestrating in Codex → judge leg is a caller-selected
interactive Claude session in an NTM pane, never `claude -p` / `--print`. No
authorized live adapter ⇒ `diversity_unsatisfied`; on a risky surface that
stops as `NOT_PROVEN` rather than converging same-family.

## Lanes (parallel build, serialized regen: A → B → C)

Each lane regenerates locally to run its checks but commits regenerated
outputs only after rebasing onto the previous lane's final tip (A commits its
own immediately; B rebases onto A then regenerates and commits; C onto B).

**L-A — the RPI contract and every surface that asserts single-pass** (L)
- `skills/rpi/SKILL.md`: step 5 becomes the repair phase under the law; the
  spiral breaker fires on a law violation or two consecutive rounds without a
  subject/evidence change, never on verdict count; new §"Waves" pointing at
  crank; §"Cross-family" pointing at the dispatch table in validate.
  Description ≤ 180 with the triggers kept.
- `skills/rpi/scripts/run_once.py`: model the loop as pure data (validate
  results with `findings[].id`, subject digest, evidence refs, `repair_rounds`);
  implement the law; `checked` per-round lines; open findings returned in the
  result object. RED first in `skills/rpi/tests/test_run_once.py`: converge in
  one round; stop at budget; stop on growing set; stop on reopened id; stop on
  unchanged digest without evidence; NOT_PROVEN resolved by evidence only;
  reworded summaries with the same id are the same finding; cross-family union;
  generated-only digest change counts; NOT_PLANNED/NOT_BUILT unchanged;
  replace the old stop-on-FAIL test at `:149`.
- Flip, under ADR-0017 (cite it in each): `scripts/check-cathedral-cut-conformance.py`
  (crank restored; "Stop regardless" replaced by the law; bounded loop in
  `run_once.py` allowed with positive canaries for the law), `workflows/rpi.js`
  (repair phase), `skills/rpi/scripts/validate.sh:9`, `evals/agentops-core/rpi-behavior.json`.
- Public surfaces: `AGENTS.md` §"Standard RPI traversal" step 4; `docs/architecture/rpi-traversal.md`
  §"Stop boundary and revision"; `PRODUCT.md:19,56`; `docs/CI-CD.md:16`;
  `docs/agent-workflow-reference.md:10`; `cli/internal/commands/quickstart/module.go:44-56`
  (+ its test if any); `tests/scripts/agents-operating-contract.bats`;
  `tests/scripts/legible-l1-codex-descriptions.bats` rpi literal.
- Acceptance: `python3 -m pytest skills/rpi/tests -q` green (≥ 9 new tests);
  `bash skills/rpi/scripts/validate.sh`; `python3 scripts/check-cathedral-cut-conformance.py`
  green; `bash scripts/check-skill-python-ratchet.sh --scope head` green with
  the grandfather file untouched; `cd cli && go build ./... && go vet ./... && go test ./...`;
  `bats tests/scripts/agents-operating-contract.bats tests/scripts/legible-l1-codex-descriptions.bats`;
  `bash scripts/regen-all.sh --check` clean after committing regen; `ao gate check --scope head`.

**L-B — crank, thin** (M)
- New `skills/crank/SKILL.md` ≤ 120 lines: v2-complete frontmatter modelled on
  `skills/rpi/SKILL.md` (tier meta, disposition keep, dependencies [rpi],
  capabilities [execute_wave], effects [dispatch_rpi_per_lane]); `## Prompt`
  and `## It's working if` blocks. Contract: input = the caller's selected wave
  (a list of lanes with scope, brief, acceptance) and the caller's
  `repair_rounds`; one invocation executes that wave by invoking RPI per lane
  (parallel only on disjoint write scopes and disjoint regen surfaces,
  otherwise serial), runs the wave's acceptance once, returns wave evidence
  (per-lane status, open findings, subject digests) and stops. Crank owns no
  wave selection, retry, budget, queue, claim, lease, Git, closure, or next
  work; it names `workflows/implement-wave.js` as the Claude conveyor only.
  No reference to `ship-beads`.
- Routing golden: a complete JSON under `evals/routing-probes/goldens/`
  (schema `pack-quality-expectations.v1`) expecting crank for "execute the
  next wave of this plan" and omitting validate; rewrite
  `tests/explicit-skill-requests/prompts/crank.txt` to the one-wave boundary
  if it exists; do not cite the archived runner as proof.
- Acceptance: `bash scripts/validate-skill-frontmatter.sh --strict`;
  `bash scripts/validate-skill-schema.sh`; `bash scripts/validate-skill-triggers.sh`;
  `python3 scripts/generate-skill-mesh.py --check` after regen;
  `bash scripts/check-routing-probe-goldens.sh` green (the new golden green);
  `bash tests/skills/test-token-budgets.sh`; `bash scripts/check-skill-probe-coverage.sh`
  green with NO change to the exclusions file; `python3 scripts/check-cathedral-cut-conformance.py`
  green only after rebasing onto L-A; `ao gate check --scope head`.

**L-C — validate's cross-family default, ADR-0017, doc repairs** (S)
- `skills/validate/SKILL.md`: §"Cross-model fresh validator" becomes
  default-on for the risky-surface list with the LAW-0 dispatch table above
  and the `diversity_unsatisfied` ⇒ `NOT_PROVEN` rule; routine rounds keep the
  bounded, receipt-driven freshness contract; the full literal CI command set
  is required once, on the final integrated subject. Description ≤ 180.
- `docs/adr/ADR-0017-loop-as-control-flow-not-knowledge.md`: Accepted; builds
  on ADR-0004/0011; records that the 07-14 cut removed control flow the ADRs
  never demoted; restores converge's criterion and crank's shape under the
  verdict contract; lists every conformance assertion flipped; states what
  stays unproven (compounding; the loop's own effect, owed a seeded probe) and
  what stays removed (`ao converge`, `ao crank`, evolve, the learn write-half).
  Built by MkDocs: `scripts/docs-build.sh --check` green, links fixed rather
  than allowlisted.
- `README.md` traversal line; `docs/plans/2026-09-02-legible-membrane-plan.md`
  header de-mangled (lines 8-15).
- Acceptance: `scripts/docs-build.sh --check`; doc-link gate; `ao gate check --scope head`;
  `rg -n 'report and stop' README.md` shows the amended line.

**I — integration:** rebase B onto A, C onto B; regen after each; final tip:
`bash scripts/regen-all.sh --check` clean; `ao gate check --full` with a
HEAD-built binary; CI's literal bats command; Go bar; lint; security quick;
one fresh validator over the whole diff against this document; one
cross-family read of the integrated diff (it touches `tests/**` and
`skills/*/scripts/**`); repair under the law with `repair_rounds = 2`; PR;
auto-merge.

## Non-goals

No knowledge store, no learn write-half, no `ao converge`/`ao crank`, no
canary, no evolve or operator-stop rules, no crank flags or lifecycle tiers,
no premortem loop, no Plan exit change, no `verdict.v2`/`rpi-report.v1`
change, no Train 2 legibility, no measurement work (move 2), no
`ship-beads` changes.

## First useful check

`skills/rpi/tests/test_run_once.py::test_repair_stops_when_a_closed_finding_reopens`
— RED on `main` because `run_once.py` has no repair loop.
