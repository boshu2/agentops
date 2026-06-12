# Missing Behaviors Review

INTENT: identify acceptance behaviors missing from `docs/plans/bdd-foundry/behaviors.md` before the landing redesign is treated as done.

Review stance: independent cross-family contract review. I am treating the Gherkin as a shippable behavior contract, not as draft prose. Prior review learning applied: Codex-style severity means gaps are judged by whether an implementer could ship an incomplete or unsafe `land.sh` while still satisfying the written scenarios.

## Gaps

1. Dirty-tree refusal does not cover untracked, staged-only, partially staged, ignored, or submodule dirt.
   B14 covers "uncommitted modification to any tracked file" only. Add scenarios for staged changes, untracked files that would collide with regen outputs, ignored generated files, dirty submodules if the repo uses any, and partially staged tracked files. Done should specify whether each class is refused, ignored, or reported separately before lock acquisition.

2. Invocation context is under-specified.
   There is no behavior for running `scripts/land.sh` from a subdirectory, from the canonical repo root, from a linked worktree, from a detached HEAD, from `main`, or from a branch with no upstream. Add scenarios requiring a deterministic root discovery rule and clear nonzero errors for detached HEAD, direct `main` invocation, missing `origin`, missing `origin/main`, and non-worktree directories.

3. "Clean working tree" does not define index and Git metadata invariants.
   B1 says clean working tree, but an implementation could ignore index state, pending cherry-pick/rebase state, `MERGE_HEAD`, `CHERRY_PICK_HEAD`, or existing `.git/rebase-*`. Add a preflight scenario refusing any in-progress Git operation before lock acquisition and proving the repo is unchanged afterward.

4. Required tool and environment failures are absent.
   The scenarios assume `git`, `jq`, generator runtimes, `scripts/regen-all.sh`, hook installers, and test harness dependencies exist. Add scenarios for missing or non-executable `scripts/regen-all.sh`, missing `jq`, missing generator dependencies, and unsupported shell options. Done should require one preflight summary with missing command names and no lock acquisition.

5. Remote safety is not bounded enough.
   The plan says sandbox clone, but no scenario proves `land.sh` refuses dangerous remotes or the wrong repository. Add a scenario where `origin` points to a non-sandbox/real remote under test and requires a dry-run or explicit sandbox marker before tests can push. This is a harness safety gap, not just an implementation detail.

6. Push failure classes are missing.
   B12 covers non-fast-forward rejection only. Add scenarios for network failure, authentication failure, remote hook rejection, remote unavailable, and permission denied. Done should require origin/main unchanged unless the push succeeded, lock released or reclaimable, exit nonzero, and output that distinguishes retryable transport errors from permanent authorization/config errors.

7. Crash timing coverage is too narrow.
   B18 kills lane A after acquiring the lock but before pushing. Add separate crash scenarios after rebase, after regen writes but before commit, after creating the regen commit, after gate pass, during push, and after successful push but before lock release/audit finalization. The "already landed" retry path must reconcile each state without duplicate commits or stranded locks.

8. `--abort` semantics are only mentioned, not specified.
   B18 says a marker that `scripts/land.sh --abort` clears, but no scenario defines `--abort` behavior. Add scenarios for abort with no active land, abort by the owner during an in-progress land, abort by a non-owner, abort after stale takeover, and abort when the repo has uncommitted regen artifacts. Done should state what is restored, what audit entry is written, and the exit codes.

9. Signal handling is underspecified.
   SIGKILL is covered only as a stale-lock case. Add scenarios for SIGINT and SIGTERM while waiting in queue, while holding the lock, during gate, and during push. Done should require graceful cleanup where possible, no queue ghost for waiters, no lock theft from live holders, and a reclaimable marker for non-clean interruption.

10. FIFO queue behavior omits cancellation, duplicates, and starvation.
   B3 verifies A,B,C ordering once. Add scenarios where the same lane invokes `land.sh` twice, a queued lane exits or is killed, a queued lane times out, and new lanes arrive while older lanes wait. Done should require no duplicate queue entries for one lane identity, removal of dead waiters, and no starvation.

