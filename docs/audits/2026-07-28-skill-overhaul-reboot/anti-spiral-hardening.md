# Anti-spiral hardening — placement and edits

Response to the 2026-07-28 spiral incident brief (three days of orchestrated
planning/validation over the skill overhaul: 326 temporary control artifacts,
zero implementation commits, RPI never invoked). The mandate: strengthen the
existing one-shot RPI contract; do not build another lifecycle.

## Edits landed in this repository

| Owner | Section | Rule |
|---|---|---|
| `skills/rpi/SKILL.md` | frontmatter description | Triggers now include "execute this plan" and any orchestration or worker-delegation request — RPI admission is automatic, the caller never has to name it. |
| `skills/rpi/SKILL.md` | Admission and phase lock (new) | After the caller accepts a plan (including a duel synthesis), Plan is closed for that intent; every later lane must return implementation evidence; a review comment is never authorization for another planning lane. |
| `skills/rpi/SKILL.md` | Continuation envelope | Spiral breaker: two consecutive control artifacts with no new implementation evidence terminate the run as `NOT_BUILT`; second non-PASS on one intent stops the lane. Neither dispatches a repair revision. |
| `skills/rpi/SKILL.md` | Report | Subject-first reporting: paths changed, commits, tests, acceptance remaining. A rising artifact count over an unchanged subject is a stop signal. |
| `skills/validate/SKILL.md` | Preconditions | The subject must be a nonempty implementation candidate; control artifacts are not completion subjects unless the caller explicitly requested document review. |
| `skills/validate/scripts/validate.py` | `store_verdict` | Mechanical guard: refuses a subject manifest with no entries (`ContractError`). |
| `AGENTS.md` | Core loop 1 / 4 | Phase-lock and spiral-stop pointers (authority lives in the rpi skill). |
| `AGENTS.md` | Constraint floor (new) | (a) A plan/duel/design is authoritative only if its frozen inputs include the active ADRs, blocking gates, and this contract — a synthesis missing an active constraint is invalid, not grandfathered. (b) ADR-0016 pointer: skill logic in Go, shell glue only, no new shipped skill Python, ratchet named. |
| `skills/{rpi,validate}/scripts/validate.sh` | guards | Positive greps pin every load-bearing phrase above. |
| `tests/scripts/anti-spiral-contract.bats` | replay tests | Stripping any pinned phrase makes the owning validator fail; `store-verdict` with an empty manifest exits nonzero; AGENTS.md carries the constraint floor. |

## Replay-scenario coverage (honest accounting)

| Brief scenario | Deterministic guard |
|---|---|
| 1. Unsolicited planning lane after accepted duel | Contract text + validator pin (runtime-behavioral; not CI-replayable) |
| 2. Two reviews, subject unchanged | Same — spiral breaker text + validator pin |
| 3. Validate over a report instead of a candidate | `store_verdict` empty-manifest refusal (mechanical) + precondition pin |
| 4. Duel packet omits ADR-0016 / a blocking gate | AGENTS.md constraint floor + bats presence test; duel-skill edit below |
| 5. New shipped Python under a skill execution path | `scripts/check-skill-python-ratchet.sh` (landed #995, blocking) |
| 6. Five workers produce reports, no code | Contract text (`NOT_BUILT` breaker); enforcement is the orchestrator's session contract |

Scenarios 1, 2, and 6 are decisions made by a live orchestrator, not
repository state; no repo gate can replay them. The deterministic surface is
the contract text plus validators that fail when that text is weakened, and
the two mechanical guards (empty-manifest refusal, python ratchet). Claiming
more would be the same theatre this brief exists to end.

## Proposed edits outside this repository

**`~/.claude/CLAUDE.md` (dotfiles: `dotfiles/claude/CLAUDE.md`), Execution
Rules — add one rule (proposed, Bo applies):**

> **Spiral breaker (universal).** Orchestration and delegation requests enter
> the one-shot loop: plan once, implement, fresh-validate, stop. After a plan
> is accepted, every dispatched lane returns implementation evidence (diff,
> commit, test, receipt) — never another plan, audit, or review without new
> explicit authorization. Two consecutive control artifacts with no new
> implementation evidence, or a second non-PASS on one intent, is a hard stop
> — report and return, never auto-dispatch a repair. Status reports lead with
> subject changes (paths, commits, tests); artifact count rising while the
> subject is unchanged IS the stop signal.

**`~/.claude/skills/dueling-idea-wizards/SKILL.md` (installed copy, not
repo-managed) — add to the packet-freeze step (applied directly; re-apply
after any jsm upgrade):**

> Before the duel output becomes authoritative, the frozen packet must
> include every applicable higher-precedence instruction, ADR, and blocking
> gate for the target repository. A synthesis whose packet omitted an active
> constraint is invalid and must not be executed or cited as precedent.

## Consolidations (rules this makes redundant)

- rpi's proportionality-guard paragraph already warned about multiplying
  control artifacts; the spiral breaker in the continuation envelope is now
  the enforceable form. Kept as one sentence of rationale, not duplicated.
- The 07-24 plan's per-tranche verdict ceremony is superseded by the reboot
  plan's two-round review ceiling; no separate repair-lane rules remain.

## Conflicts with current RPI semantics

- `NOT_BUILT` was defined as "no subject was built". The spiral breaker
  reuses it for "run terminated with control artifacts outnumbering
  implementation evidence" — consistent (no subject got built), now stated in
  the contract.
- rpi previously activated only on explicit triggers; automatic admission
  widens its trigger set. The one-writer/one-agent default and caller-owned
  continuation are unchanged.
