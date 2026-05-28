# Scenario → Test Linkage Contract

Status: active · Bead: soc-63xfx · Gate: `validate-scenario-test-linkage` (T1, blocking)

## Why

71 `.feature` files live under `skills/*/references/`. Historically only a
handful had an executing test; the rest were documentation that drifted free of
any check. A scenario nobody runs is a claim nobody verifies.

This contract makes the scenario→test relationship **mechanical**: every Gherkin
`Scenario:` in the corpus must either name the test that covers it, or its
feature file must be explicitly declared documentation-only. The gate fails on
any scenario that is neither.

It is the sibling of `executable-spec-link-integrity` (soc-58nt / soc-m8tdn),
which links **GOALS directives → scenarios**. This one links **scenarios →
tests**. Together they close the spec-to-evidence chain:

```
GOALS directive ──► scenario (.feature) ──► test (executing)
   (exec-spec-link-integrity)         (scenario-test-linkage)
```

## The convention

Tag a scenario with the test that covers it using a Gherkin tag:

```gherkin
@covered-by:tests/e2e/rpi-phased-domain.sh
Scenario: Phases run in order and never compress
  When /rpi executes
  Then it runs Research, then Plan, then Implement in order
```

Rules:

- The tag goes on its **own line directly above** the `Scenario:` it covers —
  the standard Gherkin tag position. A non-tag line (a step, `Background`, a
  blank then content) between the tag and the scenario breaks the association.
- **File-level tags** (placed above the `Feature:` line) apply to **every**
  scenario in the file.
- Multiple tags may stack; a scenario passes only if **all** of its
  `@covered-by:` targets resolve.
- Two target forms:
  - `@covered-by:<test-path>` — the test file must exist (repo-relative path).
  - `@covered-by:<test-path>::<Name>` — additionally, `<Name>` (a Go func, a
    bats `@test` label, a bash function, etc.) must appear in that file. The
    name match is a substring check, which keeps the gate language-agnostic
    while still catching a renamed or deleted test.

`<test-path>` is any executing test surface: `tests/e2e/*.sh`,
`tests/scripts/*.bats`, `cli/**/*_test.go`, and so on. The gate only asserts the
target **exists** — it does not run it. Running is the test suite's job; linking
is this gate's job.

## The allowlist

`scripts/.scenario-linkage-allow` lists feature files (repo-relative, one per
line, `#` comments allowed) that are intentionally documentation-only for now.
The gate skips scenarios in those files.

The allowlist is a **draining backlog**, not a parking lot:

- When a skill gains an executing test, tag its scenarios with `@covered-by:`
  and **remove the file from the allowlist**.
- A file may **not** be both allowlisted **and** carry `@covered-by:` tags —
  that ambiguous intent is a gate error. Pick one.
- A stale allowlist entry (a listed file that no longer exists) is a gate error.
- Do not add new feature files to the allowlist without a tracking bead; new
  specs should ship with their covering test.

## Gate behavior

`scripts/check-scenario-test-linkage.sh`:

| Condition | Result |
|---|---|
| Scenario has a resolving `@covered-by:` tag | pass |
| Scenario's feature file is allowlisted | pass (counted as doc-only) |
| Scenario has no tag and file is not allowlisted | **FAIL** |
| `@covered-by:` path does not exist | **FAIL** (dangling) |
| `@covered-by:...::Name` not found in the file | **FAIL** (dangling) |
| Allowlisted file also carries a `@covered-by:` tag | **FAIL** (ambiguous) |
| Allowlist entry points at a deleted file | **FAIL** (stale) |

Flags: `--warn-only` (advisory, exit 0), `--json` (machine-readable summary),
`-h`/`--help`. Exit codes: `0` pass, `1` fail, `2` misuse.

Wired into `.github/workflows/validate.yml` as `validate-scenario-test-linkage`,
gated on changes to `skills/**`, `**/*.sh`, or `.github/**`.

## Starter set (at landing, soc-63xfx)

3 of 71 feature files are fully linked to executing tests:

| Feature | Covering test |
|---|---|
| `skills/rpi/references/rpi.feature` | `tests/e2e/rpi-phased-domain.sh` |
| `skills/goals/references/goals.feature` | `tests/e2e/goals-measure-scenarios.sh`, `goals-steer-auto.sh`, `goals-trace-chain.sh` |
| `skills/scenario/references/scenario.feature` | `tests/e2e/goals-scenarios-link.sh` |

The remaining 68 files are allowlisted in `scripts/.scenario-linkage-allow` with
the standing goal to drain the list as skills gain executing tests.