11. Lane identity is not defined.
   Status JSON expects `holder.id`, but no scenario defines how IDs are formed or made unique across host, PID, worktree path, branch, and timestamp. Add a collision scenario with two worktrees on the same branch name and a PID reuse scenario. Done should require distinct identities and audit entries that can be traced back to the exact worktree.

12. Live-lock detection trusts PID too much.
   B7 and B8 talk about PIDs, but PID reuse can make a dead holder look alive. Add a scenario where the lock file names a PID now owned by an unrelated process. Done should require heartbeat freshness plus identity/process validation, or explicitly document that heartbeat alone is authoritative.

13. Clock and TTL behavior is incomplete.
   Stale TTL defaults to 15 minutes, but there is no behavior for configurable TTL, zero/negative TTL, clock skew, heartbeat timestamps in the future, or a corrupt/missing heartbeat timestamp. Add scenarios requiring validation and safe fail-closed handling.

14. Lock storage failure paths are missing.
   No scenarios cover missing lock directory, read-only lock directory, corrupted lock JSON, truncated lock file, stale temp files, symlinked lock path, or audit log write failure. Done should define whether `land.sh` fails closed before rebase or can proceed with degraded audit logging. For a landing mutex, fail closed is the safer expected behavior.

15. Mutual exclusion does not prove atomic lock acquisition mechanics.
   B6 proves intervals after the fact but does not specify atomic create/rename/flock behavior under simultaneous start. Add a scenario that forces simultaneous acquisition attempts at the same timestamp and asserts exactly one atomic winner, with losers queued and no partial lock files.

16. Status mode is incomplete.
   B5 only covers status while A holds the lock and B is queued. Add scenarios for unheld status, stale-held status, corrupt-lock status, multiple queued lanes, and status when the lock directory is unreadable. Done should pin JSON schema, stable field names, exit codes, and whether `--status --json` ever returns nonzero.

17. Human log contract is too thin.
   Scenarios assert individual log snippets, but there is no behavior for where logs are stored, whether each lane gets a durable log, whether logs include timestamps, branch, start SHA, base SHA, final SHA, gate duration, and push result. Add a done condition that a failed or successful land emits a single inspectable log path and an audit correlation ID.

18. Audit log durability and growth are not specified.
   Several scenarios rely on a lock audit log, but no scenario defines its format, append atomicity, rotation, corruption handling, or whether audit write failure blocks landing. Add scenarios for concurrent audit appends, invalid existing audit lines, and audit directory read-only.

19. Derived-surface write set source of truth is not tested.
   B2/B9 mention examples of derived surfaces, but the actual regen write set can change. Add a scenario asserting `land.sh` obtains the generator-owned path set from `scripts/regen-all.sh` or a generated manifest, not from a hard-coded list in `land.sh`. Done should catch a newly added generator-owned file without editing `land.sh`.

20. Generator failure is missing.
   There is no scenario where regen itself exits nonzero, panics, writes invalid output, or leaves partial files. Add a scenario requiring nonzero exit, all generator errors summarized, origin/main unchanged, lock released, and the worktree restored or left with an explicit recoverable marker.

21. Generator nondeterminism is not covered.
   B9 checks byte identity to a fresh run after land, but no scenario catches a generator that produces different bytes on repeated runs in the same checkout. Add a scenario requiring `land.sh` or the gate to detect nondeterministic derived output before push.

22. Generated-output deletion and rename cases are absent.
   Current scenarios focus on additions and textual conflicts. Add cases where a skill is deleted or renamed and generated surfaces must remove registry/context/hash entries. Done should require no stale entries on main and no orphaned codex twin/hash files.

23. Hand-authored edits to generated files need an explicit behavior.
   B2 says authors need not regenerate, and B9 says generated divergence is resolved by generators. Add a scenario where a branch intentionally edits a generator-owned file by hand without changing source inputs. Done should state whether the edit is discarded with a warning, refused as invalid, or preserved only if the generator reproduces it.

