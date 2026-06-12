# land.sh — Phase 3 SPEC (bdd-foundry)

> **Derivation rule:** this spec exists ONLY to turn the 73 red acceptance tests
> green (`acceptance-tests/*.bats`, executable definitions of the frozen
> scenarios in `behaviors.md`). Anything not needed by a B-scenario is out of
> scope. Where a scenario allows "one documented behavior," the choice is
> **pinned here** and printed in `--help`. The observable contract (exit codes,
> env knobs, lock layout, test seams) is already pinned in
> `acceptance-tests/helpers.bash` — this spec renegotiates exactly one pin
> (§6) and changes no scenario.

---

## 1. Deliverables (4 artifacts, nothing else)

| # | Artifact | Status | Satisfies |
|---|---|---|---|
| D1 | `scripts/land.sh` — **single self-contained bash file** (the harness `install_sut` copies only this one file into every fixture; it may source nothing from the repo) | new | B1–B72 |
| D2 | `tests/landing/run-acceptance.sh` — thin executable delegator to `docs/plans/bdd-foundry/acceptance-tests/run-acceptance.sh`; body must literally contain the suite path and the tokens `focus`, `skip`, `twice`/`deterministic` (B73 greps it) | new | B73 |
| D3 | `acceptance-tests/helpers.bash` edit — install the **server-side fixture guard** pre-receive hook into the bare remote after seeding, + pin `LAND_PUSH_NONCE` in the contract header (§6 — the one sanctioned renegotiation) | edit | B17, B62, B63 |
| D4 | `behaviors.md` coverage-map tagging edit — add the unmapped ids (B15, B31, B69, B70; verify all of B1–B73 expand from the table) so the B73 map check passes. Tagging only — no scenario text changes (Phase-1-owner edit sanctioned in the Phase-2 harness notes) | edit | B73 |

Portability bound: macOS + Linux CI (B73). Bash 3.2-compatible; no `flock`,
no `setsid`, no `date +%N`, no GNU-only flags. Process-group isolation via
`perl -e 'setpgrp; exec @ARGV'` (perl is in the B22 thin-PATH whitelist).
Hard tool requirements of land.sh itself: `git`, `jq` (checked in preflight).

## 2. land.sh internal architecture

One file, function-modular. The land pipeline is a strict phase sequence; every
phase logs start/end (ISO-8601) to the per-land log and updates the
in-progress marker's `phase` field once mutation begins.

```
main dispatch ─┬─ --help/--version/usage errors        (M1)
               ├─ --status [--json]                    (M4, read-only)
               ├─ --abort                              (M9)
               ├─ --dry-run                            (M11)
               ├─ --install                            (M10)
               ├─ --verify-generated-json / --check-counts  (M7, standalone)
               └─ land:
                   M2 preflight → M4 lock acquire (queue, heartbeat)
                   → M5 fetch/base/shape → M6 rebase (+backup ref, marker)
                   → M7 regen + verifiers → M8 gate
                   → M8b push (bounded re-rebase loop) → release/cleanup
```

### M1 — CLI, config, exit taxonomy (B23, B24, B67, B72)
- Flag parsing: subcommands + knobs exactly as pinned in helpers.bash header.
  Unknown flag / incompatible combo (`--status --abort`) → usage to stderr,
  nonzero, **zero side effects** (no lock entry, no ref change) (B24).
- Config resolution per knob: **CLI flag > env (`LAND_*`) > repo git config
  `land.*` (camelCase: `land.staleTtl`, `land.waitTimeout`, …) > built-in
  default** (defaults: TTL 900, heartbeat 30, max rebase attempts 2). Validation
  (numeric, ≥0) names the bad knob and exits 10 **before** lock acquisition;
  `--wait-timeout=0` = fail immediately if held (documented in --help) (B23).
- `--dry-run` prints the effective config (knob name + value — B23 greps
  `stale…111`, `900`, etc.).
