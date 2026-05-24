# Loop Budget

Dev loop structure and latency budgets for the AgentOps repo. Every check and CI job is classified into one loop. Adding a new check requires declaring its loop affinity and proving it fits the budget.

## Loop Definitions

| Loop | Latency Budget | Question Answered | Gate Character |
|------|---------------|-------------------|----------------|
| **Inner** | <10s | "Does it compile?" | `make inner` — non-blocking, instant feedback |
| **Middle (blocking)** | <30s | "Will this break others?" | Pre-push pass 1 — blocks push |
| **Middle (advisory)** | <3min | "Are there quality concerns?" | Pre-push pass 2 — warns, does not block |
| **Outer** | <10min | "Is the system healthy?" | CI — runs on PR/push to main |

## Invocation

```bash
cd cli && make inner                           # Inner loop (<10s)
git push                                       # Two-pass gate (default):
                                               #   Pass 1: blocking (<30s)
                                               #   Pass 2: advisory (<3min)
scripts/pre-push-gate.sh --single-pass         # Full single-pass (old behavior)
```

## Pre-Push Check Classification

| # | Check | Category | Loop | Blocking? |
|---|-------|----------|------|-----------|
| 1 | Go build + vet | go | Middle (blocking) | Yes |
| 2 | Go race tests (changed scope) | go | Middle (blocking) | Yes |
| 3 | Command/test pairing | go | Middle (blocking) | Yes |
| 3a | Mutation-route bypass guard | always | Middle (blocking) | Yes |
| 3b | HOME isolation in test files | go | Middle (blocking) | Yes |
| 3b2 | Test HOME isolation (broader) | go/shell | Middle (blocking) | Yes |
| 3d | .agents/ write-surface contract | always | Middle (advisory) | No |
| 4 | cmd/ao coverage floor | go | Middle (advisory) | No |
| 4b | Per-package coverage ratchet | go | Outer (CI only) | No |
| 5 | Embedded hooks sync | hook | Middle (blocking) | Yes |
| 6 | Skill count sync | skill | Middle (blocking) | Yes |
| 7 | Worktree disposition | always | Middle (advisory) | No |
| 8 | Skill runtime/CLI parity | skill | Middle (advisory) | No |
| 9 | Codex skill parity | skill | Outer (skipped) | No |
| 10 | Codex install bundle parity | skill | Outer (skipped) | No |
| 11 | Codex runtime section format | skill | Outer (CI) | No |
| 12 | Skill integrity (refs/xrefs) | skill | Middle (advisory) | No |
| 13 | Skill lint suite | skill | Middle (advisory) | No |
| 14 | Skill schema validation | skill | Middle (advisory) | No |
| 15 | Manifest schema validation | skill | Middle (advisory) | No |
| 16 | Codex artifact metadata | skill | Outer (CI) | No |
| 17 | Codex backbone prompts | skill | Outer (CI) | No |
| 18 | Codex override coverage | skill | Outer (CI) | No |
| 19 | Next-work contract parity | always | Outer (CI) | No |
| 19b | bd closeout contract parity | always | Outer (CI) | No |
| 19c | Retrieval quality ratchet | always | Outer (CI) | No |
| 20 | Skill runtime formats | skill | Middle (advisory) | No |
| 21 | Codex RPI contract | skill | Outer (CI) | No |
| 22 | Codex lifecycle guards | skill | Outer (CI) | No |
| 23 | Skill CLI snippets | skill | Middle (advisory) | No |
| 24 | Headless runtime smoke | skill | Outer (CI) | No |
| 24b | CLI docs parity | go | Middle (advisory) | No |
| 24c | Eval canaries (deterministic) | eval | Outer (CI) | No |
| 24e | Contract canaries | contract | Outer (CI) | No |
| 25 | Doc-release gate | docs | Outer (CI) | No |
| 25b | Release audit artifact refs | docs | Outer (CI) | No |
| 26 | Contract compatibility | contract | Middle (advisory) | No |
| 27 | Hook preflight | hook | Middle (blocking) | Yes |
| 28 | Hooks/docs parity | hook | Middle (advisory) | No |
| 29 | CI policy parity | ci_policy | Outer (CI) | No |
| 30 | ShellCheck | shell | Middle (advisory) | No |
| 31 | Plugin load test (symlinks) | always | Middle (blocking) | Yes |
| 32 | Learning coherence | learning | Outer (CI) | No |
| 33 | BATS orphan hooks audit | hook | Outer (CI) | No |
| 34 | Skill citation parity | skill | Outer (CI) | No |
| 35 | Flywheel health | always | Outer (CI) | No |

## CI Job Classification

| Job | Path Group | Loop |
|-----|-----------|------|
| go-build | go | Outer |
| cli-integration | go | Outer |
| cli-docs-parity | go | Outer |
| json-flag-consistency | go | Outer |
| embedded-sync | hooks | Outer |
| hook-preflight | hooks | Outer |
| hook-output-schema-lint | hooks | Outer |
| validate-hooks-doc-parity | hooks | Outer |
| bats-tests | hooks | Outer |
| skill-integrity | skills | Outer |
| skill-lint | skills | Outer |
| skill-schema | skills | Outer |
| skill-dependency-check | skills | Outer |
| validate-headless-runtime-skills | skills | Outer |
| validate-codex-runtime-sections | codex | Outer |
| validate-codex-generated-artifacts | codex | Outer |
| validate-codex-backbone-prompts | codex | Outer |
| validate-codex-override-coverage | codex | Outer |
| validate-codex-rpi-contract | codex | Outer |
| validate-codex-lifecycle-guards | codex | Outer |
| agentops-eval-baseline-audit | eval | Outer |
| eval-workbench-verify | eval | Outer |
| agentops-eval-advisory | eval | Outer |
| eval-skill-delta | eval | Outer |
| doc-release-gate | docs | Outer |
| markdownlint | docs | Outer |
| smoke-test | always | Outer |
| shellcheck | shell | Outer |
| security-scan | always | Outer |
| security-toolchain-gate | always | Outer |
| agentops-contract-canaries | contracts | Outer |
| contract-compatibility-gate | contracts | Outer |
| validate-ci-policy-parity | ci | Outer |
| pre-push-gate-wired | always | Outer |
| registry-check | skills | Outer |
| plugin-load-test | always | Outer |
| learning-coherence | learning | Outer |
| memrl-health | always | Outer |
| file-manifest-overlap | always | Outer |
| doctor-check | always | Outer |
| check-test-staleness | always | Outer |
| swarm-evidence | always | Outer |
| windows-smoke | always | Outer |
| summary | always | Outer (gateway) |

## Policy: Adding New Checks

1. Declare the loop affinity (inner/middle-blocking/middle-advisory/outer).
2. Measure the check's runtime. It must fit within the loop's latency budget.
3. Middle-blocking checks must prevent real breakage (compilation failure, sync drift, security bypass). Quality/drift/hygiene checks belong in middle-advisory or outer.
4. If the check requires `go build` or network access, it cannot be inner-loop.
5. Update this document when adding or reclassifying a check.
