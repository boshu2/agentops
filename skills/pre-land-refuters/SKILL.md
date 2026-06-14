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
   head_sha="$(gh pr view <pr> --json headRefOid -q .headRefOid)"
   scripts/pawl-verdict.sh write <bead> <pr> \
     --disposition CONFIRMED \
     --head "$head_sha" \
     --refuter claude:CONFIRMED:.agents/council/$(date +%F)-pre-land-<slug>-claude.md \
     --refuter codex:CONFIRMED:.agents/council/$(date +%F)-pre-land-<slug>-codex.md \
     --council .agents/council/$(date +%F)-pre-land-<slug>.md
   ```
   The verdict is **EVIDENCE-BOUND and COMMIT-BOUND**: `--head` pins it to the
   commit the panel actually reviewed (a new push makes it STALE and the gate
   fail-closes), and each `--refuter family:verdict:<evidence-path>` must point at
   a **real, non-empty** reviewer-run transcript (or supply `--council` as the
   shared evidence anchor). `check` refuses a verdict with no reviewer evidence —
   a self-asserted stamp is not a review. (disposition `REFUTED` on any refuted
   refuter — the loop **auto-redoes** on REFUTED, no human; `ESCALATE`/`HOLD` only
   when a circuit breaker trips — those make the merge path HOLD, exit 5.)
   `scripts/reconcile-pr.sh`
   reads this with `scripts/pawl-verdict.sh check <bead> <pr>` and **refuses to merge
   without a CONFIRMED, cross-family, this-bead+PR verdict** — green CI alone never
   authorizes the door. Then land (commit → merge upstream if it moved → gate →
   push), re-run the pinned sweep on the landed tree, and write the free-form
   narrative in `.agents/council/YYYY-MM-DD-pre-land-<slug>.md`
   (the human-readable companion to the checkable verdict).

## Escalation — the circuit-breaker model (auto-redo by default)

The panel runs **autonomously: model reviews model.** The human is NOT a checkpoint at the
pawl by default — they are the exception a *circuit breaker* trips into. See
[docs/contracts/pawls.md](../../docs/contracts/pawls.md) "Escalation — the circuit-breaker model".

- **Both refuters CONFIRMED (+ green gate)** → land. No human.
- **Any REFUTED → AUTO-REJECT → AUTO-REDO (the default, no human).** A REFUTED verdict means
  the gate *rejected*; the orchestrator **automatically** fixes the findings forward and
  **re-dispatches the panel**. The loop redoes on REFUTED on its own — continuous
  self-correction, no human in the loop. A plain REFUTED is never an escalation.
- **ESCALATE to a human — ONLY when a tunable CIRCUIT BREAKER trips.** The breakers are
  **plural and operator-tunable**, and are the **same governor the autonomous loop already
  runs** (the evolve circuit breakers: time-based "no productive work" + oscillation
  quarantine, [`scripts/evolve/halt-check.sh`](../../scripts/evolve/halt-check.sh)):
  **max-attempts** (N re-gate cycles still REFUTED, default 3, tunable) · **time budget**
  (wall-clock with no forward progress) · **cost / quota budget** · **oscillation /
  no-forward-progress** (the same failure repeating; covers reviewer deadlock) · an
  **explicit judgment flag** a reviewer raises (value / irreversibility). This is the
  **andon** ("Hey! Listen!") — rare, earned, never the default.

**REFUTED → auto-redo (loop). Breaker-trip → HOLD/escalate.** Set the verdict disposition
accordingly: a plain REFUTED carries `REFUTED` (the loop re-works); flip it to `ESCALATE` /
`HOLD` **only when a breaker trips**, and **do not land** — a breaker-tripped pawl is never
auto-merged. The enforcing merge path (`scripts/reconcile-pr.sh` → `scripts/pawl-verdict.sh
check`) exits **5 (HOLD: no merge, no close)** on any disposition that is not `CONFIRMED`
(so a bare REFUTED also correctly refuses the merge while the loop redoes). Only
both-refuters-`CONFIRMED`, ≥2 distinct **canonical** families, real non-empty reviewer
evidence, and `head_sha` == the PR's current head, tied to this bead+PR, opens the door
(fail-closed by construction).

Even fully unattended, the gate fires at every pawl and auto-redoes on REFUTED. Human
escalation is the exception a circuit breaker trips into, not the gate.

> **Scope note.** This verdict is an **evidence-bound, commit-bound cross-family verdict that
> requires real reviewer runs** — it defends against a *sloppy agent self-stamping CONFIRMED*,
> NOT a hostile forger. No signatures / peercred / OS writer-separation; cryptographic
> un-forgeability is **intentionally out of scope** (single-operator trusted loop — the cut
> cathedral).

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
