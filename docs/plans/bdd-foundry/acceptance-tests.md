# land.sh acceptance tests — index (bdd-foundry Phase 2, ATDD)

> **Status: RED by design.** `scripts/land.sh` does not exist yet; the harness
> installs a loud not-implemented placeholder (exit 97) into every sandbox
> fixture, so all 73 tests fail until the feature is built. Each test is the
> executable definition of done for one frozen scenario in
> [`behaviors.md`](behaviors.md). **No runnable test, no bead.**
>
> **Run the suite (red):**
> ```bash
> bats docs/plans/bdd-foundry/acceptance-tests                       # one pass
> bash docs/plans/bdd-foundry/acceptance-tests/run-acceptance.sh     # B73 discipline: double-run + no-skip/no-focus + totality
> ```
> Requires `bats` (1.13+ used) and `jq`. Fully hermetic: every fixture lives in
> a `mktemp -d` sandbox with a `land-sandbox`-marked bare remote; nothing
> pushes to the real origin (B25).
>
> **Pinned observable contract** (exit codes, env knobs, lock-storage layout,
> test seams, fixture-repo seams) lives in ONE place:
> [`acceptance-tests/helpers.bash`](acceptance-tests/helpers.bash) header. The
> spec phase may renegotiate a pin by editing helpers — never by weakening a
> scenario (that re-opens Phase 1).
>
> B73 pins the repo-level entry point at `tests/landing/run-acceptance.sh`;
> the implementation phase wires that path to delegate to
> `acceptance-tests/run-acceptance.sh` (asserted, currently red).

## Scenario → test map

Test names embed the scenario id: `@test "B<n>: …"`. One test per scenario, 73/73.

