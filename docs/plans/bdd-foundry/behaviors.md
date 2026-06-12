# Landing pipeline (land.sh) — Phase 1 BEHAVIORS — **FROZEN**

> **FROZEN definition of done (2026-06-12).** This is the complete behavior
> contract for the greenfield merge/landing redesign after folding the
> independent cross-family gap review (`behaviors-codex-gaps.md`, 100 gaps:
> 94 folded, 3 rejected with reasons, 3 absorbed as standing conventions —
> full disposition table at the bottom). Every scenario below must become a
> runnable acceptance test before a bead exists (no runnable test, no bead).
> **No scenario may be added, removed, or weakened without re-opening Phase 1.**
>
> **Evidence base (2026-06-12 session):** push-to-main serializes on ~8 committed
> derived surfaces (`registry.json`, `docs/contracts/context-map.md`, catalogs,
> SKILL-TIERS, codex twins + `.agentops-generated.json` hash manifests, domain
> map, skill counts hand-asserted in 11 prose docs); 6 rebase cycles lost in one
> night to lane races; git line-merge splices duplicate `generated_hash` keys
> into hash-marker JSON on every rebase; the gate (`contracts-sync` in
> `validate.yml`, `regen-all.sh --check` locally) is fail-fast at 3–5 min/cycle;
> doctrine names am build-slots but nothing enforces them.
> Full plan context: `~/.claude/usage-data/agentops-seams-plan-FINAL-2026-06-12.md`.
>
> **Vocabulary:** *lane* = one agent worktree+branch; *landing lock* = the
> mechanical mutual-exclusion primitive (file/ref + owner + heartbeat + TTL);
> *derived surface* = any file a generator owns (the regen-all.sh write set,
> declared in the regen manifest — see B42); *gate* = the full local check
> battery run by land.sh before push; *marker* = the on-disk "land in progress"
> recovery token that `--abort` clears.
>
> **Harness assumptions (standing conventions, apply to every scenario):**
> - All scenarios run in a **sandbox clone** (bats + a fixture repo with a bare
>   sandbox-marked remote — see B25/B73); none push to the real origin/main.
>   "main" below = the sandbox main.
> - **"origin/main is unchanged" convention (gap 67):** every such assertion
>   records the remote ref's SHA *in the fixture bare remote* before the run and
>   compares after. If a fetch legitimately updated the local remote-tracking
>   ref, the remote branch itself must still be unchanged and the distinction
>   logged. Local `refs/remotes/origin/main` alone never satisfies the check.
> - Scenario kinds: **happy** = the designed path; **edge** = unusual-but-legal
>   inputs/timing; **error** = the tool must refuse/abort cleanly.

---

## Feature: one-command landing for a hot multi-lane repo

```gherkin
Feature: land.sh — one-command, serialized, regen-at-land landing
  As an agent lane on a hot repo with concurrent lanes
  I want a single command that takes my branch from "done" to "on main"
  So that landing never costs rebase loops, hand-merged generated files,
  or one-failure-at-a-time gate cycles
```

---

## §1 Core landing (B1–B18, original set; B2 and B9 strengthened at freeze)

### Happy paths

```gherkin
Scenario: B1 — single lane lands with one command and one rebase attempt
  Given a worktree on branch "feat-x" with a clean working tree
    And "feat-x" is 2 commits ahead of origin/main and 0 behind
    And no other process holds the landing lock
  When the lane runs "scripts/land.sh"
  Then the exit code is 0
    And both commits from "feat-x" are reachable from origin/main
    And "scripts/regen-all.sh --check" run on a fresh checkout of main exits 0
    And the land.sh log contains exactly one line matching "rebase attempts: 1"
    And the landing lock is free (land.sh --status reports "unheld")
```

```gherkin
Scenario: B2 — regen-at-land: author never hand-runs the 8 generators
  Given branch "feat-skill" adds "skills/zz-newskill/SKILL.md" with valid frontmatter
    And the author has NOT regenerated any derived surface
      (registry.json, context-map, domain map, SKILL-TIERS, codex twin hashes,
       skill counts are all stale relative to the new skill)
  When the lane runs "scripts/land.sh"
  Then the exit code is 0
    And on main, "scripts/regen-all.sh --check" exits 0
    And on main, "jq -e '.skills[] | select(.name==\"zz-newskill\")' registry.json" exits 0
    And the derived-surface updates are folded into the landing (a commit authored
      by land.sh with subject matching "^chore\(land\): regen derived surfaces"
      or squashed into the lane's commit), never left as uncommitted diff
    And any land.sh-authored regen commit carries a pinned identity:
      author/committer "land.sh <land@local>", a "Landed-by:" trailer naming the
      land.sh version, and a "Land-correlation:" trailer matching the audit
      correlation id — so provenance distinguishes lane-authored code from
      lander-authored generated output (folds gap 61)
```

```gherkin
Scenario: B3 — queued lanes land FIFO with zero manual rebases
  Given lane A is mid-land holding the landing lock
    And lane B then lane C invoke "scripts/land.sh" while A holds the lock
  When lane A's land completes successfully
  Then lane B acquires the lock before lane C (lock log shows acquisition order A,B,C)
    And lane B's land rebases onto the main that includes A's commits automatically
    And neither B nor C performs a manual conflict resolution
      (no land.sh log line matching "manual intervention required")
    And all three lanes exit 0 with their commits on main in landing order
```

```gherkin
Scenario: B4 — three-lane soak: the tonight-failure-mode does not reproduce
  Given three lanes with disjoint changes (each adds a different skill directory)
    And each invokes "scripts/land.sh" within the same 60-second window
  When all three invocations complete
  Then all three exit 0 and main contains all three skill directories
    And the sum of "rebase attempts: N" across the three logs is exactly 3
      (one per lane — i.e. 0 retry loops, vs 6 lost cycles on 2026-06-12)
    And "scripts/regen-all.sh --check" on final main exits 0
    And every hash-marker JSON on main parses ("jq empty" exits 0 for each)
```

