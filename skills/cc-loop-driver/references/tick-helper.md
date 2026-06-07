# tick.sh — ledger/git mechanics for the all-Claude loop

> The orchestrator (main agent) runs these subcommands; the worker/validator subagents do the work. This wraps the `bd`/`br` + `git` plumbing so the agent's job is dispatch + judgement. Substitute `br` for `bd` if the repo declares beads_rust.

## Subcommands

```
tick.sh next                                  # highest-priority ready bead id, or empty (=> NO_READY)
tick.sh status                                # NO_READY (blocked/in-progress) vs CONVERGED (none open)
tick.sh claim <id>                            # orchestrator claims (single writer)
tick.sh reopen <id>                           # FAIL path: reopen with the validator's reasons in the comment
tick.sh close <id> <commit-msg> <evidence-ref> [scoped paths...]
```

## The close-cannot-lie contract (the load-bearing part)

The `close` subcommand MUST, in order:

1. **Verify the evidence ref resolves to a real artifact** — a path that exists, or an `evidence/`-prefixed ref. Refuse the close (exit non-zero) if it points at nothing. A close with a fake evidence ref is worse than no close.
2. Record `before=$(git rev-parse HEAD)`.
3. Run `bd close <id> --reason "evidence: <ref>"`. If it fails/skips, abort — nothing was staged.
4. **Confirm the ledger shows the bead closed** (`jq` over `.beads/issues.jsonl` after a `bd sync --flush-only`). If not, reopen the bead and abort.
5. **Stage ONLY scoped paths** — the ledger (`.beads/issues.jsonl`, `.beads/metadata.json`), the evidence artifact, and the bead's explicitly-passed files. NEVER `git add -A` (sweeps untracked) or `git add -u` (sweeps all tracked mods); both are unsafe under concurrent edits.
6. `git commit -m "<conventional commit msg>"`.
7. Record `after=$(git rev-parse HEAD)`. **If `before == after` the commit didn't land — reopen the bead and exit non-zero.** Never leave a closed-but-unpersisted bead.

## Council gate (optional hardening — Themis assurance)

When running two validators for stronger assurance:

- `tick.sh verdict-gate <verdict-file>` — passes only if the verdict contains a non-empty `COMMANDS RUN` section. An unverified verdict (PASS or FAIL) is **rejected**, never acted on. This is the false-FAIL/false-PASS killer.
- `tick.sh council-gate <v1> <v2>` — each verdict is `verdict-gate`'d first, then:
  - **unanimous verified PASS** => proceed to close.
  - **any unverified, or unanimous FAIL** => fail-closed; reopen + informed retry (budget 2, then escalate to a human).
  - **mixed PASS/FAIL (both verified)** => dispatch a 3rd tie-break validator; majority of *verified* verdicts decides; fail-closed if no majority. The orchestrator never self-overrules.

## Idempotency + state words

- Every tick begins with `tick.sh next`; a closed bead is never schedulable, so re-running a tick is safe.
- `NO_READY` = no schedulable bead right now (work blocked or in-progress). `CONVERGED` = no open or in-progress beads at all. Keep the two distinct — they mean different next actions.
