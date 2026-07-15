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

if [ "$rc" -eq 0 ]; then
  echo "PASS: test-isolation ratchet (os.Chdir=$chdir/$BASELINE_CHDIR, os.Setenv=$setenv/$BASELINE_SETENV)"
fi
exit "$rc"
