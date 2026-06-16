# Evidence — M4: self-improvement ASSAY tick (age-d16-self-hosting-route-nkr.5)

**Bead:** `age-d16-self-hosting-route-nkr.5` — "Self-improvement ASSAY tick over existing miners."
**Arc:** BUILD the bounded orchestration TICK that closes the self-improvement loop; REUSE the existing miners. The last organ before the terminal integration run.

## Contract (the bead's scenarios)

1. Given a completed run's evidence in the ledger, When the ASSAY tick runs, Then it mines the evidence and emits **>=1 follow-up suggestion bead, UNATTENDED**.
2. Given the tick, When it runs, Then it is **bounded** (not daemonized/unbounded).

Architecture: **SENSOR** (ledger) → **ASSAY** (bounded miners) → **GATE** (suggestion re-enters the front door as a bead).

## What existed (re-baseline, verified file-by-file)

- **SENSOR is fed:** `docs/provenance/ledger.jsonl` carries real `verdict` rows (`"from_type":"verdict"`, `evidence_ref":"pawl-verdict ag-NNN disposition=CONFIRMED"`) produced by M1+M3 — 16 at build time. These ARE the completed-run evidence.
- **Miners that exist (reuse targets):** `ao harvest` (`cli/cmd/ao/harvest.go`, sweeps `.agents/` → learnings catalog), `skills/{forge,curate}` (transcripts → `.agents/*.md`), `skills/compile` (→ wiki), `skills/inject` (deprecated). **None of them files a follow-up bead from ledger evidence** — none closes the SENSOR→GATE loop. `skills/{dream,harvest}` do **not** exist (confirmed; not invented).
- **`scripts/assay/` exists** with one sibling, `consume-mto-recurrence.sh` (consumes an MTO handoff → finding/planning-rule). The established home + conventions (fail-closed, crisp terminal). A **ledger-mining tick is genuinely net-new.**
- **M2 shape to mirror:** `scripts/recovery-statemachine.sh` — pure-shell bounded orchestrator, one terminal JSON verdict, no spin/silent-defer/mis-close, with a failure-injection bats stubbing `br` + injected commands.

## What M4 adds

`scripts/assay/self-improvement-tick.sh` — a **bounded** pure-shell tick (no daemon, no Claude). It reads the bounded ledger window (SENSOR), derives suggestions (ASSAY), and files each as a follow-up bead through the front door (GATE).

| stage | behavior | bound |
|---|---|---|
| **SENSOR** | `tail -n <window>` of the ledger; keep only `verdict` rows | `--window` (default 50) — never scans full history |
| **ASSAY** | default: one suggestion per DISTINCT bead in the window (most-recent first); or `--mine-cmd` to reuse an existing miner (evidence on its stdin, suggestion lines out) | single pass — no loop |
| **GATE** | `br create` a labeled (`assay-suggestion`) follow-up bead per suggestion, body cites the mined bead + head commit + evidence_ref + a `## Scenarios` stub | `--max-suggestions` (default 1) caps beads filed |

**Crisp terminal invariants (structurally enforced, asserted by tests):**
- **No daemon / no spin** — a single pass over a bounded window; no loop, no `sleep`, no background. `"bounded":true,"daemonized":false,"spin":false` emitted by construction.
- **No silent defer** — every path emits exactly ONE terminal JSON verdict. Empty/verdict-less ledger → `no-evidence` (not a silent no-op); a non-zero `--mine-cmd` or `br create` → loud `gate-failed` / exit 4, never a quiet `set -e` abort.
- **No mis-close** — the tick NEVER calls `br close`. It only FILES follow-ups; acceptance/close authority stays at the merge/pawl door (`reconcile-pr.sh`, M3).

Exit codes: `0` ok (filed | no-evidence | no-suggestions | dry-run) · `2` usage · `4` gate-fail.

## Acceptance test (drives evidence-in → follow-up-bead-out)

`tests/scripts/assay-self-improvement-tick.bats` — 13 cases; `br` and the miner are injected (a fake `br` on PATH + a `--mine-cmd` string) so cases are deterministic and offline. A real-shaped fixture ledger (two distinct verdict beads + one non-verdict row that must be ignored) drives the SENSOR.

