# BDD Foundry — Beads Manifest (landing pipeline / land.sh)

> Written 2026-06-12. Sixteen ACCEPTANCE-bearing beads in `br` (prefix `ag`,
> workspace `_beads/` in the agentops main checkout — created from the main
> checkout root, never from this worktree, per the no-`br`-from-a-worktree rule).
> Each bead is self-contained: title, why, scenario delivered, and an explicit
> **ACCEPTANCE** section carrying the runnable done-criterion (`bats -f` filter
> over `docs/plans/bdd-foundry/acceptance-tests/`). All P1 tasks, labels
> `bdd-foundry,landing`.

**Overlap check (pre-write):** scanned all open `ag-*` beads for land.sh /
landing / lock / regen / gate / fixture-guard overlap — zero matches; nothing
merged, nothing duplicated. Nearest adjacent bead is `ag-arpk` (GitHub merge
queue on main) — different mechanism (GitHub-side queue vs the local `scripts/land.sh`
serializer), intentionally left separate.

## Created beads (run-local key → br id)

| # | Key | br id | Title |
|---|-----|-------|-------|
| 1 | `d3-fixture-guard` | `ag-d3-fixture-guard-yk7rq` | D3 — server-side fixture guard pre-receive hook + LAND_PUSH_NONCE pin in helpers.bash |
| 2 | `m1-m2-skeleton` | `ag-m1-m2-skeleton-1thaa` | scripts/land.sh skeleton: M1 CLI dispatch/exit taxonomy/config resolution + M2 preflight battery |
| 3 | `m3-m4-lock-core` | `ag-m3-m4-lock-core-yett0` | M3 lane identity/correlation/audit + M4 lock storage: atomic acquire, heartbeat, fail-closed, --status contract |
| 4 | `m5-m6-rebase-spine` | `ag-m5-m6-rebase-spine-sqoec` | M5 base resolution & shape analysis + M6 rebase engine (marker, backup ref, conflict partition, clean restore) |
| 5 | `m7-regen-verifiers` | `ag-m7-regen-verifiers-ckesa` | M7 regen-at-land: manifest-driven write set, regen sequence + determinism check, --verify-generated-json, --check-counts |
| 6 | `m8-gate-runner` | `ag-m8-gate-runner-medas` | M8 gate runner: family discovery + CI parity, one-pass aggregation, per-check process-group timeout, base-red attribution |
| 7 | `m8b-push-spine` | `ag-m8b-push-spine-ns7zw` | M8b push + bounded re-rebase loop + sandbox/nonce discipline + success cleanup — completes the end-to-end land spine |
| 8 | `queue-fifo-concurrency` | `ag-queue-fifo-concurrency-5dwxw` | Queue/concurrency hardening: FIFO under contention, 10-lane stress, queue hygiene, same-branch dedup, failed-holder advance |
| 9 | `liveness-staleness-signals` | `ag-liveness-staleness-signals-mo5as` | Liveness hardening: stale reclaim, live-lock sanctity, PID-reuse defense, heartbeat fail-safe, signal traps, never-stranded queue |
| 10 | `branch-shapes-e2e` | `ag-branch-shapes-e2e-m5ktj` | Branch-shape closeout: already-landed idempotence, partial-land completion, merge-flatten/empty-drop/revert pins, post-predecessor conflicts, non-interactive proof |
| 11 | `regen-e2e-guards` | `ag-regen-e2e-guards-4a0f1` | Derived-surface closeout: conflict-free derived surfaces, generated counts, hash-marker JSON gate, manifest authority, stale-entry/hand-edit/gitignore/_beads guards |
| 12 | `preflight-closeout` | `ag-preflight-closeout-u58s1` | Preflight/config closeout: full dirt taxonomy with land tails, config precedence under live holders, help/version/usage zero-mutation with audit version cross-check |
| 13 | `recovery-abort` | `ag-recovery-abort-iodl9` | M9 crash recovery: SIGKILL crash-point matrix, complete --abort contract, retry paths, recovery-point hygiene, ENOSPC safety, defined post-failure states |
| 14 | `guard-install` | `ag-guard-install-86yy3` | M10 --install client pre-push guard + nonce replay defense + self-modification refusal |
| 15 | `observability-cli` | `ag-observability-cli-3ngky` | Observability closeout: stable exit taxonomy + structured failure summaries, correlated durable logs/atomic audit, defined post-success state, full --dry-run plan, hostile-name hardening |
| 16 | `meta-suite-closure` | `ag-meta-suite-closure-eg2yn` | D2 tests/landing/run-acceptance.sh delegator + D4 behaviors.md coverage-map tagging + full-suite deterministic double-run closure |

## Dependency map (bead → blockers)

