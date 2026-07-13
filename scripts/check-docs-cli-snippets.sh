#!/usr/bin/env bash
# check-docs-cli-snippets.sh
#
# Resolve every `ao …` command cited in a LIVE doc against the live cobra tree
# and fail if a doc names a command that does not exist (a removed/renamed
# command like `ao factory start`, `ao rpi phased`, `ao evolve` on a golden
# path). Port-style sibling of scripts/validate-skill-cli-snippets.sh — it
# SHARES that gate's resolution core (scripts/lib/ao-snippet-resolve.*) rather
# than forking it (age-gate-the-ungated-egwt.4).
#
# Scope = the shared LIVE-doc set (scripts/lib/docs-scope.sh: docs/**/*.md minus
# dated archives and self-declared-historical docs). Extraction covers BOTH
# fenced code blocks and inline code spans; plain prose is NOT scanned (the
# false-positive guard).
#
# Resolution is SOUND: unlike the skills gate's byte-identical `help`-mode
# predicate, this uses `ao <chain> --help` and rejects cobra's "unknown command"
# / "Unknown help topic" — because `ao help <anything>` ALWAYS exits 0, so the
# help-mode predicate cannot detect a removed command at all. The archive-tagged
# build (`-tags "flywheel legacy"`) keeps archived-but-revivable commands
# resolvable, so this only flags TRULY removed commands.
#
# Baseline ratchet (scripts/.docs-cli-snippets-baseline): FILENAME-pinned, seeds
# every current offender. Two-way enforcement:
#   (a) a NON-baselined live doc with a dead ao ref            → exit 1
#   (b) a baselined file that no longer triggers ANY finding   → exit 1 (prune it)
# Allowlists only ever shrink.
#
# Exit: 0 clean · 1 offender / stale-baseline · 2 usage/setup error

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Shared machinery. Resolve via the pre-cd absolutized $SCRIPT_DIR (a relative
# ${BASH_SOURCE[0]} would resolve wrongly after a cd — the .1 lane's pawl catch).
# shellcheck source=scripts/lib/docs-scope.sh
. "$SCRIPT_DIR/lib/docs-scope.sh"
# shellcheck source=scripts/lib/ao-snippet-resolve.sh
. "$SCRIPT_DIR/lib/ao-snippet-resolve.sh"
# Shared shrink-only ratchet mechanics: baseline set arithmetic moved OUT of
# the python heredoc into bash (age-ratchet-lib-extraction-bv7d.6; python now
# emits offender findings only). Parse mode `strip` = original line.strip().
# shellcheck source=scripts/lib/ratchet.sh
. "$SCRIPT_DIR/lib/ratchet.sh"

# Pin the docs scope root to THIS repo (the DOCS_ROOT env seam is for lib tests).
DOCS_ROOT="$ROOT"
export DOCS_ROOT

BASELINE="${DOCS_CLI_SNIPPETS_BASELINE:-$ROOT/scripts/.docs-cli-snippets-baseline}"

# Build (or reuse) the archive-tagged ao binary; sets + exports AO_BIN.
ao_snippet_resolve_bin "$ROOT" >/dev/null
if [[ -n "${AO_SNIPPET_TMP_DIR:-}" ]]; then
  trap 'rm -rf "$AO_SNIPPET_TMP_DIR"' EXIT
fi
export REPO_ROOT="$ROOT"
export AO_RESOLVE_MODE=strict

# python emits one record per finding: rel<US>lineno<US>token<US>suggestion<US>snippet
# (US = 0x1f; suggestion may be empty). Baseline arithmetic + messages live in
# bash via the ratchet lib.
findings_raw="$(python3 - <<'PY'
import os
import pathlib
import re
import shlex
import subprocess
import sys

sys.path.insert(0, os.environ["AO_SNIPPET_LIB_DIR"])
from ao_snippet_resolve import iter_snippets, make_resolver_from_env

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
docs_root = pathlib.Path(os.environ["DOCS_ROOT"])

resolver = make_resolver_from_env()

# ---- live-doc scope + historical exemption (shared bash lib, via subprocess) --
def _sh(func):
    """Call a docs-scope.sh function and return (rc, stdout)."""
    lib = pathlib.Path(os.environ["AO_SNIPPET_LIB_DIR"]) / "docs-scope.sh"
    script = f'. "{lib}"; {func}'
    r = subprocess.run(["bash", "-c", script], stdout=subprocess.PIPE,
                       stderr=subprocess.DEVNULL, text=True,
                       env={**os.environ})
    return r.returncode, r.stdout

def live_files():
    _, out = _sh("docs_scope_live_files")
    return [line for line in out.splitlines() if line.strip()]

def is_exempt(f):
    rc, _ = _sh(f'docs_scope_is_exempt "{f}"')
    return rc == 0

# ---- nearest-live-command suggestion -----------------------------------------
def _levenshtein(a, b):
    if a == b:
        return 0
    if not a:
        return len(b)
    if not b:
        return len(a)
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i]
        for j, cb in enumerate(b, 1):
            cur.append(min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (ca != cb)))
        prev = cur
    return prev[-1]

