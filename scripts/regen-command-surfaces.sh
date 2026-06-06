#!/usr/bin/env bash
# regen-command-surfaces.sh — ONE-SHOT regenerator for the `ao`-command-landing
# derived surfaces (bead ag-jy12).
#
# WHY THIS EXISTS
#   Adding / removing / renaming an `ao` command makes several DERIVED surfaces
#   stale. regen-all.sh auto-fixes COMMANDS.md, registry.json, the skill-domain
#   map, and the context-map, but it only WARNS about two manual surfaces:
#     1. the two cobra `expectedCmds` literals  (cli/cmd/ao/cobra_commands_test.go)
#     2. the cli-command-surface heading counts (smoke fixture + matrix canary)
#   Because they were manual, each drift was discovered ONE CI round-trip at a
#   time (cost ~4 extra rounds on #720, ag-sz3h). This regenerator derives the
#   canonical command set + heading counts from ONE source of truth each and
#   rewrites all of them deterministically — "add an ao command" becomes one
#   regen, zero hand-edits.
#
# SOURCES OF TRUTH (single, each)
#   * registered command set  -> the LIVE cobra tree, via the Go test
#       `TestDumpRegisteredTopLevelCommands` (AO_DUMP_REGISTERED_CMDS=1). It walks
#       rootCmd.Commands() — the exact set the two expectedCmds literals must
#       match (sorted, `help` excluded). A test, not a subcommand, so emitting
#       the list does not itself add a registered command.
#   * heading counts          -> cli/docs/COMMANDS.md (itself generated from the
#       cobra tree by generate-cli-reference.sh). top = `### \`ao `, sub =
#       `#### \`ao `, all = `#{3,4} \`ao `.
#
# DERIVED SURFACES REWRITTEN
#   * cli/cmd/ao/cobra_commands_test.go        — BOTH expectedCmds literals
#   * evals/agentops-core/fixtures/cli-command-surface-smoke.sh   — count guard
#   * evals/agentops-core/cli-command-surface-matrix.json         — stdout_contains
#
# IDEMPOTENT: running on a no-change tree is a no-op (no file is rewritten if it
# already matches). --check exits non-zero on drift without writing.
#
# ORDER: run AFTER generate-cli-reference.sh (COMMANDS.md must be current first).
# Wired into regen-all.sh after the "cli reference (COMMANDS.md)" step.
#
# ───────────────────────────────────────────────────────────────────────────
# DELETION ⊃ ADDITION (completeness note — RubyMoose)
#   Removing or RENAMING a command is a SUPERSET of adding one. This regenerator
#   covers the ADD surfaces above. A delete/rename ALSO disturbs surfaces this
#   script does NOT auto-regenerate — they need manual / quorum attention:
#     (a) codex-contract gates that hardcode command literals
#         (scripts/validate-codex-rpi-contract.sh and siblings),
#     (b) skills-codex twins / overrides
#         (export-claude-skills-to-codex + scripts/regen-codex-hashes.sh),
#     (c) VERBATIM-PRESERVE template SHAs in frontmatter
#         (e.g. cron-loop-mode.md) — a renamed command literal changes the SHA.
#   When you DELETE or RENAME a command, run this regenerator AND audit (a)-(c)
#   by hand (grep the command literal across skills/ skills-codex/ overrides/
#   scripts/), then re-run the full pre-push gate.
# ───────────────────────────────────────────────────────────────────────────
#
# Usage:
#   scripts/regen-command-surfaces.sh            # rewrite derived surfaces (idempotent)
#   scripts/regen-command-surfaces.sh --check    # fail on drift; write nothing
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

CHECK_MODE=false
[[ "${1:-}" == "--check" ]] && CHECK_MODE=true

COBRA_TEST="cli/cmd/ao/cobra_commands_test.go"
SMOKE="evals/agentops-core/fixtures/cli-command-surface-smoke.sh"
MATRIX="evals/agentops-core/cli-command-surface-matrix.json"
DOCS="cli/docs/COMMANDS.md"

for f in "$COBRA_TEST" "$SMOKE" "$MATRIX" "$DOCS"; do
  [[ -f "$f" ]] || { echo "ERROR: missing $f" >&2; exit 2; }
done

drift=0

# ── 1. derive canonical command set from the live cobra tree ────────────────
mapfile -t CMDS < <(
  cd "$REPO_ROOT/cli" \
    && AO_DUMP_REGISTERED_CMDS=1 env -u AGENTOPS_RPI_RUNTIME \
         go test ./cmd/ao -run '^TestDumpRegisteredTopLevelCommands$' -count=1 -v 2>/dev/null \
    | grep -E '^[a-z0-9][a-z0-9-]*$' \
    | sort -u
)
if [[ "${#CMDS[@]}" -lt 45 ]]; then
  echo "ERROR: dumper returned only ${#CMDS[@]} commands (expected >=45); build/test failure?" >&2
  exit 2
fi

