#!/usr/bin/env bash
# fresh-install-conformance.sh — the integration net over the fresh-user
# onboarding path (bead age-wl5vm / FU1).
#
# It consumes the product the way a brand-new user receives it — a clean HOME,
# a fresh non-agentops git project, the ao binary, and the offline Codex
# bundle — and then exercises first-run: quick-start's advertised commands,
# doctor on a pristine install, and the installer's skill-count identity.
#
# Sibling beads already fixed the five staleness classes at the unit level
# (quick-start strings + cobra-tree guard, doctor audience calibration,
# installer self-tests, release parity, quick-start diet). This script is the
# integration net OVER them so the class cannot regrow: it exercises the REAL
# binary output, not in-process test doubles.
#
# Modes:
#   (default)                    tree mode — build ao from cli/ (release-ish
#                                dev version). Used by the push/nightly job.
#   --release-tarball <url|path> release mode — download/extract the published
#                                binary and exercise it. Used by the release
#                                job. Degrades LOUDLY when offline (SKIP, never
#                                a silent pass).
#
# Safety: every install runs against HOME=$(mktemp -d). The real $HOME is
# never written. The project dir is a throwaway `git init` OUTSIDE any
# agentops clone.
#
# Exit status: nonzero if any assertion FAILs. SKIPs do not fail the run.

# Adopt the shared preamble (strict mode + CWD-hijack-proof REPO_ROOT +
# with_tmpdir + require_cmd + portable helpers).
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

# This is a record-and-continue TEST HARNESS: it must run the whole fresh-user
# sequence to completion and report PASS/FAIL per assertion, so it deliberately
# relaxes `set -e` (a harness that aborts on the first failing assertion cannot
# report a summary). It keeps -u and pipefail from the preamble.
set +e

# ── output helpers ──────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  C_GREEN='\033[0;32m'; C_RED='\033[0;31m'; C_YELLOW='\033[0;33m'; C_NC='\033[0m'
else
  C_GREEN=''; C_RED=''; C_YELLOW=''; C_NC=''
fi

FAIL_COUNT=0
PASS_COUNT=0
SKIP_COUNT=0
declare -a SUMMARY_ROWS=()

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  SUMMARY_ROWS+=("PASS|$1")
  printf '  %bPASS%b %s\n' "$C_GREEN" "$C_NC" "$1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  SUMMARY_ROWS+=("FAIL|$1")
  printf '  %bFAIL%b %s\n' "$C_RED" "$C_NC" "$1"
  if [[ -n "${2:-}" ]]; then
    printf '       %boffending:%b %s\n' "$C_YELLOW" "$C_NC" "$2"
  fi
}

skip() {
  SKIP_COUNT=$((SKIP_COUNT + 1))
  SUMMARY_ROWS+=("SKIP|$1")
  printf '  %bSKIP%b %s\n' "$C_YELLOW" "$C_NC" "$1"
  if [[ -n "${2:-}" ]]; then
    printf '       %breason:%b %s\n' "$C_YELLOW" "$C_NC" "$2"
  fi
}

section() { printf '\n== %s ==\n' "$1"; }

die() { printf '%bfatal:%b %s\n' "$C_RED" "$C_NC" "$1" >&2; exit 2; }

# REPO_ROOT is exported by the preamble (anchored at the lib's own dir, so a
# release-tarball run outside a git checkout still resolves it).

RELEASE_TARBALL=""
REQUIRE_ASSET=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --release-tarball)
      RELEASE_TARBALL="${2:-}"
      [[ -n "$RELEASE_TARBALL" ]] || die "--release-tarball needs a url or path"
      shift 2
      ;;
    --require-asset)
      # CI release job: an unreachable published asset is the FAILURE this
      # job exists to catch — never a skip. Local offline runs omit this.
      REQUIRE_ASSET=1
      shift
      ;;
    --help|-h)
      sed -n '2,32p' "${BASH_SOURCE[0]}"
      exit 0
      ;;
    *)
      die "unknown arg: $1"
      ;;
  esac
done

