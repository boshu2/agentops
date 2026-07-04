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

BASELINE="${SCRIPTS_AO_INVOCATIONS_BASELINE:-$ROOT/scripts/.scripts-ao-invocations-baseline}"

# Build (or reuse) the archive-tagged ao binary; sets + exports AO_BIN.
ao_snippet_resolve_bin "$ROOT" >/dev/null
if [[ -n "${AO_SNIPPET_TMP_DIR:-}" ]]; then
  trap 'rm -rf "$AO_SNIPPET_TMP_DIR"' EXIT
fi
export REPO_ROOT="$ROOT"
export AO_RESOLVE_MODE=strict
export SCRIPTS_AO_BASELINE="$BASELINE"

python3 - <<'PY'
import os
import pathlib
import re
import sys

sys.path.insert(0, os.environ["AO_SNIPPET_LIB_DIR"])
from ao_snippet_resolve import make_resolver_from_env

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
baseline_path = pathlib.Path(os.environ["SCRIPTS_AO_BASELINE"])

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


findings = {}     # rel-path -> [(lineno, sub, line), ...]
triggered = set()
for path in corpus():
    hits = scan_file(path)
    if not hits:
        continue
    rel = path.relative_to(repo_root).as_posix()
    findings[rel] = hits
    triggered.add(rel)

# ---- baseline ratchet (FILENAME-pinned, two-way) -----------------------------
baselined = set()
if baseline_path.exists():
    for line in baseline_path.read_text(encoding="utf-8").splitlines():
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        baselined.add(s)

new_offenders = sorted(triggered - baselined)
stale_baseline = sorted(baselined - triggered)

failed = False

if new_offenders:
    failed = True
    print("check-scripts-ao-invocations: FAIL — script(s) invoke a removed/unknown ao command:", file=sys.stderr)
    for f in new_offenders:
        for lineno, sub, line in findings[f]:
            print(f"  {f}:{lineno}: unknown ao command `ao {sub}` in `{line}`", file=sys.stderr)
    print("", file=sys.stderr)
    print("Fix the dead ao invocation (use the live subcommand), or — only if the "
          "invocation is deliberate — add `# ao-resolve: ignore` on the line, or "
          f"baseline the file in {baseline_path.name}.", file=sys.stderr)

if stale_baseline:
    failed = True
    print("check-scripts-ao-invocations: FAIL — baseline entr(ies) no longer trigger any finding (prune them):", file=sys.stderr)
    for f in stale_baseline:
        print(f"  {f}", file=sys.stderr)
    print("", file=sys.stderr)
    print(f"The allowlist only shrinks. Remove the above line(s) from {baseline_path.name}.", file=sys.stderr)

if failed:
    sys.exit(1)

n_active = len(baselined & triggered)
print(f"check-scripts-ao-invocations: PASS — no un-baselined script invokes a removed ao command "
      f"({len(triggered)} file(s) with findings, all baselined; {n_active} baseline entr(ies) still active).")
PY
