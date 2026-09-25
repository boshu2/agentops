#!/usr/bin/env bash
# check-doc-claims-tracked.sh — docs.claims-tracked gate.
#
# WHY: on 2026-09-03 an eval doc cited `evals/skill-probes/egress.log` as
# published while the repository's `*.log` ignore rule kept that exact file out
# of the tree, and a fresh verifier accepted the absent file. A path an eval
# doc names is a claim about the repository; this gate holds it to git.
#
# WHAT (facts only; no reading of intent): in every Markdown file under
# `evals/` and `docs/evals/`, every backtick span and every double-quoted
# string is a CANDIDATE path when, after trailing `.,;:` is stripped, it:
#   * contains a `/` and no whitespace;
#   * does not start with `<`, `$`, `~`, or `http` (placeholder, variable,
#     home-relative path, URL);
#   * contains no `*` or `{` (a glob, not one file);
#   * starts with `evals/`, `docs/evals/`, `scripts/`, or `tests/`.
# Each candidate is resolved against git, in this order:
#   tracked by git                         -> fine
#   ignored by the repository's ignore rules -> offender: ignored
#   present on disk                        -> offender: untracked
#   otherwise (missing, not ignored)       -> fine (a forward or historical
#                                             reference; reviewer territory)
# Only the repository's own ignore rules count (.gitignore files and
# .git/info/exclude); a developer's global excludes file is ignored so the
# verdict matches CI.
#
# Output: one line per offender, `file:line: path <state>`.
#
# Exit codes:
#   0 - clean (or nothing to scan)
#   1 - one or more offenders found
#   2 - fail-closed error (not a git repository, an unreadable tree or file,
#       or a git lookup that answered neither yes (0) nor no (1))
#
# Usage:
#   bash scripts/check-doc-claims-tracked.sh
#   bash scripts/check-doc-claims-tracked.sh <repo-dir>   # check another checkout
#
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd git
require_cmd python3

PROG="check-doc-claims-tracked"
TARGET="${1:-$REPO_ROOT}"

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  sed -n '2,37p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit 0
fi

if ! git -C "$TARGET" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  printf '[%s] FAIL: %s is not a git repository — refusing to certify unscannable input\n' "$PROG" "$TARGET" >&2
  exit 2
fi

work=""
with_tmpdir work claims-tracked
file_list="$work/files.txt"
: > "$file_list"
find_errors="$work/find-errors.txt"
: > "$find_errors"
for base in "evals" "docs/evals"; do
  if [ -d "$TARGET/$base" ]; then
    # An unreadable directory is a scan that did not happen, never a clean one,
    # so find's exit code and its stderr are both load-bearing here.
    set +e
    portable_find "$TARGET/$base" -type f -name '*.md' >> "$file_list" 2>>"$find_errors"
    find_rc=$?
    set -e
    if [ "$find_rc" -ne 0 ]; then
      printf '[%s] FAIL: could not enumerate %s (find exit %s)\n' "$PROG" "$TARGET/$base" "$find_rc" >&2
      [ -s "$find_errors" ] && cat "$find_errors" >&2
      exit 2
    fi
  fi
done
if [ -s "$find_errors" ]; then
  printf '[%s] FAIL: could not enumerate every file under %s\n' "$PROG" "$TARGET" >&2
  cat "$find_errors" >&2
  exit 2
fi
sort -u -o "$file_list" "$file_list"

scanned=$(wc -l < "$file_list" | tr -d ' ')
if [ "$scanned" -eq 0 ]; then
  printf '[%s] OK: no Markdown files under evals/ or docs/evals/ in %s — nothing to check\n' "$PROG" "$TARGET"
  exit 0
fi

set +e
output="$(python3 - "$TARGET" "$file_list" <<'PY'
import functools
import os
import re
import subprocess
import sys

root = sys.argv[1]
with open(sys.argv[2], "r", encoding="utf-8") as listing:
    files = [line.rstrip("\n") for line in listing if line.strip()]

TOKEN_RES = (re.compile(r"`([^`\n]+)`"), re.compile(r"\"([^\"\n]+)\""))
PREFIXES = ("evals/", "docs/evals/", "scripts/", "tests/")


def is_candidate(tok):
    return (
        "/" in tok
        and not any(ch.isspace() for ch in tok)
        and not tok.startswith(("<", "$", "~"))
        and not tok.lower().startswith("http")
        and "*" not in tok
        and "{" not in tok
        and tok.startswith(PREFIXES)
    )


def git_says_yes(*args):
    # 0 is yes and 1 is no; anything else is git failing to answer, and an
    # unanswered question is never a negative answer.
    rc = subprocess.run(
        ["git", "-C", root, "-c", "core.excludesFile=/dev/null", *args],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    ).returncode
    if rc not in (0, 1):
        print(f"git {args[0]} failed for {args[-1]} (exit {rc})", file=sys.stderr)
        sys.exit(2)
    return rc == 0


@functools.lru_cache(maxsize=None)
def offence(path):
    if git_says_yes("ls-files", "--error-unmatch", "--", path):
        return None
    if git_says_yes("check-ignore", "--no-index", "-q", "--", path):
        return "ignored"
    return "untracked" if os.path.lexists(os.path.join(root, path)) else None


offenders = []
for name in files:
    try:
        with open(name, "r", encoding="utf-8") as fh:
            lines = fh.readlines()
    except (OSError, UnicodeDecodeError) as exc:
        print(f"could not read {name}: {exc}", file=sys.stderr)
        sys.exit(2)
    rel = os.path.relpath(name, root)
    for lineno, line in enumerate(lines, start=1):
        for token_re in TOKEN_RES:
            for raw in token_re.findall(line):
                tok = raw.strip().rstrip(".,;:")
                state = offence(tok) if is_candidate(tok) else None
                if state:
                    offenders.append(f"{rel}:{lineno}: {tok} {state}")

for offender in offenders:
    print(offender)
sys.exit(1 if offenders else 0)
PY
)"
rc=$?
set -e

if [ "$rc" -eq 2 ]; then
  printf '%s\n' "$output" >&2
  printf '[%s] FAIL: could not scan one or more files — see above\n' "$PROG" >&2
  exit 2
fi

if [ "$rc" -eq 1 ]; then
  printf '[%s] FAIL: eval docs name paths git does not hold:\n' "$PROG" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

printf '[%s] OK: %s Markdown file(s) under evals/ and docs/evals/ name no ignored or untracked path\n' "$PROG" "$scanned"
exit 0