| Bead | Depends on |
|------|-----------|
| `ag-d3-fixture-guard-yk7rq` | — (root) |
| `ag-m1-m2-skeleton-1thaa` | — (root) |
| `ag-m3-m4-lock-core-yett0` | `ag-m1-m2-skeleton-1thaa` |
| `ag-m5-m6-rebase-spine-sqoec` | `ag-m3-m4-lock-core-yett0` |
| `ag-m7-regen-verifiers-ckesa` | `ag-m5-m6-rebase-spine-sqoec` |
| `ag-m8-gate-runner-medas` | `ag-m7-regen-verifiers-ckesa` |
| `ag-m8b-push-spine-ns7zw` | `ag-m8-gate-runner-medas`, `ag-d3-fixture-guard-yk7rq` |
| `ag-queue-fifo-concurrency-5dwxw` | `ag-m8b-push-spine-ns7zw` |
| `ag-liveness-staleness-signals-mo5as` | `ag-m8b-push-spine-ns7zw` |
| `ag-branch-shapes-e2e-m5ktj` | `ag-m8b-push-spine-ns7zw` |
| `ag-regen-e2e-guards-4a0f1` | `ag-m8b-push-spine-ns7zw` |
| `ag-preflight-closeout-u58s1` | `ag-m8b-push-spine-ns7zw` |
| `ag-recovery-abort-iodl9` | `ag-m8b-push-spine-ns7zw` |
| `ag-guard-install-86yy3` | `ag-m8b-push-spine-ns7zw`, `ag-d3-fixture-guard-yk7rq` |
| `ag-observability-cli-3ngky` | `ag-m8b-push-spine-ns7zw` |
| `ag-meta-suite-closure-eg2yn` | `ag-queue-fifo-concurrency-5dwxw`, `ag-liveness-staleness-signals-mo5as`, `ag-branch-shapes-e2e-m5ktj`, `ag-regen-e2e-guards-4a0f1`, `ag-preflight-closeout-u58s1`, `ag-recovery-abort-iodl9`, `ag-guard-install-86yy3`, `ag-observability-cli-3ngky` |

23 edges total; `br dep cycles` reports no cycle touching any of these ids.
Spine chain: `m1-m2-skeleton` → `m3-m4-lock-core` → `m5-m6-rebase-spine` →
`m7-regen-verifiers` → `m8-gate-runner` → `m8b-push-spine` (which also needs
`d3-fixture-guard`, the second root). The seven closeout beads fan out from
`m8b-push-spine` and all converge on `meta-suite-closure` (B73).

## Scenario → bead coverage

| Bead | Scenarios |
|------|-----------|
| `ag-d3-fixture-guard-yk7rq` | B62 (harness half; enables B17, B63, and all push-path scenarios) |
| `ag-m1-m2-skeleton-1thaa` | B20, B21, B22 |
| `ag-m3-m4-lock-core-yett0` | B27, B34 |
| `ag-m5-m6-rebase-spine-sqoec` | B15, B35, B36, B40 |
| `ag-m7-regen-verifiers-ckesa` | B43, B44, B47, B48 |
| `ag-m8-gate-runner-medas` | B13, B49, B50, B51 |
| `ag-m8b-push-spine-ns7zw` | B1, B2, B12, B25, B52, B53, B54, B55, B56 |
| `ag-queue-fifo-concurrency-5dwxw` | B3, B4, B5, B6, B26, B29, B30, B33 |
| `ag-liveness-staleness-signals-mo5as` | B7, B8, B18, B28, B31, B32 |
| `ag-branch-shapes-e2e-m5ktj` | B10, B37, B38, B39, B41 |
| `ag-regen-e2e-guards-4a0f1` | B9, B11, B16, B42, B45, B46, B64, B65 |
| `ag-preflight-closeout-u58s1` | B14, B19, B23, B24 |
| `ag-recovery-abort-iodl9` | B57, B58, B59, B60, B61, B70 |
| `ag-guard-install-86yy3` | B17, B62, B63, B66 |
| `ag-observability-cli-3ngky` | B67, B68, B69, B71, B72 |
| `ag-meta-suite-closure-eg2yn` | B73 (closure over B1–B72) |

Flat coverage: B1–B73 of `behaviors.md` are all owned exactly once —
B62 split by design (harness half in `d3-fixture-guard`, client-guard half in
`guard-install`); B73 is the suite-closure bead and transitively covers the rest.

Coverage by bead key:
- `d3-fixture-guard`: B62 (harness half; enables B17, B63, and all push-path scenarios)
- `m1-m2-skeleton`: B20, B21, B22
- `m3-m4-lock-core`: B27, B34
- `m5-m6-rebase-spine`: B15, B35, B36, B40
- `m7-regen-verifiers`: B43, B44, B47, B48
- `m8-gate-runner`: B13, B49, B50, B51
- `m8b-push-spine`: B1, B2, B12, B25, B52, B53, B54, B55, B56
- `queue-fifo-concurrency`: B3, B4, B5, B6, B26, B29, B30, B33
- `liveness-staleness-signals`: B7, B8, B18, B28, B31, B32
- `branch-shapes-e2e`: B10, B37, B38, B39, B41
- `regen-e2e-guards`: B9, B11, B16, B42, B45, B46, B64, B65
- `preflight-closeout`: B14, B19, B23, B24
- `recovery-abort`: B57, B58, B59, B60, B61, B70
- `guard-install`: B17, B62, B63, B66
- `observability-cli`: B67, B68, B69, B71, B72
- `meta-suite-closure`: B73 (closure over B1–B72)

## Done-criterion shape (uniform)

Every bead's ACCEPTANCE section is RED today (placeholder `scripts/land.sh`
exits 97) and turns GREEN when its `bats -f '^B(...):'` filter over
`docs/plans/bdd-foundry/acceptance-tests` exits 0, with the previously-green
filters pinned as regression in each later bead. The full-suite gate is
`meta-suite-closure`: `bash tests/landing/run-acceptance.sh` exits 0,
double-run deterministic, 73/73, zero skipped/focused.
