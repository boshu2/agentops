# /ship-loop anti-patterns

Five observed patterns to avoid when shipping in the bot-paired fast lane.

## 1. Bundling pre-existing fixes

**Pattern:** Pre-push surfaces a WARN/FAIL in content you didn't change. Tempting to fix it inline since you're already there.

**Why it costs:** Other concurrent branches will hit the same pre-existing issue and apply the same fix inline. Result: 3 PRs each fixing the same line; the latter two are merge-conflict cleanup.

**Rule:** File the side-quest fix as its OWN atomic PR. Push it. Let it merge. Rebase your feature branch onto fresh main. Then continue your feature work.

**Evidence:** 2026-05-18 session — `../../AGENTS.md` broken link from PR #306 was fixed inline in PRs #322, #324, #325 before settling. Tracked as failure-mode F2.

## 2. Keeping copied variables after a rewrite

**Pattern:** Rewriting a script as a thin wrapper. The new code doesn't use `REPO_ROOT` / `TMP_DIR` / etc., but the rewrite preserves the old variable declarations "to be safe".

**Why it costs:** shellcheck SC2034 fires on the very next pre-push gate, requiring a cleanup PR.

**Rule:** After any script rewrite, the FIRST self-check is "are all top-level variable declarations referenced in the new body?" Run `shellcheck <path>` before commit, not after.

**Evidence:** PR #322 (`soc-3oij`) left `REPO_ROOT` unused; PR #325 cleaned it up. Tracked as failure-mode F1; mechanically closed by PR #326 (unconditional shellcheck on staged `.sh`).

## 3. Asserting local-only state in CI tests

**Pattern:** Writing a bats test that checks `[ -f .agents/learnings/<file>.md ]` to anchor the rationale.

**Why it costs:** `.agents/` is gitignored. The file does NOT exist in CI's fresh clone. The test fails in CI even though it passes locally.

**Rule:** Assert the rationale REFERENCE in the script body via `grep -q '<slug>' "$SCRIPT"`. Same intent (regression-guard the rationale link), doesn't depend on local state.

**Evidence:** Self-bug discovered mid-session on PR #326 + #329; both fixed in flight by replacing the file check with a grep on the script body.

## 4. Branches off out-of-date main

**Pattern:** Creating a feature branch when `git log main..HEAD` shows you're behind origin.

**Why it costs:** The branch needs `update-branch` immediately to catch up, and on multi-author repos the bot may attempt forward-ports of files it can't write (e.g., `.github/workflows/claude.yml`), entering a self-revert loop.

**Rule:** `git checkout main && git pull --rebase` BEFORE creating the feature branch. If `git pull --rebase` fails due to local stash/dirt, stash + retry.

**Evidence:** PR #270 sat 6 days because its branch was 195+ files behind main; the bot's claude.yml forward-port hit the `workflows: write` perm gate, the bot reverted its own merge, and `claude-review` stayed failing. Fixed by force-pushing a locally-rebased branch.

## 5. Skipping the failing-test-first step

**Pattern:** Writing the implementation first, then adding tests after — or worse, writing tests that pass after the implementation without ever having failed.

**Why it costs:** False confidence. A test that has never failed is not regression-guarding; it might assert the wrong thing entirely.

**Rule:** Per `.claude/rules/{go,python}.md`: L2-first/L1-always. Write the test that demonstrates the failure (reproduces SC2034, or the path-traversal, or the empty-Raw bug). Confirm it fails for the right reason. Then write the minimal fix that makes it green.

**Evidence:** PR #326's commit body explicitly reproduced SC2034 with a synthetic fixture before adding the fix. PR #324's tests covered 10 path-traversal subcases each rejected with `errors.Is(err, ErrInvalidRunID)`.

## When you've violated one of these

Don't hide it. Note it in the commit body — "this PR also includes an inline fix for the F2 pre-existing-blocker (see PR #X); should have been atomic, ate the cost this time." Naming it keeps the discipline honest.
