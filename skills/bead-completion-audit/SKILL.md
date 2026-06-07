---
name: bead-completion-audit
description: |
  Audit closed beads/issues to prove they actually shipped what they claimed —
  catch false-closed work, demand acceptance evidence, and enforce the rule that
  closed means verified, not merely marked done. Produces a per-bead PASS / FAIL
  / NEEDS-EVIDENCE verdict report and reopens the false-closed ones.
  Triggers: "audit closed beads", "false-closed beads", "beads compliance audit",
  "did we actually finish", "verify completed issues", "completion discipline",
  "closed = verified", "check acceptance evidence".
skill_api_version: 1
user-invocable: false
practices:
- design-by-contract
- evidence-over-assertion
- cmm-process-maturity
hexagonal_role: supporting
consumes:
- closed-beads
produces:
- compliance-report.md
context_rel:
- kind: customer-of
  with: beads-br
- kind: supplier-to
  with: post-mortem
metadata:
  tier: judgment
  dependencies:
  - beads-br
  stability: experimental
context:
  window: fork
output_contract: skills/bead-completion-audit/scripts/validate.sh
---

# bead-completion-audit — closed must mean verified

Audit a set of *closed* beads and decide, for each one, whether it actually
delivered what its spec/acceptance criteria promised. The output is a verdict
report and a list of beads to reopen because they were closed without proof.

## ⚠️ Critical Constraints

- **A bead is only PASS when the claim is backed by an external proof surface.**
  **Why:** "I closed it" is self-report; self-report is the exact failure this
  skill exists to catch. A diff, a passing test, a built artifact, or a live
  command output is proof — a comment saying "done" is not.
  - WRONG: bead status is `closed` and has a "completed" comment → mark PASS.
  - CORRECT: open the bead's acceptance criteria, then locate the commit / test
    / artifact that satisfies each one before voting PASS.

- **Never silently mutate beads while auditing.** **Why:** the audit must be
  reproducible and the operator owns the reopen decision. Read first; reopen
  only after the report is written and only the FAIL set.
  - WRONG: `br reopen $id` mid-scan as soon as one looks suspect.
  - CORRECT: collect verdicts → write report → reopen the FAIL list with a
    reason that cites the missing evidence.

- **Absence of acceptance criteria is itself a finding (NEEDS-EVIDENCE), not a
  pass.** **Why:** a bead with no defined "done" can never be verified, so it
  cannot legitimately be closed. Flag it; do not wave it through.

## Why This Exists

Issue trackers measure *activity*, not *outcomes*. Under swarm or solo pressure,
beads get closed to clear the board — the status flips to `closed` long before
the work is provably shipped, or the "fix" addressed a symptom and not the
spec. Left unchecked, the tracker becomes a fiction: green burndown, broken
product. This skill restores the contract — **closed = verified** — by forcing
each closed bead to defend itself against external evidence before it counts.

## Quick Start

```bash
# From the repo whose .beads/ you are auditing:
br list --status closed --json > /tmp/closed-beads.json   # the audit scope

# Verify the harness + scope, then run the structured audit (see Workflow):
bash scripts/validate.sh                                  # self-check this skill
```

Scope the audit deliberately: a date window, a label, an epic, or a single
swarm's output. Auditing every closed bead in a hot repo at once produces noise,
not signal.

## Workflow

### Phase 1 — Define scope and load the closed set
1. Pick the audit boundary (label, epic, assignee, or "closed since <date>").
2. `br list --status closed --json` (add `-l <label>` / `--assignee <who>` to
   narrow). This is the **only** set you audit — open/in-progress beads are out
   of scope.
3. Record the count. A report that audited 4 of 40 closed beads is not an audit.

> **Checkpoint:** you have an explicit, bounded list of bead IDs before reading
> any one of them.

### Phase 2 — Per-bead verification
For each closed bead ID:
1. `br show <id> --format json` — read the description **and** its acceptance
   criteria / definition of done. Extract each criterion as a checkable line.
