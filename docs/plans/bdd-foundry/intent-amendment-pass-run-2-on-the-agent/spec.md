# land.sh amendment pass (run 2) — Phase 3 SPEC (bdd-foundry)

> **Derivation rule:** this spec exists ONLY to turn the 21 red acceptance tests
> green (`acceptance-tests/*.bats`, executable definitions of scenarios
> B74–B94 in `behaviors.md`; B94 = the drift-guard split of B85's
> rollout-evidence concern). Red-run ground truth: 21 red on assertion, 0
> already-green, 0 harness-broken. Anything not needed by a B-scenario is out
> of scope. The observable contract (verifier paths, marker strings, JSON keys,
> fault seams, manifest shapes) is already pinned in
> [`acceptance-tests/helpers2.bash`](acceptance-tests/helpers2.bash) — this
> spec adds NO new pins to that file and changes no scenario. The run-1 spec
> (`../spec.md`) remains authoritative for land.sh's M1–M11 internals; §9 below
> is the sanctioned set of amendments to it (B84 requires them stated before
> implementation begins — they are appended at spec-freeze, not by a bead).

Path shorthand: `<run2>` = `docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent`,
`<base>` = `docs/plans/bdd-foundry/acceptance-tests` (the run-1 suite dir).
"Main checkout" = `/Users/bo/dev/agentops`; all `br` writes happen there
(`BEADS_DIR=$BR_MAIN_CHECKOUT/_beads`), never from a worktree.

---

## 1. Component inventory (what exists when this run is green)

| # | Component | New/Edit | Satisfies |
|---|---|---|---|
| C1 | `<base>/helpers.bash` + `<base>/run-acceptance.sh` — tracked `scripts/gate.d/.gitkeep` in `seed_fixture` + the literal self-check `fixture defect: scripts/gate.d untracked` | edit | B74 |
| C2 | `<base>/audit-red.sh` — standing red-on-assertion gate over the base suite | new | B75, B76 |
| C3 | `<base>/08-crash-recovery.bats` — B57 conditional rewritten with materially distinct branches | edit | B76 |
| C4 | `<base>/helpers.bash` `land()/start_land()/status_json()` honor `LAND_BIN`; all 28 direct `scripts/land.sh` invocations in run-1 `.bats` files rerouted through the helpers | edit | B88 |
| C5 | `scripts/land.sh` — install engine v2 (chain/refuse/atomic/verify), `--hook-pre-push` guard dispatch, `--check-counts`, `--gate-families`, lock-dir default + origin canonicalization, `--help` additions | edit (run-1 D1) | B79, B80, B82–B85, B88, B89, B93 |
| C6 | `scripts/regen-manifest.txt` + `scripts/check-regen-manifest.sh` | new | B78 |
| C7 | `scripts/count-docs.txt` + marker blocks in the ~11 prose docs + counts generator wired into `scripts/regen-all.sh` | new/edit | B79, B80 |
| C8 | `.github/workflows/validate.yml` `land-gate-families` declaration + `scripts/check-gate-parity.sh` | edit/new | B81 |
| C9 | `scripts/check-doctrine-docs.sh` + doctrine edits to CLAUDE.md and the pinned sister docs | new/edit | B86 |
| C10 | `<run2>/rollout-evidence.jsonl` + `scripts/check-rollout-evidence.sh` + the two-clone install/verify run | new | B85 (checker script), B94 (checked-in evidence; split from B85 at drift-guard repair) |
| C11 | `scripts/sweep-bead-acceptance.sh` + normalization of every landing-redesign bead body | new + tracker edits | B90 |
| C12 | `scripts/with-hermetic-check.sh` | new | B92 |
| C13 | `<run2>/coverage-manifest.txt` + `<run2>/acceptance-tests/check-coverage-manifest.sh` | new | B91 |
| C14 | Tracker edit: `ag-d3-fixture-guard-yk7rq` ACCEPTANCE | br edit | B77 |
| C15 | Tracker edit: `ag-arpk` disposition | br edit | B87 |
| C16 | Run-1 `../spec.md` amendments (§9 — applied at freeze) | edit | B84, B85, B88 |

