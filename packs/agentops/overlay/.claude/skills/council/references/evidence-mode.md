# Evidence Mode (`--evidence` / `--tdd`)

**Purpose:** Make council verdicts mechanically verifiable. Every finding must come with concrete test assertions that a future reader can re-run to prove the finding is still valid.

**The problem it solves:** Council verdicts without evidence are just opinions. When a judge says "this scope estimate is inflated" or "this plan has a race condition", a downstream agent or human reader has no cheap way to verify the claim. They have to re-derive the analysis from scratch. Evidence mode forces each finding to be a falsifiable claim paired with the exact check that would falsify it.

## How to invoke

```bash
/council --evidence validate "the M8 C1 Option A scope estimate"
/council --evidence --mixed validate the auth migration plan
/council --tdd validate this bead description      # --tdd is an alias for --evidence
```

Compatible with every mode (`validate`, `brainstorm`, `research`) and every backend (`--quick`, default, `--deep`, `--mixed`, `--adversarial`).

## What changes in the judge prompts

When `--evidence` is set, judge prompts get an additional mandatory directive:

> For every finding, you MUST provide at least 2 `test_assertions` — concrete, mechanical checks that would prove the finding is real. Each assertion must be:
> - **Runnable** — a shell command, grep pattern, file stat check, or `go test -run` invocation
> - **Deterministic** — same input → same output, no human judgment involved
> - **Cheap** — runs in under 1 second
>
> A finding without assertions will be rejected in consolidation and the judge's verdict will be clamped.

## Schema additions

Each finding in the output_schema gains a required `test_assertions` array:

```json
{
  "findings": [
    {
      "severity": "significant",
      "category": "architecture",
      "description": "The bead claims `cli/cmd/ao/fitness.go` exists but it does not",
      "location": "bead na-h61 description",
      "recommendation": "Close bead as not-reproducible or re-scope against the actual fitness surface",
      "why": "Citation drift — function was renamed or bead was filed against a different branch",
      "ref": "cli/cmd/ao/inject_learnings.go:50",
      "test_assertions": [
        {
          "description": "The cited file does not exist",
          "verifies_by": "stat",
          "command": "test -f cli/cmd/ao/fitness.go",
          "expected_result": "exit 1 (not found)"
        },
        {
          "description": "The cited function exists in a different file",
          "verifies_by": "grep",
          "command": "grep -rn 'func collectLearnings' cli/",
          "expected_result": "cli/cmd/ao/inject_learnings.go:50 (only match)"
        }
      ]
    }
  ],
  "schema_version": 4
}
```

The `schema_version` bumps from 3 to 4 when `--evidence` is set. Consolidation readers can key on version:

- `schema_version: 3` (legacy): assertions are optional/absent
- `schema_version: 4`: assertions are required on every finding

## Consolidation rules under `--evidence`

**Clamping rule:** If any finding in any judge's output lacks `test_assertions` (or has fewer than 2), consolidation clamps the overall verdict to at least `WARN`, regardless of individual judge verdicts. The consolidated report must list the clamped findings explicitly so reviewers can see why the verdict didn't reach PASS.

**Disagreement handling under --evidence is easier:** When two judges disagree, the tie-breaker is "which judge's assertions actually hold when re-run". The consolidator (or a human reviewer) can execute the assertions from both judges and let the filesystem decide.

**Cross-session verifiability:** A future session reading a `--commit-ready` report in `docs/council-log/` can re-run every assertion without re-spawning the council. If an assertion now fails (e.g., the cited file was since deleted), the reader knows the finding has drifted and the verdict no longer holds.

## Pair with `--commit-ready`

The strongest pairing: `/council --evidence --commit-ready validate <target>`. This writes a verdict with concrete, re-runnable assertions to `docs/council-log/`. The commit message that closes the decision can reference the council log path, and any future session debugging the decision can execute the assertions in under a minute.

## Authoring good test_assertions

**Good:**
- `grep -rn 'func Compute(cwd string)' cli/internal/corpus/` → expect 1 match at `fitness.go:74`
- `test -d cli/internal/overnight/checkpoint_test.go` → expect exit 0
- `go test ./internal/overnight/ -run TestCheckpoint_NewCommitRollback_HappyPath` → expect PASS

**Bad:**
- "The code is clean" → not falsifiable
- "Check the README for details" → not a check, no expected result
- "Run the full test suite" → not cheap, not targeted

**Rule of thumb:** If a reader can't execute the assertion in under 10 seconds and get a deterministic answer, it's not an assertion — it's a hand-wave.

## Cost

Adds ~15-30% to judge latency (judges spend extra time forming assertions). Worth it when:

- The council verdict will be cited in a commit message
- The decision is load-bearing for >1 session's work
- The target has history of scope drift or stale descriptions
- You want the council report to survive across machines / agents / rebases

Skip `--evidence` for routine mid-implementation sanity checks where context is cheap and re-running the council is faster than reading back assertions.

## Concrete case

See `docs/council-log/2026-04-11-validate-m8-assumption-validation.md` — the canonical example. Three judges ran in `--tdd` mode against a stale bead description. Judge 1 returned falsifiable assertions that proved `cli/cmd/ao/fitness.go` did not exist and `collectLearnings` lived elsewhere. Judge 2 returned assertions proving the staging infrastructure already existed at specific line ranges. Both sets of assertions were re-run by the lead agent during consolidation (via simple grep/stat commands) and confirmed within 30 seconds. The resulting verdict collapsed the na-h61 scope estimate from 395 LOC to ~300 LOC and unblocked same-session implementation.