24. Source and generated changes in one commit are under-specified.
   If a lane edits both a skill source and stale generated files, `land.sh` could preserve bad generated edits while still passing some checks. Add a scenario requiring generated files to be reset to canonical generator output after rebase, regardless of branch-side generated edits.

25. Gate all-failures behavior is narrower than the gate surface.
   B13 covers three specific violations. Add scenarios where failures span shell scripts, Go tests, markdown/link checks, JSON parsing, generator drift, and hook install checks. Done should prove the aggregation framework reports all independent gate families it can run, while still failing closed on infrastructure preflight failures.

26. Gate timeout behavior is missing.
   The spec gives gate runtime context but no behavior for a hung test or generator. Add a scenario for a gate command exceeding a configured timeout. Done should require killing the process group, reporting the timed-out check, releasing/reclaiming the lock, and leaving origin/main unchanged.

27. Gate rerun policy is incomplete.
   B12 allows at most one bounded re-rebase and gate rerun for one out-of-band push. Add a scenario where out-of-band pushes happen twice, or where the second gate fails after absorbing out-of-band changes. Done should specify maximum attempts, exact exit behavior, and whether the lane remains landable on retry.

28. Branch history shape is under-specified.
   B6 asserts final main is linear for trivial branches, but no scenario covers a lane branch containing merge commits, revert commits, empty commits, or multiple roots after a weird rebase. Add scenarios specifying whether `land.sh` preserves commits via rebase, squashes, refuses merge commits, or accepts them while keeping main linear.

29. Already-landed detection can produce false positives.
   B10 covers a branch whose every commit is reachable. Add scenarios where a branch has the same patch-id but different commit SHAs, a reverted commit, a cherry-picked equivalent commit on main, or only some commits landed. Done should define whether "already landed" means commit reachability only or semantic patch equivalence.

30. Partial-land detection is missing.
   If one of two branch commits reached main and the second did not, `land.sh` should not say "already landed" or duplicate the first commit. Add a scenario requiring it to land only missing commits or fail with a clear partial-land message.

31. Local branch/worktree post-success state is not defined.
   Success scenarios assert origin/main but not local state. Add done conditions for whether the worktree remains on the feature branch, whether the branch is rebased to the landed SHA, whether local `main` is updated, whether generated files are clean, and what `git status` shows.

32. Local branch/worktree post-failure state is only partly defined.
   B15 covers real source conflicts. Add failure-state scenarios for gate failure, push failure, regen failure, timeout, and interrupted wait. Done should require clean status or an explicit marker, original branch preserved, and instructions for retry.

33. Remote main moving during queue wait is not tested.
   B3 says B rebases onto main that includes A. Add a case where origin/main advances repeatedly while a lane waits before acquiring the lock. Done should require fresh fetch/base determination only after acquiring the lock, not before queue entry.

34. Fetch behavior is missing.
   No scenario says whether `land.sh` fetches origin/main before rebase, how it handles fetch failure, stale remote-tracking refs, shallow clones, or missing network. Add scenarios requiring a fetch at the correct time and fail-closed behavior if the authoritative base cannot be determined.

35. No behavior covers hooks fired by `git` itself.
   B17 discusses direct-push blocking, but not pre-rebase, post-commit, pre-push, or commit-msg hooks that may mutate output or block. Add scenarios requiring either controlled hook disabling/enabling for internal operations or explicit propagation of hook failures without leaving the lock held.

36. The land.sh bypass marker can be spoofed.
   B17 says direct push is rejected unless land.sh's invocation marker is set. Add a scenario where an agent manually exports/copies that marker and pushes directly. Done should require an unspoofable or sufficiently narrow guard in the sandbox, or explicitly state the limitation if only client-side hooks are used.

37. The direct-push guard install/update lifecycle is absent.
   B17 assumes landing discipline is installed. Add scenarios for first install, already installed, stale guard version, disabled hooks path, and uninstall or bypass attempts. Done should prove a clone that has not installed the discipline fails loudly or self-installs before relying on protection.

