---
name: pre-land-refuters
description: "Dispatch cross-family refuters to attack a completion claim at the mutate-shared-trunk pawl before landing, at any complexity. Triggers: pre-land validation, refute pre-push."
practices:
- llm-eval-harness
- ai-assisted-dev
hexagonal_role: driving-adapter
consumes:
- validate
- codex-exec
produces:
- .agents/council/*.md
context_rel:
- kind: customer-of
  with: validate
- kind: customer-of
  with: codex-exec
skill_api_version: 1
user-invocable: true
metadata:
  tier: judgment
  dependencies:
  - validate
  - codex-exec
  internal: false
output_contract: .agents/council/YYYY-MM-DD-pre-land-*.md
---

# /pre-land-refuters — unbiased dual-model validation before landing

> Proven in the ag-s43tg prune landing (2026-06-12): the refuter panel caught 9
> real misses self-review passed over — a silently-failed edit, a CI-breaking
> test, stale image manifests, gate-weakening test retirements, and an upstream
> delete/modify conflict. Self-review is biased toward "looks good"; refuters
> are prompted to win by finding what's wrong.

## When to fire

Fire at a **pawl** — a one-way door on the canonical static list
([docs/contracts/pawls.md](../../docs/contracts/pawls.md)): **mutate shared
trunk** (push/merge to main or rewrite a shared ref), **delete**,
**external-send / shared-state mutation**, **schema/contract change**,
**credential/authority change**, **spend**. The pawl is the only place the
cross-family panel runs. This is the ratchet's Filter: gate at the irreversible
door, nowhere else. (pawls.md is the source of truth — if it changes, this list
follows it.)

**NOT on a tread.** Routine edits, builds, tests, drafts, intermediate RPI
slices, mock→real swaps, throwaway experiments — all run as chaos, **ungated**.
The panel costs two agent runs; spend it at the door, never per-step. A pawl on
every step is waterfall (validate every tread) — exactly the thing the ratchet
exists to avoid. Check the action against the pawl list (a lookup); if it isn't
there, just run it.

## Constraints

- **Pin acceptance BEFORE the work.** The claim under test must be mechanical:
  grep-able fixtures (pinned phrases, counts, ledger states) frozen before
  implementation, not chosen post-hoc. No pins → write them first.
- **Refuters are read-only and stake-free.** Fresh context, no session history,
  no authorship of the change. Prompt them to REFUTE, default to skepticism.
- **Two model families minimum.** One Fable/Claude subagent + one `codex exec
  --sandbox read-only` validator. Same-family redundancy misses shared blind spots.
- **Findings are fixed forward, never disarmed.** A refuted contract test gets
  an honest repoint to the surviving surface or a real fix — not deletion.
- **Orchestrator stays the single writer.** Refuters report; only the
  orchestrator edits. Run the panel concurrently with the final full gate.
- **Re-verify pins on the landed tree** after merge/push, not just pre-commit.

## Workflow

1. **Freeze the claim.** State it in one sentence with mechanical acceptance
   (e.g. "all N pinned phrases grep green; ledger has N terminal rows; staged
   set is one revert unit").
2. **Dispatch the Fable refuter** (background subagent, fresh context):
   verify counts, sweep every pinned fixture, audit the ledger, hunt stragglers
   referencing removed paths, spot-check routing, check revert-unit coherence
   and upstream drift (`git fetch` + behind-count). Output: VERDICT
   CONFIRMED/REFUTED + numbered findings with evidence.
3. **Dispatch the codex refuter** (`codex exec --sandbox read-only -C <repo>`):
   focus on judgment-sensitive edits — for each contract-test/canary/validator
   change in the diff, judge: honest repoint vs gate-weakening. Same verdict
   shape.
4. **Run the full local gate concurrently** (it is the third, mechanical
   refuter).
5. **Triage findings**: fix each forward; classify pre-existing vs introduced;
   re-run only the affected validators.
6. **Write the machine-checkable verdict, THEN land.** Before the merge/push,
   record the panel result as the cross-family pawl verdict the merge path
   enforces against:
   ```bash
   scripts/pawl-verdict.sh write <bead> <pr> \
     --disposition CONFIRMED \
     --refuter claude:CONFIRMED --refuter codex:CONFIRMED \
     --council .agents/council/$(date +%F)-pre-land-<slug>.md
   ```
   (disposition `REFUTED` on any refuted refuter; `ESCALATE`/`HOLD` on
   non-convergence — those make the merge path HOLD, exit 5.) `scripts/reconcile-pr.sh`
   reads this with `scripts/pawl-verdict.sh check <bead> <pr>` and **refuses to merge
   without a CONFIRMED, cross-family, this-bead+PR verdict** — green CI alone never
   authorizes the door. Then land (commit → merge upstream if it moved → gate →
   push), re-run the pinned sweep on the landed tree, and write the free-form
   narrative in `.agents/council/YYYY-MM-DD-pre-land-<slug>.md`
   (the human-readable companion to the checkable verdict).

## Escalation — autonomous panel, human only on non-convergence

The panel runs **autonomously: model reviews model.** The human is NOT a checkpoint at the
pawl by default — they are the exception. See [docs/contracts/pawls.md](../../docs/contracts/pawls.md)
"Escalation".

- **Both refuters CONFIRMED (+ green gate)** → land. No human.
- **Any REFUTED** → orchestrator fixes the findings forward and **re-dispatches the panel**,
  autonomously, up to **N attempts (default 3)**. The loop self-corrects model-to-model; no
  human in the loop. This is the ordinary path.
- **ESCALATE to a human — ONLY when the models can't converge:** (1) **reviewer deadlock** —
  refuters contradict and stay contradicted after a re-gate and the diff can't arbitrate;
  (2) **N attempts exhausted** — still REFUTED after the default 3 re-work/re-gate cycles;
  (3) a refuter **explicitly flags a value / irreversibility judgment** models should not make
  alone. This is the **andon** ("Hey! Listen!") — rare, earned, never the default.

**ESCALATE / N-attempts-exhausted means HOLD — the door stays closed until a human resolves
it.** Record the disposition as `ESCALATE` (or `HOLD`) in the machine-checkable verdict and
**do not land**: a non-convergent pawl is never auto-merged. The enforcing merge path
(`scripts/reconcile-pr.sh` → `scripts/pawl-verdict.sh check`) exits **5 (HOLD: no merge, no
close)** on any disposition that is not `CONFIRMED`. Only both-refuters-`CONFIRMED`, ≥2
distinct families, tied to this bead+PR, opens the door (fail-closed by construction).

Even fully unattended, the gate fires at every pawl. Escalation is the exception, not the gate.

## Output Specification

**Format:** a council artifact at `.agents/council/YYYY-MM-DD-pre-land-<slug>.md`
containing: the frozen claim, both refuter verdicts (verbatim findings), the
fix-forward disposition per finding, and the post-land pin re-verification.

## Quality Rubric

- [ ] Claim frozen with mechanical acceptance before refuters dispatched
- [ ] Two model families, both read-only, both prompted to refute
- [ ] Every REFUTED finding has a fix-forward disposition (none ignored, none disarmed)
- [ ] Pins re-verified green on the landed tree
- [ ] Council artifact persisted

## Examples

**User says:** "land this prune, don't cut corners"
**Do:** freeze the pinned-manifest claim → dispatch Fable refuter (Agent tool,
fresh context) + codex refuter (`codex exec --sandbox read-only "...judge each
contract-test edit: honest repoint vs gate-weakening..."`) + full gate, all in
parallel → fix findings forward → land → re-sweep pins.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Refuter says CONFIRMED instantly | Prompt lacked mechanical checks | Re-dispatch with explicit per-fixture commands; "try to refute" + checklist |
| Findings contradict each other | Different scopes | Triage per finding with evidence; the diff is the arbiter |
| Panel too slow | Run was serial | Dispatch both refuters + gate concurrently; they are read-only |

## See Also

- [validate](../validate/SKILL.md) — verdict contract the panel reports in
- [codex-exec](../codex-exec/SKILL.md) — the codex refuter lane
- [codex-approval](../codex-approval/SKILL.md) — the inverse direction (Codex asks Fable)
- [red-team](../red-team/SKILL.md) — adversarial probing of docs/plans (pre-work); this skill is pre-land
- [rpi](../rpi/SKILL.md) — invokes this panel at the merge-to-main pawl **regardless of complexity** (rpi:154); complexity scales the panel's DEPTH (full council vs 2-judge minimum), never exempts the gate
- [pre-mortem](../pre-mortem/SKILL.md) — plan-time twin (move 4); this skill is the landing twin (move 6 exit)
- [post-mortem](../post-mortem/SKILL.md) — consumes the council artifact as landing evidence (move 7)