```gherkin
Scenario: B5 — lock status is observable for swarm tending
  Given lane A holds the landing lock and lane B is queued
  When any process runs "scripts/land.sh --status --json"
  Then stdout is valid JSON containing fields:
      holder.id (A's lane identity), holder.pid, holder.heartbeat_age_seconds,
      queue (an array whose first element identifies lane B)
    And the command exits 0 without acquiring or modifying the lock
```

### Edge cases

```gherkin
Scenario: B6 — mutual exclusion is mechanical: no two holders, ever
  Given a stress harness launches 10 concurrent "scripts/land.sh" invocations
      against the same sandbox repo (each on its own trivial branch)
  When all invocations complete
  Then the lock audit log shows 10 acquire/release pairs whose hold intervals
      are strictly non-overlapping (assertable by sorting timestamps)
    And no invocation pushed to main while another held the lock
    And main's final commit graph is linear (no merge commits, no lost commits:
      all 10 branch tips reachable from main)
```

```gherkin
Scenario: B7 — stale lock from a dead lane is reclaimed, not waited on forever
  Given the landing lock is held by a holder whose PID no longer exists
    And the holder's heartbeat is older than the stale TTL (config default 15 min)
  When lane B runs "scripts/land.sh"
  Then lane B reclaims the lock within 30 seconds
    And the lock audit log records a "stale-takeover" entry naming the dead
      holder's id, pid, and last heartbeat timestamp
    And lane B's land proceeds to completion with exit 0
```

```gherkin
Scenario: B8 — a live lock is never stolen
  Given the landing lock is held by a live process heartbeating every 30s
  When lane B runs "scripts/land.sh" with a wait timeout of 60 seconds
  Then lane B does NOT remove or overwrite the lock
    And lane B either waits in queue or exits nonzero with a message matching
      "lock held by .* — queued|timed out"
    And the holder's land completes unaffected (exit 0)
```

```gherkin
Scenario: B9 — derived surfaces never present a textual conflict to anyone
  Given branch "feat-a" adds skill "aa-one" and main has advanced with a landed
      skill "bb-two" (so registry.json, context-map, domain map, skill counts,
      and SKILL-TIERS differ on both sides)
  When "scripts/land.sh" rebases "feat-a" onto main
  Then no rebase conflict on any generator-owned path reaches a human or agent
      (land.sh log contains no "CONFLICT" line for paths in the regen write set)
    And land.sh resolves every derived-surface divergence by re-running the
      generators after rebase, not by line-merging
    And on main, each derived surface is byte-identical to a fresh run of its
      generator (sha256 of file == sha256 of generator output)
    And — invariants regardless of the strategy chosen (remove-before-rebase,
      checkout-from-main, merge driver, or reset-after-rebase) — generated paths
      never influence conflict decisions on source paths, and no source-path
      change is lost when generated paths are reset (folds gap 58)
```

```gherkin
Scenario: B10 — re-running land.sh on an already-landed branch is a no-op
  Given branch "feat-x" whose every commit is already reachable from origin/main
  When the lane runs "scripts/land.sh"
  Then the exit code is 0 with output matching "already landed"
    And no new commit appears on main (main SHA unchanged before/after)
    And the lock, if taken, is held for less than 10 seconds
```

```gherkin
Scenario: B11 — counts are never hand-asserted: prose counts are generated or gated
  Given the repo's prose docs that today hand-assert skill counts (the 11-doc set)
  When "scripts/land.sh" lands a branch that adds one skill
  Then every skill-count occurrence in those docs sits inside generator-owned
      marker blocks (e.g. "<!-- count:skills -->...<!-- /count -->") and now
      shows the incremented value
    And a repo-wide sweep for numeric skill-count literals OUTSIDE marker blocks
      (the checker script's own grep) returns 0 matches
    And planting a wrong value INSIDE a marker block is corrected by regen at land
      (the landed doc shows the generator's value, not the planted one)
```

```gherkin
Scenario: B12 — out-of-band push between gate and push is absorbed, never force-pushed
  Given lane A's land has passed the gate and is about to push
    And a commit lands on origin/main out-of-band (bypassing the lock)
  When lane A's push is rejected as non-fast-forward
  Then land.sh performs at most one bounded re-rebase + gate re-run, then either
      lands (exit 0) or aborts (exit nonzero) — log shows "rebase attempts: 2" max
    And land.sh never invokes "git push --force" or "--force-with-lease"
      (asserted by grepping the land.sh execution trace)
    And the audit log records the out-of-band SHA it had to absorb
```

### Error cases

```gherkin
Scenario: B13 — all gate failures reported in ONE pass, not one per cycle
  Given branch "feat-bad" carrying three independent gate violations:
      (1) a context_rel.with target naming a nonexistent skill,
      (2) a codex-twin hash mismatch for an edited skill,
      (3) a doc-table /skill reference to a nonexistent skill
  When the lane runs "scripts/land.sh"
  Then exactly one gate pass executes (one "== gate ==" header in the log)
    And the run continues past the first failure and the final summary lists
      ALL THREE failures, each with: check name, offending file path, and remedy
    And the exit code is nonzero, origin/main is unchanged,
      and the landing lock is released (land.sh --status reports "unheld")
```

```gherkin
Scenario: B14 — dirty working tree is refused before any lock or rebase
  Given a worktree with an uncommitted modification to any tracked file
  When the lane runs "scripts/land.sh"
  Then it exits nonzero within 5 seconds with a message matching
      "working tree dirty — commit or stash"
    And the landing lock was never acquired (lock audit log has no entry)
    And the worktree and branch are bit-identical to before the invocation
      (full dirt taxonomy: B19)
```

```gherkin
Scenario: B15 — a real source conflict aborts cleanly and reports, leaving no wreckage
  Given branch "feat-y" and main both modified the SAME hunk of a hand-authored
      file (skills/crank/SKILL.md), a genuine semantic conflict
  When the lane runs "scripts/land.sh"
  Then land.sh aborts the rebase (no .git/rebase-merge directory remains)
    And the worktree is restored to its pre-land state (git status clean,
      HEAD back on "feat-y" at the original SHA)
    And the output names the conflicting file(s) and exits nonzero
    And origin/main is unchanged and the landing lock is released
```

