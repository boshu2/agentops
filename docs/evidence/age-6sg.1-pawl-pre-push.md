# age-6sg.1 pre-push pawl evidence — ao claim check resolves repo root from subdirectories

**Bead:** age-6sg.1 — Make `ao claim check` resolve the repo root from subdirectories.
**Mode:** fresh-context (default). Author context: `age-6sg1-claude-main`. Refuter context: a fresh-context subagent (distinct invocation).

## The bug

`runClaimCheck` resolved the registry via `resolveProjectDir()` (raw cwd), so `claimproof.Check` joined `cwd + docs/contracts/claim-registry.yaml`. Run from `cli/` (or any subdir), that became `cli/docs/contracts/claim-registry.yaml` → "not found". Only the repo root worked.

## The fix

`repoRootOrCwd()` (projectdir.go): resolves the effective project dir (honoring the `testProjectDir` hook), asks git for that dir's top-level via the existing `resolveRepoRoot(cwd)` (`git -C cwd rev-parse --show-toplevel`, with a clean `gitDiscoveryEnv`), and falls back to the dir itself when not in a repo / git unavailable. `runClaimCheck` now uses it. Correct from any subdir and inside linked worktrees. `claimBindingsPath` deliberately keeps `resolveProjectDir` (bindings are cwd-relative — only the repo-rooted registry read moved).

## Live evidence

```
# the bead's repro — now succeeds from subdirectories:
cd cli           && ao claim check --changed --base origin/main   # exit 0 (was: registry not found)
cd cli/cmd/ao    && ao claim check --changed --base origin/main   # exit 0
cd <repo root>   && ao claim check --changed --base origin/main   # exit 0 (unchanged)

go build ./... && go vet ./cmd/ao/ && go test ./cmd/ao/ -run 'Claim|RepoRootOrCwd'   # ok
ao gate check --fast --scope head                                                     # 26/26 pass, 0 fail
```

## Adversarial review (CONFIRMED)

A fresh-context reviewer verified and **CONFIRMED**:

- **Correctness end-to-end** — traced resolveProjectDir → resolveRepoRoot(cwd) (clean gitDiscoveryEnv strips inherited GIT_DIR/GIT_WORK_TREE; trailing newline trimmed); returns the repo root from `cli/` and `cli/internal/`, never `""`.
- **Live repro genuinely exercises the fixed path** — `claimproof.Check` calls `loadRegistry(repoRoot)` unconditionally before any claim is found; the reviewer reproduced the exact old error (`open .../cli/docs/contracts/claim-registry.yaml: no such file`) under raw-cwd, so the 0-claim green still proves resolution reached the registry.
- **Test bites** — reverting `repoRootOrCwd` to `resolveProjectDir()` makes `TestRepoRootOrCwd_ResolvesFromSubdirectory` fail (returns the subdir). EvalSymlinks comparison sound (/var→/private/var). Fallback test proves the no-repo case.
- **No root-case regression; linked-worktree edge correct** — from the repo root show-toplevel returns the root unchanged; inside a linked worktree it returns the worktree root (the right answer). Existing testProjectDir-based claim tests untouched.
