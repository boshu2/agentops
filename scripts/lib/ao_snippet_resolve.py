"""ao_snippet_resolve — shared resolution core for `ao …` CLI snippets in prose.

Extracted from scripts/validate-skill-cli-snippets.sh (age-gate-the-ungated-egwt.4)
so that BOTH the skills-tree snippet gate and the docs-tree snippet gate resolve
`ao` commands against the SAME live cobra tree with ONE piece of machinery — the
"share the core, don't fork it" contract.

What lives here (the reusable core):
  * snippet extraction from a line (backtick spans + a bare `ao …` line)
  * shell-token trimming at control operators / redirects
  * the longest-leading-wordish-run candidate resolution against `ao`'s help tree
  * flag checking against the resolved command's help text

What is INJECTED by the caller (the one axis the two gates differ on):
  * the *resolution predicate* — "does this command chain name a real command?"
    - RESOLVE_MODE="help"   → the ORIGINAL skills-validator semantics:
                              `ao help <chain>` and trust returncode == 0.
                              (Byte-for-byte behavior-preserving for that gate.)
    - RESOLVE_MODE="strict" → a SOUND check for the docs gate:
                              `ao <chain> --help` and reject output that carries
                              cobra's "unknown command" / "Unknown help topic".
                              The `help`-mode predicate is unsound here because
                              `ao help <anything>` ALWAYS exits 0 (even for a
                              removed command like `ao factory` / `ao rpi`), so it
                              cannot detect the dead-command class the docs gate
                              exists to kill.

The AO binary path comes from the AO_BIN env var (set by the bash driver, which
builds `ao` with `-tags "flywheel legacy"` so archived-but-revivable commands
still resolve — two prior escapes: the default spine build omits them).
"""

import os
import re
import shlex
import subprocess

_WORDISH = re.compile(r"^[a-z][a-z0-9-]*$")
_CONTROL_TOKENS = {"|", "||", "&&", ";", "&"}
_UNKNOWN_CMD = re.compile(r"unknown command|Unknown help topic", re.IGNORECASE)

# Passthrough command chains: commands that disable flag parsing and forward
# every arg/flag verbatim to a child process. `ao beads exec` (age-3mdu) forwards
# to the resolved tracker (bd or br), so flags after it (`--reason`, `--status`,
# `--type`, `-p`, `--description`, …) are the TRACKER's, never ao's — they will
# never appear in ao's own help, so snippet flag-validation must skip them.
_PASSTHROUGH_PREFIXES = (("beads", "exec"),)


class Resolver:
    """Resolve `ao` command chains against the live cobra tree.

    mode: "help" (byte-identical skills-validator semantics) or "strict"
    (sound `--help`-based check for the docs gate).
    """

    def __init__(self, ao_bin, mode="help"):
        self.ao_bin = ao_bin
        if mode not in ("help", "strict"):
            raise ValueError(f"unknown resolve mode: {mode!r}")
        self.mode = mode
        self._help_cache = {}

    # ---- resolution primitive (the one injected axis) ---------------------
    def _probe(self, command):
        """Return (ok, help_text). `ok` is True when `command` names a real
        command chain under `ao`. help_text is the command's help output (used
        for flag validation) — for the `help` mode it is exactly the original
        validator's `ao help <chain>` stdout so flag checks are unchanged."""
        key = tuple(command)
        if key in self._help_cache:
            return self._help_cache[key]
        if self.mode == "help":
            result = subprocess.run(
                [self.ao_bin, "help", *command],
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            ok = result.returncode == 0
            out = result.stdout
        else:  # strict
            result = subprocess.run(
                [self.ao_bin, *command, "--help"],
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            ok = not _UNKNOWN_CMD.search(result.stdout)
            out = result.stdout
        self._help_cache[key] = (ok, out)
        return ok, out

    def global_help(self):
        # Global help is always the root command's help; identical in both modes
        # (`ao help` == `ao --help` content for flag lookups).
        return self._probe([])[1]

    # ---- shared machinery -------------------------------------------------
    @staticmethod
    def trim_shell_tokens(tokens):
        trimmed = []
        for token in tokens:
            if token in _CONTROL_TOKENS:
                break
            if token.startswith(("|", ">", "<")):
                break
            if token.endswith((";", "&&", "||")):
                trimmed.append(token.rstrip(";"))
                break
            trimmed.append(token)
        return [token for token in trimmed if token]

    def resolve_command(self, tokens):
        """Return (command, help_text) for the longest leading run of wordish
        subcommand tokens that names a real command, else (None, None)."""
        candidates = []
        for token in tokens[1:]:
            if token.startswith("-"):
                break
            if any(ch in token for ch in "<>[]{}=$("):
                break
            if not _WORDISH.match(token):
                break
            candidates.append(token)

        for end in range(len(candidates), 0, -1):
            candidate = candidates[:end]
            ok, help_text = self._probe(candidate)
            if ok:
                return candidate, help_text
        return None, None

    @staticmethod
    def normalize_flag(token):
        if "=" in token:
            token = token.split("=", 1)[0]
        return token

    @staticmethod
    def is_regex_like(tokens):
        return any(re.search(r"[\[\]\(\)\^\*\+\?]", token) for token in tokens[1:])

    @staticmethod
    def is_passthrough(command):
        """True when `command` is (or begins with) a flag-forwarding passthrough
        chain like `ao beads exec`. Its trailing flags are forwarded to a child
        (bd/br) and are never in ao's help, so callers must skip flag-checking."""
        if not command:
            return False
        ct = tuple(command)
        return any(ct[: len(p)] == p for p in _PASSTHROUGH_PREFIXES)


def iter_snippets(text):
    """Yield (lineno, snippet) for every `ao …` command in text.

    Covers BOTH backtick-delimited code spans (inline code) AND a line whose
    stripped form starts with `ao ` (a bare fenced-code-block command line) —
    the same extraction the skills validator uses. Plain prose that merely
    mentions "ao" without a code span or a leading `ao ` is NOT scanned (the
    false-positive guard the docs gate depends on).
    """
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


def make_resolver_from_env():
    """Build a Resolver from AO_BIN + AO_RESOLVE_MODE env vars."""
    ao_bin = os.environ["AO_BIN"]
    mode = os.environ.get("AO_RESOLVE_MODE", "help")
    return Resolver(ao_bin, mode=mode)