```gherkin
Scenario: B16 — hash-marker JSON corruption can never land (the duplicate generated_hash class)
  Given a branch whose rebase would, under plain line-merge, splice a duplicate
      "generated_hash" key into a skills-codex/**/.agentops-generated.json
      (fixture reproducing the 2026-06-12 corruption)
  When "scripts/land.sh" runs to completion
  Then on main, every skills-codex/**/.agentops-generated.json and
      skills-codex/.agentops-manifest.json parses as strict JSON
    And each contains exactly one "generated_hash" key
      (jq --stream key-count == 1 for every file)
    And the standalone negative test for the verifier itself lives in B47
```

```gherkin
Scenario: B17 — direct pushes to main outside land.sh are mechanically blocked
  Given the landing discipline is installed in a clone (hooks/branch protection)
  When an agent runs "git push origin main" directly with new commits
  Then the push is rejected and stderr contains a message matching
      "use scripts/land.sh"
    And when land.sh itself pushes (its own invocation marker set), the same
      guard permits the push — i.e. the block is bypassable ONLY via land.sh
      (marker anti-spoofing: B63; install lifecycle: B62)
```

```gherkin
Scenario: B18 — gate failure or crash mid-land never strands the queue
  Given lane A's land is killed with SIGKILL after acquiring the lock but
      before pushing (simulating a crashed agent pane)
  When lane B runs "scripts/land.sh" after A's heartbeat exceeds the stale TTL
  Then lane B reclaims the lock and lands successfully (exit 0)
    And main contains none of lane A's commits (A's partial work never half-landed)
    And A's worktree, when inspected, is recoverable: either pre-land state or a
      single "land in progress" marker that "scripts/land.sh --abort" clears,
      after which A's branch can re-land cleanly (exit 0)
      (full crash matrix: B57; full --abort contract: B58)
```

---

## §2 Preflight & invocation (B19–B25)

```gherkin
Scenario: B19 — every class of dirt is refused or explicitly classified before lock
  Given five worktree variants: (a) staged-only change to a tracked file,
      (b) partially staged tracked file, (c) untracked file whose path collides
      with a generator-owned output, (d) untracked file outside the regen write
      set, (e) ignored file inside the regen write set
  When "scripts/land.sh" runs in each variant
  Then (a) and (b) exit nonzero matching "working tree dirty" before lock acquisition
    And (c) exits nonzero naming the colliding path and the owning generator
    And (d) and (e) follow ONE documented rule (refuse or proceed) printed in
      --help, and the test asserts that exact rule — never implementation accident
    And in every refused variant the lock audit log has no entry and the
      worktree is bit-identical to before
```

```gherkin
Scenario: B20 — root discovery works; broken invocation contexts are refused
  Given a sandbox clone with a linked worktree
  When "scripts/land.sh" is invoked from a SUBDIRECTORY of the worktree
  Then it resolves the repo root deterministically and behaves exactly as from
      the root (same land, same log paths)
  When invoked on a detached HEAD, on "main" itself, in a repo with no "origin"
      remote, with origin lacking a "main" branch, and from a non-worktree directory
  Then each exits nonzero within 5 seconds with a distinct message naming the
      refused context ("detached HEAD", "refusing to land main onto itself",
      "no origin remote", "origin/main not found", "not a git worktree")
    And no lock is acquired in any refused case
```

```gherkin
Scenario: B21 — an in-progress git operation is refused before lock
  Given worktree variants with, respectively: .git/rebase-merge present,
      .git/rebase-apply present, MERGE_HEAD present, CHERRY_PICK_HEAD present
  When "scripts/land.sh" runs in each
  Then each exits nonzero matching "git operation in progress" before lock
      acquisition
    And the in-progress state files are bit-identical afterward (land.sh did not
      "helpfully" abort someone else's operation)
```

```gherkin
Scenario: B22 — missing tooling is reported in ONE preflight summary, no lock taken
  Given a sandbox where "jq" is absent from PATH and "scripts/regen-all.sh" is
      missing or non-executable
  When "scripts/land.sh" runs
  Then it exits nonzero with ONE preflight summary listing BOTH broken
      dependencies by name (not just the first)
    And the lock audit log has no entry and no git ref changed
```

```gherkin
Scenario: B23 — configuration has pinned precedence and validates before lock
  Given stale TTL, wait timeout, remote name, and base branch each set
      differently via CLI flag, environment variable, and repo config
  When "scripts/land.sh --dry-run" reports its effective configuration
  Then precedence is CLI flag > environment > repo config > built-in default,
      proven independently for each knob
    And invalid values ("--wait-timeout=-5", a non-numeric TTL) exit nonzero
      naming the bad knob BEFORE lock acquisition
    And "--wait-timeout=0" has a documented meaning (fail immediately if held)
      and is asserted
    And an unsupported layout (no "origin", base branch absent on the remote)
      is refused as a config error up front, never a mid-land surprise
```

```gherkin
Scenario: B24 — help, version, and usage errors never mutate anything
  Given a sandbox clone in any state
  When "scripts/land.sh --help", "--version", an unknown flag, and an
      incompatible flag combination are each invoked
  Then --help and --version exit 0 printing usage/version (the version string
      matches what audit entries record); unknown/incompatible flags exit
      nonzero with usage
    And in all four cases: no lock entry, no ref change, no worktree change
```

```gherkin
Scenario: B25 — the harness refuses to operate on a non-sandbox remote
  Given a clone whose "origin" does not carry the fixture sandbox marker
      (fixture bare remotes are created with a "land-sandbox" marker)
  When the acceptance suite (or land.sh under its test mode) would push
  Then it refuses before any push, exits nonzero matching
      "refusing non-sandbox remote", and no push connection is attempted
```

---

## §3 Lock & queue (B26–B34)

```gherkin
Scenario: B26 — lock acquisition is atomic: exactly one winner at the same instant
  Given a barrier harness releases 10 acquisition attempts simultaneously
      (processes blocked on a fifo/barrier, released together)
  When all attempts execute
  Then exactly one acquires — via an atomic primitive (mkdir / O_EXCL /
      rename), proven by the audit log showing exactly one "acquire" before
      any "release"
    And the 9 losers queue or exit per their wait policy
    And no partial or temp lock file remains in the lock directory afterward
```

