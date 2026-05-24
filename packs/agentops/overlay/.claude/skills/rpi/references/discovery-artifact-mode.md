# Discovery Artifact Mode (`--discovery-artifact=<path>`)

**Purpose:** Skip Phase 1 (`/discovery`) when discovery has already been done — for example, by an upstream `/council --evidence --commit-ready` validation, a hand-written plan, or a mid-session pivot where the agent has already produced a sound execution packet.

## The problem it solves

The default `/rpi` flow runs `/discovery` as Phase 1: brainstorm → design → search → research → plan → pre-mortem. This is expensive (several skill invocations, potentially spawning council judges) and redundant when the caller already has a validated scope document. Without this flag, the only way to skip Phase 1 was `--from=implementation`, which requires an existing epic ID in the bead tracker. That left a gap: **how do you resume RPI mid-session with a council-validated plan but no bead?**

## When to use

Use `--discovery-artifact=<path>` when **any** of the following is true:

1. An upstream `/council --evidence` run just produced a consolidated report with the scope, risks, file manifest, and test matrix you need for implementation.
2. A prior session wrote a plan to `docs/council-log/` or `.agents/plans/` that is still valid and you want to act on it now.
3. You are mid-session, discovered scope via ad-hoc research, and want to formalize the handoff into an execution packet without re-running discovery's gates.
4. The scope is small enough that a full `/discovery` invocation would cost more than the implementation itself.

**Do NOT use** for fresh, un-validated goals. Discovery exists to catch premature implementation; skipping it on a goal that hasn't been vetted recreates the very failure mode `/rpi` is designed to prevent.

## What the artifact must contain

The `/rpi` orchestrator reads the artifact and extracts the following fields into `.agents/rpi/execution-packet.json`:

| Field | Required? | Source in typical artifact |
|---|---|---|
| `goal` | yes | Heading or first paragraph |
| `scope.in_scope` | yes | A "scope" / "what ships" / "file manifest" section |
| `scope.out_of_scope` | recommended | An "out of scope" or "not shipping" section |
| `scope.loc_estimate` | optional | LOC estimate from the plan |
| `abort_gates` | recommended | Any "abort if X" / "stop and escalate if Y" conditions |
| `tdd_matrix` | recommended | A list of test assertions (especially for `--evidence` artifacts) |
| `risks` | optional | A "risks" / "unknowns" section |

If the artifact is a markdown file without explicit sections, `/rpi` falls back to treating the entire document as the `goal` and running with empty scope — this degrades gracefully but is less safe. Prefer structured artifacts.

**The canonical format is the `/council --evidence --commit-ready` output.** Those artifacts have all the required fields by construction.

## How it works

```bash
/rpi --auto --from=implementation --discovery-artifact=docs/council-log/2026-04-11-validate-m8-assumption-validation.md
```

Step by step:

1. **Validate artifact path.** If the file doesn't exist or isn't readable, emit `<promise>BLOCKED</promise>` with reason `discovery-artifact not found`. Do not proceed.
2. **Parse the artifact.** Extract goal, scope, abort gates, tdd matrix, risks. For council reports, look for the consolidation section and the findings with test assertions.
3. **Write execution packet.** Produce `.agents/rpi/execution-packet.json` with the extracted fields, the artifact path under `discovery_artifacts`, and `phase: "implementation"`. This is the same shape a normal `/discovery` run produces.
4. **Log Phase 1 completion.** `PHASE 1 COMPLETE ✓ (discovery) — artifact: <path>`
5. **Proceed to Phase 2 (crank).** Pass the execution packet to `/crank` exactly as if Phase 1 had run normally.
6. **Phase 3 (validation) is unchanged** — it still runs vibe + post-mortem + retro + forge.

## Gate behavior

The pre-mortem gate that normally runs at the end of `/discovery` is **assumed to have been passed** by whatever upstream process produced the artifact. This is the core trust trade-off: you are skipping the gate in exchange for the work done upstream. To preserve safety:

- The orchestrator **requires** the artifact to contain at least one of: an explicit verdict (e.g., "PASS with HIGH confidence"), abort gates, or a test matrix.
- If the artifact is empty or lacks any of the above, `/rpi` downgrades to treating it as an informational hint and runs `/discovery` anyway. Do NOT silently skip gates when the artifact is too thin to validate.

## Example: continuation from in-session council

The 2026-04-11 na-h61 M8 session used this pattern:

1. Session started with `/council --tdd` validating the scope of a deferred bead (the bead description had `collectLearnings` listed against a nonexistent file).
2. Council returned a consolidated report with: corrected scope (~300 LOC not 395), file touch inventory, test matrix (T1-T6), and an abort gate (">5 broken existing tests").
3. Report was written to `.agents/council/2026-04-11-m8-assumption-validation-consolidated.md`.
4. User invoked `/rpi --auto`. The orchestrator recognized the artifact as the discovery output, declared Phase 1 complete, and proceeded directly to crank.
5. Phase 2 implementation landed with 1 commit, under the abort gate.
6. Phase 3 validation passed inline.

With this flag, the pattern becomes explicit instead of improvised:

```bash
/rpi --auto --from=implementation \
  --discovery-artifact=.agents/council/2026-04-11-m8-assumption-validation-consolidated.md
```

## See also

- `skills/council/SKILL.md` + `skills/council/references/evidence-mode.md` — the upstream source of high-quality discovery artifacts
- `skills/discovery/SKILL.md` — the Phase 1 orchestrator that this flag bypasses
- `docs/council-log/README.md` — where committable council artifacts live
