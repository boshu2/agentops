#!/usr/bin/env bash
# check-test-isolation.sh (ag-k38x) — ratchet: raw os.Chdir / os.Setenv in Go test
# files must not GROW.
#
# Idiomatic test isolation is t.Chdir / t.Setenv: they auto-restore on cleanup AND
# fail fast if the test (or a parent) called t.Parallel. Raw os.Chdir/os.Setenv mutate
# process-global cwd/env and are a latent flake landmine the moment any test opts into
# parallelism (cf. ag-jfzs — the memory-sync flake was exactly this class). This gate
# pre-empts the class by preventing NEW raw forms.
#
# This is a BASELINE RATCHET, not a zero-tolerance ban: ~163 os.Chdir + 22 os.Setenv
# inline sites predate it. The baseline only goes DOWN — new raw forms must use the
# t.* idiom (or the sanctioned chdirTemp/chdirTo helpers in cli/cmd/ao/testutil_test.go,
# which are excluded here and themselves use t.Chdir). Migrate inline sites to drive the
# baselines toward zero, then lower the constants below to lock each gain (ag-k38x.1).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Baselines captured 2026-06-03 (excludes the testutil_test.go helper home).
# CHDIR lowered 137->0 (age-cmd-ao-test-floor-hvb): the full cli/cmd/ao test suite was
# migrated off raw os.Chdir to t.Chdir (~62 blocks), which fixed a non-restoring chdir leak
# (orchestrate_test.go) that kept the package deterministically RED under -shuffle, plus
# internal/paths chdir helper. Cwd-behavior tests now use t.Chdir or isolated subprocesses.
# os.Setenv lowered 22->12 on 2026-06-06 (ag-k38x #bulk-migration). The remaining
# are intentional and cannot use t.Setenv: TestMain sites (no *testing.T), string-literal
# fixtures in fix_cliconfig_test.go, and unset-semantics helpers (t.Setenv cannot unset).
# 12->14 on 2026-06-25: cmd/ao/main_test.go TestMain sets+restores TMUX_TMPDIR to isolate
# the tmux socket (a test poisoned the dev's real tmux server with HOME=tempdir). TestMain
# has no *testing.T, so t.Setenv is unavailable — these two are intentional, like HOME above.
BASELINE_CHDIR=0
BASELINE_SETENV=10

chdir=$({ grep -rho --include='*_test.go' --exclude='testutil_test.go' 'os\.Chdir(' cli 2>/dev/null || true; } | wc -l | tr -d ' ')
setenv=$({ grep -rho --include='*_test.go' --exclude='testutil_test.go' 'os\.Setenv(' cli 2>/dev/null || true; } | wc -l | tr -d ' ')

rc=0
if [ "$chdir" -gt "$BASELINE_CHDIR" ]; then
  echo "FAIL: raw os.Chdir in *_test.go rose to $chdir (baseline $BASELINE_CHDIR)."
  echo "      Use t.Chdir (auto-restores, blocks t.Parallel) or the testutil chdirTemp/chdirTo helpers."
  rc=1
fi
if [ "$setenv" -gt "$BASELINE_SETENV" ]; then
  echo "FAIL: raw os.Setenv in *_test.go rose to $setenv (baseline $BASELINE_SETENV)."
  echo "      Use t.Setenv (auto-restores, blocks t.Parallel)."
  rc=1
fi

if [ "$chdir" -lt "$BASELINE_CHDIR" ] || [ "$setenv" -lt "$BASELINE_SETENV" ]; then
  echo "NOTE: isolation ratchet improved (os.Chdir=$chdir<=$BASELINE_CHDIR, os.Setenv=$setenv<=$BASELINE_SETENV)."
  echo "      Lower BASELINE_CHDIR/BASELINE_SETENV in $(basename "$0") to lock the gain."
fi

# --- git-exec env-scrub rule (age-gate-scripts-worktree-gitdir-p62wo) ---
# A *_test.go that shells out to git with cmd.Dir/-C but an INHERITED env is not
# isolated: git hooks export GIT_DIR, and under that leak a "scoped"
# `git config user.name Test` writes the LEAKED repo's shared config, and a
# no-dir-arg `git init --bare` sets core.bare=true there (both observed
# 2026-07-18; both empirically verified). House pattern: build the child env
# from os.Environ() minus GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR/GIT_INDEX_FILE
# (see gitDiscoveryEnv in cli/cmd/ao/git_read.go and the scrubbedGitEnv test
# helpers).
#
# WARN-only ratchet for now: 3 legacy files predate the rule (projectdir_test,
# check_runtime_test, measure_cwd_test — read-mostly, but the init calls are
# still exposed). Flip to FAIL (rc=1) once GIT_WARN_BASELINE reaches 0 and
# stays there — like BASELINE_CHDIR above, the count only goes DOWN.
GIT_WARN_BASELINE=3
git_offenders=""
while IFS= read -r f; do
  # A file that execs git must carry an env-scrub marker: its own scrubbed-env
  # helper, the cmd/ao production helper, or an explicit Env assignment.
  if ! grep -qE 'scrubbedGitEnv|gitDiscoveryEnv|\.Env = ' "$f"; then
    git_offenders="$git_offenders$f"$'\n'
  fi
done < <(grep -rl --include='*_test.go' 'exec\.Command(\(ctx, \)\?"git"' cli 2>/dev/null || true)
git_count=$(printf '%s' "$git_offenders" | grep -c . || true)
if [ "$git_count" -gt "$GIT_WARN_BASELINE" ]; then
  echo "FAIL: *_test.go files exec-ing git WITHOUT env scrubbing rose to $git_count (baseline $GIT_WARN_BASELINE):"
  printf '%s' "$git_offenders" | sed 's/^/      /'
  echo "      Build cmd.Env from os.Environ() minus GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR/GIT_INDEX_FILE"
  echo "      (copy the scrubbedGitEnv helper; see cli/cmd/ao/git_read.go gitDiscoveryEnv)."
  rc=1
elif [ "$git_count" -gt 0 ]; then
  echo "WARN: $git_count *_test.go file(s) exec git without env scrubbing (baseline $GIT_WARN_BASELINE, ratchet-only):"
  printf '%s' "$git_offenders" | sed 's/^/      /'
elif [ "$git_count" -lt "$GIT_WARN_BASELINE" ]; then
  echo "NOTE: git env-scrub ratchet improved ($git_count<=$GIT_WARN_BASELINE). Lower GIT_WARN_BASELINE to lock the gain."
fi

if [ "$rc" -eq 0 ]; then
  echo "PASS: test-isolation ratchet (os.Chdir=$chdir/$BASELINE_CHDIR, os.Setenv=$setenv/$BASELINE_SETENV, git-unscrubbed=$git_count/$GIT_WARN_BASELINE)"
fi
exit "$rc"