```gherkin
Scenario: B27 — unreadable lock storage fails closed before any mutation
  Given three variants: lock JSON truncated mid-object, lock directory
      read-only, lock path replaced by a dangling symlink
  When "scripts/land.sh" runs in each
  Then each exits nonzero matching "lock state unreadable|lock storage"
      WITHOUT rebasing, regenerating, or pushing (fail closed — a landing
      mutex never guesses)
    And no variant silently deletes the corrupt lock; any cleanup writes a
      "corrupt-lock" audit entry first (when the audit log is writable)
```

```gherkin
Scenario: B28 — lane identity is unique and PID reuse cannot fake liveness
  Given two worktrees in two clones both on a branch named "feat-x"
  When each is mid-land and "--status --json" is read
  Then their holder/queue ids differ and each id resolves to its exact
      worktree path (identity = host + worktree + pid + start-time, not branch
      name alone)
  Given a lock whose recorded PID now belongs to an unrelated live process
      (PID-reuse fixture) and whose heartbeat exceeds the stale TTL
  When lane B runs "scripts/land.sh"
  Then B treats the lock as stale (identity validation + heartbeat age, not
      bare PID liveness) and reclaims it with a "stale-takeover" audit entry
```

```gherkin
Scenario: B29 — queue hygiene: no duplicate entries, no ghosts, no starvation
  Given lane A holds the lock
  When lane B invokes "scripts/land.sh" twice concurrently from one worktree
  Then the queue holds exactly one entry for B's lane identity
  When queued lane C is SIGKILLed while waiting
  Then C's entry is removed or expires and never blocks later lanes
  When lanes keep arriving while lane D waits
  Then D acquires before any lane that enqueued after it — FIFO holds across a
      10-lane arrival storm (no starvation)
```

```gherkin
Scenario: B30 — concurrent lands of the same branch: one land, one clean no-op
  Given two processes run "scripts/land.sh" concurrently for branch "feat-x"
      (variant 1: same worktree; variant 2: two worktrees on the same branch)
  When both complete
  Then exactly one pushed feat-x's commits to main
    And the other exits 0 matching "already landed" or nonzero matching
      "branch is being landed by" — and main contains each patch-id exactly
      once (no duplicate commits, no branch corruption)
```

```gherkin
Scenario: B31 — heartbeat stays fresh through long phases; heartbeat write failure is fail-safe
  Given a gate stubbed to run 3x the heartbeat interval
  When lane A lands and "--status --json" is polled every interval
  Then every poll shows holder.heartbeat_age_seconds < 2x the interval —
      the heartbeat continues through fetch, rebase, regen, gate, and push
  Given the heartbeat file becomes unwritable mid-land while A stays alive
  Then EITHER no waiter takes over while A's identity+process validate as live,
      OR A aborts itself and releases cleanly — one behavior, asserted; never
      a takeover racing a live holder's push
```

```gherkin
Scenario: B32 — SIGINT/SIGTERM at every phase exit cleanly, no wreckage
  Given lands interrupted by SIGINT and by SIGTERM in each phase:
      waiting in queue, holding pre-rebase, during gate, during push
  When each interrupted process exits
  Then a queued waiter leaves no ghost queue entry
    And a holder either releases the lock cleanly or leaves the documented
      "land in progress" marker (recoverable per B57/B58)
    And no interrupt leaves .git/rebase-merge wreckage or a half-pushed main
    And every interrupted exit code is nonzero and distinct from success
```

```gherkin
Scenario: B33 — a failed holder releases and the queue advances
  Given lane A's land fails at the gate while lanes B and C wait queued
  When A exits nonzero
  Then B acquires the lock within one poll interval and lands (exit 0), then C
    And A pushed nothing, so B's recorded base SHA is the pre-A main tip
```

```gherkin
Scenario: B34 — the status contract is total: every lock state has pinned JSON
  Given the lock in each state: unheld, held-live, held-stale (dead PID + old
      heartbeat), corrupt lock file, unreadable lock directory
  When "scripts/land.sh --status --json" runs against each
  Then stdout is valid JSON with the pinned schema:
      state ∈ {"unheld","held","stale","corrupt","unreadable"},
      holder.{id,pid,heartbeat_age_seconds} (null when unheld),
      queue (array, possibly empty)
    And exit code is 0 for unheld/held/stale and a documented nonzero ONLY for
      corrupt/unreadable
    And no --status invocation ever mutates lock or queue state
```

---

## §4 Rebase & branch shapes (B35–B41)

```gherkin
Scenario: B35 — the rebase base comes from a post-acquisition fetch, and fetch failure fails closed
  Given lane B queues while origin/main advances 3 times during B's wait
  When B acquires the lock
  Then B fetches and rebases onto the FINAL origin/main tip — the log's
      recorded base SHA equals the remote tip at acquisition time, not at
      enqueue time
  Given the fetch itself fails (remote unreachable fixture)
  Then land.sh exits nonzero matching "cannot determine base", releases the
      lock, and performed no rebase, regen, or push
```

```gherkin
Scenario: B36 — nothing-to-land shapes exit fast and mutate nothing
  Given three branches: 0 ahead / 0 behind origin/main; 0 ahead / N behind;
      and a landable branch in a clone whose LOCAL "main" ref diverges from
      origin/main
  When "scripts/land.sh" runs on each
  Then the first two exit 0 matching "nothing to land", hold the lock <10s,
      and push nothing
    And in the third, origin/main is the authoritative base and the local
      "main" ref is bit-identical before/after (land.sh never mutates local main)
```

```gherkin
Scenario: B37 — already-landed is patch-aware; partial lands are completed, not duplicated
  Given branch "feat-p" whose single commit was cherry-picked onto main
      (same patch-id, different SHA)
  When "scripts/land.sh" runs
  Then it exits 0 matching "already landed" and pushes nothing — equivalence is
      patch-id-based, not SHA-reachability-only
  Given branch "feat-q" with 2 commits where only the first's patch is on main
  When "scripts/land.sh" runs
  Then exactly the missing patch lands: main gains one new patch-id and its log
      contains no duplicate patch-ids
```

