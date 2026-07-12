---
name: goal-design
description: Create validated goal-design packets.
---
# $goal-design — Validated Intent Packet Authoring

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), run:

```bash
ao codex ensure-start 2>/dev/null || true
```

The CLI records startup once per thread and skips duplicates automatically.

> **Loop position:** pre-discovery adapter for move 1 of the operating loop.
> It turns a human goal into a checked `intent.md` + `driver.md` packet that
> `$discovery` and `$plan` can consume without relying on chat context.

**Execute this workflow. Do not only describe it.**

## Constraints

- Keep `intent.md` and `driver.md` as the contract of record and the dispatch prompt as a pointer only, because chat or prompt paraphrases drift from validated intent.
- Run the deterministic checker before independent validation and use `mark-validated` for status transitions, because hand-edited status or stale digests can falsely claim readiness.
- Route every plain `REFUTED` verdict directly through repeated `AUTO-REDO`; do not consult a helper or raise an andon unless the circuit breaker trips. A breaker trip enters `HOLD` and gets exactly one bounded helper consultation: `UNSTUCK` resumes the automatic loop, while `ESCALATE` reaches a human. Refusal-lane work, explicit judgment, and exhausted budgets go directly to a human because routine blockers are not andons.

## Purpose

Use `$goal-design` when the goal is important enough to leave chat but not yet
ready to become beads. The skill writes `.agents/goal-design/<slug>/intent.md`
and `driver.md`, refreshes the driver digest, runs the packet checker, and
requires an independent validation verdict before the packet drives work.

Do not use `$goals` for this. `$goals` maintains `GOALS.md` fitness specs;
`$goal-design` creates a per-objective intent packet.

## Inputs

Given `$goal-design "<goal>" [--slug <slug>]`:

| Input | Meaning |
| --- | --- |
| goal | Human objective to shape as BDD |
| `--slug` | Optional packet slug; default is kebab-case from goal |
| `--scenario-id` | Optional first scenario id; default `S1` |
| `--bounded-context` | Optional BC tag; default `bc-loop` until evidence says otherwise |

## Workflow

1. **Shape WHAT before HOW.** Write the objective, why, bounded context,
   non-goals, rollback/containment, stale assumptions, and at least one
   Given/When/Then scenario.
2. **Create the packet with the helper.** Prefer the digest-safe tool:

   ```bash
   scripts/goal-design-packet.py new <slug> \
     --objective "<goal>" \
     --scenario-name "<observable behavior>" \
     --first-failing-proof "<test or command>" \
     --write-scope "<path or glob>"
   ```

3. **Edit deliberately.** If you edit `intent.md`, refresh `driver.md` before
   checking:

   ```bash
   scripts/goal-design-packet.py refresh-digest .agents/goal-design/<slug>
   ```

4. **Run the deterministic checker.**

   ```bash
   scripts/goal-design-packet.py check .agents/goal-design/<slug>
   ```

   The checker must fail closed on stale digest, slug drift, misleading
   `intent_ref.path`, unknown scenario ids, unmapped candidate behavior, schema
   violations, and self-grading language.

5. **Get independent validation.** Invoke `$validate .agents/goal-design/<slug>`
   or an equivalent fresh-context validator. A checker-clean packet with no
   independent verdict is not ready to drive work. Record the verdict with the
   evidence-bound transition (never hand-edit `status:`):

   ```bash
   scripts/goal-design-packet.py mark-validated .agents/goal-design/<slug> \
     --verdict "PASS (<validator>, <date>)"
   ```

   It refuses FAIL/empty verdicts, flips both statuses, stamps the driver's
   `Last validation verdict` line, refreshes the digest, and re-runs the checker
   in one move.

6. **Hand off.** After validation, pass the packet path to `$discovery` or
   `$plan`. Preserve scenario ids and names exactly; do not paraphrase `S1`,
   `S2`, or candidate behavior labels away.
7. **Emit the dispatch prompt.** When the packet executes out-of-session via a
   goal API (codex goals, claude goals, an NTM pane, `bushido spawn`),
   emit the small copyable prompt that points the worker at the packet:

   ```bash
   scripts/goal-design-packet.py prompt .agents/goal-design/<slug>
   ```

   Paste the output verbatim into the goal command. It fails closed on a
   draft packet (validate first; `--allow-draft` for a preview) and on prompts
   over 4000 characters (the codex goal-API limit). The packet files remain
   the contract of record; the prompt only aims the worker at them.

## Andon router (author it into driver.md)

A long autonomous run is only safe if the goal carries its own escalation
policy — a per-goal **class → tier** router, never a flat "escalate to me"
(doctrine: `docs/architecture/the-flywheel.md`, the three-tier andon;
contract: `docs/contracts/pawls.md` §Escalation). The
tool's `new` command scaffolds the canonical router table into the driver
body — replace its TODO row with the goal-specific one-way-door rows before
dispatch; do not leave the placeholder. When
authoring `driver.md`, write the router as a table in the driver **body** and
mirror its escalation semantics in the schema-validated `route_back_rules`
frontmatter (auto → `validation_fails`, helper →
`promotion_contradicts_intent` + the helper-pass clause in
`validation_fails`, human → a breaker-trip clause in those rules):