Cutover note: B79 asserts `-x scripts/land.sh` in the REAL repo — the run-1 D1
artifact lands on the real repo as part of this run's cutover lane, not only in
the fixture.

---

## 2. §A Harness repair — C1, C2, C3 (B74–B76)

**C1 (B74).** `seed_fixture` writes `scripts/gate.d/.gitkeep` before its
`git add -A` commit, so every lane clone carries the directory (git cannot
track an empty dir — root cause of the 10-test redirect-crash class). A
self-check in `<base>/run-acceptance.sh` (and/or `helpers.bash`) runs after
seeding: if `git -C "$SEED" ls-files -- scripts/gate.d/` is empty, fail the
run with the exact string `fixture defect: scripts/gate.d untracked` (B74
greps both files for the literal).

**C2 (B75) — `audit-red.sh`.** Executable, lives in `<base>`, sources
`helpers2.bash` (relative: `../intent-amendment-pass-run-2-on-the-agent/acceptance-tests/helpers2.bash`)
for the shared lints. Algorithm:

1. Force the SUT red: export `LAND_BIN` to the Phase-2 red placeholder
   (exit 97) and run the base suite entry point, capturing TAP + traces.
2. Derive EXPECTED count — never hardcoded: `bats --count <base>` cross-checked
   against a floor of the base behaviors map (B1–B73) **plus** the count of
   `bats:`-kind entries in the B91 `coverage-manifest.txt` whose mapped file
   lives in `<base>` (the script greps the manifest by path — the literal
   `coverage-manifest` appears in the script; the literals `-eq 73`/`== 73` do
   not).
3. Assert: TAP plan `1..EXPECTED`, zero `ok`, `not ok` == EXPECTED.
4. Assert zero harness crashes: no `No such file or directory` attributable to
   `setup`/`seed_fixture`/`new_lane` frames in the captured output.
5. Classify every failure trace: bats's `(in test file <f>, line <n>)` must
   point INTO the `@test` body of a `.bats` file (never `helpers.bash`, never
   a `setup()` block) — the ten previously-poisoned tests (B13, B33, B49, B50,
   B52, B59, B60, B67, B70, B71) are checked by name.
6. Run `find_identical_if_else` over `<base>/*.bats`; any hit fails.

Exit 0 iff all hold; nonzero naming every test whose failure is a crash (and
every identical-branch if/else). This is the standing gate B75 demands, run
forever after — at green it still passes because `LAND_BIN` forces red.

**C3 (B76).** Rewrite the B57 test so the two arms differ materially, exactly
as 12-amend-harness.bats now asserts:
- post-push crash phases (`push`, `pre-release`): rerun exits 0, output matches
  `already landed`, remote patch-id set unchanged;
- pre-push crash phases (`rebase`, `regen-write`, `regen-commit`, `gate`):
  rerun exits 0, the lane's skill lands exactly once (no duplicate patch-ids),
  `already landed` not required.
No land.sh design change: already-landed detection (patch-id of the lane's
commits present on `origin/main` → exit 0 + `already landed`) is run-1 M8b
behavior; B76 repairs the TEST, the audit-red lint keeps dead conditionals
extinct.

---

## 3. §B Bead repair — C14 (B77)

Amend `ag-d3-fixture-guard-yk7rq` in br from the main checkout. Its
`## ACCEPTANCE` section becomes exactly one runnable done-criterion, in
backticks so the test can extract it:

```
`bats docs/plans/bdd-foundry/acceptance-tests -f '^B62'`
```