38. Error messages are not consistently actionable.
   Some scenarios assert regex snippets, but there is no universal requirement that errors include the failed phase, command, branch, current SHA, base SHA, and next action. Add a scenario for each major failure phase that asserts a structured summary with "retryable: yes/no" and a concrete remediation.

39. Exit code taxonomy is missing.
   Scenarios mostly say 0 or nonzero. Add a behavior defining stable exit codes for preflight refusal, lock timeout, conflict, gate failure, push failure, internal error, and already-landed no-op. This matters for swarm supervisors deciding whether to retry, abandon, or alert.

40. Dry-run behavior is missing.
   A landing tool normally needs `--dry-run` to let agents see the planned base, commits, derived reset, and gate commands without pushing. Add scenarios for dry-run with clean branch, dirty tree, queued lock, and gate failures. Done should require no lock mutation or no push, depending on the chosen dry-run contract.

41. Wait-timeout CLI contract is not pinned.
   B8 mentions "with a wait timeout of 60 seconds" but no flag/env name, default, or boundary values. Add scenarios for `--wait-timeout=0`, negative values, invalid durations, and default behavior. Done should specify whether timeout means exit immediately, enqueue then leave, or wait in foreground.

42. Configuration precedence is missing.
   TTL, wait timeout, remote name, branch name, lock path, and gate timeout will likely be configurable. Add scenarios proving CLI flag > environment > repo config > default precedence, and that invalid config fails before lock acquisition.

43. Concurrent same-branch lands are missing.
   Add a scenario where two processes in the same worktree or two worktrees on the same branch run `land.sh` concurrently. Done should require one winner and one no-op/clear refusal, with no duplicate pushes and no branch corruption.

44. Same source conflict after queued rebase is not fully covered.
   B15 covers conflict between branch and main. Add a multi-lane case where lane B was conflict-free when queued but conflicts with lane A after A lands. Done should require B abort cleanly, C still proceeds or remains queued correctly, and the queue is not blocked behind B forever.

45. Queue behavior after a failed holder is missing.
   B13 and B15 require lock release, but not what happens to queued lanes. Add a scenario where A fails gate/conflict while B and C wait. Done should require B acquires after A releases, unless the failure indicates repository-wide invalid main state.

46. Repository-wide broken main is not covered.
   If origin/main already fails `scripts/regen-all.sh --check` before applying the lane, `land.sh` should not blame the lane blindly. Add a scenario where sandbox main starts red. Done should require a distinct "base main is already failing" error, no push, and evidence separating base failures from branch-introduced failures.

47. Out-of-band main rewrite is absent.
   B12 covers non-fast-forward from an extra commit, not force-pushed or rewound origin/main. Add a scenario where origin/main moves backward or is rewritten between fetch and push. Done should require fail-closed behavior and no force push.

48. Multiple remotes/default branch names are not specified.
   Add scenarios for remote named something other than `origin`, default branch not named `main`, and local `main` tracking a different upstream. Done should either explicitly refuse unsupported layouts or support config with clear precedence.

49. Large branch and performance bounds are not tested.
   The scenarios use small branches. Add a soak case with a realistic queue and a branch with many commits/files. Done should set acceptable lock-hold time, gate timeout, and log size so a lane cannot hold the lock indefinitely without heartbeat/progress.

50. Heartbeat progress semantics are missing.
   B5 exposes `heartbeat_age_seconds`, but no scenario proves heartbeat continues during long gate runs, fetches, rebases, or pushes. Add a scenario where a gate runs longer than one heartbeat interval and status shows a fresh heartbeat throughout.

51. No scenario covers heartbeat write failure while holding the lock.
   If disk becomes read-only or full during land, a live holder may appear stale. Add a scenario requiring fail-closed release/abort behavior or a clear rule that missed heartbeats cannot cause takeover until process validation also fails.

52. Disk-full and temp-file failures are absent.
   Landing and regen may write large generated files and temp logs. Add scenarios for disk full during regen, audit append, lock heartbeat, and git commit. Done should require no partial commit on main, no corrupt lock/audit JSON, and recoverable worktree state.