```
1..13
ok 1 evidence-in -> files >=1 follow-up suggestion bead (default assay), exit 0
ok 2 filed bead body cites the mined bead + head commit (most-recent evidence)
ok 3 bounded: a miner emitting many suggestions files at most --max-suggestions
ok 4 no daemon: emits exactly ONE terminal verdict line and terminates
ok 5 no-evidence: empty ledger -> no-evidence verdict, NO bead filed, exit 0
ok 6 no-evidence: ledger with only non-verdict rows -> no-evidence, NO bead
ok 7 gate-fail: non-zero br create -> loud terminal gate-failed, exit 4 (no silent set -e abort)
ok 8 gate-fail: non-zero --mine-cmd -> loud terminal gate-failed, exit 4
ok 9 no-suggestions: miner emits nothing -> no-suggestions verdict, NO bead, exit 0
ok 10 dry-run: emits the decision with NO br mutation
ok 11 usage: unknown argument exits 2
ok 12 usage: non-numeric --max-suggestions exits 2
ok 13 usage: zero --window exits 2
```

- Case 1 is the **core acceptance**: evidence-in → a real `br create` (labeled) → the filed id reported in the verdict; NO `br close`.
- The bounded cap, no-daemon, no-silent-defer (failed gate/miner → loud exit 4) and crisp no-op terminals are each asserted.

### Merge-to-main pawl (fresh-context refuter) — REFUTED ×2 → fixed

A fresh-context refuter (then re-verified) caught **two** silent-failure holes on the degraded paths my self-review missed — both the M2 fail-open class, both now fixed and regression-locked:

1. **Silent defer (set -e abort, no verdict):** the default-assay command substitution `SUGGESTIONS="$(… | reverse | awk …)"` was unguarded. On a host lacking `tac` where `tail -r` errors, `reverse()` exited non-zero → the substitution aborted under `set -euo pipefail` with **exit 1 and no terminal JSON** (the always-`exit 0` stub `br` could not surface it). Fixed two ways: (a) `reverse()` is now a portable always-exit-0 reverser (`tac` → probed `tail -r` → pure-awk fallback that exists everywhere); (b) the substitution is wrapped in `if ! …; then emit_terminal gate-failed; exit 4; fi`. Locked by the `default-assay pipeline failure -> loud gate-failed, exit 4` case (stub `awk` exit non-zero).
2. **Silent drop (under-file):** `printf '%s' "$EVIDENCE"` (no trailing newline) left the final ledger row unterminated, so `tac`/`tail -r` **glued it onto its neighbor**; the awk then matched only the first bead on that line and the second distinct bead's suggestion was silently dropped at `--max-suggestions ≥2`. Fixed: `printf '%s\n'`. Locked by the `default assay surfaces ALL distinct beads (no silent drop from reverse-glue)` case (3-bead ledger, both surface).

Re-refute confirmed both closed (reproduced the GNU-like-PATH break now reverses/escalates correctly; reproduced both new regression cases red-before / green-after) and found no further fail-open / unbounded / mis-close hole.

## Live end-to-end (real `br` binary, isolated ledger)

Beyond the stubbed bats, the loop was driven against the **real `br`** in an isolated temp `BEADS_DIR` (no touch to the real `_beads`):

```
$ tick --ledger <1 verdict row for ag-999> --max-suggestions 1
{"tick":"self-improvement-assay","state":"filed",...,"suggestions_filed":1,"filed_beads":["tt-4hy"]}
$ br list
○ tt-4hy [● P2] [task] - ASSAY follow-up from ag-999
```

Evidence-in → real follow-up bead `tt-4hy` filed UNATTENDED, labeled, title citing the mined bead. And a `--dry-run` against the **real** `docs/provenance/ledger.jsonl` parsed all 16 live evidence rows → 1 bounded suggestion (`would file follow-up bead for ag-742`), no mutation.

## Gate

- `bash -n` + `shellcheck` clean on the new script.
- `ao gate check --fast --scope head` — green (the changed-shell check covers the new script + bats).

## Scope boundary

Non-goals held: no unbounded/daemonized mining (single pass, capped); no rebuilt miner (reuse via `--mine-cmd`); no `dream`/`harvest` invention; no flywheel/gold/wiki/corpus-PROMOTE (Mossy's lane); no `br close` authority (M3 owns it). The live wire (scheduling the tick after a real run + pointing `--mine-cmd` at `ao harvest`) is the epic done-test's integration step, not this slice.
