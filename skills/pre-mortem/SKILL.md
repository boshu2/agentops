---
name: pre-mortem
description: 'Stress-test plans before work. Use when: a plan is drafted but not yet executed and you want to surface failure modes, risks, and what would prove it wrong before committing.'
practices:
- adr
- mythical-man-month
- design-by-contract
hexagonal_role: domain
consumes:
- standards
produces:
- result.json
- verdict.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
metadata:
  tier: judgment
  dependencies:
  - council
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
output_contract: skills/council/schemas/verdict.json
---
# Pre-Mortem Skill

> **Purpose:** Is this plan/spec good enough to implement?

> **Mandatory for 3+ issue epics.** Pre-mortem is enforced by hook when `/crank` is invoked on epics with 3+ child issues. 6/6 consecutive positive ROI. Bypass: `--skip-pre-mortem` flag or `AGENTOPS_SKIP_PRE_MORTEM_GATE=1`.

## Loop position

Pre-flight check between moves **3 (slice plan)** and **4 (TDD per slice)** of the [operating loop](../../docs/architecture/operating-loop.md). Consumes the [slice validation plan](../../docs/templates/slice-validation.md); produces a PASS/WARN/FAIL verdict on the plan AND on the wave-validity rows (distinct write scopes, no shared migration/contract/CLI surface, owner per slice, discard path per slice). A wave can only be claimed parallel if pre-mortem confirms every conflict-free row. FAIL on wave-validity → run slices sequential or send the plan back to `/plan` for re-slicing.

Run `/council validate` on a plan or spec to get multi-model judgment before committing to implementation.

---

## Quick Start

```bash
/pre-mortem                                         # validates most recent plan (inline, no spawning)
/pre-mortem path/to/PLAN.md                         # validates specific plan (inline)
/pre-mortem --deep path/to/SPEC.md                  # 4 judges (thorough review, spawns agents)
/pre-mortem --mixed path/to/PLAN.md                 # cross-vendor (Claude + Codex)
/pre-mortem --preset=architecture path/to/PLAN.md   # architecture-focused review
/pre-mortem --explorers=3 path/to/SPEC.md           # deep investigation of plan
/pre-mortem --debate path/to/PLAN.md                # two-round adversarial review
```

---

## Execution Steps

### Step 0: Bead-Input Pre-Flight (Mandatory)

When the input to `/pre-mortem` is a bead ID (matches pattern `[a-z]{2,6}-[0-9a-z.]+`) AND complexity is "full" OR the bead is older than 7 days OR the bead description was filed by a prior session, automatically run `ao beads verify <bead-id>` as the very first action. If verify reports any STALE citations, present them to the user and ask for scope re-validation before proceeding. This implements the shared stale-scope validation rule.

### Step 1: Find the Plan/Spec

**If path provided:** Use it directly.

**If no path:** Find most recent plan:
```bash
ls -lt .agents/plans/ 2>/dev/null | head -3
ls -lt .agents/specs/ 2>/dev/null | head -3
```

Use the most recent file. If nothing found, ask user.

### Step 1.4: Retrieve Prior Learnings & Compiled Prevention (Mandatory)

Run `ao lookup` for the plan's domain, then load compiled checks from `.agents/pre-mortem-checks/*.md` (fall back to `.agents/findings/registry.jsonl`). Include matched entries in the council packet as `known_risks` and record `ao metrics cite` influence. Full contract (fail-open rules, section-evidence handling, ranking heuristics, citation lifecycle) in [references/compiled-prevention.md](references/compiled-prevention.md). This file also contains Step 1a (flywheel search, skipped under `--quick`) and Step 1b (PRODUCT.md auto-include).

Fail-open reader behavior is mandatory: missing or empty compiled prevention inputs skip silently; malformed line -> warn and ignore that line; unreadable file -> warn once and continue without findings.

### Step 1a: Flywheel Search (Skip if --quick)

Run the flywheel search from [references/compiled-prevention.md](references/compiled-prevention.md) unless `--quick` is active.

### Step 1b: PRODUCT.md Context (Skip if --quick)

When `PRODUCT.md` exists and full council mode is active, add one product judge: 3 judges total (2 plan-review + 1 product).

### Step 1.5: Fast Path (--quick mode)

**By default, pre-mortem runs inline (`--quick`)** — single-agent structured review, no spawning. This catches real implementation issues at ~10% of full council cost (proven in ag-nsx: 3 actionable bugs found inline that would have caused runtime failures).