53. Strict JSON duplicate-key verification is too narrow.
   B16 covers `generated_hash` duplicate keys in codex manifests. Add behavior for duplicate keys elsewhere in generated JSON and invalid UTF-8/trailing garbage. Done should require the strict JSON checker's file glob and duplicate-key policy to be explicit.

54. The generated-hash guard's negative test is embedded in one scenario.
   B16 asks to feed a planted duplicate fixture, but it is not a separate behavior with setup/teardown. Split it into its own scenario so the verifier can be tested independently of a successful land.

55. Scenario B11 needs a concrete 11-doc source of truth.
   It refers to "the 11-doc set" but does not name a manifest. Add a scenario requiring the checker to read the doc list from a maintained manifest or discover it deterministically. Done should fail when a new prose count is added outside marker blocks.

56. Count-marker edge cases are missing.
   B11 covers wrong value inside a marker and literal outside. Add cases for nested markers, missing closing marker, duplicate marker IDs, marker blocks in generated docs, and non-skill numeric counts that should not be flagged.

57. "No manual conflict resolution" is not machine-checkable enough.
   B3 asserts no manual intervention log line. Add a stronger behavior that no process reads from TTY/stdin, no editor opens, and `GIT_EDITOR`/sequence editor are disabled during automated rebase/commit. Done should prove `land.sh` is non-interactive under conflicts and commit creation.

58. Rebase strategy is under-specified for generated paths.
   B9 says no generated conflict reaches a human, but does not state whether generated files are removed before rebase, checked out from main, merged with a driver, or reset after rebase. Add behavior-level invariants: generated paths must not influence conflict decisions, and source changes must not be lost when generated paths are reset.

59. Source files that feed generators but are also generated are not covered.
   If a file is both a source input and in the regen write set by mistake, `land.sh` could discard real changes. Add a scenario that detects overlap between source-owned and generator-owned paths and fails with a clear contract error.

60. Generated surface ownership drift is missing.
   Add a scenario where `scripts/regen-all.sh` writes a file not declared generator-owned or stops writing one that is declared. Done should fail the manifest/checker so `land.sh` cannot silently miss a new derived surface.

61. Success evidence does not include commit authorship details.
   B2 allows a regen commit authored by `land.sh` or squashed. Add a scenario pinning author, committer, trailers, and message body requirements for any automated regen commit so audit/provenance can distinguish lane-authored code from lander-authored generated output.

62. Push atomicity with tags or ancillary refs is not specified.
   If `land.sh` pushes only `main`, say so. If it pushes tags or notes, add scenarios. Done should ensure no tags, notes, or other refs are modified by a normal land unless explicitly requested.

63. No scenario covers protected files outside generated surfaces.
   Add behavior for branches modifying lock files, hook files, `scripts/land.sh` itself, `scripts/regen-all.sh`, or gate definitions. Done should specify whether self-modifying land changes are allowed, require a two-phase land, or are refused because the running script cannot validate its own replacement safely.

64. Self-update of `land.sh` is a special missing case.
   If a branch changes `scripts/land.sh`, the current process may run old code while landing new code. Add a scenario requiring either refusal, re-exec after rebase, or explicit acceptance with a warning and post-land validation.

65. Security around shell inputs is not covered.
   Add scenarios with branch names, worktree paths, and remote URLs containing spaces, quotes, shell metacharacters, newlines, and Unicode. Done should prove no command injection, broken logging, or malformed audit JSON.

66. Worktree path and repo path edge cases are absent.
   Add cases for paths with spaces, symlinked repo paths, nested worktrees, and very long paths. Done should require correct root resolution, lock path resolution, and readable status output.

67. Main unchanged assertions should include local and remote SHA checks.
   Several error scenarios say origin/main unchanged. Add a reusable done condition: record pre-run `origin/main` SHA and local `refs/remotes/origin/main`; after failure both are either unchanged or, if fetch updated remote-tracking refs, the remote branch itself is unchanged and this distinction is logged.

