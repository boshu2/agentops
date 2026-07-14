# Strict Delegation Contract (shared)

> Applies to all top-level orchestrator skills: `/rpi`, `/discovery`, `/validate`.
> Strict sub-skill delegation is the **default**, not opt-in.

This source reference is canonical. Runtime twins must be generated from it;
receipt semantics never originate in a projection-only copy.

## The Contract

Strict delegation preserves **typed responsibility and authority**, not a quota
of model calls or Markdown files. Discovery owns intent and tranche shape;
Crank owns implementation and targeted wave evidence; Validate owns independent
semantic proof; Learn owns deterministic post-verdict bookkeeping. The
orchestrator owns ordering, retry/re-plan, and delivery handoff.

Use the runtime's native skill invocation when available. A thin phase runner or
subagent may transport a responsibility when the runtime lacks that boundary.
Only Premortem and Validate require fresh independent context; Learn must not
spawn another model merely to copy a verdict into bookkeeping.

## Phase-Isolated Transport

Strict delegation names the contract. Transport isolation names where that
contract runs.

For high-cost or independence-sensitive phases, the desired runtime shape is:

1. The visible orchestrator keeps the lifecycle objective, phase order, and
   retry policy.
2. A phase runner receives only the phase skill name, the bounded handoff
   artifact, and the minimum objective context.
3. The runner executes one declared skill contract in the cheapest context that
   preserves its authority. Validate and Premortem are fresh; deterministic
   Learn may run in the orchestrator context.
4. The orchestrator receives only artifact path, verdict, and next action.

The forbidden move is erasing a responsibility or letting the producer self-issue
the independent verdict. Sharing a context for deterministic bookkeeping is not
compression.

## Anti-Pattern: Compression

Do not erase typed responsibilities, let one role acquire another role's
authority, or skip independent semantic proof. Typical rationalizations to reject:

- *"The tests passed, so the producer can issue the semantic verdict."*
- *"Let me do discovery inline — I already know what to do."*
- *"Nested `Skill()` calls waste context; I'll spawn an `Agent()` instead."*
- *"The implementation is validated by tests passing; skipping `/validate`."*
- *"The plan looks good, skipping premortem to save time."*
- *"I'll just spawn 3 judges directly — it's what `/validate` does anyway."*
- *"Learn can reinterpret or upgrade the validator's verdict."*

The opposite failure is **delegation theater**: spawning a fresh model for
deterministic bookkeeping, rerunning exact-input checks at every boundary,
requiring four hand-written phase summaries, or forcing Validate/Learn between
unchanged low-risk waves. Those acts add cost without adding independence.

### Premortem Anti-Rationalization Clause

The following do **NOT** count as a premortem and **MUST NOT** be used to skip
the delegated `/premortem` pass:

1. **An inline risk or "honest risk" section the author wrote.** The author's
   own risk assessment is autocorrelated with the plan — the same blind spots
   that shaped the plan shape the risk section. It is not an independent check.