```gherkin
Scenario: B38 — branch shapes that would denormalize main are handled explicitly
  Given three branches containing, respectively: a merge commit, an empty
      commit, a revert commit
  When "scripts/land.sh" runs on each
  Then main remains strictly linear after every accepted land
    And the merge-commit branch is either flattened by the rebase or refused
      with "merge commits not supported" — one documented behavior, asserted;
      a merge commit NEVER silently reaches main
```

```gherkin
Scenario: B39 — a lane that becomes conflicted only after its predecessor lands
  Given lanes A and B both edit the same hunk of skills/crank/SKILL.md and both
      queue while still conflict-free against the original main
  When A lands first and B's post-acquisition rebase hits the conflict
  Then B aborts cleanly with all B15 invariants (no wreckage, lock released,
      nonzero exit)
    And queued lane C (disjoint change) acquires next and lands exit 0 —
      B's failure never wedges the queue
```

```gherkin
Scenario: B40 — conflicts are classified: modify/delete, rename/rename, binary
  Given three conflict fixtures: branch edits a file main deleted; both sides
      rename the same file differently; both sides change a binary file
  When "scripts/land.sh" runs on each
  Then each aborts cleanly (B15 invariants) and the report names EVERY
      conflicted path WITH its class: "modify/delete", "rename/rename", "binary"
    And no textual merge is ever attempted on the binary file
```

```gherkin
Scenario: B41 — land.sh is provably non-interactive and leaves git config untouched
  Given a clone with rerere.enabled=true, rebase.autoStash=true, a repo
      commit-msg hook that prompts on a TTY, and GIT_EDITOR set to a stub that
      exits 1 if ever invoked
  When "scripts/land.sh" runs to completion in both a success variant and a
      conflict-abort variant (run detached from any TTY, stdin /dev/null)
  Then the editor stub is never invoked and no process reads stdin
    And rebase.autoStash is neutralized for land.sh's operations — the B14
      dirty-tree refusal still fires despite autoStash
    And rerere cannot silently auto-resolve a conflict land.sh would report
    And "git config --list" (repo and global) is bit-identical before/after —
      land.sh makes no persistent config change outside an explicit, logged
      "--install" (B62)
```

---

## §5 Derived surfaces & regen (B42–B48)

```gherkin
Scenario: B42 — the derived write set is a manifest, never a hard-coded list
  Given a new generator owning "docs/NEW-SURFACE.md" is registered in the regen
      manifest ONLY (scripts/land.sh itself is not edited)
  When a land makes that surface stale
  Then land.sh regenerates docs/NEW-SURFACE.md at land — new surfaces are
      covered with zero land.sh changes
    And a manifest-drift check fails when regen-all.sh writes a path not
      declared generator-owned, or stops writing a declared one
    And a path declared BOTH source-owned and generator-owned fails preflight
      with a contract error naming the overlap (folds gaps 19, 59, 60)
```

```gherkin
Scenario: B43 — generator failure mid-land aborts with everything restored
  Given one generator stubbed to exit nonzero after writing a partial file
  When "scripts/land.sh" runs
  Then exit is nonzero and the summary names the failing generator and carries
      its stderr
    And origin/main is unchanged, the lock is released, and the worktree is
      restored to its pre-land state — no partial generated file is committed
      or left dirty
```

```gherkin
Scenario: B44 — nondeterministic generator output is detected before push
  Given a generator stubbed to emit a timestamp (different bytes per run)
  When "scripts/land.sh" runs
  Then the land fails BEFORE push, matching "nondeterministic generator output"
      and naming the file and generator
    And origin/main is unchanged
```

```gherkin
Scenario: B45 — deleting or renaming a skill leaves no stale derived entries
  Given branch "rm-skill" deletes skills/aa-one/ and branch "mv-skill" renames
      skills/bb-two/ to skills/bb-three/
  When each lands via "scripts/land.sh"
  Then on main no derived surface (registry.json, context-map, domain map,
      SKILL-TIERS, counts) references "aa-one", and "bb-two" survives only as
      "bb-three"
    And no orphaned codex twin or .agentops-generated.json remains for removed
      or renamed paths
    And "scripts/regen-all.sh --check" on main exits 0
```

```gherkin
Scenario: B46 — hand edits to generator-owned files are reset to canonical output
  Given branch "sneaky" hand-edits registry.json with no source input change,
      and branch "mixed" edits a skill source AND hand-edits the stale
      registry.json in the same commit
  When each lands via "scripts/land.sh"
  Then on main every generator-owned file is byte-identical to a fresh
      generator run — the hand edits are gone
    And land.sh logged a warning naming each discarded generated-path edit
    And in "mixed" the SOURCE change landed intact; only the generated-path
      edit was discarded (folds gaps 23, 24)
```

```gherkin
Scenario: B47 — the strict-JSON verifier is broad and independently testable
  Given the post-land JSON verifier run STANDALONE against fixtures: a
      duplicate key in a non-codex generated JSON, invalid UTF-8 bytes,
      trailing garbage after the closing brace, and a clean tree
  Then it exits nonzero for each corrupt fixture naming the file and the defect
      class, and 0 for the clean tree
    And the set of files it checks derives from the regen manifest — adding a
      generated-JSON manifest entry makes the new file checked with no verifier
      edit (folds gaps 53, 54; B16's guard is this verifier wired into land)
```

```gherkin
Scenario: B48 — the count checker reads a manifest and survives marker edge cases
  Given the count-bearing doc set is listed in a checked-in manifest (today's
      11 docs), not hard-coded in the checker
  When a NEW doc adds a bare numeric skill-count outside marker blocks
  Then the checker fails naming the doc and line (the manifest/sweep catches
      docs not yet listed)
  Given fixtures with: a marker block missing its closing tag, duplicate marker
      ids in one doc, and a non-count numeric ("we tried 47 times")
  Then missing-closing and duplicate-id each fail with distinct errors, and the
      non-count numeric is NOT flagged (folds gaps 55, 56)
```

---

## §6 Gate (B49–B52)