MODE="tree"
[[ -n "$RELEASE_TARBALL" ]] && MODE="release"

require_cmd python3
require_cmd git

# ── scratch state (all under mktemp; real HOME untouched) ────────────────────
# with_tmpdir installs a single EXIT trap that removes every dir it made, so we
# must NOT register our own EXIT trap afterward.
with_tmpdir WORK conformance
with_tmpdir FRESH_HOME conformance-home
with_tmpdir PROJECT conformance-project
BIN="$WORK/bin"
mkdir -p "$BIN"

AO="$BIN/ao"
# Curated PATH for the doctor step: exactly one `ao` (ours) plus the system
# essentials. This is what proves the pristine-install-green invariant without
# a dev machine's shadowing `ao` copies tripping the build-integrity detector.
DOCTOR_PATH="$BIN:/usr/bin:/bin:/usr/sbin:/sbin"

# ── the extraction + validation helper (mirrors #907's advertised_commands) ──
PARSE="$WORK/parse.py"
cat > "$PARSE" <<'PY'
import json
import re
import sys

# Mirrors advertisedAoInvocationRE in cli/cmd/ao/advertised_commands.go: a token
# run ends at the first thing that is not a lowercase command word or a --flag,
# so placeholders (<topic>) and prose punctuation never leak in.
AO_RE = re.compile(
    r"""(?:^|[\s`"'($])ao ([a-z][a-z0-9-]*(?: (?:[a-z][a-z0-9-]*|--[a-z][a-z0-9-]*(?:=\S+)?))*)"""
)
BR_RE = re.compile(r"""(?:^|[\s`"'($])br ([a-z][a-z0-9-]*)""")
# A slash-command mention: preceded by start-or-space, a lowercase name, and
# NOT part of a filesystem path (no trailing word/slash/dot char). This keeps
# "/var/folders" and "docs/first-value-path.md" out of the skill set.
SKILL_RE = re.compile(r"(?:(?<=\s)|^)/([a-z][a-z0-9][a-z0-9-]*)(?![\w/.-])")

# English function words that can follow "ao" in help prose without naming a
# subcommand (copied from advertisedProseStopwords). A removed command name
# must never be added here.
STOPWORDS = {
    "a", "an", "and", "are", "as", "by", "can", "command", "commands", "does",
    "for", "if", "in", "is", "of", "on", "or", "that", "the", "this", "to",
    "was", "with",
}


def extract_ao(text):
    return AO_RE.findall(text)


def extract_br(text):
    return BR_RE.findall(text)


def extract_skills(text):
    out = []
    for line in text.splitlines():
        out.extend(SKILL_RE.findall(line))
    return out


def load_caps(path):
    with open(path, encoding="utf-8") as handle:
        data = json.load(handle)
    paths = {}
    for command in data.get("commands", []):
        full = command.get("path", "")
        if full == "ao":
            continue
        if full.startswith("ao "):
            full = full[3:]
        paths[full] = command.get("args") == "subcommands-only"
    return paths


def is_group(paths, path):
    # The root ("") only routes to subcommands.
    if path == "":
        return True
    return paths.get(path, False)


def validate_ao(inv, paths):
    """Port of validateAdvertisedAoInvocation. Returns None if the invocation
    resolves against the live command tree, else a human reason string."""
    tokens = inv.split()
    cur = ""
    i = 0
    while i < len(tokens):
        tok = tokens[i]
        if tok.startswith("-"):
            break
        key = (cur + " " + tok).strip()
        if key in paths:
            cur = key
            i += 1
            continue
        # No such child.
        if is_group(paths, cur):
            where = ("ao " + cur).strip()
            return "%r is not a subcommand of %r" % (tok, where)
        # Runnable command: the rest are positional args.
        break
    return None


