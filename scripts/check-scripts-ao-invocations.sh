#!/usr/bin/env bash
# check-scripts-ao-invocations.sh
#
# Resolve every LITERAL first-token `ao <sub>` invocation in an EXECUTABLE
# script (scripts/**/*.sh) or test (tests/**/*.{sh,bats}) against the live cobra
# command tree, and fail when a script calls a removed/renamed subcommand (the
# "stale-retired-surface" class — e.g. `ao rpi status`, `ao factory start`,
# `ao evolve` left behind after the command was archived/deleted; historical
# escapes: age-sydq, age-zei7). Sibling of scripts/validate-skill-cli-snippets.sh
# (skills prose) and scripts/check-docs-cli-snippets.sh (docs prose) — it SHARES
# their resolution core (scripts/lib/ao-snippet-resolve.*) rather than forking it,
# but it scans EXECUTABLE callers, not prose, so it extracts real shell
# invocations, not backtick spans.
#
# EXTRACTION — literal first-token invocations only (soundness over recall):
#   Recognized binary forms at a command position (line start, or after a shell
#   control operator `| || && ; ( ) { } $( ` `` `then`/`do`/`else`):
#       ao <sub>        "$AO_BIN" <sub>     "$AO" <sub>
#       $AO_BIN <sub>   $AO <sub>          ./cli/bin/ao <sub>   cli/bin/ao <sub>
#   Only the FIRST token after the binary is taken as the candidate subcommand,
#   and only when it is a literal wordish name (^[a-z][a-z0-9-]*$). Deliberately
#   SKIPPED (never flagged):
#     * comment lines (first non-blank char `#`)
#     * heredoc bodies (the whole `<<DELIM … DELIM` span, incl. `<<-` / quoted)
#     * dynamic dispatch — `ao "$var"`, `ao $sub`, `ao <bead>`, `ao ...`
#     * a flag as the first arg — `ao --version` (subcommands aren't scoped here)
#     * any line carrying the inline suppress pragma `# ao-resolve: ignore`
#       (use it on a deliberate bad-subcommand error-path test).
#
# Resolution is SOUND (AO_RESOLVE_MODE=strict → `ao <sub> --help`, reject cobra's
# "unknown command" / "Unknown help topic"; the `help`-mode predicate is unsound
# because `ao help <anything>` always exits 0). The archive-tagged build
# (`-tags "flywheel legacy"`) keeps archived-but-revivable commands (e.g.
# `ao harvest`, `ao forge`) resolvable, so this only flags TRULY dead commands.
#
# Baseline ratchet (scripts/.scripts-ao-invocations-baseline): FILENAME-pinned,
# seeded from every current offender. Two-way enforcement (same shape as the docs
# gate; allowlists only shrink):
#   (a) a NON-baselined script with a dead `ao <sub>` invocation → exit 1
#   (b) a baselined file that no longer triggers ANY finding      → exit 1 (prune)
#
# Exit: 0 clean · 1 offender / stale-baseline · 2 usage/setup error
#
# Env seams (for the bats twin): SCRIPTS_AO_INVOCATIONS_BASELINE overrides the
# baseline path; AGENTOPS_AO_BIN short-circuits the ao build (documented fast path).

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Shared build + resolver machinery. Resolve via the pre-cd absolutized
# $SCRIPT_DIR (a relative ${BASH_SOURCE[0]} would resolve wrongly after a cd).
# shellcheck source=scripts/lib/ao-snippet-resolve.sh
. "$SCRIPT_DIR/lib/ao-snippet-resolve.sh"
# Shared shrink-only ratchet mechanics: the baseline set arithmetic moved OUT
# of the python heredoc into bash so all ratchet gates share one implementation
# (age-ratchet-lib-extraction-bv7d.5; python now emits the offender findings
# only). Parse mode `strip` = the original python line.strip() behavior.
. "$SCRIPT_DIR/lib/ratchet.sh"

