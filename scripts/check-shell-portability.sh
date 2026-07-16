#!/usr/bin/env bash
# check-shell-portability.sh — static guard against the one shell-portability
# bug class that has NO safe inline form: GNU `find -printf`.
#
# Why a static lint (not a runtime test): `find -printf` is a GNU-find extension.
# BSD/macOS `/usr/bin/find` errors on it ("unknown primary or operator") and
# exits non-zero — and under `set -euo pipefail` that suppressed failure
# propagates and breaks the script on macOS. But on Linux CI the runtime SUCCEEDS,
# so a functional test can't catch a reintroduction. Four real instances of this
# bug shipped undetected (age-iue5 #850, age-7jm6) precisely because no gate
# caught the class. This grep does — on every platform.
#
# Scope: ALL tracked first-party shell, not just *.sh under scripts/. By default
# it enumerates `git ls-files` (tracked only — so worktree copies, vendored, and
# gitignored trees are excluded for free) and selects shell scripts by EITHER a
# .sh/.bash suffix OR a shell shebang (so extensionless hooks like
# .githooks/* and bin/* are covered). tests/ is
# excluded: test fixtures legitimately embed the pattern as data.
#
# Precision: the token `-printf` (with a LEADING dash) is only ever a `find`
# primary; the command is `printf` (no dash). The search pattern is written
# `-print[f]` so this very file is not a literal self-match. Full-line comments
# are excluded so explanatory comments don't self-trip. Escape hatch: a trailing
# COMMENT marker `# portability-ok` (must follow a `#`, so it cannot be smuggled
# inside a string/variable) allows a deliberate, justified use.
#
# Other GNU/BSD divergences (sed -i, date -d, stat -c, readlink -f) are NOT
# linted here: each HAS a safe guarded form (uname / `date --version` / BSD-first
# fallback) already used across the tree, so a static grep would false-positive
# on correct code. This guard covers only the never-safe pattern.
#
# Usage: check-shell-portability.sh [--root <dir>]
#   --root <dir>  scan a directory tree directly (find-based) instead of the
#                 tracked set — used by the bats test and for non-git use.
# Exit:  0 clean · 1 found a non-portable `find -printf` · 2 usage error.
set -euo pipefail

ROOT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --root) ROOT="${2:-}"; shift 2 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "check-shell-portability: unknown arg: $1" >&2; exit 2 ;;
  esac
done

self="$(basename "$0")"

# Is $1 a shell script? .sh/.bash by suffix, else by shell shebang. Known
# non-shell extensions short-circuit so we don't read the head of every file.
is_shell() {
  case "$1" in
    *.sh|*.bash|*.zsh|*.ksh) return 0 ;;
    *.go|*.md|*.json|*.yaml|*.yml|*.txt|*.py|*.js|*.ts|*.tsx|*.jsx|*.html|*.css|\
    *.png|*.jpg|*.jpeg|*.gif|*.svg|*.ico|*.lock|*.mod|*.sum|*.toml|*.cfg|*.ini|\
    *.feature|*.bats|*.go.tmpl|*.tmpl|*.csv|*.tsv|*.pdf|*.gz|*.tar) return 1 ;;
    # Any POSIX-ish shell shebang (sh/bash/dash/zsh/ksh) — find is the same
    # binary regardless of shell, so all of them break on BSD `-printf`.
    *) head -1 "$1" 2>/dev/null | grep -qE '^#!.*\b(ba|z|k|da)?sh\b' ;;
  esac
}

# Validate --root in the MAIN shell: an exit inside the process-substitution
# subshell below would not abort the script (it would fall through to "0 found").
if [[ -n "$ROOT" && ! -d "$ROOT" ]]; then
  echo "check-shell-portability: root not found: $ROOT" >&2
  exit 2
fi

# Enumerate candidate files. Explicit --root => find-based (testing / non-git).
# Default => tracked files only (git ls-files excludes worktrees/vendored/ignored).
enumerate() {
  if [[ -n "$ROOT" ]]; then
    find "$ROOT" -type f 2>/dev/null
  elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git ls-files
  else
    find . -type f 2>/dev/null
  fi
}

# Order matters for speed over a few-thousand-file tracked set: is_shell's case
# fast-paths most files with NO subprocess; only then do the (pure-bash) self-name
# and the stat run, and only on the small shell-candidate subset.
shell_files=()
while IFS= read -r f; do
  [[ -n "$f" ]] || continue
  case "$f" in tests/*|*/tests/*) continue ;; esac   # test fixtures embed the pattern as data
  is_shell "$f" || continue
  # Exclude the linter by basename — intentionally ALL copies, not just this path:
  # any copy necessarily prints the literal pattern in its own error messages and
  # would false-positive on itself. (A non-linter file coincidentally named the
  # same and carrying a real `find -printf` is the accepted, contrived gap.)
  [[ "${f##*/}" == "$self" ]] && continue
  [[ -f "$f" ]] || continue
  shell_files+=("$f")
done < <(enumerate)

if [[ ${#shell_files[@]} -eq 0 ]]; then
  echo "OK shell-portability: 0 shell script(s) found to check."
  exit 0
fi

# One grep over the whole set (fast even as an always-run gate). `-nH` forces the
# FILE:LINENO: prefix uniformly; then drop full-line comments (`:NN:  #...`) and
# lines whose use is justified by a trailing `# portability-ok` COMMENT marker.
# The marker must directly follow a `#` (a comment hash), which rejects a bare
# `portability-ok` smuggled inside a string; trailing justification text after it
# is allowed (`# portability-ok: <reason>`). Fully distinguishing a comment `#`
# from a `#` inside a quoted string needs a shell parser — the residual (a real
# `find -printf` on the SAME line as a `#...portability-ok` STRING) is the
# accepted, self-defeating gap.
hits="$(grep -nHE -- '-print[f]' "${shell_files[@]}" 2>/dev/null \
          | grep -vE ':[0-9]+:[[:space:]]*#' \
          | grep -vE '#[[:space:]]*portability-ok' || true)"

if [[ -n "$hits" ]]; then
  echo "FAIL shell-portability: GNU-only \`find -printf\` found (breaks BSD/macOS find under set -e)." >&2
  printf '%s\n' "$hits" | sed 's/^/  /' >&2
  echo "fix: replace with a portable form, e.g. \`find ... -exec basename {} \\;\` for %f, or" >&2
  echo "     stat -f %m||stat -c %Y per file + a global \`sort -n | tail\` for %T@ mtime sorting." >&2
  echo "     (deliberate, justified use: append a trailing \`# portability-ok\` comment.)" >&2
  exit 1
fi

echo "OK shell-portability: ${#shell_files[@]} shell script(s) checked, no GNU-only \`find -printf\`."