2. `br comments <id> list` and `br audit log <id>` — read the closing rationale
   and any attribution. Treat these as *claims to verify*, never as proof.
3. For each acceptance criterion, locate the **proof surface**:
   - code change → the commit/PR that touches the named files (`git log`)
   - behavior → a passing test or a reproduced command output
   - artifact → the built/published file actually exists
   - doc/spec → the section actually present in the named file
4. Assign a verdict:
   - **PASS** — every criterion has a located proof surface.
   - **FAIL** — a criterion's proof is missing, or the change contradicts it
     (false-closed).
   - **NEEDS-EVIDENCE** — no acceptance criteria existed, or proof is plausible
     but unlocated; the bead cannot be verified as written.

> **Checkpoint:** every bead in scope has exactly one verdict and a one-line
> justification citing (or naming the absence of) its proof surface.

### Phase 3 — Report and enforce
1. Write `compliance-report.md` (see Output Specification).
2. Reopen the FAIL set, citing the missing evidence:
   ```bash
   br reopen <id> --reason "compliance audit: <criterion> unverified — no <proof>"
   ```
3. For NEEDS-EVIDENCE, add a comment requesting the proof rather than reopening,
   unless the operator directs otherwise:
   ```bash
   br comments <id> add "compliance audit: closed without acceptance criteria — define done + attach proof"
   ```
4. Hand the report to `post-mortem` for the systemic root cause if the FAIL rate
   is high.

## Output Specification

Write `compliance-report.md` at the repo root (or a path the operator names):

```markdown
# Bead Completion Audit — <scope> — <date>
Scope: <label/epic/window>   Audited: <N> closed beads

| Bead | Verdict | Criterion checked | Proof surface (or gap) |
|------|---------|-------------------|------------------------|
| ab-12 | PASS | tests pass for X | commit deadbe0, `go test ./x` green |
| ab-13 | FAIL | endpoint returns 200 | no handler in router.go; reopened |
| ab-14 | NEEDS-EVIDENCE | (none defined) | closed with no acceptance criteria |

## Summary
PASS: n   FAIL: n (reopened)   NEEDS-EVIDENCE: n
False-closed rate: n%
## Reopened
- ab-13 — <reason>
## Systemic note (if FAIL rate high)
<one line → route to post-mortem>
```

## Quality Rubric

- Every closed bead in the declared scope appears in the table exactly once with
  a verdict — no bead silently dropped.
- Every PASS names a concrete, locatable proof surface (commit hash, test
  command, file:section) — never "looks done".
- Every FAIL was reopened with a reason that names the missing proof, and the
  Reopened section matches the FAIL rows.

## Examples

- **False-closed catch:** `ab-13` claimed a new endpoint; `br show` lists
  acceptance "GET /health → 200"; `git log` shows no router change → FAIL →
  `br reopen ab-13 --reason "compliance audit: /health handler absent"`.
- **Legit pass:** `ab-12` acceptance "X covered by tests"; commit `deadbe0`
  adds `x_test.go`, `go test ./x` green → PASS with both cited.
- **Undefined done:** `ab-14` closed with only "done" comment, no criteria →
  NEEDS-EVIDENCE → comment requesting acceptance criteria + proof.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `br: command not found` | beads_rust not installed/on PATH | install `br`; run `br --help` to confirm |
| `br show` has no acceptance criteria | bead closed without a definition of done | verdict = NEEDS-EVIDENCE; do not infer one |
| Hundreds of closed beads | scope too wide | narrow with `-l <label>`, `--assignee`, or a date window |
| Can't find a commit for a claim | wrong repo / squashed history | check the bead's linked PR; widen `git log --all` |
| Reopen rejected by policy gate | `.beads/policy.yaml` gate | reopen is allowed; if blocked, cite the policy and escalate to operator |

## See Also

- `beads-br` — the underlying `br` issue tracker this skill audits.
- `post-mortem` — consume this report's FAIL set to find the systemic cause.
- `agentops:validate` — produce PASS/WARN/FAIL verdicts on artifacts/plans/gates.