BASELINE="${SCRIPTS_AO_INVOCATIONS_BASELINE:-$ROOT/scripts/.scripts-ao-invocations-baseline}"

# Build (or reuse) the archive-tagged ao binary; sets + exports AO_BIN.
ao_snippet_resolve_bin "$ROOT" >/dev/null
if [[ -n "${AO_SNIPPET_TMP_DIR:-}" ]]; then
  trap 'rm -rf "$AO_SNIPPET_TMP_DIR"' EXIT
fi
export REPO_ROOT="$ROOT"
export AO_RESOLVE_MODE=strict

# python emits one record per finding: rel<US>lineno<US>sub<US>line
# (US = 0x1f unit separator — never occurs in paths or shell source lines we
# scan). Baseline arithmetic + messaging happen in bash via the ratchet lib.
findings_raw="$(python3 - <<'PY'
import os
import pathlib
import re
import sys

sys.path.insert(0, os.environ["AO_SNIPPET_LIB_DIR"])
from ao_snippet_resolve import make_resolver_from_env

repo_root = pathlib.Path(os.environ["REPO_ROOT"])

resolver = make_resolver_from_env()

PRAGMA = "# ao-resolve: ignore"

# A literal ao-binary token at a command position, followed by its first arg.
# The command-position prefix requires a real shell boundary before the binary
# (line start, a control operator, `$(`, a `(`/`{` group, `!`, or a leading shell
# keyword like `if`/`run`/`exec`) so an `ao` that is merely a word inside a string
# or another command's argument is NOT treated as an invocation. Group 1 is the
# binary token (its start feeds the in-a-string guard); group 2 is the first arg.
_AO_BIN = r'(?:ao|"\$AO_BIN"|"\$AO"|\$AO_BIN|\$AO|\./cli/bin/ao|cli/bin/ao)'
_KW = r'run|if|elif|while|until|then|do|else|command|exec|xargs|time|sudo|env'
_CMD_START = r'(?:^|[|;&(){}`]|\$\(|!|\b(?:' + _KW + r')\b)'
INVOKE = re.compile(_CMD_START + r'[ \t]*(' + _AO_BIN + r')[ \t]+(\S+)')
WORDISH = re.compile(r'^[a-z][a-z0-9-]*$')
HEREDOC_OPEN = re.compile(r'<<(-?)\s*(["\']?)([A-Za-z_][A-Za-z0-9_]*)\2')
_TOKEN_BREAK = re.compile(r'[|&;<>)`]')


def in_string(prefix):
    """Heuristic: an odd count of unescaped single/double quotes before the
    match means the `ao` token sits INSIDE a string literal (prose), not at a
    real command position — e.g. `echo "...(cli/bin/ao not built)"` or the
    markdown span `'for `ao evolve`:'`. Skip those (soundness over recall)."""
    return prefix.count("'") % 2 == 1 or prefix.count('"') % 2 == 1


def first_subcommand(raw):
    """The literal wordish subcommand from a captured first-arg token, or None
    for a flag / dynamic dispatch / placeholder (never flagged)."""
    tok = _TOKEN_BREAK.split(raw, 1)[0].strip()
    if not tok:
        return None
    if tok.startswith("-"):
        return None  # flag first arg — subcommands are what this gate scopes
    if not WORDISH.match(tok):
        return None  # $var / "$x" / <bead> / ... — dynamic or placeholder
    return tok


def scan_file(path):
    """Return list of (lineno, subcommand, line) for dead first-token ao calls."""
    findings = []
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return findings
    heredoc = None       # active closing delimiter, or None
    heredoc_strip = False
    for lineno, line in enumerate(text.splitlines(), start=1):
        if heredoc is not None:
            probe = line.lstrip("\t") if heredoc_strip else line
            if probe.strip() == heredoc:
                heredoc = None
            continue
        if PRAGMA in line:
            continue
        if line.lstrip().startswith("#"):
            continue
        for m in INVOKE.finditer(line):
            if in_string(line[:m.start(1)]):
                continue
            sub = first_subcommand(m.group(2))
            if sub is None:
                continue
            command, _ = resolver.resolve_command(["ao", sub])
            if command is None:
                findings.append((lineno, sub, line.strip()))
        # A heredoc opened on THIS line skips the body from the NEXT line on.
        hm = HEREDOC_OPEN.search(line)
        if hm:
            heredoc = hm.group(3)
            heredoc_strip = hm.group(1) == "-"
    return findings