68. The sandbox remote model is under-specified.
   Some scenarios require pushing to `origin/main`, while others mention a fresh checkout. Add behavior for the fixture remote: bare remote path, clone count, branch protection simulation, hook install, and cleanup. Otherwise tests can pass while accidentally relying on local refs only.

69. No behavior covers Git LFS, submodules, or file mode changes.
   If unsupported, add refusal scenarios. If supported, add cases for LFS pointer changes, submodule SHA changes, executable-bit changes on scripts, and generated files with mode changes.

70. There is no explicit "definition of done" for `land.sh` itself.
   The behaviors say every scenario must become runnable before a bead exists, but there is no meta-scenario requiring B1-B18 plus added gaps to run in CI/local gate with isolated fixtures. Add a done condition that the acceptance suite has a single command, is hermetic, leaves no real repo mutations, and fails if any scenario is skipped.

71. Skipped or flaky acceptance tests are not prohibited.
   Add a scenario or meta-check that no BDD scenario is marked pending/skip/focus, no test depends on wall-clock sleeps longer than the fixture budget, and repeated runs produce the same result.

72. "All three lanes exit 0" omits process supervision details.
   For B3/B4, add done conditions proving each concurrent invocation's stdout/stderr/log is captured separately, exit status is attributed to the right lane, and background processes are waited on and cleaned up.

73. Queue status and lock status can leak private paths or sensitive data.
   Add a behavior for redaction in `--status --json` and human logs if lane IDs include full paths, remotes, or user names. Done should decide whether full local paths are acceptable in this repo's threat model.

74. Branch deletion or cleanup policy is missing.
   After success, should the feature branch remain, be fast-forwarded, or be deleted locally/remotely? Add a scenario pinning the expected branch cleanup behavior so swarm lanes know whether retry/no-op is available.

75. The plan does not cover concurrent branch names with different remotes.
   Add a scenario where two clones have branch `feat-x` with different commits. Done should require identity and audit records use commit SHA/worktree, not branch name alone.

76. No behavior covers branch containing commits already pushed to another remote branch.
   If a lane branch tracks `origin/feat-x`, `land.sh` could accidentally push/update it. Add a scenario proving only `origin/main` is pushed and feature branch remotes are not mutated.

77. Conflict reporting only names files, not conflict class.
   B15 requires conflicting file names. Add behavior requiring source conflict vs generated conflict vs binary conflict vs rename/delete conflict classification, because retries/remedies differ.

78. Rename/delete conflicts are missing.
   Add scenarios where branch edits a file deleted on main, branch deletes a file edited on main, and both sides rename a file differently. Done should require clean abort with all paths listed and no lock leak.

79. Binary file conflicts are missing.
   Add a scenario for conflicting binary or non-text files. Done should require no textual merge attempt, clear conflict report, origin/main unchanged, and worktree restored.

80. Empty branch/no-op branch is missing.
   B10 covers already-landed commits, but not a branch with no commits ahead of main from the start. Add a scenario requiring exit 0 no-op, no gate or only minimal preflight, no new commit, and no long lock hold.

81. Branch behind main but with no local commits is missing.
   Add a scenario where feature branch is 0 ahead and N behind. Done should define whether `land.sh` updates/rebases the branch, exits "nothing to land", or refuses.

82. Branch with unpushed local main divergence is missing.
   Add a scenario where local `main` differs from `origin/main`. Done should require `land.sh` uses `origin/main` as authoritative and does not mutate local main unexpectedly.

83. The "fresh checkout of main" verifier is repeated but not defined.
   B1/B2/B4 mention fresh checkout. Add a scenario or fixture helper contract requiring the checkout to come from the remote after push, not the same local working tree, so uncommitted local state cannot satisfy verification.

84. No behavior covers cleanup of temporary branches/worktrees used internally.
   If `land.sh` creates temp branches, temp worktrees, patch files, or backup refs, add scenarios requiring cleanup on success and bounded retained evidence on failure.