- Exit taxonomy (pinned in helpers): `0` ok/no-op · `3` dry-run-blocked ·
  `10` preflight refusal · `11` lock wait timeout · `12` source conflict ·
  `13` gate/regen failure · `14` push failure · `20` internal error.
  `--status` exits 20 only for corrupt/unreadable lock states (B34). Signal
  exits use 128+sig (distinct from all of the above) (B32, B67).
- Every failure path ends with one structured summary: phase, failed command,
  branch, current SHA, base SHA, `retryable: yes|no`, one-line next action
  (B67, B70); hostile branch names always passed as `--`-separated single args
  and JSON-encoded via `jq -n --arg` in audit lines (B72).

### M2 — Preflight battery (B14, B19–B25, B42-overlap, B22)
Order: (1) root/context refusals → (2) aggregated tooling/manifest summary →
(3) dirt taxonomy. All before any lock or ref mutation; refused cases leave
the worktree bit-identical and the audit log untouched.
- Root discovery: `git rev-parse --show-toplevel` (works from subdirs);
  distinct messages, verbatim per B20: `"not a git worktree"`,
  `"detached HEAD"`, `"refusing to land main onto itself"`,
  `"no origin remote"`, `"origin/main not found"` — each within 5 s.
- In-progress git op: `.git/rebase-merge`, `rebase-apply`, `MERGE_HEAD`,
  `CHERRY_PICK_HEAD` (via `--git-dir`-relative paths, linked-worktree-safe) →
  `"git operation in progress"`, state files untouched (B21).
- Tooling/manifest check, **one aggregated summary**: `git`, `jq`,
  `scripts/regen-all.sh` present+executable, `scripts/regen-manifest.txt`,
  `scripts/count-docs.txt`; list every missing item by name (B22).
- Manifest overlap: any path declared generator-owned that is also a tracked
  source path committed outside generator commits → contract error naming the
  overlap (B42 clause 3).
