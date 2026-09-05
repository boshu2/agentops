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
# A fenced code block is skipped ONLY when it quotes a COMMAND, because a
# command line is an example invocation rather than a claim about the tree:
#   * its info string is a shell (`bash`, `sh`, `shell`, `console`, `zsh`), or
#   * it has NO info string and its first non-blank line starts with `$`, with
#     `./`, or with a known command word.
# Every other fence is SCANNED — `text`, `json`, and any other data fence is
# exactly where a scorecard or a probe README shows its own layout, and
# exempting all fences let those claims outrun the tree unchecked.
#
# A code span's trailing sentence punctuation (`.`, `,`, `;`, `:`) is stripped
# before the token is treated as a path, so "lives at `scripts/x.sh`." resolves
# `scripts/x.sh` rather than a file named `scripts/x.sh.`.
#
# For each candidate path, relative to the repository root:
#   * exists on disk but is NOT tracked by git      -> offender: untracked
#   * does NOT exist on disk, and EITHER the same line uses one of the words
#     "published", "tracked", "committed", or "lives at" (case-insensitive) OR
#     the token sits inside a scanned DATA fence -> offender: missing.
#     A data fence is a record of what IS: a scorecard block's
#     `"status": "published"` and the path it names sit on different lines, so
#     matching claim words per line let the sentence outrun the tree. Inside a
#     data block every named path is a claim about the tree, full stop.
#   * anything else (tracked-and-present, or absent-with-no-claim-word, which
#     reads as a forward reference rather than a claim) is not an offender.
#
# Output: one line per offender, `file:line: path <state>`.
#
# Exit codes:
#   0 - clean (or nothing to scan)
#   1 - one or more offenders found
#   2 - fail-closed error (git missing/unusable, python3 missing, a scanned
#       file could not be read as UTF-8 text, or a `git ls-files` lookup that
#       answered neither "tracked" (0) nor "untracked" (1) — an unanswered
#       question is never a negative answer)
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
  sed -n '2,58p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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
    # A swallowed enumeration error certifies whatever it could not read. An
    # unreadable directory is a scan that did not happen, never a clean one, so
    # find's exit code and its stderr are both load-bearing here.
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
import os
import re
import subprocess
import sys

repo_root = sys.argv[1]
with open(sys.argv[2], "r", encoding="utf-8") as _fl:
    files = [line.rstrip("\n") for line in _fl if line.strip()]

BACKTICK_RE = re.compile(r"`([^`\n]+)`")
# Inside a DATA fence a repo path is usually a plain JSON or YAML string, not a
# code span: a scorecard writes "artifact": "evals/.../x.log". Scanning only
# backtick spans there left the commonest real shape unchecked.
QUOTED_RE = re.compile(r"\"([^\"\n]+)\"")
PREFIXES = ("evals/", "docs/evals/", "scripts/", "tests/")
CLAIM_WORDS = ("published", "tracked", "committed", "lives at")
FENCE_RE = re.compile(r"^\s{0,3}(`{3,}|~{3,})\s*(\S*)")
# A fence is exempt only when it quotes a COMMAND. These are the info strings
# that say so outright.
SHELL_INFO = frozenset({"bash", "sh", "shell", "console", "zsh"})
# ...and these are the first words that say so for an info-less fence. An
# allowlist, not a heuristic: "anything that looks like a word" would exempt
# every data fence and reopen the hole this rule closes.
COMMAND_WORDS = frozenset({
    "ao", "bash", "bats", "bd", "br", "bv", "cat", "cd", "chmod", "cp", "curl",
    "docker", "echo", "export", "find", "git", "go", "grep", "helm", "jq",
    "kubectl", "ls", "make", "mkdir", "mv", "node", "npm", "npx", "printf",
    "python", "python3", "rg", "rm", "sed", "sh", "shellcheck", "sort", "tar",
    "touch", "uv", "yarn", "zsh",
})
# Sentence punctuation that follows a path inside a code span.
TRAILING_PUNCTUATION = ".,;:"


def command_fence(first_line: str) -> bool:
    """True when an info-less fence's first content line is a command."""
    stripped = first_line.strip()
    if not stripped:
        return False
    if stripped.startswith("$") or stripped.startswith("./"):
        return True
    return stripped.split()[0] in COMMAND_WORDS


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
    # 0 is "tracked" and 1 is "not tracked"; anything else is git failing to
    # answer. Reading a failure as "untracked" would report a false offender
    # and, worse, would let a real answer be replaced by a broken tool.
    if result.returncode not in (0, 1):
        print(
            f"git ls-files failed for {relpath} (exit {result.returncode})",
            file=sys.stderr,
        )
        sys.exit(2)
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
    # PASS 1: classify each line as skipped, data-fence, or prose, and group the
    # prose into blank-line-delimited paragraphs. The claim words are a property
    # of the PARAGRAPH, not of the line: "the log is published at the\npath `x`"
    # wrapped across two lines and the claim went blind.
    KIND_SKIP, KIND_DATA, KIND_PROSE = 0, 1, 2
    kinds = [KIND_SKIP] * len(lines)
    paragraph_of = [-1] * len(lines)
    paragraphs = []
    fence_marker = None
    fence_decided = None
    current = -1
    for index, line in enumerate(lines):
        opener = FENCE_RE.match(line)
        if fence_marker is None:
            if opener is not None:
                fence_marker = opener.group(1)[0]
                fence_info = opener.group(2).split(",")[0].strip().lower()
                # A shell fence is exempt on its info string alone; an
                # info-less fence waits for its first content line.
                fence_decided = True if fence_info in SHELL_INFO else (
                    None if fence_info == "" else False
                )
                current = -1
                continue
        else:
            if opener is not None and opener.group(1)[0] == fence_marker:
                fence_marker = None
                fence_decided = None
                continue
            if fence_decided is None:
                fence_decided = command_fence(line)
            if not fence_decided:
                kinds[index] = KIND_DATA
            continue
        if not line.strip():
            current = -1
            continue
        if current == -1:
            paragraphs.append("")
            current = len(paragraphs) - 1
        paragraphs[current] += line.lower()
        kinds[index] = KIND_PROSE
        paragraph_of[index] = current

    paragraph_claims = [
        any(word in text for word in CLAIM_WORDS) for text in paragraphs
    ]

    # PASS 2: resolve the candidates each line carries.
    for lineno, line in enumerate(lines, start=1):
        kind = kinds[lineno - 1]
        if kind == KIND_SKIP:
            continue
        # A data block is a record of what IS, so every path it names is a
        # claim; in prose the claim is the paragraph the path sits in.
        claims = kind == KIND_DATA or paragraph_claims[paragraph_of[lineno - 1]]
        raw_tokens = list(BACKTICK_RE.findall(line))
        if kind == KIND_DATA:
            raw_tokens += QUOTED_RE.findall(line)
        for raw_tok in raw_tokens:
            # A path at the end of a sentence carries the sentence's
            # punctuation inside the span; strip it before resolving.
            tok = raw_tok.strip().rstrip(TRAILING_PUNCTUATION)
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