| Scenario | Test (in file under `acceptance-tests/`) |
|---|---|
| B1 | `B1: single lane lands with one command and one rebase attempt` — `01-core-landing.bats` |
| B2 | `B2: regen-at-land — author never hand-runs the 8 generators; regen commit carries pinned identity` — `01-core-landing.bats` |
| B3 | `B3: queued lanes land FIFO with zero manual rebases` — `01-core-landing.bats` |
| B4 | `B4: three-lane soak — the tonight-failure-mode does not reproduce` — `01-core-landing.bats` |
| B5 | `B5: lock status is observable for swarm tending` — `01-core-landing.bats` |
| B6 | `B6: mutual exclusion is mechanical — no two holders, ever (10-lane stress)` — `01-core-landing.bats` |
| B7 | `B7: stale lock from a dead lane is reclaimed, not waited on forever` — `01-core-landing.bats` |
| B8 | `B8: a live lock is never stolen` — `01-core-landing.bats` |
| B9 | `B9: derived surfaces never present a textual conflict to anyone` — `01-core-landing.bats` |
| B10 | `B10: re-running land.sh on an already-landed branch is a no-op` — `01-core-landing.bats` |
| B11 | `B11: counts are never hand-asserted — prose counts are generated or gated` — `01-core-landing.bats` |
| B12 | `B12: out-of-band push between gate and push is absorbed, never force-pushed` — `01-core-landing.bats` |
| B13 | `B13: all gate failures reported in ONE pass, not one per cycle` — `01-core-landing.bats` |
| B14 | `B14: dirty working tree is refused before any lock or rebase` — `01-core-landing.bats` |
| B15 | `B15: a real source conflict aborts cleanly and reports, leaving no wreckage` — `01-core-landing.bats` |
| B16 | `B16: hash-marker JSON corruption can never land (duplicate generated_hash class)` — `01-core-landing.bats` |
| B17 | `B17: direct pushes to main outside land.sh are mechanically blocked` — `01-core-landing.bats` |
| B18 | `B18: gate failure or crash mid-land never strands the queue` — `01-core-landing.bats` |
| B19 | `B19: every class of dirt is refused or explicitly classified before lock` — `02-preflight.bats` |
| B20 | `B20: root discovery works; broken invocation contexts are refused` — `02-preflight.bats` |
| B21 | `B21: an in-progress git operation is refused before lock` — `02-preflight.bats` |
| B22 | `B22: missing tooling is reported in ONE preflight summary, no lock taken` — `02-preflight.bats` |
| B23 | `B23: configuration has pinned precedence and validates before lock` — `02-preflight.bats` |
| B24 | `B24: help, version, and usage errors never mutate anything` — `02-preflight.bats` |
| B25 | `B25: the harness refuses to operate on a non-sandbox remote` — `02-preflight.bats` |
| B26 | `B26: lock acquisition is atomic — exactly one winner at the same instant` — `03-lock-queue.bats` |
| B27 | `B27: unreadable lock storage fails closed before any mutation` — `03-lock-queue.bats` |
| B28 | `B28: lane identity is unique and PID reuse cannot fake liveness` — `03-lock-queue.bats` |
| B29 | `B29: queue hygiene — no duplicate entries, no ghosts, no starvation` — `03-lock-queue.bats` |
| B30 | `B30: concurrent lands of the same branch — one land, one clean no-op` — `03-lock-queue.bats` |
| B31 | `B31: heartbeat stays fresh through long phases; heartbeat write failure is fail-safe` — `03-lock-queue.bats` |
| B32 | `B32: SIGINT/SIGTERM at every phase exit cleanly, no wreckage` — `03-lock-queue.bats` |
| B33 | `B33: a failed holder releases and the queue advances` — `03-lock-queue.bats` |
| B34 | `B34: the status contract is total — every lock state has pinned JSON` — `03-lock-queue.bats` |
| B35 | `B35: rebase base comes from a post-acquisition fetch; fetch failure fails closed` — `04-rebase-shapes.bats` |
| B36 | `B36: nothing-to-land shapes exit fast and mutate nothing` — `04-rebase-shapes.bats` |
| B37 | `B37: already-landed is patch-aware; partial lands are completed, not duplicated` — `04-rebase-shapes.bats` |
| B38 | `B38: branch shapes that would denormalize main are handled explicitly` — `04-rebase-shapes.bats` |
| B39 | `B39: a lane that becomes conflicted only after its predecessor lands` — `04-rebase-shapes.bats` |
| B40 | `B40: conflicts are classified — modify/delete, rename/rename, binary` — `04-rebase-shapes.bats` |
| B41 | `B41: land.sh is provably non-interactive and leaves git config untouched` — `04-rebase-shapes.bats` |
| B42 | `B42: the derived write set is a manifest, never a hard-coded list` — `05-derived-regen.bats` |
| B43 | `B43: generator failure mid-land aborts with everything restored` — `05-derived-regen.bats` |
| B44 | `B44: nondeterministic generator output is detected before push` — `05-derived-regen.bats` |
| B45 | `B45: deleting or renaming a skill leaves no stale derived entries` — `05-derived-regen.bats` |
| B46 | `B46: hand edits to generator-owned files are reset to canonical output` — `05-derived-regen.bats` |
| B47 | `B47: the strict-JSON verifier is broad and independently testable` — `05-derived-regen.bats` |
| B48 | `B48: the count checker reads a manifest and survives marker edge cases` — `05-derived-regen.bats` |
| B49 | `B49: the gate runs every required family and aggregates across all of them` — `06-gate.bats` |
| B50 | `B50: a hung gate check is killed by timeout, not waited on forever` — `06-gate.bats` |
| B51 | `B51: a base main that is already red is reported as such, never blamed on the lane` — `06-gate.bats` |
| B52 | `B52: post-land verification runs the full gate on a fresh clone of the remote` — `06-gate.bats` |
| B53 | `B53: push failures are classified and leave no half-land` — `07-push.bats` |
| B54 | `B54: a rewound or force-pushed origin/main fails closed` — `07-push.bats` |
| B55 | `B55: a land pushes exactly one ref — refs/heads/main, fast-forward by construction` — `07-push.bats` |
| B56 | `B56: repeated out-of-band churn exhausts the bounded retry and aborts cleanly` — `07-push.bats` |
| B57 | `B57: crash-point matrix — SIGKILL at every phase is recoverable to exactly-once` — `08-crash-recovery.bats` |
| B58 | `B58: --abort has a complete contract` — `08-crash-recovery.bats` |
| B59 | `B59: every documented failure has a clean retry path` — `08-crash-recovery.bats` |
| B60 | `B60: destructive steps record a recovery point; temp artifacts are cleaned` — `08-crash-recovery.bats` |
| B61 | `B61: disk-full never corrupts main, the lock, or the audit log` — `08-crash-recovery.bats` |
| B62 | `B62: guard install lifecycle is explicit; an unprotected clone is never silent` — `09-guards.bats` |
| B63 | `B63: the land.sh bypass marker cannot be replayed outside a live land` — `09-guards.bats` |
| B64 | `B64: branch-side .gitignore edits cannot blind the checks` — `09-guards.bats` |
| B65 | `B65: private _beads paths can never be staged or pushed by land.sh` — `09-guards.bats` |
| B66 | `B66: a branch that modifies land.sh itself is handled, not trusted blindly` — `09-guards.bats` |
| B67 | `B67: exit codes are a stable taxonomy and every error is structured` — `10-observability.bats` |
| B68 | `B68: every land emits one durable, correlated log; audit appends are atomic` — `10-observability.bats` |
| B69 | `B69: post-success local state is defined — branch kept, rebased, clean` — `10-observability.bats` |
| B70 | `B70: post-failure local state is defined for every failure class` — `10-observability.bats` |
| B71 | `B71: --dry-run reports the full plan and mutates nothing` — `10-observability.bats` |
| B72 | `B72: hostile branch names and paths cannot break land.sh` — `10-observability.bats` |
| B73 | `B73: the acceptance suite is itself gated — one command, hermetic, total, deterministic` — `11-meta-suite.bats` |

## Harness notes (for the spec + implementation phases)

- **Fixture repo** (built by `helpers.bash :: seed_fixture`) models the real
  repo's seams in miniature: `skills/*/SKILL.md` sources; mini generators in
  `scripts/generators/*.sh` producing `registry.json`, `docs/context-map.md`,
  `docs/SKILL-TIERS.md`, `skills-codex/**/.agentops-generated.json` (single
  `generated_hash` key), and marker-block counts in `docs/COUNTS.md`;
  `scripts/regen-all.sh [--check]`; the declared write set in
  `scripts/regen-manifest.txt` (B42); count-doc manifest in
  `scripts/count-docs.txt` (B48); extra gate checks in `scripts/gate.d/*.sh`;
  CI gate-family declaration in `.github/workflows/validate.yml` (B49 parity).
- **ENOSPC (B61)** is exercised via portable write-failure stand-ins (partial
  generator write, unwritable object store, read-only audit log), not a real
  quota filesystem — the asserted invariants are identical.
- **Coverage-map check (B73)** currently surfaces unmapped scenarios in the
  frozen behaviors coverage table (e.g. B15/B31/B69/B70 appear in no risk-class
  row) — that is the map check doing its job; resolving it is a Phase-1-owner
  edit to the table (tagging, not scenario change).
- **Known deliberate red:** `tests/landing/run-acceptance.sh` does not exist
  yet (B73). The functional runner ships here as
  `acceptance-tests/run-acceptance.sh`; wiring the pinned path is
  implementation work.
