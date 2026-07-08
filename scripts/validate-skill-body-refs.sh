#!/usr/bin/env bash
# validate-skill-body-refs.sh — block stale `ao` command/flag references in
# skill+reference PROSE (inline-code spans), the complement of the fenced-block
# gate validate-skill-cli-snippets.sh.
#
# ag-4x8 — systemic fix for oracle gap #3. validate-skill-cli-snippets.sh only
# inspects fenced/`ao`-leading snippets; a stale command or flag named inside an
# inline `code span` mid-sentence (e.g. "use `ao context assemble`" or "`ao
# schedule` runs nightly") slips through. After every CLI rename those prose
# refs regenerate. This gate scans every inline-code span in the prose of
# SKILL.md + references/*.md across skills/ and skills-codex/ for `ao <command>`
# and `ao <command> --<flag>` tokens and validates each against the live `ao`
# help tree.
#
# Why inline-code spans only: a bare-prose "(if ao available)" or "ao not
# installed" is English, not a command invocation. Authors mark literal commands
# with backticks. Restricting to inline `code` is the precise signal that the
# author means a real command — it eliminates English false positives while
# still catching the genuine stale refs (which are always backtick-quoted).
# Fenced ``` blocks and lines that *start* with `ao ` are out of scope here —
# validate-skill-cli-snippets.sh owns those.
#
# Justified historical references are skipped two ways:
#   1. Per-line / per-file historical marker — a line (or the file's head)
#      containing one of: "Superseded", "Historical", "(historical)",
#      "removed in", "retained for history" (case-insensitive). A marked line is
#      skipped; a marker in the first 40 lines skips the whole file (migration
#      records / deprecation notes whose stale refs are the point).
#   2. scripts/skill-body-refs-allowlist.txt — minimal, commented allowlist of
#      repo-relative file paths (migration records, ADRs, release notes).
#
# Exit 0 when every inline-code ref resolves (or is justified); exit 1 with
# file:line on stale refs.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AO_BIN="${AGENTOPS_AO_BIN:-}"

if [[ -z "$AO_BIN" ]]; then
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  AO_BIN="$TMP_DIR/ao"
  (
    cd "$REPO_ROOT/cli"
    # Build with the ADR-0012 archive tags so skill body-refs that document
    # archived-but-revivable commands (e.g. `ao harvest`, `ao turn verify`,
    # behind //go:build flywheel|legacy) validate against the FULL command
    # surface. The default `ao` omits them, but a skill may legitimately
    # reference any command; validating only the spine would false-fail those.
    # (Same escape/fix as validate-skill-cli-snippets.sh, bead age-sydq.)
    go build -tags "flywheel legacy" -o "$AO_BIN" ./cmd/ao
  )
fi

[[ -x "$AO_BIN" ]] || {
  echo "Missing or non-executable ao binary: $AO_BIN" >&2
  exit 1
}

export AO_BIN
export REPO_ROOT

python3 - <<'PY'
import json
import os
import pathlib
import re
import subprocess
import sys

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
ao_bin = os.environ["AO_BIN"]

# Scan roots default to skills/ + skills-codex/. AGENTOPS_SKILL_BODY_ROOTS
# (colon-separated absolute paths) overrides them — used by the bats fixture to
# point the gate at a throwaway tree without mutating tracked skills.
roots_override = os.environ.get("AGENTOPS_SKILL_BODY_ROOTS", "").strip()
if roots_override:
    roots = [pathlib.Path(p) for p in roots_override.split(":") if p]
else:
    roots = [repo_root / "skills", repo_root / "skills-codex"]

HISTORICAL_MARKERS = (
    "superseded",
    "historical",
    "removed in",
    "retained for history",
)

allowlist_path = repo_root / "scripts" / "skill-body-refs-allowlist.txt"
allowlist = set()
if allowlist_path.is_file():
    for raw in allowlist_path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if line and not line.startswith("#"):
            allowlist.add(line)

WORDISH = re.compile(r"^[a-z][a-z0-9-]*$")


# ---------------------------------------------------------------------------
# Build the authoritative command tree from the live binary.
#
# Top-level names come from `ao capabilities` (the only strict source — cobra's
# `ao help <unknown>` returns 0 and cannot be trusted). Subcommands come from
# parsing help text, recognising BOTH the standard "Available Commands:" block
# AND grouped layouts (e.g. `ao goals` lists subcommands under category headers
# like "Measurement:" / "Analysis:"). Flags come from each command's --help.
# ---------------------------------------------------------------------------

