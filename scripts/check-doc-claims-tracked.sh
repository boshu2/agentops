#!/usr/bin/env bash
# check-doc-claims-tracked.sh — docs.claims-tracked gate.
#
# WHY: on 2026-09-03 a doc sentence said an egress log was "published" while
# the repository's `*.log` ignore rule kept that exact file out of the tree,
# and the fresh verifier accepted the absent file because nothing checked the
# doc sentence against the working tree. Four separate judge rounds on that
# same day filed the identical finding class: "a doc sentence outruns the
# tree." A doc claiming a path is published, tracked, committed, or that it
# "lives at" some path is a factual claim about the repository, and nothing
# mechanical held it to the repository.
#
# WHAT: for every Markdown file under `evals/` and `docs/evals/` (this
# includes `evals/skill-probes/*.md` — no separate handling is needed, it is
# already inside `evals/`), scan every backtick-quoted inline token. A token
# is a CANDIDATE claim path when it:
#   * contains a `/` and no whitespace;
#   * does not start with `<`, `$`, `~`, or `http` (a placeholder, an
#     interpolated variable, a home-relative path, or a URL, never a claim
#     about THIS repository's tree);
#   * does not contain `*` or `{` (a glob pattern, not one file);
#   * starts with one of `evals/`, `docs/evals/`, `scripts/`, or `tests/` —
#     the four trees this gate can reason about.
# Text inside a fenced code block (``` ... ```) is skipped entirely: a fenced
# block quotes an example command, not a claim about the tree.
#
# For each candidate path, relative to the repository root:
#   * exists on disk but is NOT tracked by git      -> offender: untracked
#   * does NOT exist on disk, and the same line uses one of the words
#     "published", "tracked", "committed", or "lives at" (case-insensitive)
#     -> offender: missing
#   * anything else (tracked-and-present, or absent-with-no-claim-word, which
#     reads as a forward reference rather than a claim) is not an offender.
#
# Output: one line per offender, `file:line: path <state>`.
#
# Exit codes:
#   0 - clean (or nothing to scan)
#   1 - one or more offenders found
#   2 - fail-closed error (git missing/unusable, python3 missing, a scanned
#       file could not be read as UTF-8 text)
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
  sed -n '2,45p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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
for base in "evals" "docs/evals"; do
  if [ -d "$TARGET/$base" ]; then
    portable_find "$TARGET/$base" -type f -name '*.md' >> "$file_list" 2>/dev/null || true
  fi
done
sort -u -o "$file_list" "$file_list"

scanned=$(wc -l < "$file_list" | tr -d ' ')
if [ "$scanned" -eq 0 ]; then
  printf '[%s] OK: no Markdown files under evals/ or docs/evals/ in %s — nothing to check\n' "$PROG" "$TARGET"
  exit 0
fi

set +e
output="$(python3 - "$TARGET" "$file_list" <<'PY'
import os
import re
import subprocess
import sys

repo_root = sys.argv[1]
with open(sys.argv[2], "r", encoding="utf-8") as _fl:
    files = [line.rstrip("\n") for line in _fl if line.strip()]

BACKTICK_RE = re.compile(r"`([^`\n]+)`")
PREFIXES = ("evals/", "docs/evals/", "scripts/", "tests/")
CLAIM_WORDS = ("published", "tracked", "committed", "lives at")


def is_candidate(tok: str) -> bool:
    if not tok or " " in tok or "\t" in tok:
        return False
    if "/" not in tok:
        return False
    if tok.startswith(("<", "$", "~")):
        return False
    if tok.lower().startswith("http"):
        return False
    if "*" in tok or "{" in tok:
        return False
    return tok.startswith(PREFIXES)


def is_tracked(relpath: str) -> bool:
    result = subprocess.run(
        ["git", "-C", repo_root, "ls-files", "--error-unmatch", "--", relpath],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    return result.returncode == 0


offenders = []
for display_path in files:
    try:
        with open(display_path, "r", encoding="utf-8") as fh:
            lines = fh.readlines()
    except (OSError, UnicodeDecodeError) as exc:
        print(f"could not read {display_path}: {exc}", file=sys.stderr)
        sys.exit(2)

    rel_display = os.path.relpath(display_path, repo_root)
    in_fence = False
    for lineno, line in enumerate(lines, start=1):
        if line.strip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        lowered = line.lower()
        claims = any(word in lowered for word in CLAIM_WORDS)
        for raw_tok in BACKTICK_RE.findall(line):
            tok = raw_tok.strip()
            if not is_candidate(tok):
                continue
            abs_path = os.path.join(repo_root, tok)
            exists = os.path.exists(abs_path) or os.path.islink(abs_path)
            if exists:
                if not is_tracked(tok):
                    offenders.append(f"{rel_display}:{lineno}: {tok} untracked")
            elif claims:
                offenders.append(f"{rel_display}:{lineno}: {tok} missing")

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
  printf '[%s] FAIL: doc claims outrun the tree:\n' "$PROG" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

printf '[%s] OK: %s Markdown file(s) under evals/ and docs/evals/ make no untracked or missing path claims\n' "$PROG" "$scanned"
exit 0