- Dirt taxonomy (B14, B19, B65) — **pinned rules, printed in --help**:
  - staged / unstaged / partially-staged tracked changes → refuse
    `"working tree dirty — commit or stash"` (≤5 s).
  - untracked file at a manifest path → refuse, naming the colliding path and
    the owning generator (B19c).
  - **(d) untracked outside the write set and (e) ignored inside it: PROCEED
    with a logged warning** (--help text: "untracked files outside the
    generated write set are allowed and ignored"). B19 reads the rule from
    --help and asserts whichever is printed — the word "refus"/"block" must
    therefore NOT appear in the untracked help line.
  - `_beads/` paths: exempt from the dirty check, reported separately, never
    staged — all land.sh `git add` calls use pathspec `:!_beads` and write-set
    pathspecs only (B65).
- Sandbox guard (test mode, B25): when `LAND_TEST_MODE=1`, refuse before any
  push attempt unless the origin's git dir contains the `land-sandbox` marker
  file: `"refusing non-sandbox remote"`. Checked in preflight (cheap local
  remote) AND immediately pre-push.

### M3 — Identity, correlation, logging (B28, B68)
- Lane id: `host:realpath(worktree):pid:start_time` — start_time read from
  `ps -p $$ -o lstart=` normalized to epoch (mismatch with a lock's recorded
  pair defeats PID reuse, B28).
- Correlation id: `<epoch>-<8 hex from /dev/urandom>`; appears in the log
  file name, every audit entry for the land, and the regen commit trailer
  (B2, B68).
- Per-land log: `$LAND_LOCK_DIR/logs/<correlation>.log`; final stdout includes
  `log: <path>`. Contents: ISO timestamps, lane identity, branch, start SHA,
  base SHA, final main SHA or failure phase, per-phase durations, gate and
  push results, and the **git execution trace** — every `git` command land.sh
  runs is echoed into the log (B12 and B55 grep this trace for the absence of
  `--force` and the exact push refspec).
- Audit appends: one `printf '%s\n'` per event, `>>` (O_APPEND, line < 4 KB ⇒
  atomic); events: `acquire`, `release`, `stale-takeover`, `corrupt-lock`,
  `abort`, plus out-of-band-SHA and expected/observed-SHA records (B12, B54).
  `acquire` entries carry `holder.id` (jq-readable — B3 sorts them). Readers
  validate per line and skip invalid lines (torn append / pre-existing garbage
  never crashes later lands or `--status`) (B61, B68). Audit entries record the
  land.sh version (B24 cross-checks against `--version`).

### M4 — Lock & queue (B5–B8, B26–B34)
Storage exactly as pinned: `lock.json` (holder record with `id`, `pid`,
`start_time`, `heartbeat`, `nonce`), `queue/`, `audit.jsonl` under
`$LAND_LOCK_DIR`.
- **Acquire** (atomic, B26): `(set -C; printf … > lock.json)` — O_EXCL
  noclobber create. Heartbeat refresh and stale takeover replace via
  `tmp + mv` (matches the harness's fabricated/live-holder writers). Takeovers
  serialize through a micro-mutex `mkdir takeover.d` with re-validation inside,
  so two waiters can't both claim a stale lock.
- **Queue** (FIFO, B29): one JSON entry per waiter at
  `queue/<seq>-<idhash>.json`; `seq` from an atomic counter file incremented
  under a `mkdir queue/.seq.d` micro-mutex (portable, no `%N`). A lane
  enqueues at most one entry (re-invocation reuses it); waiters garbage-collect
  entries whose `pid`+`start_time` no longer validate (ghosts, B29); the lock
  is taken only by the lowest-seq live waiter ⇒ FIFO, no starvation.
- **Liveness** (B7, B8, B28): a lock is *stale* iff heartbeat age > TTL **and**
  holder identity fails live-validation (pid dead, or pid alive with different
  start_time = PID reuse). Live locks are never stolen; stale locks are
  reclaimed within one poll (poll interval 1 s ≪ 30 s bound) with a
  `stale-takeover` audit entry naming dead id/pid/last heartbeat.
- **Heartbeat** (B31): background subshell rewrites `lock.json` (tmp+mv,
  preserving nonce) every `LAND_HEARTBEAT_INTERVAL`, through all phases.
  **Pinned failure behavior:** if a heartbeat write fails, the holder
  self-aborts — signals the main process, which releases (rm) the lock, runs
  the abort restore, and exits nonzero. Never a takeover racing a live push.
- **Fail closed** (B27): unreadable/corrupt lock storage (truncated JSON,
  unreadable dir, dangling symlink) → exit nonzero matching
  `"lock state unreadable"`/`"lock storage"` with **no** rebase/regen/push and
  no silent deletion; any cleanup is preceded by a `corrupt-lock` audit entry.
- **`--status [--json]`** (B5, B34): read-only, never mutates. Pinned schema:
  `{state: unheld|held|stale|corrupt|unreadable, holder: {id,pid,heartbeat_age_seconds}|null, queue: [...]}`;
  plain mode prints `unheld`/`held …`. Exit 0 for unheld/held/stale, 20 for
  corrupt/unreadable.
- **Signals** (B32): traps on INT/TERM — queued: remove own queue entry;
  holding pre-mutation: release + audit; holding post-mutation: leave the
  in-progress marker (recoverable via `--abort`); never leave rebase wreckage
  (run `git rebase --abort` in the trap when wreckage exists). Exit 128+sig.
- Failed holder always releases on exit (trap-guaranteed), queue advances
  within one poll (B33, B18).

### M5 — Base resolution & branch shapes (B35–B38, B10, B30)
- Post-acquisition `git fetch origin` decides the base = `origin/main` tip at
  acquisition (logged as base SHA); fetch failure → `"cannot determine base"`,
  release, exit nonzero, nothing mutated (B35).
- Shape analysis before any mutation (all patch-id-based via
  `git rev-list --no-merges` + `git patch-id --stable`):
  - every lane patch already on main → exit 0 `"already landed"`, lock held
    <10 s (B10, B37); SHA-different cherry-picks count as landed.
  - 0 ahead → exit 0 `"nothing to land"` (B36). Local `main` ref is never
    read or written; origin/main is the only base authority (B36 clause 3).
  - Concurrent same-branch lands (B30): the loser either finds all patches
    landed (`"already landed"`, exit 0) or sees the same lane-branch named in
    the holder record → nonzero `"branch is being landed by"`.
- **Pinned shape rules (B38), printed in --help:** merge commits are
  **flattened** by the linearizing rebase (`--no-rebase-merges`, logged
  "flattening N merge commit(s)"); empty commits dropped (`--empty=drop`);
  reverts land as ordinary commits. Main stays strictly linear.

### M6 — Rebase engine (B9, B15, B39–B41, B60, B66)
- Before first mutation: write the in-progress marker
  `<git-dir>/land-in-progress.json` `{correlation, branch, orig_sha,
  backup_ref, phase}` and the backup ref `refs/land/backup/<idhash>` =
  original tip (B60). On success both are removed; ref list and git dir return
  to pre-land state modulo landed commits.
- Non-interactive hardening (B41): every git child runs with
  `GIT_EDITOR=false GIT_SEQUENCE_EDITOR=: GIT_PAGER=cat`, stdin `</dev/null`,
  and per-invocation `-c rebase.autoStash=false -c rerere.enabled=false`.
  No `git config` write, ever, outside `--install` hook file creation (config
  itself untouched even then). Internal commits use `--no-verify` (repo
  commit-msg prompt hooks can't fire).
- Rebase `feat-x` onto base. **Pinned derived-surface strategy (B9):
  auto-resolve-then-regen** — at each conflict stop, partition conflicted
  paths against the regen manifest:
  - all conflicted paths generator-owned → resolve to the base side
    (`git checkout --ours -- <paths>`; content is irrelevant — M7 regenerates
    them), `git add`, `git rebase --continue`. No CONFLICT line for manifest
    paths ever reaches the report.
  - any source-owned path conflicted → `git rebase --abort`, restore pre-land
    state (HEAD back on branch at orig SHA, status clean, no
    `.git/rebase-merge`), release lock, report **only source paths** with
    conflict class — `modify/delete`, `rename/rename`, `binary` (from
    porcelain status + `git diff --numstat` `-` markers; binary files get no
    textual merge) — exit 12 (B15, B39, B40).
- Self-modification check (B66) — **pinned policy: refuse.** If lane commits
  touch `scripts/land.sh` or `scripts/regen-all.sh`:
  `"self-modifying land — land manually with review"`, restore, exit 10.

### M7 — Regen & derived-surface verifiers (B2, B11, B16, B42–B48, B64)
- Write set = `scripts/regen-manifest.txt` (paths + dir prefixes). land.sh
  hard-codes **no** surface list (B42): a new manifest entry is covered with
  zero land.sh changes.
- Sequence after a clean rebase:
  1. Run `scripts/regen-all.sh` (failure → capture stderr, name the generator,
     full restore via backup ref + write-set clean, release, exit 13 — B43).
  2. **Determinism check** (B44): run the battery a second time; sha256
     (`shasum -a 256`/`sha256sum` shim) over the write set must match run 1,
     else `"nondeterministic generator output"` naming file+generator, abort
     before push, exit 13.
  3. **Hand-edit detection** (B46): write-set paths whose lane-committed
     content differs from generator output → warning naming each discarded
     generated-path edit; source changes are untouched (they live outside the
     write set by the B42 overlap check).
  4. **Manifest drift check** (B42): `git status` scoped to the write set vs.
     untracked new files claimed by generators — a generator writing an
     undeclared path, or a declared path no generator wrote, fails the land.
  5. Commit any write-set diff: `git add -f -- <write-set> ':!_beads'`
     (`-f` defeats branch-side .gitignore blinding — B64), committed as
     `chore(land): regen derived surfaces` with author+committer
     `land.sh <land@local>` and trailers `Landed-by: land.sh <version>` +
     `Land-correlation: <correlation>` (B2). Never left uncommitted.
- **`--verify-generated-json`** (standalone subcommand = the B16/B47 verifier,
  also a gate family): file list derived from the manifest (declared `.json`
  paths + `*.json` under declared dirs). Checks per file: strict jq parse,
  UTF-8 validity, trailing garbage, and duplicate keys via `jq --stream` path
  counting (exactly one `generated_hash` per codex file). Nonzero names file +
  defect class; 0 on a clean tree.
- **`--check-counts`** (standalone = the B11/B48 checker, also a gate family):
  reads `scripts/count-docs.txt`; validates marker-block structure
  (missing closing tag and duplicate marker ids are **distinct** errors);
  repo-wide sweep of docs for count-pattern literals (`<N> skills` shapes)
  outside marker blocks — catches not-yet-listed docs, ignores non-count
  numerics ("we tried 47 times" carries no marker context and doesn't match
  the anchored pattern). Planted wrong values inside markers are simply
  overwritten by regen at land (B11).

### M8 — Gate runner (B13, B49–B51) and push (B12, B53–B56, B25)
- Families: `regen-check` (`regen-all.sh --check`), `json-verify` (M7),
  `counts` (M7), `gate.d` (each `scripts/gate.d/*.sh`). **CI parity** (B49):
  parse `# land-gate-families:` from `.github/workflows/validate.yml`; any CI
  family missing from land.sh's list fails the gate.
- One pass, one `== gate ==` header; run every family to completion, aggregate
  ALL failures into the final summary — check name, offending file, remedy
  (B13, B49). Infrastructure breakage (missing tool) stays fail-fast in M2.
- Per-check timeout `LAND_GATE_TIMEOUT`: each check runs in its own process
  group (`perl setpgrp` wrapper); on timeout, `kill -TERM/-KILL -<pgid>` — no
  orphan survivors, exit ≤ 2× timeout, named check (B50).
- **Base-red attribution** (B51): on gate failure, re-run the failing families
  in a throwaway worktree (`git worktree add --detach`) of the base SHA;
  failures reproduced there are reported under `"base main is already
  failing"`, separated from branch-introduced failures; nothing pushed.
- Test seams honored here: `LAND_TEST_GATE_SLEEP` (extra sleep inside the gate
  phase), `LAND_TEST_AFTER_GATE_CMD` (run after each gate pass — injects
  out-of-band churn/push faults), `LAND_TEST_CRASH_AFTER` (kill -9 self after
  phase ∈ {rebase, regen-write, regen-commit, gate, push, pre-release}).
- **Push:** re-check the sandbox marker (B25), export
  `LAND_PUSH_NONCE=<holder nonce>`, then exactly
  `git push origin HEAD:refs/heads/main` — the only refspec, never any tag,
  note, or feature ref; never `--force`/`--force-with-lease` (trace-asserted,
  B55, B12).
  - Non-fast-forward reject → fetch; if observed remote tip is not a
    descendant of the expected tip → rewound remote, fail closed
    (`"origin/main moved unexpectedly|rewound"`, audit expected+observed SHAs,
    exit 14, B54). Otherwise bounded loop: re-rebase + full gate re-run,
    total attempts ≤ `LAND_MAX_REBASE_ATTEMPTS` (default 2); log
    `rebase attempts: N` once per land; exhaustion →
    `"out-of-band churn"`, exit 14, branch left rebased and landable (B12,
    B56). Absorbed out-of-band SHAs are audited; gate failures after
    absorption are attributed to the absorbed base (B56 clause 3).
  - Failure classification (B53): stderr-pattern map → network/unreachable =
    `retryable: yes`; auth, remote-hook reject = `retryable: no`. No half-land:
    local branch keeps its rebased commits.
- Success: verify pushed SHA == remote tip, release lock (audit `release`),
  clear marker, delete backup ref + temp worktrees/files (B60), leave the
  worktree on the lane branch at the landed SHA, status clean, local `main`
  and the lane's remote branch untouched (B69).

### M9 — `--abort` & crash recovery (B18, B57–B61, B70)
- Marker is the single recovery token. `--abort` contract (B58):
  (a) no marker + no wreckage → exit 0 `"nothing to abort"`;
  (b) own marker → `git rebase --abort` if needed, checkout original branch,
  `git reset --hard <orig_sha>`, clean write-set strays (`git clean` scoped to
  the manifest — removes stray regen artifacts, case d), clear marker, release
  own lock, audit `abort`, exit 0;
  (c) a different live lane holds the lock → refuse nonzero
  `"lock held by live lane"`, lock untouched.
- Crash matrix (B57): every `LAND_TEST_CRASH_AFTER` point leaves either
  pre-land state or marker+backup-ref state; stale TTL frees the lock (B18);
  rerun (after `--abort` where a marker demands it) converges to exactly-once
  via M5's patch-id analysis — crashed-after-push reruns report
  `"already landed"`. Retry paths for gate-fix/conflict-fix/post-takeover all
  reduce to a fresh land on the new base (B59).
- Post-failure state is always one of: clean tree on original branch@SHA, or
  exactly one marker (B70); each class's summary names its retry instruction.
- ENOSPC (B61): lock/marker writes are tmp+rename (atomic or absent); audit is
  single-line append with per-line-validating readers; generator/commit
  write failures route through the B43 restore. main, lock, and audit never
  corrupt.

### M10 — Guard install (`--install`) (B17, B62, B63)
- Writes the client `.git/hooks/pre-push` guard carrying a
  `# land-guard-version: <version>` stamp. Hook logic: pushes updating
  `refs/heads/main` are rejected with `"use scripts/land.sh"` unless
  `LAND_PUSH_NONCE` matches the **live** lock holder's nonce in
  `$LAND_LOCK_DIR/lock.json` (holder pid+start_time validate live) — marker
  presence alone is worthless, replay fails when the lock is unheld (B63).
- Idempotent: rerun → `"already installed"`, exit 0; stale stamp → in-place
  upgrade with logged old→new version (B62).
- Prints the origin's branch-protection **posture** (informational):
  `"sandbox remote — fixture guard active"` / `"branch protection not
  detectable for this origin"` (matches the B62 `protect|posture|sandbox`
  grep).
- `--install` is the only operation allowed to create files under
  `.git/hooks/`; it still writes no git config (B41).

### M11 — `--dry-run` (B71, B20, B23)
Full preflight + fetch + shape analysis, **no lock, no queue entry, no push,
worktree bit-identical**. Clean landable case: prints effective config,
resolved base SHA, the commits that would land, the write-set surfaces that
would regenerate, and the gate commands; exit 0. Blocked cases (dirty, held
lock, would-fail-gate context) report what WOULD block; exit 3 (documented).

## 3. Behavior → component matrix

| Behaviors | Owner |
|---|---|
| B1, B3, B4 (end-to-end) | pipeline M2→M8 composed |
| B14, B19–B25 | M2 (+M1 config, M11 dry-run) |
| B5–B8, B26–B34 | M4 (+M3 identity/audit) |
| B10, B30, B35–B38 | M5 |
| B9, B15, B39–B41, B60, B66 | M6 |
| B2, B11, B16, B42–B48, B64 | M7 |
| B13, B49–B52¹ | M8 gate |
| B12, B53–B56 | M8 push |
| B18, B57–B61, B70 | M9 (+M4 stale reclaim) |
| B17, B62, B63 | M10 + D3 server fixture hook |
| B65 | M2 + M7 pathspec discipline |
| B67–B72 | M1 + M3 (+M11) |
| B73 | D2 + D4 + existing runner |

¹ B52 (fresh-clone post-land verification) is **suite-owned** — already
implemented in `helpers.bash::fresh_clone*`; land.sh's only obligation is that
a fresh clone of the pushed main passes the gate, which M7/M8 guarantee by
construction.

## 4. Pinned "one documented behavior" register (all printed in `--help`)

| Scenario fork | Pin |
|---|---|
| B19 (d)/(e) untracked-outside / ignored-inside | proceed with warning |
| B31 heartbeat write failure | holder self-aborts and releases |
| B38 merge-commit branch | flattened by linearizing rebase |
| B62 naked clone | direct push blocked by the server-side fixture guard (sandbox); `--install` manages the client hook |
| B66 self-modifying branch | refuse: `self-modifying land — land manually with review` |
| B71 blocked dry-run exit code | 3 |
| B34 `--status` on corrupt/unreadable | exit 20 |
| B9 derived-surface strategy | auto-resolve-to-base-then-regen (invariants per scenario hold under it) |

## 5. Key cross-cutting invariants (enforced by construction)

1. **Nothing before preflight, nothing after release.** No audit entry, ref
   change, or lock mutation in any refused/usage/help path (B14, B19–B24).
2. **Single push primitive.** Exactly one `git push` call site, refspec
   `HEAD:refs/heads/main`, in M8b — force flags cannot exist anywhere in the
   trace (B12, B54, B55).
3. **The manifest is the only surface authority.** M2 collision check, M6
   conflict partition, M7 regen/commit/verifiers, M9 cleanup all read
   `regen-manifest.txt`; no path literals in land.sh (B42).
4. **`:!_beads` on every add.** (B65)
5. **Trap-guaranteed lock release or marker.** Every exit path is covered by
   one EXIT/INT/TERM trap dispatcher keyed on current phase (B18, B32, B33).

## 6. Harness renegotiation (D3 — the only helpers.bash change)

Per the Phase-2 index ("the spec phase may renegotiate a pin by editing
helpers — never by weakening a scenario"):

- `make_bare_remote`/`seed_fixture`: after the seed push, install a
  **pre-receive hook in the fixture bare remote** (the "server-side fixture
  hook" variant B62 explicitly names) that rejects any update of
  `refs/heads/main` with `"use scripts/land.sh"` on stderr unless the pusher's
  `LAND_PUSH_NONCE` env (inherited — local-path transport) matches the live
  holder nonce in `$LAND_LOCK_DIR/lock.json`. This makes B62's
  `direct_push_blocked=1` branch deterministic, satisfies B17 on every lane
  clone, and gives B63's replay a mechanical rejection while B1-style lands
  (no explicit `--install`) still pass.
- Contract header: add `LAND_PUSH_NONCE` as the pinned push-authorization
  marker name.

No scenario text changes; B-assertions are untouched.

## 7. Build order (input to Phase 4 beads — acceptance-gated slices)

1. **Skeleton + M1 + M2** → greens B14, B19–B25 (minus lock interplay), B24.
2. **M3 + M4 lock/queue/status** → B5–B8, B26–B34.
3. **M5 + M6 rebase core** → B1 (minus regen), B10, B15, B35–B41.
4. **M7 regen + verifiers** → B2, B9, B11, B16, B42–B48, B64, B65.
5. **M8 gate + push** → B3, B4, B12, B13, B49–B56.
6. **M9 recovery** → B18, B32 (full), B57–B61, B70.
7. **M10 guard + D3** → B17, B62, B63, B66.
8. **Observability polish (M1/M3/M11)** → B67–B72.
9. **D2 + D4** → B73; run `run-acceptance.sh` double-pass to closure.

Each slice's done = its listed scenarios green AND no previously-green
scenario regressed (`bats docs/plans/bdd-foundry/acceptance-tests`).