# ---- corpus: executable scripts + tests --------------------------------------
def corpus():
    files = set()
    scripts_dir = repo_root / "scripts"
    tests_dir = repo_root / "tests"
    if scripts_dir.is_dir():
        files |= set(scripts_dir.rglob("*.sh"))
    if tests_dir.is_dir():
        files |= set(tests_dir.rglob("*.sh"))
        files |= set(tests_dir.rglob("*.bats"))
    return sorted(files)


for path in corpus():
    hits = scan_file(path)
    if not hits:
        continue
    rel = path.relative_to(repo_root).as_posix()
    for lineno, sub, line in hits:
        print(f"{rel}\x1f{lineno}\x1f{sub}\x1f{line}")
PY
)" || { echo "check-scripts-ao-invocations: python3 offender scan failed — cannot certify (environment error)" >&2; exit 2; }

US=$'\x1f'
baseline_name="$(basename "$BASELINE")"

# Distinct files with findings (python emission order preserves per-file order;
# sort -u here matches the original python sorted() over ASCII paths).
triggered="$(printf '%s\n' "$findings_raw" | awk -F"$US" 'NF { print $1 }' | LC_ALL=C sort -u | grep -v '^$' || [ $? -eq 1 ])"

# ---- baseline ratchet (FILENAME-pinned, two-way) via the shared lib ---------
# Empty triggered set is the happy path — guard it explicitly (a grep -v filter
# here would rc-1 under pipefail and fake an environment error).
new_offenders=""
if [[ -n "$triggered" ]]; then
  new_offenders="$(printf '%s\n' "$triggered" | ratchet_new_violations "$BASELINE" strip)" || exit 2
fi
stale_baseline="$(printf '%s\n' "$triggered" | ratchet_stale_entries "$BASELINE" strip)" || exit 2

failed=0

if [[ -n "$new_offenders" ]]; then
  failed=1
  echo "check-scripts-ao-invocations: FAIL — script(s) invoke a removed/unknown ao command:" >&2
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    while IFS="$US" read -r rel lineno sub line; do
      [[ "$rel" == "$f" ]] || continue
      echo "  $f:$lineno: unknown ao command \`ao $sub\` in \`$line\`" >&2
    done <<< "$findings_raw"
  done <<< "$new_offenders"
  echo "" >&2
  echo "Fix the dead ao invocation (use the live subcommand), or — only if the invocation is deliberate — add \`# ao-resolve: ignore\` on the line, or baseline the file in $baseline_name." >&2
fi

if [[ -n "$stale_baseline" ]]; then
  failed=1
  echo "check-scripts-ao-invocations: FAIL — baseline entr(ies) no longer trigger any finding (prune them):" >&2
  while IFS= read -r f; do
    [[ -n "$f" ]] && echo "  $f" >&2
  done <<< "$stale_baseline"
  echo "" >&2
  echo "The allowlist only shrinks. Remove the above line(s) from $baseline_name." >&2
fi

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

n_triggered="$(printf '%s' "$triggered" | grep -c . || true)"
n_pinned="$(ratchet_load_pinned "$BASELINE" strip | grep -c . || true)"
n_stale="$(printf '%s' "$stale_baseline" | grep -c . || true)"
n_active=$((n_pinned - n_stale))
echo "check-scripts-ao-invocations: PASS — no un-baselined script invokes a removed ao command ($n_triggered file(s) with findings, all baselined; $n_active baseline entr(ies) still active)."
