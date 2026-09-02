#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_src="$repo_root/scripts/cherry-pick-wave.sh"

tmpdir="$(mktemp -d)"
cleanup() {
    chmod -R u+w "$tmpdir" 2>/dev/null || true
    /bin/rm -rf "$tmpdir"
}
trap cleanup EXIT

assert_contains() {
    local haystack="$1"
    local needle="$2"

    if [[ "$haystack" != *"$needle"* ]]; then
        echo "expected output to contain: $needle" >&2
        echo "$haystack" >&2
        exit 1
    fi
}

fake_repo="$tmpdir/repo"
mock_bin="$tmpdir/bin"
mkdir -p \
    "$fake_repo/scripts" \
    "$fake_repo/.claude/worktrees/agent-1" \
    "$mock_bin"

cp "$script_src" "$fake_repo/scripts/cherry-pick-wave.sh"
chmod +x "$fake_repo/scripts/cherry-pick-wave.sh"
printf 'sentinel\n' >"$fake_repo/.claude/worktrees/agent-1/sentinel.txt"

rm_log="$tmpdir/rm.log"
git_log="$tmpdir/git.log"

cat >"$mock_bin/git" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >>"$git_log"

case "\$*" in
  *"diff --quiet"*|*"diff --cached --quiet"*)
    exit 0
    ;;
  *"rev-parse --is-inside-work-tree"*)
    exit 0
    ;;
  *"rev-parse --abbrev-ref HEAD"*)
    if [[ "\$*" == *".claude/worktrees/agent-1"* ]]; then
      printf '%s\n' feature
    else
      printf '%s\n' main
    fi
    exit 0
    ;;
  *"merge-base main feature"*)
    printf '%s\n' deadbeef
    exit 0
    ;;
  *"rev-list --reverse deadbeef..feature"*)
    exit 0
    ;;
  *"worktree remove --force"*)
    printf '%s\n' "git worktree remove forced failure" >&2
    exit 1
    ;;
  *"worktree prune"*)
    exit 0
    ;;
esac

printf '%s\n' "unexpected git invocation: \$*" >&2
exit 1
EOF
chmod +x "$mock_bin/git"

cat >"$mock_bin/rm" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >>"$rm_log"
printf '%s\n' "rm fallback should not be used" >&2
exit 99
EOF
chmod +x "$mock_bin/rm"

run_cleanup() {
    (
        cd "$fake_repo"
        PATH="$mock_bin:$PATH" bash "$fake_repo/scripts/cherry-pick-wave.sh" --cleanup-only --yes
    )
}

cleanup_output="$(run_cleanup 2>&1)"
assert_contains "$cleanup_output" "Cleaning up worktrees..."
assert_contains "$cleanup_output" "Removed agent-1 (no changes)"
assert_contains "$cleanup_output" "Done."

if [[ -e "$rm_log" ]]; then
    echo "expected rm fallback not to run" >&2
    cat "$rm_log" >&2
    exit 1
fi

if [[ ! -f "$fake_repo/.claude/worktrees/agent-1/sentinel.txt" ]]; then
    echo "expected cleanup to leave the path in place when git worktree remove fails" >&2
    exit 1
fi

if pattern_output="$(cd "$fake_repo" && bash "$fake_repo/scripts/cherry-pick-wave.sh" --pattern '../escape' 2>&1)"; then
    echo "expected invalid pattern to fail" >&2
    exit 1
fi
assert_contains "$pattern_output" "Invalid --pattern: must not contain '..' or '/'."

echo "PASS: cherry-pick-wave cleanup safety"