```gherkin
Scenario: B49 — the gate runs every required family and aggregates across all of them
  Given a branch with one violation in each gate family: a failing shell/bats
      check, a failing Go test, a broken doc link, an invalid generated JSON,
      and generator drift
  When "scripts/land.sh" runs
  Then ONE gate pass reports ALL five failures (B13 aggregation generalized
      across families)
    And land.sh's gate-family list is checked for parity against the CI gate —
      the parity check fails if land.sh omits a family validate.yml requires
      (folds gaps 25, 94)
    And an infrastructure preflight failure (missing tool) still fails fast
      per B22 — aggregation applies to check failures, not broken plumbing
```

```gherkin
Scenario: B50 — a hung gate check is killed by timeout, not waited on forever
  Given one gate check stubbed to sleep forever
    And the gate timeout configured to 10 seconds
  When "scripts/land.sh" runs
  Then within 2x the timeout the land exits nonzero naming the timed-out check
    And the stub's entire process group is dead (no orphan sleeper survives)
    And origin/main is unchanged and the lock is released
```

```gherkin
Scenario: B51 — a base main that is already red is reported as such, never blamed on the lane
  Given sandbox origin/main already fails "regen-all.sh --check" before the
      lane branched (drift planted on main itself)
  When "scripts/land.sh" runs on a clean lane branch
  Then it exits nonzero matching "base main is already failing", listing base
      failures separately from (zero) branch-introduced failures
    And nothing is pushed and the lane's branch is untouched
```

```gherkin
Scenario: B52 — post-land verification runs the full gate on a fresh clone of the remote
  Given any successful land (e.g. B1's)
  When the suite verifies the result
  Then verification clones FROM the fixture bare remote into a new directory
      and runs the full gate bundle there — never reusing the lane's working
      tree, so uncommitted local state can never fake a green main
    And the gate bundle passes on the fresh clone (folds gaps 83, 93)
```

---

## §7 Push (B53–B56)

```gherkin
Scenario: B53 — push failures are classified and leave no half-land
  Given three push fixtures: remote unreachable (network), authentication
      rejected, and a remote pre-receive hook that rejects
  When "scripts/land.sh" reaches its push in each
  Then each exits nonzero and the summary classifies the failure:
      "retryable: yes" (network) vs "retryable: no" (auth, hook-reject)
    And origin/main is unchanged in all three, the lock is released, and the
      local branch keeps its rebased commits ready for retry
```

```gherkin
Scenario: B54 — a rewound or force-pushed origin/main fails closed
  Given origin/main is force-rewound to an ancestor between lane A's fetch and
      its push
  When A's push executes
  Then land.sh detects the rewrite (push rejection or tip-regression check),
      exits nonzero matching "origin/main moved unexpectedly|rewound", and
      never force-pushes
    And the audit log records both the expected and the observed remote SHAs
```

```gherkin
Scenario: B55 — a land pushes exactly one ref: refs/heads/main, fast-forward by construction
  Given a lane branch that also carries a local tag and tracks origin/feat-x
  When "scripts/land.sh" lands it
  Then the push refspec targets ONLY refs/heads/main (asserted from the
      execution trace), with no --force/--force-with-lease and no tag, notes,
      or feature-branch refs
    And after landing, the remote's tags and origin/feat-x are bit-identical to
      before (folds gaps 62, 76, 90)
```

```gherkin
Scenario: B56 — repeated out-of-band churn exhausts the bounded retry and aborts cleanly
  Given a fixture that lands an out-of-band commit on origin/main after EACH of
      lane A's gate passes (twice in a row)
  When lane A runs "scripts/land.sh"
  Then A performs at most the configured max rebase attempts (default 2), then
      exits nonzero matching "out-of-band churn"
    And the branch remains landable: a rerun against a quiet remote lands it
      (exit 0)
    And if the gate FAILS after absorbing out-of-band changes, the land aborts
      per B13 with the failure attributed to the absorbed base, not the lane
```

---

## §8 Crash & recovery (B57–B61)

```gherkin
Scenario: B57 — crash-point matrix: SIGKILL at every phase is recoverable to exactly-once
  Given lands SIGKILLed at each instrumented phase: after rebase; after regen
      writes but before the regen commit; after the regen commit; after gate
      pass; during push; after a successful push but before lock release
  When each crashed lane's lock goes stale and the SAME lane reruns
      "scripts/land.sh" (preceded by "--abort" wherever a marker demands it)
  Then every rerun converges: the branch's patches appear on main EXACTLY once
      (no duplicate patch-ids), and the crashed-after-push case reports
      "already landed"
    And no rerun encounters a stranded lock, stale queue entry, or rebase
      wreckage (folds gap 7)
```

```gherkin
Scenario: B58 — --abort has a complete contract
  Given four states: (a) no land in progress; (b) the invoking lane owns an
      in-progress marker; (c) a DIFFERENT live lane holds the lock; (d) a
      crashed land left uncommitted regen artifacts
  When "scripts/land.sh --abort" runs in each
  Then (a) exits 0 matching "nothing to abort", changing nothing
    And (b) restores the worktree to pre-land state (original branch + SHA,
      clean status), clears the marker, releases the lock, writes an "abort"
      audit entry, and exits 0
    And (c) refuses (nonzero, "lock held by live lane") without touching the
      live lock
    And (d) removes the stray regen artifacts as part of restoration
```

```gherkin
Scenario: B59 — every documented failure has a clean retry path
  Given three prior failures: (1) B13's gate failure, after which the lane
      fixes all three violations; (2) B15's conflict abort, after which the
      lane resolves the conflict in its branch; (3) B18's stale takeover,
      after which lane B landed first
  When each lane reruns "scripts/land.sh"
  Then each retry lands (exit 0) with no stale queue entries and no duplicate
      regen commits
    And in (3), A's retry rebases onto the main containing B's commits — B's
      work is in A's ancestry and nothing pre-takeover is resurrected
```

```gherkin
Scenario: B60 — destructive steps record a recovery point; temp artifacts are cleaned
  Given any land that reaches the rebase
  Then BEFORE the first history mutation the original tip SHA is recorded
      (backup ref "refs/land/backup/<lane-id>" or an audit field) — asserted
      present
  When the land later fails at any phase
  Then the recorded SHA still resolves and "--abort" restores the branch to it
  When the land succeeds
  Then no temp branches, temp worktrees, patch files, or backup refs created by
      land.sh remain — the ref list and .git dir match pre-land state modulo
      the landed commits
```

