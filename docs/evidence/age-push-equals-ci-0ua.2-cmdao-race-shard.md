# age-push-equals-ci-0ua.2 cmd/ao race shard evidence

Bead: `age-push-equals-ci-0ua.2`
Date: 2026-06-19
Base: `d5adccaffbecc142aec650ad27f9481a26c4f8b9`

## Change

`TestOrchestrateToolsExecuteJSON` and `TestOrchestratePreflightExecuteJSON`
were moved behind the `integration` build tag in
`cli/cmd/ao/orchestrate_integration_test.go`. The tests now run from a temp
contract fixture with stubbed orchestration binaries, so the preflight smoke
still proves CLI execution and temp-ledger append behavior without appending
`preflight:dual-pane` rows to the tracked repository ledger.

The pre-push hook still runs the default full race suite for pushes to main and
then runs the explicit cmd/ao integration shard:

```sh
go test ./cmd/ao -tags=integration -run "$cmdao_integration_tests" -race -shuffle=on -count=1
```

## Timing

Before split, measured from this worktree before edits:

```text
go test ./cmd/ao -race -shuffle=on -count=1
ok github.com/boshu2/agentops/cli/cmd/ao 47.428s
```

After split:

```text
go test ./cmd/ao -race -shuffle=on -count=1
ok github.com/boshu2/agentops/cli/cmd/ao 32.636s
real 41.40
```

Integration shard:

```text
go test ./cmd/ao -tags=integration -run 'TestOrchestrate(Tools|Preflight)ExecuteJSON' -race -shuffle=on -count=1
ok github.com/boshu2/agentops/cli/cmd/ao 2.840s
real 11.58
```

## Selector proof

Default tier does not list the moved tests:

```text
go test ./cmd/ao -list 'TestOrchestrate(Tools|Preflight)ExecuteJSON'
ok github.com/boshu2/agentops/cli/cmd/ao 0.555s
```

Integration tier lists both moved tests:

```text
go test ./cmd/ao -tags=integration -list 'TestOrchestrate(Tools|Preflight)ExecuteJSON'
TestOrchestrateToolsExecuteJSON
TestOrchestratePreflightExecuteJSON
ok github.com/boshu2/agentops/cli/cmd/ao 0.317s
```

## Validation

```text
sh -n scripts/hooks/pre-push.local
bash -n scripts/install-pre-push-gate.sh scripts/pawl-land.sh
shellcheck -S warning scripts/hooks/pre-push.local scripts/install-pre-push-gate.sh scripts/pawl-land.sh
bats tests/scripts/pre-push-local.bats
```

`bats tests/scripts/pre-push-local.bats` passed 7/7 and now checks that the
cmd/ao integration shard runs before the serial mutable push lock.
