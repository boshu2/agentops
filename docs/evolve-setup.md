# Setting Up /evolve

Bootstrap with `ao goals init` — it interviews you about your repo and generates mechanically verifiable goals. Or write them by hand:

```markdown
# GOALS.md

## test-pass-rate
- **check:** `make test`
- **weight:** 10
All tests pass.

## code-complexity
- **check:** `gocyclo -over 15 ./...`
- **weight:** 6
No function exceeds cyclomatic complexity 15.
```

Migrating from GOALS.yaml? Run `ao goals migrate --to-md`. Manage goals with `ao goals steer add/remove/prioritize` and prune stale ones with `ao goals prune`.

`/evolve` measures them, picks the worst gap by weight, runs `/rpi` to fix it, re-measures ALL goals (regressed commits auto-revert), and loops. In the v2 CLI, use `ao evolve` as the terminal-native entrypoint; it delegates to the same engine as `ao rpi loop --supervisor`. It commits locally — you control when to push. Kill switch: `echo "stop" > ~/.config/evolve/KILL`

**Built for overnight runs.** Cycle state lives on disk, not in LLM memory — survives context compaction. Every cycle writes to `cycle-history.jsonl` with verified writes, a regression gate that refuses to proceed without a valid fitness snapshot, and a watchdog heartbeat for external monitoring. If anything breaks the tracking invariant, the loop stops rather than continuing ungated. See `skills/SKILL-TIERS.md` for the two-tier execution model that keeps the orchestrator visible while workers fork.

Maintain over time: `/goals` shows pass/fail status, `/goals prune` finds stale or broken checks.

## Pairing GOALS.md with PROGRAM.md

Use `GOALS.md` for strategic fitness and `PROGRAM.md` for operational control.

- `GOALS.md` answers what good looks like.
- `PROGRAM.md` answers what the autonomous loop may touch, how one experiment is bounded, which validations decide success, and when to stop or escalate.
- `/evolve` now loads `PROGRAM.md` before cycle 1, filters out-of-scope work, and uses the program's validation and decision policy in its cycle keep/revert gate.

Initialize the operational contract with:

```bash
ao autodev init
ao autodev validate
```

This split keeps repo goals stable while allowing the autonomous runtime policy to evolve independently. See [Autodev Program Contract](contracts/autodev-program.md) for the required sections and semantics.

## See Also

- [README.md](https://github.com/boshu2/agentops/blob/main/README.md) — repo overview and `/evolve` demo
- [How It Works](how-it-works.md) — runtime mechanics
- [The Science](the-science.md) — decay model behind fitness scoring
