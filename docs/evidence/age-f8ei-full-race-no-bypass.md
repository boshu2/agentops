# age-f8ei full-race no-bypass proof

Bead: `age-f8ei`
Source gap: `age-3pdt`
Verified head: `35b08ca2906a3b03cd7ea38e264f540e4789d803`
Run date: 2026-06-19

## Reason

`age-3pdt` landed `f61c5f0e711a` and removed the `ao rpi` command
surface, but its close reason said the push-to-main full race suite was
bypassed with `AGENTOPS_PREPUSH_SKIP_FULL_RACE` because of a pre-existing
`TestGoals_Integration_MeasureDirectivesJSON` goals/Cobra-global isolation
flake.

This evidence reconciles that local release-membrane gap against current
`origin/main`. No code change was required: the suspected goals test and the
full push-equivalent race suite pass on current head without the bypass.

## Proof commands

All commands ran in `/Users/bo/dev/agentops-wt/age-f8ei-full-race-proof/cli`
with no `AGENTOPS_PREPUSH_SKIP_FULL_RACE` environment variable set.

```bash
go test ./cmd/ao -run TestGoals_Integration_MeasureDirectivesJSON -count=20 -shuffle=on
```

Result:

```text
ok  	github.com/boshu2/agentops/cli/cmd/ao	0.622s
```

```bash
go test ./cmd/ao -race -shuffle=on -count=1
```

Result:

```text
ok  	github.com/boshu2/agentops/cli/cmd/ao	43.910s
```

```bash
go test ./... -race -shuffle=on -count=1
```

Result: exit 0. The full CLI package set passed. Representative terminal
lines:

```text
ok  	github.com/boshu2/agentops/cli/cmd/ao	49.876s
ok  	github.com/boshu2/agentops/cli/internal/worktree	8.591s
ok  	github.com/boshu2/agentops/cli/internal/yieldledger	1.287s
ok  	github.com/boshu2/agentops/cli/pkg/vault	1.148s
```

## Code context

The push-to-main hook now runs the local release membrane with randomized
order and without inherited Git hook discovery environment:

- `scripts/hooks/pre-push.local` runs `go test ./... -race -shuffle=on -count=1`
  for pushes to `main` or `master`.
- The same block unsets `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`,
  `GIT_PREFIX`, `GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`, and
  `GIT_NAMESPACE` before the suite.
- `tests/scripts/pre-push-local.bats` guards both the `-shuffle=on` command and
  the environment scrub.

The goals test path has local reset coverage:

- `cli/cmd/ao/goals_integration_test.go` restores `goalsMeasureDirectives`
  around the direct `--directives` execution.
- `cli/cmd/ao/testutil_test.go` includes a broader `resetCommandState`
  snapshot/reset for goals package-level flag globals.

## Conclusion

The `age-3pdt` full-race bypass is reconciled on current `origin/main`.

Closeout checks also passed:

- Fresh-context pawl review:
  `.agents/pawl-verdicts/evidence/age-f8ei-codex.md` ended
  `VERDICT: CONFIRMED`.
- `ao gate check --fast --scope head`: 22 checks passed, 0 failed.

Land this evidence commit and close `age-f8ei` with this file and the
validation commands as proof.

Transient `preflight:dual-pane` rows appeared in `docs/provenance/ledger.jsonl`
during proof work. They are unrelated sensor output and must not be staged with
this evidence.
