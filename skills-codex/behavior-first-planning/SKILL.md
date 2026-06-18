---
name: behavior-first-planning
description: Behavior-first planning — intent to Gherkin to executed-red acceptance tests to spec to an acceptance-gated bead DAG. No runnable acceptance test, no bead.
---

# behavior-first-planning (Codex)

Codex-native entry point for the `behavior-first-planning` discipline.

The AgentOps source skill `../../skills/behavior-first-planning/SKILL.md` is the
source of truth for the four phases, the mechanical gate, and the closing
independent-review gate. Its own behavior is pinned by an executable spec,
`behavior-first-planning.feature` — the same Gherkin → runnable-acceptance shape
it asks every plan to produce. Read it first, then use `prompt.md` for the Codex
runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Run the four phases in order: (1) intent → frozen Gherkin behaviors (happy +
  edge + error); (2) each scenario → a runnable acceptance test, then RUN the
  suite and OBSERVE red (do not assert it); (3) derive a tight spec to make
  those tests pass; (4) an acceptance-gated bead DAG — every bead carries a real
  `scenario_ref` and an invocable `acceptance_test`, gated mechanically
  (runnable=exactly-one-test, valid ref, coverage-complete, cycle-free).
- In Phase 1, apply the standing adversarial dimension checklist to every input
  / trust boundary / mutation / failure path / external-state behavior, emitting
  the missing attack-vector scenario for each: FAIL-CLOSED on every failure path;
  NO FORGEABLE TRUST MARKER (re-derive provenance, never trust a caller-settable
  signal); NO RAW UNTRUSTED STRING past a boundary (canonicalize before display/
  argv/serialization); ENFORCE AT THE SINK not the source; NO OVERCLAIMING TEST
  (a harness-only proof is not a production proof); INPUT-CHANNEL variants (stdin
  vs argv, heredoc, symlink/case/unicode aliases, TOCTOU). Also apply any repo
  gate-findings ledger (the ratchet). The source skill is the SoT for this list.
- The rule is absolute: **no runnable acceptance test, no bead.** Prose-only or
  unrun acceptance is rejected.
- One invocation per test — never pass multiple positional test-name filters to a
  single test command (it arg-errors before any test runs); chain with `&&`.
- Closing gate: an INDEPENDENT fresh-context reviewer (the author never grades
  their own plan) scores crank-readiness and spot-runs ≥2 acceptance commands;
  write to the tracker (`br` from the main checkout, never a worktree) ONLY on a
  clearing verdict that is coverage-complete + cycle-free + drift-guard green.
- Return concrete evidence: commands run, the observed red/green counts, files
  touched, and any remaining blocker.