| One-way-door class | Tier | Machinery (reuse, never rebuild) |
| --- | --- | --- |
| Gate / validation failure | **auto** | AUTO-REDO + `ao gate check --fast --scope head` |
| Architecture fork / plan-shape one-way door | **helper** | `$council` + `ao plan-pawl decide` (PASS/REDO/BLOCKED) + `$converge` |
| Circuit-breaker trip: N failed validation rounds, oscillation, scope-creep flag | **helper** | HOLD, then exactly one bounded helper pass — a fresh context or cross-family model (`codex exec`, `$council`) gets the blocker, the evidence, and what was tried; returns UNSTUCK (a concrete next action) or ESCALATE |
| Money / legal / irreversible-external (the refusal lane), explicit judgment flag, exhausted time/cost budget | **human** | ESCALATE / HOLD — hand back to the operator; the helper is skipped |

The validation/blocker escalation state machine is exact; copy these rows into
the goal-specific router without merging transitions:

| From | To | Required action |
| --- | --- | --- |
| `REFUTED` | `AUTO-REDO` | Repair from the evidence and rerun validation; do not consult a helper or human. |
| `AUTO-REDO` | `REFUTED` | Repeat the automatic repair loop while the circuit breaker remains closed. |
| `CIRCUIT-BREAKER-TRIP` | `HOLD` | Freeze mutation and preserve the blocker evidence. |
| `HOLD` | `HELPER` | Run exactly one bounded helper consultation for this blocker class. |
| `HELPER-UNSTUCK` | `AUTO-REDO` | Apply the concrete next action and resume the automatic repair loop. |
| `HELPER-ESCALATE` | `HUMAN` | Hand back the preserved evidence and helper verdict. |
| `REFUSAL-LANE / EXPLICIT-JUDGMENT / BUDGET-EXHAUSTED` | `HUMAN` | Skip the helper and ask the operator. |

A plain `REFUTED` verdict never enters `HOLD` and never invokes a helper. Never
run a second helper consultation for the same blocker class. The helper is an
advisor, never a second driver: it reasons about the blocker and returns a
recommendation; it does not take over the work or own the loop.

**Schema limit (do not hack it):** the driver v1 schema is
`additionalProperties: false` with no dedicated andon field, so the class →
tier table lives in the driver body while `route_back_rules` carries the
machine-checkable semantics. A driver v2 `andon_router` field is a candidate
follow-up — do not modify the landed schemas, templates, or checker to add it.

## Output Specification

- **Path:** write the packet to the artifact directory `.agents/goal-design/<slug>/` and emit the optional dispatch prompt to stdout.
- **Filename:** every packet contains exactly `intent.md` and `driver.md`; validation evidence keeps the path or summary returned by the independent validator.
- **Format:** both files are Markdown with the schema-governed frontmatter produced by `goal-design-packet.py`; the dispatch prompt is plain text under 4000 characters.
- **Validation command:** run `bats tests/scripts/{goal-design-packet,check-goal-design-packet}.bats`, then `scripts/check-goal-design-packet.sh .agents/goal-design/<slug>` and the independent validator.
- **Downstream handoff:** pass the validated packet path and preserved scenario ids to `$discovery` or `$plan`, or paste the helper-emitted prompt verbatim into the selected goal API.

## Quality Checklist

- The driver digest matches current intent and both packet identities agree with the directory slug.
- Every candidate behavior maps to a stable scenario id, a failing proof, and an explicit write scope.
- The TODO router row is replaced, the exact breaker/HOLD/helper transitions are present, and an independent PASS/WARN verdict names the next action.

## Done

`$goal-design` is done only when:

1. The packet contains both required files.
2. `scripts/check-goal-design-packet.sh .agents/goal-design/<slug>` exits 0.
3. The independent validator returns `PASS` or `WARN` with no blocker.
4. The next action is explicit: `$discovery .agents/goal-design/<slug>`,
   `$plan .agents/goal-design/<slug>`, or an emitted dispatch prompt handed to
   a goal API (codex/claude goals).

## Scenarios

```gherkin
Feature: Goal-design packets carry validated intent into the loop
  Scenario: Create a checked packet before discovery
    Given a human objective that should not stay only in chat
    When $goal-design writes intent.md and driver.md
    Then scripts/check-goal-design-packet.sh passes
    And the packet names an independent validation verdict before $discovery or $plan consumes it

  Scenario: Reject stale or inconsistent packet identity
    Given driver.md points at stale, mismatched, or unknown intent content
    When the packet checker runs
    Then it exits non-zero before planning or implementation starts

  Scenario: Emit a dispatch prompt for goal APIs
    Given a validated goal-design packet
    When goal-design-packet.py prompt runs against it
    Then a copyable prompt under 4000 characters points the worker at intent.md and driver.md
    And a draft packet is refused unless --allow-draft is passed
```

## Non-Goals

- Do not add or edit `GOALS.md`.
- Do not create beads directly unless `$plan` is invoked.
- Do not add a dedicated Goal Design CLI command until this skill proves repeated use.
- Do not track generated repo-root `.agents/goal-design` packets unless the
  write-surface contract changes.

## Validation

```bash
bats tests/scripts/goal-design-packet.bats
bats tests/scripts/check-goal-design-packet.bats
scripts/check-goal-design-packet.sh .agents/goal-design/<slug>
```