def _top_level_commands():
    # Parse `ao --help` for the visible top-level command names.
    _, out = resolver._probe([])
    names = []
    for line in out.splitlines():
        m = re.match(r"^  ([a-z][a-z0-9-]+)\s{2,}\S", line)
        if m:
            names.append(m.group(1))
    return sorted(set(names))

_TOP = None
def suggest(token):
    global _TOP
    if _TOP is None:
        _TOP = _top_level_commands()
    if not _TOP:
        return None
    best = min(_TOP, key=lambda c: _levenshtein(token, c))
    # only suggest a genuinely-near match
    if _levenshtein(token, best) <= max(2, len(token) // 2):
        return best
    return None

# A line that DESCRIBES a command's removal (negation / past-tense) is documenting
# the retirement, not prescribing the dead command — not an offender. Mirrors the
# REMOVAL_LANG exemption for retired-tech wording.
_REMOVAL_LANG = re.compile(
    r"no `ao|removed|retired|deleted|deprecat|superseded|no longer|is gone|are gone|"
    r"not a (?:selectable|valid)|deprecation pointer|gets? a deprecation",
    re.IGNORECASE,
)

# ---- resolve one snippet; return the offending token or None -----------------
def offending_token(snippet):
    try:
        tokens = shlex.split(snippet)
    except ValueError:
        return None
    tokens = resolver.trim_shell_tokens(tokens)
    if not tokens or tokens[0] != "ao":
        return None
    if resolver.is_regex_like(tokens):
        return None
    command, _ = resolver.resolve_command(tokens)
    if command:
        return None
    if len(tokens) == 1:
        return None
    if all(t.startswith("-") for t in tokens[1:]):
        return None  # all-flags form (e.g. `ao --version`); flags not scoped here
    # first wordish subcommand token is the offending name
    first = tokens[1]
    # skip placeholders / non-command tokens (e.g. `ao ...`, `ao <bead>`)
    if not re.match(r"^[a-z][a-z0-9-]*$", first):
        return None
    return first

# ---- scan ---------------------------------------------------------------------
for f in live_files():
    if is_exempt(f):
        continue
    p = docs_root / f
    try:
        text = p.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        continue
    lines = text.splitlines()
    for lineno, snippet in iter_snippets(text):
        tok = offending_token(snippet)
        if tok is None:
            continue
        src_line = lines[lineno - 1] if 0 <= lineno - 1 < len(lines) else ""
        if _REMOVAL_LANG.search(src_line):
            continue  # describing the removal, not prescribing the dead command
        sug = suggest(tok) or ""
        print(f"{f}\x1f{lineno}\x1f{tok}\x1f{sug}\x1f{snippet}")
PY
)" || { echo "check-docs-cli-snippets: python3 offender scan failed — cannot certify (environment error)" >&2; exit 2; }

US=$'\x1f'
baseline_rel="${BASELINE#"$ROOT"/}"

triggered="$(printf '%s\n' "$findings_raw" | awk -F"$US" 'NF { print $1 }' | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ])"

# ---- baseline ratchet (two-way) via the shared lib ---------------------------
new_offenders=""
if [[ -n "$triggered" ]]; then
  new_offenders="$(printf '%s\n' "$triggered" | ratchet_new_violations "$BASELINE" strip)" || exit 2
fi
stale_baseline="$(printf '%s\n' "$triggered" | ratchet_stale_entries "$BASELINE" strip)" || exit 2

failed=0

if [[ -n "$new_offenders" ]]; then
  failed=1
  echo "check-docs-cli-snippets: FAIL — live doc(s) cite a removed/unknown ao command:" >&2
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    while IFS="$US" read -r rel lineno tok sug snippet; do
      [[ "$rel" == "$f" ]] || continue
      hint=""
      [[ -n "$sug" ]] && hint=" (did you mean \`ao $sug\`?)"
      echo "  $f:$lineno: unknown ao command \`ao $tok\` in \`$snippet\`$hint" >&2
    done <<< "$findings_raw"
  done <<< "$new_offenders"
  echo "" >&2
  echo "Fix the dead ao reference (use the live equivalent, or historical wording), or — only if the page is a dyr0-lane golden path — add it to $baseline_rel." >&2
fi

if [[ -n "$stale_baseline" ]]; then
  failed=1
  echo "check-docs-cli-snippets: FAIL — baseline entr(ies) no longer trigger any finding (prune them):" >&2
  while IFS= read -r f; do
    [[ -n "$f" ]] && echo "  $f" >&2
  done <<< "$stale_baseline"
  echo "" >&2
  echo "The allowlist only shrinks. Remove the above line(s) from $baseline_rel." >&2
fi

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

n_triggered="$(printf '%s' "$triggered" | grep -c . || true)"
n_pinned="$(ratchet_load_pinned "$BASELINE" strip | grep -c . || true)"
n_stale="$(printf '%s' "$stale_baseline" | grep -c . || true)"
n_baselined=$((n_pinned - n_stale))
echo "check-docs-cli-snippets: PASS — no un-baselined live doc cites a removed ao command ($n_triggered file(s) with findings, all baselined; $n_baselined baseline entr(ies) still active)."