# ── 2. derive heading counts from COMMANDS.md ───────────────────────────────
top_count="$(grep -cE '^### `ao ' "$DOCS")"
sub_count="$(grep -cE '^#### `ao ' "$DOCS")"
all_count="$(grep -cE '^#{3,4} `ao ' "$DOCS")"

# ── helpers ────────────────────────────────────────────────────────────────
# Rewrite both `expectedCmds := []string{ ... }` blocks in the cobra test to a
# canonical one-element-per-line form (gofmt-stable). Done in Go-free Python so
# the formatting is deterministic regardless of source wrapping.
rewrite_cobra() {
  local outfile="$1"
  python3 - "$COBRA_TEST" "$outfile" "${CMDS[@]}" <<'PY'
import sys, re
src_path, out_path = sys.argv[1], sys.argv[2]
cmds = sys.argv[3:]
with open(src_path) as fh:
    text = fh.read()

# Canonical literal body: one element per line, tab+tab indent (inside a func).
body = "".join(f'\t\t"{c}",\n' for c in cmds)
new_literal = "expectedCmds := []string{\n" + body + "\t}"

# Replace every `expectedCmds := []string{ ... }` (non-greedy to the matching
# `\n\t}` that closes the literal). The literals are indented one tab inside a
# func, so the close line is exactly "\n\t}".
pattern = re.compile(r"expectedCmds := \[\]string\{.*?\n\t\}", re.DOTALL)
new_text, n = pattern.subn(lambda _m: new_literal, text)
if n == 0:
    sys.stderr.write("ERROR: no expectedCmds literal found in cobra test\n")
    sys.exit(3)

with open(out_path, "w") as fh:
    fh.write(new_text)
PY
}

# ── 3. cobra expectedCmds (×2) ──────────────────────────────────────────────
tmp_cobra="$(mktemp)"
rewrite_cobra "$tmp_cobra"
# gofmt to canonical form so a clean tree is a stable no-op.
gofmt "$tmp_cobra" > "${tmp_cobra}.fmt" && mv "${tmp_cobra}.fmt" "$tmp_cobra"
if ! cmp -s "$tmp_cobra" "$COBRA_TEST"; then
  drift=1
  if $CHECK_MODE; then
    echo "DRIFT: $COBRA_TEST expectedCmds out of sync with the cobra tree" >&2
  else
    cp "$tmp_cobra" "$COBRA_TEST"
    echo "  updated $COBRA_TEST (expectedCmds ×2 -> ${#CMDS[@]} commands)"
  fi
fi
rm -f "$tmp_cobra"

# ── 4. cli-command-surface heading counts (smoke fixture) ───────────────────
tmp_smoke="$(mktemp)"
sed -E \
  -e "s/\\[\\[ \"\\\$top_count\" != \"[0-9]+\" \\|\\| \"\\\$sub_count\" != \"[0-9]+\" \\|\\| \"\\\$all_count\" != \"[0-9]+\" \\]\\]/[[ \"\$top_count\" != \"${top_count}\" || \"\$sub_count\" != \"${sub_count}\" || \"\$all_count\" != \"${all_count}\" ]]/" \
  -e "s/-ne [0-9]+ \]\]/-ne ${all_count} ]]/" \
  "$SMOKE" > "$tmp_smoke"
if ! cmp -s "$tmp_smoke" "$SMOKE"; then
  drift=1
  if $CHECK_MODE; then
    echo "DRIFT: $SMOKE heading counts out of sync (want top=$top_count sub=$sub_count all=$all_count)" >&2
  else
    cp "$tmp_smoke" "$SMOKE"
    echo "  updated $SMOKE (top=$top_count sub=$sub_count all=$all_count)"
  fi
fi
rm -f "$tmp_smoke"

# ── 5. cli-command-surface heading counts (matrix canary) ───────────────────
tmp_matrix="$(mktemp)"
sed -E "s/cli-command-headings: top=[0-9]+ sub=[0-9]+ all=[0-9]+/cli-command-headings: top=${top_count} sub=${sub_count} all=${all_count}/" \
  "$MATRIX" > "$tmp_matrix"
if ! cmp -s "$tmp_matrix" "$MATRIX"; then
  drift=1
  if $CHECK_MODE; then
    echo "DRIFT: $MATRIX heading counts out of sync (want top=$top_count sub=$sub_count all=$all_count)" >&2
  else
    cp "$tmp_matrix" "$MATRIX"
    echo "  updated $MATRIX (top=$top_count sub=$sub_count all=$all_count)"
  fi
fi
rm -f "$tmp_matrix"

if $CHECK_MODE; then
  if [[ $drift -ne 0 ]]; then
    echo "command-surface DRIFT — run scripts/regen-command-surfaces.sh (no flag) to fix" >&2
    exit 1
  fi
  echo "command surfaces in sync (cobra expectedCmds + heading counts)"
else
  if [[ $drift -eq 0 ]]; then
    echo "command surfaces already in sync — no changes"
  fi
fi
exit 0