plus any required env, copy-paste runnable from the repo root. Remove the
`^B25` smoke proxy; remove every manual operative verb (`manually`, `by hand`,
`inspect`, `eyeball`). No change to the B62 test itself is needed — the run-1
B62 test already exercises the fixture guard (push without `LAND_PUSH_NONCE`
rejected by the bare remote's pre-receive hook; push with it accepted), which
is what 13-amend-bead.bats verifies structurally.

---

## 4. §C Cutover: real-repo migration — C6, C7, C8 (B78–B81)

All verifier scripts are committed + executable (they must exist inside
`real_repo_clone()` clones). Every mutating step they perform targets a
disposable clone/scratch copy (B92).

**C6 (B78) — `scripts/regen-manifest.txt` + `scripts/check-regen-manifest.sh`.**
- Manifest: the FULLY-GENERATED write set of `scripts/regen-all.sh` — one
  normalized repo-relative path per line; `#` comments; optional single-dir
  globs (`<dir>/*`) only. Marker-block partial updates to prose docs are NOT
  declared here — they are owned by `count-docs.txt` (C7).
- Checker (`[--repo <dir>] [--manifest <file>]`), all failures exit nonzero
  **naming the offending line/path**:
  1. *Format*: reject duplicates, `./`/`../` segments, absolute paths, any
     glob broader than one directory, any entry (or glob match) that also
     matches the pinned source-owned set embedded in the script header
     (`CLAUDE.md`, `AGENTS*.md`, `README.md`, `skills/**/SKILL.md`, `cli/**`,
     `docs/plans/**`) — the B42 overlap rule on the real manifest.
  2. *Reality, derived from an ACTUAL regen run, never from re-reading the
     manifest*: clone the repo's HEAD to mktemp; **pass 1** — run
     `regen-all.sh` on the untouched clone and require
     `git status --porcelain` ⊆ manifest (every written path declared:
     underdeclared drift named); **pass 2** — `git rm` every declared path,
     commit, rerun `regen-all.sh`, and require the recreated set == declared
     set (a declared path NOT recreated = overdeclared/stale, named — this is
     what catches the bogus-added-line and what makes a deleted line fail via
     pass 1/2's recreated-but-undeclared path).
- Migration bead: make `regen-all.sh` tolerate regenerating from a tree where
  its outputs are absent (pass-2 requirement).

**C7 (B79, B80) — `scripts/count-docs.txt` + markers + generator.**
- `count-docs.txt`: the ~11 prose docs carrying a generated skill count, same
  strict format rules as C6 (normalized, no dups, violation named by line).
- Every skill-count occurrence in those docs is converted to
  `<!-- count:skills -->N<!-- /count -->` (generator-owned). The generator is
  the existing `scripts/sync-skill-counts.sh`, made marker-driven and invoked
  from `regen-all.sh` (so B80's edit-then-regen byte-restoration holds, and
  `regen-all.sh --check` passes on the cutover commit).
- `scripts/land.sh --check-counts` (C5 surface): (1) validate `count-docs.txt`
  format, naming the line; (2) for each listed doc, recompute the count and
  compare every marker-block value; (3) repo-wide sweep — over
  `git ls-files -co --exclude-standard '*.md'` (tracked AND untracked, so
  B79's planted rogue doc is caught) — for the pattern
  `\b[0-9]+\+?\s+skills?\b` OUTSIDE marker blocks; any hit in a doc not
  manifested (or outside markers in one that is) exits nonzero naming doc and
  line. Migration bead: fix or manifest every existing out-of-marker count
  literal repo-wide so the sweep is 0 on the cutover commit.

**C8 (B81) — validate.yml declaration + `scripts/check-gate-parity.sh`.**
- Declaration: exactly one workflow-level `env:` key —
  `land-gate-families: "<space-separated family tokens>"` — in
  `.github/workflows/validate.yml`. Each token must equal an actual job id (or
  a step `name:`) in the workflow.
- Checker (`[--repo <dir>] [--workflow <file>]`), STRUCTURAL parse via
  `python3` + PyYAML (verified available; never grep): rejects, naming the
  defect — a commented-out declaration (key absent from the parsed tree),
  duplicate declarations (raw two-pass key scan of parsed mappings),
  an empty family list, a family token mapping to no job/step, and any
  declared family missing from land.sh's list. land.sh's list is read via the
  new `scripts/land.sh --gate-families` verb (C5; prints one family per line —
  the run-1 B49 family source exposed as a verb); parity = land.sh families ⊇
  declared families, removal of one token from a working copy fails naming it.
- **Bead constraint (recorded per B81/B90):** the bead landing this edit MUST
  coordinate with or sequence after the active lane holding `validate.yml`
  (am reservation / dependency edge) — stated in the bead body.

---

## 5. §D Install engine v2 + doctrine — C5, C9, C10 (B82–B86, B93)

### 5.1 Segment model (C5)

`.git/hooks/pre-push` is parsed as ordered segments:
- *beads segment*: `# BEGIN BEADS INTEGRATION` … `# END BEADS INTEGRATION`
  (version-suffix tolerated) — never touched, byte-preserved.
- *local dispatch*: any `pre-push.local` invocation — never touched;
  `pre-push.local`'s file is never opened.
- *guard segment*: `# BEGIN LAND GUARD v<semver>` … `# END LAND GUARD` —
  the ONLY bytes `--install` owns. Guard version == land.sh `--version`.

Recognition classes: **hookless** (no pre-push) → install a fresh hook
(shebang + guard segment). **Recognized** (has beads markers and/or guard
markers) → append/replace the guard segment at the END of the hook (documented
order: beads segment → pre-push.local cockpit gate → land guard; printed by
`--help`). **Foreign** (file exists — executable or not — with neither
marker set) → REFUSE: exit nonzero, message contains `not recognized` and
`chain manually`, zero byte changes, no guard text written anywhere under
`.git/hooks/`. The refusal is unconditional across control-flow traps
(`exit 0` head, trailing `exec`, no shebang, non-executable) — no variant is
chained, overwritten, or "fixed". Policy pinned here and in `--help` (and in
run-1 spec §9 — B84's freeze requirement).

### 5.2 Guard dispatch through the seam (C5, B82/B88)

The guard segment never hardcodes the SUT path as its decision-maker. Its
body is:

```sh
"${LAND_BIN:-"$(git rev-parse --show-toplevel)/scripts/land.sh"}" --hook-pre-push "$@" || exit $?
```

New internal verb `--hook-pre-push`: reads the standard pre-push stdin/args;
permits the push iff a live land lock + valid `LAND_PUSH_NONCE` is present
(B63 semantics); otherwise prints the B17 rejection containing
`use scripts/land.sh` and exits nonzero. With `LAND_BIN` pointed at a probe
stub, the installed chain consults the probe (B88's dispatch test); with it
unset, the default is the repo's `scripts/land.sh` (the documented
compatibility shim for an `ao land` substrate).

### 5.3 Idempotency, upgrade, crash-safety (C5, B83/B93)

- Rerun with current guard present and version == self: exit 0,
  `already installed`, hook byte-identical, backup untouched.
- Guard present but older version: rewrite ONLY the guard segment bytes, log
  `guard upgraded <old> -> <new>`, exit 0; beads segment + `pre-push.local`
  sha256-identical.
- Every write path: compose the full new hook → write to
  `pre-push.tmp.$$` in the hooks dir → `chmod +x` temp → back up the prior
  hook to `.git/hooks/pre-push.pre-land-install.bak` (only when content will
  change; idempotent reruns never touch it) → atomic `mv` over `pre-push`.
  Never an in-place edit.
- Fault seams (active only under `LAND_TEST_MODE=1`):
  `LAND_TEST_INSTALL_FAIL={write,rename,chmod}` aborts AT that step (write =
  temp abandoned before rename). Any failure (injected or real, incl.
  read-only hooks dir): exit nonzero with a structured error naming the failed
  step, surviving hook byte-identical, no `pre-push*.tmp*` wreckage left.
  `--help` documents the backup path and the recovery step.

### 5.4 `--install --verify` (C5, B85)

Machine-readable, deterministic (two consecutive runs byte-identical), JSON
keys exactly `guard_present`, `guard_version`, `chain`, `defects`; keys and
exit codes documented in `--help`. Exit 0 iff guard present and `defects` is
empty. Detected defects, one distinct token each (helpers2 pin): stale guard
version, duplicate guard segment, unpaired marker, missing executable bit,
chain order (guard not in the documented last position). Naked clone:
`guard_present:false`, nonzero, names what is missing. Resolution is pure
(no lock/file creation). `--help` additionally carries the pinned residual
statement: the landing lock is **host-local** — `LAND_LOCK_DIR` cannot
serialize a Mac land against a bushido land; cross-host serialization is out
of scope for land.sh v1 and owned by the **ag-arpk** disposition.

### 5.5 Rollout evidence (C10, B94 — split from B85 at drift-guard repair)

After the cutover lands: on BOTH live clones (Mac main checkout, bushido
checkout — via `ssh bushido`) run `scripts/land.sh --install` then
`--install --verify`, and append one JSON record each to
`<run2>/rollout-evidence.jsonl`: keys `host`, `repo_sha`, `guard_version`,
`command`, `timestamp` (ISO-8601), `verify` (raw JSON). Checked in.
`scripts/check-rollout-evidence.sh [--manifest <file>]`: requires ≥2 records
with all keys; staleness rule — each record's `repo_sha` must be a commit
known to the repo whose `scripts/land.sh` blob equals HEAD's
(`git rev-parse <sha>:scripts/land.sh` == `git rev-parse HEAD:scripts/land.sh`)
and each `guard_version` must equal the working tree land.sh's version;
violation → nonzero (stale evidence rejected, not grandfathered — the zeroed
SHA fixture fails the known-commit check).

### 5.6 Doctrine flip (C9, B86)

Doc edits (cutover commit):
- CLAUDE.md Phases step 4 (`^4\. \*\*Land\.`) and the Branch+PR-shape `| Land |`
  row both instruct `scripts/land.sh`; the old operative `Push to \`main\``
  phrasing leaves those lines.
- CLAUDE.md's pre-push hook description names the chained guard (beads
  segment + cockpit gate / `pre-push.local` + land guard) matching §5.1.
- Sister docs (`AGENTS.md`, `AGENTS-WORKFLOW.md`, `AGENTS-CI.md`,
  `AGENTS-CODEX.md`, `AGENTS-RUNTIME.md`, `README.md`,
  `docs/agent-workflow-reference.md`): every LIVE landing instruction names
  `scripts/land.sh`; direct-push language survives only inside explicitly
  marked historical sections.
- `scripts/check-doctrine-docs.sh [--repo <dir>]`: sweeps exactly the pinned
  doc list above (each name appears literally in the script), greps for the
  operative phrases (`Push to \`main\`` as an instruction,
  `rebase-on-reject (git serializes`), exits nonzero naming any doc with a
  live hit. Marker convention, documented in the script header (the words
  `historical`/`superseded` appear in the first 40 lines): text between
  `<!-- doctrine:historical -->` and `<!-- /doctrine:historical -->` (or under
  a heading containing `Historical` / `(superseded)`) is exempt.

---

## 6. §E ag-arpk disposition — C15 (B87)

Tracker edit from the main checkout. Chosen path (pinned now, not late-bound):
**keep merge-queue planned** — ag-arpk stays the named cross-host serializer,
sequenced AFTER the land.sh epic via a dependency edge (`br dep add ag-arpk
<land.sh epic id>`), so `br ready --limit 0` and bv triage show it blocked,
never unclaimed-ready. Body text must state: deferral + re-evaluation trigger
(land.sh cutover verified on both clones); the exact residual — cross-host
(Mac ↔ bushido) landing serialization is NOT provided by land.sh's host-local
lock; GitHub merge queue remains the only listed serializer for that gap; and
the residual-handling choice above (reader can tell what is and is not
protected after cutover). Machine state must agree with prose (status/labels/
deps), per the 17-amend-arpk.bats greps (`defer|supersed`,
`cross-host|host-local`, `land.sh`, `merge.?queue`, blocked-edge visibility).

---

## 7. §F Seam, lock default, bead hygiene — C4, C5, C11 (B88–B90)

**C4 (B88).** Run-1 `helpers.bash`: `land()`, `start_land()`, `status_json()`
invoke `"${LAND_BIN:-$lane/scripts/land.sh}"`. Reroute all 28 direct
invocations found by `find_direct_sut_invocations` in run-1 `.bats` files
through the helpers (fixture-authoring heredocs exempt by the scanner). The
installed-hook half of the seam is §5.2. The substrate note is §9.3.

**C5 (B89) — lock-dir default.** With no `LAND_LOCK_DIR`/`--lock-dir`:

```
lock_dir = ${XDG_STATE_HOME:-$HOME/.local/state}/land/<digest>
digest   = sha256 of the CANONICALIZED origin identity (first 16 hex)
```

Canonicalization (pinned, printed in `--help` with the formula and the word
`canonical`): parse `git remote get-url origin`; scp form `user@host:path` ≡
`ssh://user@host/path`; strip scheme, credentials, and default ports;
lowercase host; strip leading `/`, trailing `/` and trailing `.git` from the
path; identity = `host/path`. Local remotes: `file:///abs/path` ≡ `/abs/path`
→ identity `local/<abs-path>`. Same identity ⇒ same lock_dir across spelling
variants (the four GitHub forms digest identically); different host or repo
path ⇒ different lock_dir. `--status` resolution stays pure (B34) and reports
`lock_dir` in its JSON. Mutual exclusion flows through the default unchanged
(run-1 M4 lock protocol + `audit.jsonl` in the lock dir).

**C11 (B90) — `scripts/sweep-bead-acceptance.sh`** (env `BR_MAIN_CHECKOUT`,
`BR_BEADS_DIR`; optional explicit bead-id args):
1. *Fail closed first*: `br` on PATH, main checkout `.git` present, ledger dir
   present, every bead id resolvable via live `br show` — any miss exits
   nonzero naming the missing tool/path/id; never a skip, never cached prose.
2. Bead set: ids from the run-1 + run-2 beads manifests (the 16 existing +
   this run's new ones), or the explicit args.
3. Per bead: extract every command from ACCEPTANCE/regression sections;
   reject (naming bead + phrase) any shorthand criterion (`stays green`,
   `still pass`, `previous filters`, …) without an adjacent explicit command.
4. EXECUTE each extracted command from a clean repo root with only the
   declared env; fail naming bead + command on: zero selected tests (TAP plan
   `1..0`/`0 tests`), stale/missing path, or harness death (the audit-red
   trace classifier, shared, distinguishes crash from assertion). Passing
   outcomes per command: red-on-assertion or green only.
- Tracker edit rider: normalize every landing-redesign bead body so each
  criterion carries the full runnable command (bats path + filter + env).

---

## 8. §G Meta-gates — C12, C13 (B91, B92)

**C13 (B91).** `<run2>/coverage-manifest.txt`: one line per appended behavior,
`B<n> <kind>:<ref>`, kind ∈ {bats, script, cmd}; bats ref =
`<file>#<id>` within `<run2>/acceptance-tests`; every id B74–B94 exactly once.
`<run2>/acceptance-tests/check-coverage-manifest.sh [--manifest <file>]`,
exits nonzero naming the defect: missing/duplicate behavior id; `bats:` entry
whose file the run-2 entry point does not select (file not in the suite dir or
no `@test "B<n>:` in it); `script:`/`cmd:` entry whose named script does not
exist or is not executable. Under the red placeholder every mapped bats test
fails red-on-assertion — already enforced by C2 + the run-2 entry point.

**C12 (B92).** `scripts/with-hermetic-check.sh <cmd...>`: capture
`git status --porcelain` + `git rev-parse HEAD` of `$PWD`'s repo before; run
the command; capture after; on any delta exit nonzero printing
`verifier residue` and every leftover path / the SHA mismatch; otherwise
propagate the wrapped command's exit status (a clean failing command stays a
plain failure — no residue claim). The wrapper records pre/post itself, so a
mid-run crash cannot hide residue behind a forgotten restore. Mutating cutover
verifiers (C6–C9) remain hermetic *by construction* (disposable clones /
scratch `--manifest`/`--workflow` copies) — the wrapper is the belt over those
suspenders and the test's probe surface.

---

## 9. Amendments to the run-1 spec (`../spec.md`) — C16, applied at freeze

Appended verbatim as a "Run-2 amendments" section (the run-2 tests grep that
file for these):

1. **B84 foreign-hook policy (pinned at freeze):** `--install` installs on
   hookless clones; on an existing pre-push that is **not recognized** (no
   beads markers, no guard markers) it REFUSES — exit nonzero, message
   "existing pre-push not recognized — chain manually" — and never modifies
   the foreign hook.
2. **B85 residual:** the landing lock is **host-local**; cross-host
   (Mac ↔ bushido) serialization is out of scope for land.sh v1 and owned by
   the **ag-arpk** disposition (merge queue kept-planned, sequenced after the
   land.sh epic).
3. **B88 implementation — DECIDED: Go (`ao land`), not bash (operator call,
   2026-06-13).** This supersedes the run-1 D1 line ("single self-contained bash
   file") and the earlier "permitted/preferred-if-cheaper" framing. land.sh ships
   as a subcommand of the existing `ao` Go CLI, NOT a 2k-line bash script.
   Rationale: types, real unit-testability, and first-class concurrency
   primitives (goroutines + channels for the lock/queue/heartbeat) — the lock,
   regen, and gate are concurrency-critical and bash is the riskiest substrate
   for that. The **acceptance suite is unchanged and remains the contract**: the
   Go binary must pass the identical bats suite via the **LAND_BIN** seam (the
   tests shell out to `$LAND_BIN`; point it at the `ao land` binary). The D1
   bash artifact is retired; D2/D3/D4 (runner delegator, helpers guard, coverage
   tags) stand. Installed guard segments dispatch through the configured land
   command (§5.2), never a hardcoded path.

   *Mechanical consequence for the beads:* the engine beads (m1–m11) now target
   `cli/cmd/ao/land*.go` + `cli/internal/land/` instead of `scripts/land.sh`;
   their bats acceptance is unchanged (LAND_BIN seam). The bash-portability
   bound in §1 (no flock/setsid/date+%N) NO LONGER APPLIES — Go gets real
   `flock`/file-locking, process groups, and monotonic clocks. Re-scope the
   engine beads to the Go surface before cranking.

### 📌 PIN — resolve BEFORE any engine bead is cranked (operator-flagged 2026-06-13)

**Does `ao land` become a long-lived daemon, not a one-shot command?** The
operator raised that a Go implementation invites a daemon shape — a resident
process owning the lock/queue and serializing lands across all in-process
lanes via channels, rather than a fresh `ao land` invocation per land
contending a file-lock. Possibly even a "refiner" that does more than serialize
(batches, pre-warms gates, reorders the queue). **This is explicitly NOT decided
and NOT in scope to design now** — it is parked as the first thing to work
through before the M4 lock/queue bead is built, because daemon-vs-oneshot
changes the lock mechanism, the crash-recovery model (M9), the `--status`
surface (M4), and whether cross-host (the ag-arpk residual) folds in here after
all. Do not crank the engine until this fork is settled; the acceptance suite is
substrate-agnostic so it survives either answer, but the spec's M3/M4/M9
internals do not. **One-shot file-lock is the current assumed baseline; the
daemon question is the gate.**

---

## 10. Behavior → component matrix

| Behavior | Components | Behavior | Components |
|---|---|---|---|
| B74 | C1 | B84 | C5 §5.1, C16 |
| B75 | C2, C13 | B85 | C5 §5.4, C10, C16 |
| B76 | C2, C3 | B86 | C9 |
| B77 | C14 | B87 | C15 |
| B78 | C6 | B88 | C4, C5 §5.2, C16 |
| B79 | C5, C7 | B89 | C5 §7 |
| B80 | C7 | B90 | C11 + tracker edits |
| B81 | C8 | B91 | C13 |
| B82 | C5 §5.1–5.2 | B92 | C12 |
| B83 | C5 §5.3 | B93 | C5 §5.3 |
| | | B94 | C10 (split from B85) |

## 11. Sequencing constraints (bead-graph inputs, not test assertions)

1. C16 (run-1 spec amendments) and C13's manifest exist before any
   implementation bead closes — B84/B91 gate everything else.
2. C2 depends on C1 and C13 (audit-red reads the coverage manifest; the base
   suite must be crash-free to classify).
3. C8's validate.yml edit must coordinate with / sequence after the active
   lane holding `validate.yml` (am reservation; recorded in the bead body).
4. C10's evidence run happens only after the cutover (C5–C9) is on the real
   repo's `main` and both clones are pulled; it is the rollout criterion, the
   last lane.
5. All tracker writes (C14, C15, C11's normalization) run from the main
   checkout `/Users/bo/dev/agentops` with `BEADS_DIR=$PWD/_beads` — never from
   a worktree.