def main():
    mode = sys.argv[1]
    if mode == "check-ao":
        text = open(sys.argv[2], encoding="utf-8").read()
        paths = load_caps(sys.argv[3])
        invs = [i for i in extract_ao(text) if i.split()[0] not in STOPWORDS]
        bad = 0
        for inv in invs:
            reason = validate_ao(inv, paths)
            if reason is not None:
                bad += 1
                print("BAD `ao %s` :: %s" % (inv, reason))
        print("TOTAL %d BAD %d" % (len(invs), bad))
        # Empty extraction means the regex or the source regressed, not that
        # everything is clean (matches the unit sweep's total>0 guard).
        if len(invs) == 0:
            print("BAD (extraction found zero ao invocations)")
            return 1
        return 1 if bad else 0
    if mode == "check-skills":
        text = open(sys.argv[2], encoding="utf-8").read()
        available = set()
        with open(sys.argv[3], encoding="utf-8") as handle:
            for line in handle:
                name = line.strip()
                if name:
                    available.add(name)
        skills = sorted(set(extract_skills(text)))
        bad = 0
        for skill in skills:
            if skill not in available:
                bad += 1
                print("BAD /%s :: not an installed or repo skill" % skill)
        print("TOTAL %d BAD %d" % (len(skills), bad))
        return 1 if bad else 0
    if mode == "list-br":
        text = open(sys.argv[2], encoding="utf-8").read()
        for tok in sorted(set(extract_br(text))):
            print(tok)
        return 0
    print("unknown mode: %s" % mode, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
PY

# ── read a top-level integer JSON field without jq (sed, like install-codex) ─
json_int_field() {
  local path="$1" key="$2"
  [[ -f "$path" ]] || return 0
  sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" "$path" | head -1
}

# ════════════════════════════════════════════════════════════════════════════
printf 'fresh-install conformance harness (age-wl5vm / FU1)\n'
printf '  mode:       %s\n' "$MODE"
printf '  repo:       %s\n' "$REPO_ROOT"
printf '  fresh HOME: %s\n' "$FRESH_HOME"
printf '  project:    %s\n' "$PROJECT"

# ── (a) obtain the ao binary the way this audience receives it ───────────────
section "a. acquire ao binary ($MODE mode)"
if [[ "$MODE" == "tree" ]]; then
  command -v go >/dev/null 2>&1 || die "tree mode needs the go toolchain"
  if (cd "$REPO_ROOT/cli" && go build -o "$AO" ./cmd/ao) >/dev/null 2>&1; then
    pass "built ao from tree ($("$AO" --version 2>/dev/null | head -1))"
  else
    fail "go build of ao from tree failed"
    (cd "$REPO_ROOT/cli" && go build -o "$AO" ./cmd/ao) 2>&1 | tail -5 >&2 || true
  fi
else
  # Release mode: fetch/extract the published binary. Degrade LOUDLY offline.
  TARBALL_LOCAL="$WORK/release.tar.gz"
  fetched=0
  if [[ -f "$RELEASE_TARBALL" ]]; then
    cp "$RELEASE_TARBALL" "$TARBALL_LOCAL" && fetched=1
  elif command -v curl >/dev/null 2>&1 && curl -fsSL "$RELEASE_TARBALL" -o "$TARBALL_LOCAL" 2>/dev/null; then
    fetched=1
  fi
  if [[ "$fetched" -eq 0 ]]; then
    if [[ "$REQUIRE_ASSET" -eq 1 ]]; then
      fail "published release asset unreachable or missing: $RELEASE_TARBALL" \
        "--require-asset: a release whose asset cannot be fetched must fail, not skip"
      printf '\n%bFRESH-INSTALL CONFORMANCE: FAIL%b\n' "$C_RED" "$C_NC"
      exit 1
    fi
    skip "release tarball unreachable (offline?): $RELEASE_TARBALL" "no network / path — refusing to silently pass"
    printf '\nrelease mode could not fetch the asset; nothing was exercised.\n'
    printf 'SKIPPED (offline) — this is a loud skip, not a pass (CI passes --require-asset).\n'
    exit 0
  fi
  if tar -xzf "$TARBALL_LOCAL" -C "$WORK" 2>/dev/null; then
    found="$(find "$WORK" -type f -name ao -perm -u+x 2>/dev/null | head -1)"
    if [[ -z "$found" ]]; then
      found="$(find "$WORK" -type f -name ao 2>/dev/null | head -1)"
    fi
    if [[ -n "$found" ]]; then
      cp "$found" "$AO" && chmod +x "$AO"
      pass "extracted released ao ($("$AO" --version 2>/dev/null | head -1))"
    else
      fail "released tarball contains no ao binary" "$RELEASE_TARBALL"
    fi
  else
    fail "could not extract released tarball" "$RELEASE_TARBALL"
  fi
fi
[[ -x "$AO" ]] || die "no usable ao binary; cannot continue"

CAPS="$WORK/capabilities.json"
if "$AO" capabilities >"$CAPS" 2>/dev/null && [[ -s "$CAPS" ]]; then
  pass "ao capabilities emitted a command contract"
else
  fail "ao capabilities produced no contract"
fi

# ── (b) link skills from this checkout into the fresh HOME ───────────────────
section "b. offline skills link (ao skills link)"
INSTALL_LOG="$WORK/install.log"
if (cd "$REPO_ROOT" && env HOME="$FRESH_HOME" \
    "$AO" skills link >"$INSTALL_LOG" 2>&1); then
  pass "ao skills link exited 0 against a fresh HOME"
else
  fail "ao skills link failed" "$(tail -3 "$INSTALL_LOG" | tr '\n' ' ')"
fi

LINKED_SKILLS="$FRESH_HOME/.agents/skills"
CODEX_SKILLS="$FRESH_HOME/.codex/skills"

# ── (c) quick-start: every advertised command must resolve ───────────────────
section "c. quick-start advertised-command conformance"
cd "$PROJECT" || die "cannot enter project dir"
git init -q
QS="$WORK/quickstart.txt"
# quick-start may exit nonzero in a minimal env (no reviewers) — we only need
# its EMITTED output, so capture regardless of exit status.
env HOME="$FRESH_HOME" "$AO" quick-start >"$QS" 2>&1 || true
if [[ ! -s "$QS" ]]; then
  fail "quick-start emitted no output"
else
  ao_out="$(python3 "$PARSE" check-ao "$QS" "$CAPS")"
  ao_rc=$?
  if [[ "$ao_rc" -eq 0 ]]; then
    pass "every advertised \`ao\` command resolves against the binary ($(printf '%s' "$ao_out" | sed -n 's/^TOTAL \([0-9]*\).*/\1/p') tokens)"
  else
    fail "an advertised \`ao\` command does not resolve" "$(printf '%s' "$ao_out" | grep '^BAD' | tr '\n' ' ')"
  fi

  # Skills: mentioned /skill must exist in the linked roots or repo skills/.
  SKILLS_LIST="$WORK/skills.txt"
  : >"$SKILLS_LIST"
  for skills_root in "$LINKED_SKILLS" "$CODEX_SKILLS"; do
    if [[ -d "$skills_root" ]]; then
      find "$skills_root" -mindepth 1 -maxdepth 1 \( -type d -o -type l \) -exec basename {} \; >>"$SKILLS_LIST"
    fi
  done
  if [[ -d "$REPO_ROOT/skills" ]]; then
    find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; >>"$SKILLS_LIST"
  fi
  sk_out="$(python3 "$PARSE" check-skills "$QS" "$SKILLS_LIST")"
  sk_rc=$?
  if [[ "$sk_rc" -eq 0 ]]; then
    pass "every advertised /skill exists in the installed bundle or repo skills/ ($(printf '%s' "$sk_out" | sed -n 's/^TOTAL \([0-9]*\).*/\1/p') mentions)"
  else
    fail "an advertised /skill does not exist" "$(printf '%s' "$sk_out" | grep '^BAD' | tr '\n' ' ')"
  fi

  # br tokens: extracted for completeness only. br is an external tracker, not
  # part of the ao surface — and its name appears in quick-start prose ("br
  # tracker already initialized") that is not a command, so distinguishing a
  # real `br` invocation from prose is unreliable without br's own command
  # tree. The task specifies hard assertions on `ao` tokens and /skill mentions
  # only; br is reported, never asserted.
  br_tokens="$(python3 "$PARSE" list-br "$QS" | tr '\n' ' ')"
  if [[ -n "${br_tokens// /}" ]]; then
    printf '  %bINFO%b advertised br tokens (informational, not asserted): %s\n' \
      "$C_YELLOW" "$C_NC" "$br_tokens"
  fi
fi

# ── (d) doctor: pristine install is green, no repo-relative fix strings ──────
section "d. pristine-install doctor conformance"
DOC="$WORK/doctor.txt"
env HOME="$FRESH_HOME" PATH="$DOCTOR_PATH" "$AO" doctor >"$DOC" 2>&1
doc_rc=$?
if [[ "$doc_rc" -eq 0 ]]; then
  pass "ao doctor exits 0 on a pristine install"
else
  fail "ao doctor exited $doc_rc on a pristine install" "$(grep -E '^(✗|!)' "$DOC" | tr '\n' ' ')"
fi

summary_line="$(grep -E 'checks passed' "$DOC" | tail -1)"
if [[ "$summary_line" =~ ^[0-9]+/[0-9]+\ checks\ passed$ ]]; then
  pass "doctor summary is clean: $summary_line"
else
  fail "doctor summary reports warnings or failures" "${summary_line:-<no summary line>}"
fi

warn_lines="$(grep -E '^(✗|!) ' "$DOC" || true)"
if [[ -z "$warn_lines" ]]; then
  pass "doctor emits no non-info warnings or failures"
else
  fail "doctor emits a warning/failure line" "$(printf '%s' "$warn_lines" | tr '\n' '|')"
fi

# The exact staleness class FU fixed: a fresh user told to run a repo-relative
# script they do not have. Every suggested fix must be runnable from the user's
# own context (the unit test pins the ^(ao |br |brew |npm |curl |https://)
# allowlist on the Fix field; here we pin the observable equivalent).
script_refs="$(grep -nE 'scripts/|[[:alnum:]_./-]+\.sh([^a-zA-Z0-9]|$)' "$DOC" || true)"
if [[ -z "$script_refs" ]]; then
  pass "no doctor line names a repo-relative script (fix strings are runnable)"
else
  fail "doctor names a repo-relative script a fresh user cannot run" "$(printf '%s' "$script_refs" | tr '\n' '|')"
fi

# ── (e) linked skill identity (source checkout vs live links) ────────────────
section "e. linked skill identity"
repo_count="$(find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')"
linked_count=0
if [[ -d "$LINKED_SKILLS" ]]; then
  linked_count="$(find "$LINKED_SKILLS" -mindepth 1 -maxdepth 1 \( -type d -o -type l \) 2>/dev/null | wc -l | tr -d ' ')"
fi
detail="repo=$repo_count linked=$linked_count root=$LINKED_SKILLS"
if [[ -n "$repo_count" && "$repo_count" -gt 0 && "$linked_count" -gt 0 ]]; then
  pass "skills linked into fresh HOME ($detail)"
else
  fail "expected linked skills under ~/.agents/skills after ao skills link" "$detail"
fi

# ── summary table ────────────────────────────────────────────────────────────
section "summary"
for row in "${SUMMARY_ROWS[@]}"; do
  st="${row%%|*}"
  label="${row#*|}"
  case "$st" in
    PASS) printf '  %bPASS%b  %s\n' "$C_GREEN" "$C_NC" "$label" ;;
    FAIL) printf '  %bFAIL%b  %s\n' "$C_RED" "$C_NC" "$label" ;;
    SKIP) printf '  %bSKIP%b  %s\n' "$C_YELLOW" "$C_NC" "$label" ;;
  esac
done
printf '\n  %d passed, %d failed, %d skipped\n' "$PASS_COUNT" "$FAIL_COUNT" "$SKIP_COUNT"

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  printf '%bFRESH-INSTALL CONFORMANCE: FAIL%b\n' "$C_RED" "$C_NC"
  exit 1
fi
printf '%bFRESH-INSTALL CONFORMANCE: PASS%b\n' "$C_GREEN" "$C_NC"
exit 0
