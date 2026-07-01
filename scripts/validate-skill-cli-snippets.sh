#!/usr/bin/env bash
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
    # Build with the ADR-0012 archive tags so snippets that document
    # archived-but-revivable commands (e.g. `ao harvest`, `ao forge`, behind
    # //go:build flywheel|legacy) still validate against the full command
    # surface. The default `ao` omits them, but a skill may legitimately
    # reference any command; validating only the spine would false-fail those.
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
import os
import pathlib
import re
import shlex
import subprocess
import sys

repo_root = pathlib.Path(os.environ["REPO_ROOT"])
ao_bin = os.environ["AO_BIN"]
roots = [repo_root / "skills", repo_root / "skills-codex"]
allowed_suffixes = {".md", ".sh"}
wordish = re.compile(r"^[a-z][a-z0-9-]*$")
control_tokens = {"|", "||", "&&", ";", "&"}
stale_beads_resolver = re.compile(r"BEADS_DIR=\$PWD/_beads|git -C _beads|git add \.beads|git add _beads")
stale_beads_allowed = re.compile(r"\b(anti-pattern|do not|don't|must not|never|reject|fails?|historical|retired)\b", re.IGNORECASE)

failures = []
help_cache = {}

def iter_snippets(path: pathlib.Path):
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return

    for lineno, line in enumerate(text.splitlines(), start=1):
        if "ao " not in line:
            continue

        snippets = []
        for match in re.finditer(r"`([^`]*\bao\b[^`]*)`", line):
            snippets.append(match.group(1).strip())

        stripped = line.strip()
        if stripped.startswith("ao "):
            snippets.append(stripped)

        for snippet in snippets:
            yield lineno, snippet

def command_help(command):
    key = tuple(command)
    if key not in help_cache:
        result = subprocess.run(
            [ao_bin, "help", *command],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        help_cache[key] = result
    return help_cache[key]

def global_help():
    return command_help([])

def trim_shell_tokens(tokens):
    trimmed = []
    for token in tokens:
        if token in control_tokens:
            break
        if token.startswith(("|", ">", "<")):
            break
        if token.endswith((";", "&&", "||")):
            trimmed.append(token.rstrip(";"))
            break
        trimmed.append(token)
    return [token for token in trimmed if token]

def resolve_command(tokens):
    candidates = []
    for token in tokens[1:]:
        if token.startswith("-"):
            break
        if any(ch in token for ch in "<>[]{}=$("):
            break
        if not wordish.match(token):
            break
        candidates.append(token)

    for end in range(len(candidates), 0, -1):
        candidate = candidates[:end]
        result = command_help(candidate)
        if result.returncode == 0:
            return candidate, result.stdout
    return None, None

def normalize_flag(token):
    if "=" in token:
        token = token.split("=", 1)[0]
    return token

def is_regex_like(tokens):
    return any(re.search(r"[\[\]\(\)\^\*\+\?]", token) for token in tokens[1:])

def validate_snippet(path: pathlib.Path, lineno: int, snippet: str):
    try:
        tokens = shlex.split(snippet)
    except ValueError:
        return

    tokens = trim_shell_tokens(tokens)
    if not tokens or tokens[0] != "ao":
        return

    if is_regex_like(tokens):
        return

    command, help_text = resolve_command(tokens)
    if not command:
        if len(tokens) == 1:
            return
        if all(token.startswith("-") for token in tokens[1:]):
            help_text = global_help().stdout
            for flag in tokens[1:]:
                normalized = normalize_flag(flag)
                if normalized not in help_text:
                    failures.append(
                        f"{path.relative_to(repo_root)}:{lineno}: flag {normalized} not found in help for ao"
                    )
            return
        failures.append(f"{path.relative_to(repo_root)}:{lineno}: unknown ao command in snippet: {snippet}")
        return

    flags = []
    for token in tokens[1 + len(command):]:
        if not token.startswith("-"):
            continue
        if len(token) > 1 and token[1:].isdigit():
            continue
        flags.append(normalize_flag(token))
    for flag in flags:
        if flag not in help_text:
            failures.append(
                f"{path.relative_to(repo_root)}:{lineno}: flag {flag} not found in help for {' '.join(['ao', *command])}"
            )

for root in roots:
    if not root.exists():
        continue
    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        if path.suffix not in allowed_suffixes:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for lineno, line in enumerate(text.splitlines(), start=1):
            if stale_beads_resolver.search(line) and not stale_beads_allowed.search(line):
                failures.append(
                    f"{path.relative_to(repo_root)}:{lineno}: stale beads resolver; use BEADS_DIR=\"$(ao beads dir)\" and git -C \"$(ao beads dir)\""
                )
        for lineno, snippet in iter_snippets(path):
            validate_snippet(path, lineno, snippet)

if failures:
    print("Skill CLI snippet validation FAILED:", file=sys.stderr)
    for failure in failures:
        print(f"  {failure}", file=sys.stderr)
    sys.exit(1)

print("Skill CLI snippet validation passed.")
PY