def capabilities_top_level(ao):
    res = subprocess.run(
        [ao, "capabilities"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        check=False,
    )
    if res.returncode != 0:
        raise SystemExit("ao capabilities failed; cannot build command tree")
    data = json.loads(res.stdout)
    names = set()
    for group in data.get("command_groups", []):
        for cmd in group.get("commands", []):
            name = cmd.get("name")
            if name and name not in {"help", "completion"}:
                names.add(name)
    return names


def help_text(ao, path):
    res = subprocess.run(
        [ao, *path, "--help"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )
    return res.stdout


# A cobra command row: 2+ space indent, a lowercase command token, optional
# "(alias)", then whitespace and a description. Excludes flag rows (start with
# "-") and usage examples (skipped separately because they contain `ao `).
COMMAND_ROW = re.compile(r"^\s{2,}([a-z][a-z0-9-]*)(?:\s+\([a-z0-9-]+\))?\s+\S")
LONG_FLAG = re.compile(r"--[a-z][a-z0-9-]+")
# Cobra "Aliases:" block: "  ao foo\n\nAliases:\n  foo, f, fo".
ALIASES_LINE = re.compile(r"^Aliases:\s*$")


def parse_subcommands(text):
    subs = set()
    in_flags = False
    for line in text.splitlines():
        stripped = line.strip()
        # Once we hit a Flags section, stop treating rows as subcommands.
        if re.match(r"^(Flags|Global Flags):\s*$", stripped):
            in_flags = True
            continue
        if stripped.endswith(":") and not stripped.startswith("-"):
            # A new section header (Usage:, Measurement:, Examples:, ...).
            # Flag sections were handled above; a non-flag header re-opens
            # command parsing (covers grouped layouts like `ao goals`).
            in_flags = False
            continue
        if in_flags:
            continue
        # Skip usage/example lines that mention `ao ` (those are invocations,
        # not the command-listing rows).
        if "ao " in line:
            continue
        m = COMMAND_ROW.match(line)
        if m:
            name = m.group(1)
            if name not in {"help", "completion"}:
                subs.add(name)
    return subs


def parse_aliases(text):
    """Parse the `Aliases:` block of a command's help into alias names."""
    lines = text.splitlines()
    for idx, line in enumerate(lines):
        if ALIASES_LINE.match(line.strip()):
            if idx + 1 < len(lines):
                names = [n.strip() for n in lines[idx + 1].split(",")]
                return {n for n in names if WORDISH.match(n)}
            break
    return set()


def parse_flags(text):
    flags = set()
    for m in LONG_FLAG.finditer(text):
        flags.add(m.group(0))
    return flags


def build_tree(ao, max_depth=3):
    valid_paths = set()
    flags_by_path = {}
    flags_by_path[()] = parse_flags(help_text(ao, []))

    frontier = [(name,) for name in sorted(capabilities_top_level(ao))]
    for p in frontier:
        valid_paths.add(p)

    depth = 1
    while frontier and depth <= max_depth:
        nxt = []
        for path in frontier:
            text = help_text(ao, list(path))
            cmd_flags = parse_flags(text)
            flags_by_path[path] = cmd_flags
            # Register command aliases as alternate leaf paths sharing flags so
            # `ao quickstart` (alias of `quick-start`) resolves like the canonical
            # name. Aliases are leaves — we don't re-walk their subtree.
            for alias in parse_aliases(text):
                alias_path = path[:-1] + (alias,)
                if alias_path not in valid_paths:
                    valid_paths.add(alias_path)
                    flags_by_path.setdefault(alias_path, cmd_flags)
            for sub in parse_subcommands(text):
                child = path + (sub,)
                if child not in valid_paths:
                    valid_paths.add(child)
                    nxt.append(child)
        frontier = nxt
        depth += 1
    return valid_paths, flags_by_path


valid_paths, flags_by_path = build_tree(ao_bin)


def resolve_command(cmd_tokens):
    """Longest valid command-path prefix of cmd_tokens, or None."""
    for end in range(len(cmd_tokens), 0, -1):
        path = tuple(cmd_tokens[:end])
        if path in valid_paths:
            return path
    return None


# Passthrough command chains disable flag parsing and forward args/flags verbatim
# to a child process. `ao beads exec` (age-3mdu) forwards to the resolved tracker
# (bd or br), so flags after it are the tracker's, never ao's — skip flag-checking.
PASSTHROUGH_PREFIXES = (("beads", "exec"),)


def is_passthrough(command):
    if not command:
        return False
    ct = tuple(command)
    return any(ct[: len(p)] == p for p in PASSTHROUGH_PREFIXES)


def line_is_historical(line):
    low = line.lower()
    return "(historical)" in low or any(m in low for m in HISTORICAL_MARKERS)


def file_is_historical(text):
    head = "\n".join(text.splitlines()[:40]).lower()
    return "(historical)" in head or any(m in head for m in HISTORICAL_MARKERS)


# Within an inline `code span`, parse the `ao <cmd...> [--flag]` shape.
INLINE_AO = re.compile(
    r"\bao\s+"
    r"(?P<cmd>[a-z][a-z0-9-]*(?:\s+[a-z][a-z0-9-]*)*)"
    r"(?P<rest>\s+--[a-z][a-z0-9-]+.*)?"
)
INLINE_SPAN = re.compile(r"`([^`\n]+)`")


def iter_inline_ao_refs(line):
    """Yield (command_tokens, flag_or_None) for each `ao ...` inline-code span."""
    for span in INLINE_SPAN.finditer(line):
        content = span.group(1).strip()
        m = INLINE_AO.match(content)
        if not m:
            continue
        cmd_tokens = m.group("cmd").split()
        if not cmd_tokens:
            continue
        # First long flag after the command, if any.
        flag = None
        rest = m.group("rest")
        if rest:
            fm = re.search(r"(--[a-z][a-z0-9-]+)", rest)
            if fm:
                flag = fm.group(1)
        yield cmd_tokens, flag


failures = []

for root in roots:
    if not root.is_dir():
        continue
    targets = []
    for path in sorted(root.rglob("*")):
        if not path.is_file() or path.suffix != ".md":
            continue
        if path.name == "SKILL.md" or "references" in path.parts:
            targets.append(path)

    for path in targets:
        try:
            rel = str(path.relative_to(repo_root))
        except ValueError:
            # Fixture root outside repo_root (bats): fall back to absolute path
            # for reporting; the allowlist is repo-relative so it won't match.
            rel = str(path)
        if rel in allowlist:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        if file_is_historical(text):
            continue

        for lineno, line in enumerate(text.splitlines(), start=1):
            if "ao " not in line:
                continue
            if line_is_historical(line):
                continue
            for cmd_tokens, flag in iter_inline_ao_refs(line):
                resolved = resolve_command(cmd_tokens)
                if resolved is None:
                    failures.append(
                        f"{rel}:{lineno}: unknown ao command `ao {' '.join(cmd_tokens)}`"
                    )
                    continue
                # `ao beads exec …` forwards flags to the tracker (bd/br); its
                # flags are never in ao's help, so do not flag-check it.
                if is_passthrough(resolved):
                    continue
                if flag:
                    known = flags_by_path.get(resolved, set())
                    global_flags = flags_by_path.get((), set())
                    if flag not in known and flag not in global_flags:
                        failures.append(
                            f"{rel}:{lineno}: flag {flag} not found in help for "
                            f"`ao {' '.join(resolved)}`"
                        )

if failures:
    print("Skill BODY command-ref validation FAILED:", file=sys.stderr)
    seen = set()
    shown = 0
    for failure in failures:
        if failure in seen:
            continue
        seen.add(failure)
        print(f"  {failure}", file=sys.stderr)
        shown += 1
        if shown >= 100:
            remaining = len(set(failures)) - shown
            if remaining > 0:
                print(f"  ... and {remaining} more", file=sys.stderr)
            break
    print("", file=sys.stderr)
    print(
        "These `ao <command>`/`ao <command> --flag` inline-code prose refs do "
        "not resolve against the live CLI help tree.",
        file=sys.stderr,
    )
    print(
        "Fix the ref, or — if it is a justified migration record — add an "
        "explicit historical marker (Superseded / Historical / removed in / "
        "retained for history / (historical)) on the line/file, or list the "
        "file in scripts/skill-body-refs-allowlist.txt.",
        file=sys.stderr,
    )
    sys.exit(1)

print("Skill BODY command-ref validation passed.")
PY
