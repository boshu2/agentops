#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_path="$repo_root/scripts/check-worktree-disposition.sh"

if [[ ! -x "$script_path" ]]; then
    echo "script is not executable: $script_path" >&2
    exit 1
fi

tmpdir="$(mktemp -d)"
cleanup() {
    chmod -R u+w "$tmpdir" 2>/dev/null || true
    rm -rf "$tmpdir"
}
trap cleanup EXIT

init_repo() {
    local root="$1"

    git init -q -b main "$root"
    git -C "$root" config user.name "Codex"
    git -C "$root" config user.email "codex@example.com"
    printf '# test\n' >"$root/README.md"
    printf 'regular\n' >"$root/regular.txt"
    git -C "$root" add README.md regular.txt
    git -C "$root" commit -q -m "init"
}

run_gate() {
    local workdir="$1"

    (
        cd "$workdir"
        "$script_path"
    )
}

run_gate_with_feature_git_env() {
    local workdir="$1"

    (
        cd "$workdir"
        local git_dir
        local git_common_dir
        git_dir="$(git rev-parse --git-dir)"
        git_common_dir="$(git rev-parse --git-common-dir)"
        GIT_DIR="$git_dir" \
        GIT_WORK_TREE="$workdir" \
        GIT_COMMON_DIR="$git_common_dir" \
        "$script_path"
    )
}

run_gate_with_feature_git_dir_only_env() {
    local workdir="$1"

    (
        cd "$workdir"
        local git_dir
        local git_common_dir
        git_dir="$(git rev-parse --git-dir)"
        git_common_dir="$(git rev-parse --git-common-dir)"
        GIT_DIR="$git_dir" \
        GIT_COMMON_DIR="$git_common_dir" \
        "$script_path"
    )
}

assert_contains() {
    local haystack="$1"
    local needle="$2"

    if [[ "$haystack" != *"$needle"* ]]; then
        echo "expected output to contain: $needle" >&2
        echo "$haystack" >&2
        exit 1
    fi
}

canonical="$tmpdir/canonical"
feature="$tmpdir/feature"
foreign="$tmpdir/foreign"

init_repo "$canonical"
git -C "$canonical" worktree add -q -b codex/feature "$feature" main

output="$(run_gate "$feature")"
assert_contains "$output" "PASS: canonical root"
assert_contains "$output" "current branch codex/feature"

hook_output="$(run_gate_with_feature_git_env "$feature")"
assert_contains "$hook_output" "PASS: canonical root"
assert_contains "$hook_output" "current branch codex/feature"

hook_dir_only_output="$(run_gate_with_feature_git_dir_only_env "$feature")"
assert_contains "$hook_dir_only_output" "PASS: canonical root"
assert_contains "$hook_dir_only_output" "current branch codex/feature"

git -C "$canonical" switch --detach HEAD >/dev/null 2>&1
if detached_output="$(run_gate "$feature" 2>&1)"; then
    echo "expected detached canonical root to fail" >&2
    exit 1
fi
assert_contains "$detached_output" "is detached"
git -C "$canonical" switch main >/dev/null 2>&1

printf 'dirty\n' >>"$canonical/regular.txt"
if dirty_output="$(run_gate "$feature" 2>&1)"; then
    echo "expected dirty canonical root to fail" >&2
    exit 1
fi
assert_contains "$dirty_output" "has uncommitted changes"
assert_contains "$dirty_output" "Dirty paths from git status --porcelain"
assert_contains "$dirty_output" "Other dirty paths detected"
assert_contains "$dirty_output" "regular.txt"
git -C "$canonical" checkout -- regular.txt

mkdir -p "$canonical/cli/docs"
printf 'commands\n' >"$canonical/cli/docs/COMMANDS.md"
git -C "$canonical" add cli/docs/COMMANDS.md
git -C "$canonical" commit -q -m "add generated fixture"
printf 'dirty\n' >>"$canonical/cli/docs/COMMANDS.md"
if generated_dirty_output="$(run_gate "$feature" 2>&1)"; then
    echo "expected generated dirty canonical root to fail" >&2
    exit 1
fi
assert_contains "$generated_dirty_output" "Generated/gate-managed paths detected"
assert_contains "$generated_dirty_output" "cli/docs/COMMANDS.md"
git -C "$canonical" checkout -- cli/docs/COMMANDS.md

git -C "$canonical" worktree add -q -b codex/foreign "$foreign" main
if foreign_output="$(run_gate "$feature" 2>&1)"; then
    echo "expected foreign branch-attached worktree to fail" >&2
    exit 1
fi
assert_contains "$foreign_output" "unexpected branch-attached worktrees"
assert_contains "$foreign_output" "codex/foreign attached at "
assert_contains "$foreign_output" "/foreign"

echo "PASS: check-worktree-disposition.sh"