2. **An earlier adversarial pass on an INPUT or premise, not THIS plan.** A
   prior council/siege/refutation that challenged a *premise* (e.g. "is this
   the right goal?") does not validate the *implementation plan* derived from
   that premise. Different artifact, different failure modes.
3. **"A related council already ran."** A council on a sibling plan, a prior
   version of the plan, or a different artifact in the same epic does not
   transfer. Premortem is plan-specific.

**Premortem = INDEPENDENT (author != reviewer) + fresh-context + bound to THIS
plan version and risk class.** Reuse that verdict while acceptance, dependencies,
write scope, and risk are unchanged. An inline author section satisfies none; a
prior-premise adversarial pass is not bound to this plan.

All of these are contract violations. A live compression was observed 2026-04-19
(see [`docs/learnings/orchestrator-compression-anti-pattern.md`](../../../docs/learnings/orchestrator-compression-anti-pattern.md)).
Its lesson is separation of authority and evidence—not a permanent requirement
for duplicate artifacts or a fixed count of invocations.

## `Agent()` vs `Skill()`

These are **not interchangeable**:

| Call | When to use |
|------|-------------|
| `Skill(skill="<name>", ...)` | Invoking a declared skill contract when the runtime exposes native skill calls. |
| `Agent(subagent_type="...", ...)` | Fresh-context transport for an independent skill role, or parallel work inside a skill when explicitly admitted. |
| Phase runner | Runtime transport that executes one declared skill contract in an isolated context and returns only the bounded phase artifact. |

If a runtime lacks a native `Skill()`-fork boundary, a phase runner may use a
subagent, daemon job, or process wrapper as transport. That wrapper must be
thin: load the declared skill, execute the skill workflow, write the expected
artifact, and return a compact result. It must not perform the phase directly.

## Supported Compression Escapes

These flags scale gate depth or scope. They never waive the four typed
responsibilities or the fresh semantic-verdict boundary:

### `/rpi`
- `--quick` / `--fast-path` — narrow research and deterministic scope; one
  fresh Premortem and one fresh Validate still bind the plan and candidate.
- `--from=<phase>` — resume only when earlier canonical artifacts still match
  their exact inputs.

### `/discovery`
- `--quick` — passes a smaller claim packet to one fresh Premortem judge.
- `--skip-brainstorm` — skip STEP 1 when the goal is specific (>50 chars, no vague keywords)
- `--interactive` / `--auto` — control human-gate behavior in research and plan
- `--no-scaffold` — skip STEP 4.5 scaffold auto-invocation (canonical name; `--no-lifecycle` is a deprecated alias through v2.40.0)

### `/validate`
- `--quick` — narrow the claim set and reuse exact-input factual receipts; the
  accountable validator remains fresh and distinct from the author.
- Surface-specific exclusions reduce scope only when recorded in `not_checked`;
  they never turn missing mandatory evidence into PASS.

**If tempted to shortcut outside this list: stop and delegate.**

## Positive Pattern: What Correct Delegation Looks Like

A correct `/rpi` invocation preserves the four typed transitions while paying
proof cost once per bounded tranche:

```
Skill(skill="discovery", args="<goal> --auto")      # Phase 1
  → <promise>DONE</promise>
  → reads .agents/rpi/execution-packet.json
Skill(skill="crank", args="<packet-path> [--test-first]")   # 1-3 admitted waves
  → canonical wave evidence; targeted facts only
  → another Crank wave may run when the Premortem-bound plan is unchanged
Skill(skill="validate", args="--complexity=<level> [--strict-surfaces]")   # Phase 3
  → one fresh independent verdict on the frozen tranche
  → writes canonical result.json
Skill(skill="learn", args="<validate-verdict-path>")   # Phase 4
  → <promise>DONE</promise>
  → writes canonical learn-receipt.json without another judge
```

Exact-input deterministic receipts are reused. One consolidated repair may
receive affected-claim closure; a second distinct repair need returns REPLAN.

When phase-isolated transport is available, the transcript may show a phase
runner. The execution packet carries one ordered receipt index pointing at the
canonical artifact for each responsibility. It is not copied into every child
artifact, and legacy phase summaries are link-only projections.

## Detection for Reviewers

When auditing a session that claims to have run `/rpi`, check the transcript for:

1. Discovery evidence binds the admitted tranche plan and Premortem verdict.
2. Crank evidence covers each admitted wave with targeted deterministic facts.
3. One frozen tranche has one fresh author-not-equal-validator semantic verdict.
4. One Learn receipt binds that verdict without changing it.
5. The execution packet's ordered receipt index resolves each canonical artifact.

Missing a typed transition or independent verdict is non-compliant. Missing a
duplicate Markdown summary is not.

## Enforcement Layers (defense in depth)

1. **This contract document** — read before / during orchestrator invocation.
2. **Loud text in each orchestrator's SKILL.md** — anti-pattern section with explicit examples.
3. **Durable learning** at `docs/learnings/orchestrator-compression-anti-pattern.md` — surfaced through the orchestrator skill contracts.
4. **One receipt index** — file-backed `phase_receipts` references canonical
   artifacts without duplicating their analysis.
5. **Optional future**: runtime hook that inspects the skill invocation trace,
   cross-checks it against phase receipts, and blocks downstream work when
   phases were skipped. Not implemented; deferred to a follow-up initiative.

Contract strength alone is not enforcement. These layers preserve authority and
evidence while rejecting invocation-count and artifact-count theater.