```gherkin
Scenario: B61 — disk-full never corrupts main, the lock, or the audit log
  Given a quota-limited fixture filesystem that returns ENOSPC during, in three
      variants: (a) a generator write, (b) the regen commit, (c) an audit append
  When "scripts/land.sh" runs in each
  Then each exits nonzero; origin/main is unchanged; no half-written commit is
      reachable from any ref
    And the lock file and audit log still parse afterward (strict JSON/JSONL) —
      a torn append never breaks later "--status" calls
```

---

## §9 Guards & protected paths (B62–B66)

```gherkin
Scenario: B62 — guard install lifecycle is explicit; an unprotected clone is never silent
  Given a fresh sandbox clone that has never installed the landing discipline
  When an agent pushes directly to main from it
  Then EITHER the push is blocked (guard self-installs / server-side fixture
      hook) OR land.sh refuses to run until "scripts/land.sh --install"
      completes — one documented behavior; a naked clone never operates silently
    And "--install" is idempotent ("already installed", exit 0 on rerun) and
      upgrades a stale guard version in place with a logged version change
    And land.sh REPORTS the real origin's branch-protection posture when
      detectable (informational only — administration stays out of scope;
      folds gaps 37, 98)
```

```gherkin
Scenario: B63 — the land.sh bypass marker cannot be replayed outside a live land
  Given an agent copies the env marker/token land.sh uses to authorize its own
      push
  When the agent runs "git push origin main" directly with the copied marker
      but WITHOUT holding the landing lock
  Then the push is rejected matching "use scripts/land.sh" — the guard
      validates the marker against the live lock holder (id + nonce), not mere
      presence of the variable
```

```gherkin
Scenario: B64 — branch-side .gitignore edits cannot blind the checks
  Given branch "hide-it" adds .gitignore entries covering registry.json and the
      lock/audit directory
  When "scripts/land.sh" runs on it
  Then drift detection, regen --check, and lock/audit operations behave exactly
      as without the ignore edits — the branch's generated drift is still
      caught and "--status" still reads the lock
```

```gherkin
Scenario: B65 — private _beads paths can never be staged or pushed by land.sh
  Given a worktree with modified and untracked files under _beads/ and an
      otherwise landable branch
  When "scripts/land.sh" runs
  Then no _beads path appears in any commit land.sh creates (asserted via
      "git log --stat" of all new main commits)
    And the _beads dirt is handled by ONE documented rule (exempt from the
      dirty check, or reported separately) — but is NEVER auto-staged
```

```gherkin
Scenario: B66 — a branch that modifies land.sh itself is handled, not trusted blindly
  Given branch "self-mod" edits scripts/land.sh and scripts/regen-all.sh
  When the lane runs "scripts/land.sh"
  Then land.sh detects the self-modification and applies its documented policy:
      refuse with "self-modifying land — land manually with review" OR re-exec
      the post-rebase version before gating — one behavior, asserted
    And the running process never silently executes a half-old/half-new mix
    And if landed, the result passes the full gate on a fresh clone (B52)
      (folds gaps 63, 64)
```

---

## §10 Observability & CLI contract (B67–B72)

```gherkin
Scenario: B67 — exit codes are a stable taxonomy and every error is structured
  Given the documented exit-code table: 0 = success or no-op; distinct nonzero
      codes for preflight refusal, lock wait timeout, source conflict, gate
      failure, push failure, and internal error
  When the suite drives one instance of each failure class
  Then each observed exit code matches the table exactly (swarm supervisors can
      branch on them)
    And every failure's final summary includes: phase, failed command, branch,
      current SHA, base SHA, "retryable: yes|no", and a one-line next action
      (folds gaps 38, 39)
```

```gherkin
Scenario: B68 — every land emits one durable, correlated log; audit appends are atomic
  Given any land, success or failure
  When it completes
  Then the last stdout lines include "log: <path>", and that file contains:
      ISO-8601 timestamps, lane identity, branch, start SHA, base SHA, final
      main SHA (or failure phase), per-phase durations, gate result, push
      result, and a correlation id
    And the same correlation id appears on that land's audit entries
    And two lands appending to the audit log concurrently produce two intact
      JSONL lines (no torn/interleaved lines), and a pre-existing invalid audit
      line does not crash later lands or "--status" (folds gaps 17, 18, 95)
```

```gherkin
Scenario: B69 — post-success local state is defined: branch kept, rebased, clean
  Given lane A lands "feat-x" successfully
  Then the worktree is still on branch "feat-x", now at the landed SHA (an
      ancestor of origin/main), and "git status --porcelain" is empty
    And local "main" was not touched, and "feat-x" was deleted neither locally
      nor remotely — branch cleanup is the lane's choice, never land.sh's
      (folds gaps 31, 74)
```

```gherkin
Scenario: B70 — post-failure local state is defined for every failure class
  Given one failure of each class: gate failure, push failure, regen failure,
      gate timeout, interrupted queue wait
  When each exits
  Then in every case "git status --porcelain" is empty OR exactly one
      documented "land in progress" marker exists — never ambiguous wreckage
    And HEAD is on the original branch (at the original SHA for all pre-push
      failures)
    And the final summary states the retry instruction for that class
```

```gherkin
Scenario: B71 — --dry-run reports the full plan and mutates nothing
  Given four contexts: a clean landable branch; a dirty worktree; a held lock;
      a branch that would fail the gate
  When "scripts/land.sh --dry-run" runs in each
  Then the clean case prints: resolved base SHA, the commits that would land,
      the derived surfaces that would regenerate, and the gate commands — and
      exits 0
    And the blocked cases report what WOULD block, with the documented dry-run
      exit code
    And in ALL cases: no lock mutation, no queue entry, no push, and the
      worktree is bit-identical before/after
```