In `--quick` mode, skip Steps 1a and 1b as standalone pre-processing phases. If `PRODUCT.md` exists, Step 1b's product context is still loaded inline during the quick review. `--deep`, `--mixed`, `--debate`, and `--explorers` add the dedicated product perspective and wider council fan-out.

To escalate to full multi-judge council, use `--deep` (4 judges) or `--mixed` (cross-vendor).

### Step 1.5.1: Reversibility self-check — size the gate to the stakes (Mandatory)

Before selecting gate depth, **state the plan's blast radius and reversibility in one sentence.** If the plan is
reversible (content recoverable, deletion non-destructive, no shared schema/CLI/contract/migration surface), **say so and
default to the lightest gate** — inline `--quick` plus a single blind sub-agent for the no-self-grading floor (Step 2.9).
Escalate to `--deep` / `--mixed` / full council **only on a named irreversible surface** — a one-way door per the
[blast-radius rule](../../docs/contracts/pawls.md#the-blast-radius-rule-the-list-is-examples-not-the-boundary)
(schema migration, public API, architecture fork, security posture, deletion, mutate-shared-trunk). This is the de-escalation
dual of Step 2.10's escalation rule: 2.10 says *add* rigor for one-way doors; this says *notice and drop* rigor when the op
is reversible. Running a cross-family duel on a reversible doc/refactor is the waterfall the ratchet exists to avoid.

### Step 1.6: Scope Mode Selection

Determine review posture — EXPANSION, HOLD SCOPE, or REDUCTION — and commit `scope_mode: <expansion|hold|reduction>` in the council packet. Auto-detection rules and mode-specific judge prompts are in [references/scope-mode.md](references/scope-mode.md).

### Step 1.7: Load Council FAIL Patterns (Mandatory)

Read [references/council-fail-patterns.md](references/council-fail-patterns.md) for the top 8 council FAIL patterns to check against. These patterns are derived from 124 analyzed FAIL verdicts across 946 council sessions. They apply to both `--quick` and `--deep` modes.

### Step 2: Run Council Validation

**Default (inline, no spawning):**
```
/council --quick validate <plan-path>
```
Single-agent structured review. Catches real implementation issues at ~10% of full council cost. Sufficient for most plans (proven across 6+ epics).

Default (2 judges with plan-review perspectives) applies when you intentionally run non-quick council mode.

**With --deep (4 judges with plan-review perspectives):**
```
/council --deep --preset=plan-review validate <plan-path>
```
Spawns 4 judges:
- `missing-requirements`: What's not in the spec that should be? What questions haven't been asked?
- `feasibility`: What's technically hard or impossible here? What will take 3x longer than estimated?
- `scope`: What's unnecessary? What's missing? Where will scope creep?
- `spec-completeness`: Are boundaries defined? Do conformance checks cover all acceptance criteria? Is the plan mechanically verifiable?

Use `--deep` for high-stakes plans (migrations, security, multi-service, 7+ issues).

**With --mixed (cross-vendor):**
```
/council --mixed --preset=plan-review validate <plan-path>
```
3 Claude + 3 Codex agents for cross-vendor plan validation with plan-review perspectives.

**With explicit preset override:**
```
/pre-mortem --preset=architecture path/to/PLAN.md
```
Explicit `--preset` overrides the automatic plan-review preset. Uses architecture-focused personas instead.

**With explorers:**
```
/council --deep --preset=plan-review --explorers=3 validate <plan-path>
```
Each judge spawns 3 explorers to investigate aspects of the plan's feasibility against the codebase. Useful for complex migration or refactoring plans.

**With debate mode:**
```
/pre-mortem --debate
```
Enables adversarial two-round review for plan validation. Use for high-stakes plans where multiple valid approaches exist. See `/council` docs for full --debate details.

### Steps 2.4–2.8: Mandatory Council Checks

Five mandatory checks run during council validation — temporal interrogation, error-&-rescue map, council FAIL pattern check, test pyramid coverage, and input validation for enum-like fields. Each has auto-trigger conditions and judge-prompt snippets. Full step text and check tables in [references/mandatory-checks.md](references/mandatory-checks.md).

When a plan introduces a regex, grep, glob, or similar scope predicate, also apply [references/scope-predicate-positive-negative-cases.md](references/scope-predicate-positive-negative-cases.md): require positive and negative examples before approval.

**Re-baseline against what exists (mandatory when the plan proposes NEW
construction).** A plan that says "build X" / "X is missing" / "the unbuilt
arm" must prove X does not already exist — `grep`/read the codebase for the
capability, the command, the function, the table — *before* the effort estimate
is accepted. The dominant scoping failure is estimating new construction for
machinery that is already built (and only needs integration), which inflates
effort 2× and risks a duplicate/competing implementation. Judge prompt: "For
each 'build/add/missing' claim, was the absence verified by a search, or
assumed? Name the search." Treat an unverified "it's missing" as a WARN at
minimum; FAIL if the plan's effort/sequencing depends on it.

### Step 2.9: No-self-grading invariant (author ≠ validator)

The pre-mortem verdict must NOT be graded by the plan's own author. A verdict produced by the authoring context is autocorrelated — the same assumptions that shaped the plan pass it. This is the no-self-grading invariant (`ag-lmdx.4`): the independent-trust-domain check on the plan-acceptance verdict.

**Rule:** the judge context MUST be distinct from the author context. Validation MAY run inside the authoring session, but the judge MUST be a **blind sub-agent** — a fresh, context-isolated agent acting as if it has no authoring context. Record `judge_id` (the isolated sub-agent context) distinct from `author_id` (the planning context). The `--deep`/`--mixed` council judges satisfy this when they are context-isolated sub-agents; an inline self-review by the planning agent does NOT.

**Refuse** to emit a PASS verdict when the judge context equals the author context (`judge_id == author_id`) — re-run the verdict through a blind sub-agent judge instead.

**Escape:** `--allow-self` (default OFF) waives the invariant for the inline fallback only (e.g. no sub-agent runtime available). Using it stamps the verdict as self-graded; downstream `ao turn verify` reports it as waived, not independently validated.

**Enforcement:** `ao turn verify <bead>` evaluates the `author_neq_validator` predicate from the turn-input file's `author_id`/`judge_id` and fails the Evidenced-Turn DoD on a self-graded verdict unless `--allow-self` is passed.

**Cross-family requirement for one-way-door plans:** when the plan is a strategy, experiment, or one-way-door decision, the judge MUST be from a **different model family** than the author (e.g. author=Claude → judge=Codex, or vice versa). Same-family judges share training-data-correlated blind spots — the dominant failure mode for high-stakes plan review. Use `--mixed` or `codex exec` to satisfy this. Record `judge_family` in the verdict alongside `judge_id`.

### Step 2.10: Pre-Registered Decision Rule (strategy / experiment / one-way-door plans)

When the plan under review is a **strategy**, **experiment-driven** plan, or a **one-way-door** decision (irreversible: schema migration, public API change, architecture fork, security posture change, data deletion), the pre-mortem MUST require and record a **pre-registered decision rule** — defined BEFORE the council judges deliberate.

A pre-registered decision rule answers three questions:

1. **What result changes the decision?** Name the specific finding, metric, or evidence that would cause the plan to be rejected or materially altered. (Not "if judges say FAIL" — that's tautological.)
2. **What threshold or CI gate kills the claim?** Name a concrete, mechanically verifiable condition: a test that must pass, a metric that must stay within bounds, a property that must hold. If no such gate exists, the plan is unfalsifiable — FAIL.
3. **What negative result redirects?** Name what happens on a real negative: pivot to alternative X, defer to next cycle, escalate to human. "Try harder" is not a redirect.

Record the decision rule in the council packet frontmatter as `decision_rule:` before judges deliberate. Judges evaluate the plan AGAINST the decision rule — not just "is this plan good" but "does this plan survive its own kill conditions."

**Why:** without a pre-registered decision rule, pre-mortem degenerates into "does this plan seem reasonable" — a question the author already answered yes to. The decision rule makes the pre-mortem falsifiable. Surfaced by a cross-family (Codex) pre-mortem that found real problems an inline review missed because the inline review had no kill conditions to test against.

### Step 2.11: Plan-Pawl Duel Checklist (the cross-family invariant)

Steps 2.9 + 2.10 are **one invariant** — *a plan's acceptance verdict must come from an independent, cross-family adversary, against pre-registered kill conditions.* That invariant has **two delivery forms of the same thing**, not two different gates:

- **`/pre-mortem --mixed`** (this skill) — a cross-vendor council judges the plan artifact.
- **The discovery plan-pawl duel** ([`discovery`](../discovery/SKILL.md) STEP 3.5 → `ao plan-pawl decide`, the [`plan-pawl` row](../../docs/contracts/pawls.md)) — two distinct-family judge panes duel over the `SynthesisPacket`; that duel verdict **IS** the pre-mortem verdict for fanout-class discovery (do not run a second council).

For fanout class the duel **satisfies no-self-grading by construction**: the two judges are fresh, context-isolated, distinct-family panes, so `author_id ≠ judge_id` and `judge_family ≠ author_family` hold automatically. Before accepting ANY plan acceptance verdict (either form), check:

- [ ] **Independent judge** — `judge_id` ≠ `author_id` (no inline self-review; `--allow-self` waives only for the no-subagent fallback, and stamps the verdict self-graded).
- [ ] **Cross-family for one-way doors** — strategy / experiment / irreversible plan ⇒ `judge_family` ≠ `author_family` (≥2 distinct roster families; the duel's quorum floor).
- [ ] **Pre-registered decision rule** — `decision_rule:` recorded BEFORE deliberation; judges evaluate the plan against its own kill conditions.
- [ ] **Not a behavior substitute** — the plan-pawl gates plan SHAPE; it never replaces the acceptance-test layer (2026-06-12 auth-bypass learning).

### Step 3: Interpret Council Verdict

| Council Verdict | Pre-Mortem Result | Action |
|-----------------|-------------------|--------|
| PASS | Ready to implement | Proceed |
| WARN | Review concerns | Address warnings or accept risk |
| FAIL | Not ready | Fix issues before implementing |

### Step 4: Write Pre-Mortem Output

Write to `.agents/council/YYYY-MM-DD-pre-mortem-<topic>.md` using the full template (frontmatter, verdict table, pseudocode-fix format, decision gate) in [references/write-pre-mortem-output.md](references/write-pre-mortem-output.md). That reference also contains Step 4.5 (persist reusable findings to `.agents/findings/registry.jsonl`) and Step 4.6 (copy pseudocode fixes verbatim into plan issues so workers do not reimplement them from scratch).

When Step 4.5 writes reusable findings, include `dedup_key` and refresh compiled findings with `finding-compiler.sh` when that hook exists.

The generated report must preserve this exact heading because downstream validators and ledger readers extract verdicts with a regex anchored to it:

## Council Verdict: PASS / WARN / FAIL

### Step 5: Record Ratchet Progress

```bash
ao ratchet record pre-mortem 2>/dev/null || true
```

### Step 6: Report to User

Tell the user:
1. Council verdict (PASS/WARN/FAIL)
2. Key concerns (if any)
3. Recommendation
4. Location of pre-mortem report

---

## Integration with Workflow

```
/plan epic-123
    │
    ▼
/pre-mortem                    ← You are here
    │
    ├── PASS → /implement
    ├── WARN → Review, then /implement or fix
    └── FAIL → Fix plan, re-run /pre-mortem
```

---

## Examples & Troubleshooting

See [references/examples.md](references/examples.md) for worked examples (default inline, `--mixed` cross-vendor, auto-find recent, `--deep` high-stakes) and the troubleshooting table (timeouts, FAIL on valid plans, missing product perspectives, gate-blocking, spec-completeness warnings, mandatory-for-epics enforcement).

## Troubleshooting

See [references/examples.md](references/examples.md) for the troubleshooting table.

---

## See Also

- `skills/council/SKILL.md` — Multi-model validation council
- [`pre-land-refuters`](../pre-land-refuters/SKILL.md) — same adversarial stance aimed at the finished diff: this skill attacks the plan pre-work; that one attacks the completion claim pre-push
- `skills/plan/SKILL.md` — Create implementation plans
- `skills/validate/SKILL.md` — Validate code after implementation

## Reference Documents

- [references/pre-mortem.feature](references/pre-mortem.feature) — Executable spec: plan PASS/WARN/FAIL verdict before work, wave-validity gates parallelism, --quick inline default (soc-qk4b)

- [references/compiled-prevention.md](references/compiled-prevention.md)
- [references/scope-mode.md](references/scope-mode.md)
- [references/mandatory-checks.md](references/mandatory-checks.md)
- [references/scope-predicate-positive-negative-cases.md](references/scope-predicate-positive-negative-cases.md)
- [references/write-pre-mortem-output.md](references/write-pre-mortem-output.md)
- [references/examples.md](references/examples.md)
- [references/council-fail-patterns.md](references/council-fail-patterns.md)
- [references/enhancement-patterns.md](references/enhancement-patterns.md)
- [references/error-rescue-map-template.md](references/error-rescue-map-template.md)
- [references/failure-taxonomy.md](references/failure-taxonomy.md)
- [references/simulation-prompts.md](references/simulation-prompts.md)
- [references/prediction-tracking.md](references/prediction-tracking.md)
- [references/spec-verification-checklist.md](references/spec-verification-checklist.md)
- [references/temporal-interrogation.md](references/temporal-interrogation.md)
- Shared stale-scope validation rule — re-validate inherited scope estimates against HEAD before acting on deferred beads or handoff docs.