85. Backup/recovery refs are not specified.
   For destructive operations like rebase/reset, add behavior requiring a backup ref or recorded original SHA before mutation, and proving it is available after failure for recovery.

86. The "one command" surface lacks help/version behavior.
   Add scenarios for `scripts/land.sh --help`, invalid flags, incompatible flag combinations, and `--version` or script version output if audit records include versions. Done should require no repo mutation for help/usage errors.

87. Re-running after a gate failure is missing.
   Add a scenario where the user fixes B13's three violations and reruns `land.sh`. Done should require the retry lands without stale failure markers, stale queue entries, or duplicate regen commits.

88. Re-running after a source conflict resolution is missing.
   B15 aborts cleanly. Add a scenario where the lane resolves the conflict manually in its branch after the abort, reruns `land.sh`, and lands. Done should prove the previous abort did not poison the queue or lock state.

89. Re-running after stale takeover of the same lane is missing.
   B18 says A can re-land after abort. Add a scenario where A retries after B has reclaimed the stale lock and landed. Done should require A rebases onto B, not resurrecting pre-takeover state.

90. "No force push" should be enforced across aliases and config.
   B12 greps execution trace for `--force` and `--force-with-lease`. Add scenarios where git config aliases or wrapper scripts could hide a force push, and require push refspecs that are fast-forward-only by construction.

91. Git config side effects are not covered.
   Add a scenario proving `land.sh` does not permanently change repo/global Git config such as `pull.rebase`, merge drivers, hooksPath, rerere, or user identity unless explicitly installing landing discipline with a logged change.

92. Rerere/autostash behavior is missing.
   If `rerere.enabled` or `rebase.autoStash` is set, Git may auto-resolve or stash changes in ways the scenarios do not catch. Add a scenario requiring `land.sh` neutralizes or logs relevant Git config so "no manual conflict" and "dirty refused" remain true.

93. Main red after successful push should be impossible and tested.
   Success scenarios check regen drift, but not the full gate after pushing to a fresh clone. Add a scenario requiring the final pushed main passes the same gate bundle, or explicitly justifies why only regen drift is checked post-push.

94. Local gate versus CI gate parity is under-specified.
   The plan references `contracts-sync` and local `regen-all.sh --check`; scenarios do not pin which full gate commands land.sh runs. Add a behavior that enumerates gate families and fails if a required local pre-push gate is omitted from land.sh.

95. Landed commit provenance is incomplete.
   Add a scenario requiring the final main commits or audit log to record original branch name, original tip SHA, base SHA, land start/end times, lane identity, and whether regen was squashed or committed separately.

96. No behavior covers branches that modify `.gitignore` or generated ignore rules.
   Add a scenario where `.gitignore` changes would hide generated drift or lock files. Done should require generated and lock/audit paths remain visible to the checks regardless of branch-side ignore edits.

97. No scenario protects the private `_beads` nested repo.
   This repo contract says `_beads` is private and must not be committed. Add a scenario where a lane has changes under `_beads` or attempts to add `_beads` paths. Done should refuse or ignore them explicitly and prove `git add` in land.sh cannot stage private bead data.

98. `land.sh` self-tests for branch protection are out of scope but still a done-risk.
   B17 is sandbox-only. Add a done condition that real-origin branch protection/hook posture is at least checked and reported, even if administration remains out of scope. Otherwise the feature can pass sandbox tests while production remains bypassable.

99. Cross-platform assumptions are not stated.
   The repo appears macOS-oriented, but shell behavior can differ. Add scenarios or an explicit constraint for macOS-only vs Linux CI. Done should cover BSD/GNU differences for `stat`, `date`, `mktemp`, `flock`, and process checks if the script uses them.

100. The scenario suite lacks a coverage-to-failure-mode trace for newly added gaps.
   The current coverage map only maps B1-B18 to tonight's known failures. Add a requirement that every newly accepted gap scenario maps to one or more risk classes: preflight, lock, queue, rebase, regen, gate, push, recovery, observability, or harness safety.
