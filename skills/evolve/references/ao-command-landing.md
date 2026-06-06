# AO-Command Landing — the surfaces a new `ao` command must regenerate

Adding (or renaming/removing) an `ao` command or subcommand touches more than the
generated CLI reference. `scripts/regen-all.sh` regenerates the bulk of the derived
surfaces, but it only **WARNS** about two that it cannot regenerate for you — those
are the dominant fix-and-repush cause when landing a command. Regenerate them in the
**same pass** before opening the PR, not piecemeal.

This is the command-surface parallel of [new-skill-landing.md](new-skill-landing.md)
(the six derived surfaces a *skill* edit must regenerate).

## The shortcut

`scripts/regen-all.sh` regenerates `cli/docs/COMMANDS.md`, `registry.json`, the
skill/context maps, the embedded skills, and the cli-surface inventory in one pass —
prefer it over running the individual generators. Then hand-fix the two surfaces it
only warns about (below), and finally run `scripts/regen-all.sh --check`.

## The two surfaces regen-all only WARNS about

1. **Cobra `expectedCmds` (x2 lists)** — `cli/cmd/ao/cobra_commands_test.go` hardcodes
   the registered-command list **twice**: once in `TestCobraCommandTreeRegistration`
   and once in `TestCobraExpectedCmdsMatchRegistration` (the second comment even says
   "Same list as TestCobraCommandTreeRegistration"). Both `expectedCmds` slices must
   gain the new top-level command name, or the second test fails with
   "registered command %q is not in expectedCmds — add it to keep the list in sync".

2. **`cli-command-surface` heading counts** — the generated reference's `ao` heading
   counts (`top`/`sub`/`all`) are asserted in **two** offline canaries that must move
   together:
   - `evals/agentops-core/fixtures/cli-command-surface-smoke.sh` — the
     `top_count`/`sub_count`/`all_count` literals (currently `76`/`192`/`268`).
   - `evals/agentops-core/cli-command-surface-matrix.json` — the public canary that
     enumerates every documented command/subcommand help page.

   **Run the smoke fixture to read the exact new counts** rather than computing them by
   hand:

   ```bash
   bash evals/agentops-core/fixtures/cli-command-surface-smoke.sh
   # On a count mismatch it prints: unexpected command heading counts: top=.. sub=.. all=..
   # Update the literals in the fixture (and the matrix.json cases) to the printed values.
   ```

   (Automating this regen is tracked as `ag-jy12`.)

## See also

- [new-skill-landing.md](new-skill-landing.md) — the six derived surfaces a *skill*
  edit regenerates (the sibling discipline for the skills axis).
- [gate-hygiene.md](gate-hygiene.md) — pre-push diff-scope check + pre-existing-vs-mine
  red triage that keeps a command-landing PR one-shot-green.