```gherkin
Scenario: B72 — hostile branch names and paths cannot break land.sh
  Given a branch named "feat/we;rd $(touch pwned) 'qu\"ote'", a sandbox repo
      whose path contains spaces and a unicode segment, and a symlinked repo
      root
  When "scripts/land.sh" lands a trivial change in each
  Then each either lands (exit 0) or refuses the name with a clear validation
      error — deterministically, documented which
    And no file named "pwned" is created anywhere (no shell injection)
    And logs render the branch name intact and every audit JSONL line still
      parses (folds gaps 65, 66)
```

---

## §11 Meta: the acceptance suite itself (B73)

```gherkin
Scenario: B73 — the acceptance suite is itself gated: one command, hermetic, total, deterministic
  Given the suite entry point "tests/landing/run-acceptance.sh"
  When it runs twice on a clean machine, on both macOS and Linux CI
  Then ONE command runs EVERY scenario in this document; any skip/pending/focus
      marker fails the whole run; both runs produce identical pass/fail results
    And all fixtures are hermetic: temp dirs plus the B25 sandbox-marked bare
      remote; zero writes outside fixture roots; no network beyond the fixture
      remote
    And every concurrent-lane scenario captures per-lane stdout/stderr/exit
      status attributed to the correct lane, with all background processes
      reaped before the scenario reports
    And the coverage map below tags every scenario with ≥1 risk class
      (preflight, lock, queue, rebase, regen, gate, push, recovery,
      observability, harness) and a map check fails on any unmapped scenario
      (folds gaps 68, 70, 71, 72, 99, 100)
```

---

## Coverage map (failure-mode → scenario)

| Failure mode / risk class | Scenario(s) |
|---|---|
| Lane races / 6 lost rebase cycles | B3, B4, B6, B12, B35, B56 |
| Serialization on 8 derived surfaces | B2, B9, B42–B46 |
| `generated_hash` duplicate splice on rebase | B16, B47, B4 |
| Fail-fast gate: 1 failure per 3–5 min cycle | B13, B49 |
| Counts hand-asserted in 11 prose docs | B11, B48 |
| Build-slots named in doctrine, nothing enforces them | B6, B7, B8, B17, B26–B30, B62, B63 |
| No one-command landing | B1, B2, B10, B20, B23, B24, B71 |
| Crash/abandonment on a hot repo | B7, B18, B32, B57–B61 |
| Force-push / history damage risk | B12, B54, B55, B60 |
| Swarm-tending blindness (who holds the land?) | B5, B34, B67, B68 |
| Preflight / refused contexts | B14, B19–B25 |
| Queue integrity | B29, B30, B33, B39 |
| Branch-shape & rebase semantics | B36–B41 |
| Gate integrity (timeout, parity, red base) | B49–B52 |
| Push failure classes | B53–B56 |
| Recovery & retry | B57–B61, B59 |
| Guard integrity & protected paths | B62–B66 |
| Hostile inputs / environment | B41, B61, B64, B72 |
| Harness safety & suite quality | B25, B52, B73 |

## Gap disposition table (behaviors-codex-gaps.md → this contract)

**Folded (94):**
1→B19 · 2→B20 · 3→B21 · 4→B22 · 5→B25 · 6→B53 · 7→B57 · 8→B58 · 9→B32 ·
10→B29 · 11→B28 · 12→B28 · 13→B23 · 14→B27 · 15→B26 · 16→B34 · 17→B68 ·
18→B68 · 19→B42 · 20→B43 · 21→B44 · 22→B45 · 23→B46 · 24→B46 · 25→B49 ·
26→B50 · 27→B56 · 28→B38 · 29→B37 · 30→B37 · 31→B69 · 32→B70 · 33→B35 ·
34→B35 · 35→B41 · 36→B63 · 37→B62 · 38→B67 · 39→B67 · 40→B71 · 41→B23 ·
42→B23 · 43→B30 · 44→B39 · 45→B33 · 46→B51 · 47→B54 · 48→B23 · 50→B31 ·
51→B31 · 52→B61 · 53→B47 · 54→B47 · 55→B48 · 56→B48 · 57→B41 ·
58→B9(strengthened)+B46 · 59→B42 · 60→B42 · 61→B2(strengthened) · 62→B55 ·
63→B66 · 64→B66 · 65→B72 · 66→B72 · 68→B73+B25 · 70→B73 · 71→B73 · 72→B73 ·
74→B69 · 75→B28 · 76→B55 · 77→B40 · 78→B40 · 79→B40 · 80→B36 · 81→B36 ·
82→B36 · 83→B52 · 84→B60 · 85→B60 · 86→B24 · 87→B59 · 88→B59 · 89→B59 ·
90→B55 · 91→B41 · 92→B41 · 93→B52 · 94→B49 · 95→B68 · 96→B64 · 97→B65 ·
98→B62 · 99→B73 · 100→B73

**Absorbed as standing conventions (not scenarios):**
- 67 — the "origin/main unchanged" assertion convention is pinned in the
  harness-assumptions preamble (remote-bare-SHA before/after, applied to every
  scenario that claims it).

**Rejected (one-line reasons):**
- 49 (large-branch performance bounds) — performance tuning, not acceptance
  behavior; no-indefinite-hold liveness is already covered by B31's heartbeat
  semantics and the B50 gate timeout.
- 69 (Git LFS / submodules / file modes) — this repo uses neither LFS nor
  submodules, and executable-bit changes are ordinary rebase content already
  exercised by the conflict scenarios.
- 73 (status/log path redaction) — solo-operator repo with a private fleet:
  full local paths in status output and logs are explicitly inside the threat
  model; redaction would only obscure swarm tending.

## Out of scope for this feature (recorded so no scenario sneaks in later)

- The CI-side `contracts-sync` advisory/strict phasing (owned by W1.1 of the
  seams plan, not the landing redesign).
- De-committing derived surfaces entirely (a design option Phase 2+ may pick;
  the behaviors above are deliberately design-agnostic: merge-driver,
  regen-at-land, or de-commit all satisfy B9/B16/B42–B47 as written).
- `br`/bead-tracker mechanics and the `_beads` substrate (B65 protects the
  boundary; it does not test the tracker).
- GitHub branch-protection administration on the real origin (B17/B62/B63 are
  asserted against the sandbox clone's installed guard; B62 only *reports*
  real-origin posture).
