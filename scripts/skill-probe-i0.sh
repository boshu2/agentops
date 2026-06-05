#!/usr/bin/env bash
# skill-probe-i0.sh — I0-INFORMATIONAL retrieval-probe receipt generator.
#
# Wraps the deterministic lexical trigger ranker
# (skills/skill-builder/scripts/scan_descriptions.py --probe, ag-7led) as an
# I0 step: it RUNS and REPORTS a per-skill JSON receipt artifact, but is NOT a
# PR check — it never blocks a merge and is not advisory. (I0 = informational:
# runs + reports an artifact, does not appear as a PR check.) The CI step that
# invokes it is `continue-on-error: true` inside an existing job (skill-eval),
# so no new GitHub check is created.
#
# What it does, for every `trigger_probes:` phrase declared anywhere in the
# corpus:
#   1. runs the deterministic probe and writes the JSON receipt to
#      .agents/ao/skill-eval/<declaring-skill-id>.json;
#   2. re-runs the SAME probe and asserts the two JSON outputs are
#      BYTE-IDENTICAL — the determinism assertion. A non-deterministic probe
#      input/ranker is caught HERE, in the informational lane, before the probe
#      could ever be promoted to a blocking gate.
#
# 2-WEEK STABILITY BASELINE (gate-promotion gate): this receipt lane must run
# green and byte-stable across the corpus for at least 2 weeks of merges before
# `--probe` is promoted from I0-informational to a blocking gate (i.e. before
# the determinism/rank-#1 assertion is allowed to fail a PR check). Do not
# flip this to a required check until that baseline is met. See
# docs/contracts/ci-jobs.yaml (skill-eval entry) and the SKILL.md probe note.
#
# Exit semantics (I0): always 0 on the happy path AND on "nothing to probe".
# A NON-DETERMINISTIC probe (byte-diff across two runs) is the one condition
# this script surfaces with a non-zero exit, so the CI step can `::warning::`
# it — the caller pins `continue-on-error: true`, so even that does not block.
#
# Usage: scripts/skill-probe-i0.sh [SKILLS_DIR] [RECEIPT_DIR]
set -euo pipefail

SKILLS_DIR="${1:-skills}"
RECEIPT_DIR="${2:-.agents/ao/skill-eval}"
SCANNER="${SKILLS_DIR}/skill-builder/scripts/scan_descriptions.py"

if [[ ! -d "$SKILLS_DIR" ]]; then
  echo "skill-probe-i0: skills dir '${SKILLS_DIR}' not found; nothing to probe." >&2
  exit 0
fi
if [[ ! -f "$SCANNER" ]]; then
  echo "skill-probe-i0: scanner '${SCANNER}' not found; nothing to probe." >&2
  exit 0
fi

mkdir -p "$RECEIPT_DIR"

# Collect every (skill-id, phrase) pair declared in `trigger_probes:` blocks.
# Format: "<skill-id>\t<phrase>". Pure text walk — no YAML dep, byte-stable.
mapfile -t pairs < <(
  for skill_md in "$SKILLS_DIR"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    sid="$(basename "$(dirname "$skill_md")")"
    awk -v sid="$sid" '
      # A YAML doc/frontmatter boundary (--- or ...) always ends the block.
      /^(---|\.\.\.)[[:space:]]*$/ { inblock=0; next }
      # Flow form: `trigger_probes: ["a", "b"]` (inline YAML list). Mirrors the
      # scanner (scan_descriptions.py parse_trigger_probes) so the two parsers
      # do not diverge — flow-form skills must NOT be silently skipped.
      /^trigger_probes:[[:space:]]*\[/ {
        flow=$0
        sub(/^trigger_probes:[[:space:]]*\[/, "", flow)
        sub(/\][[:space:]]*$/, "", flow)
        n=split(flow, items, ",")
        for (k=1; k<=n; k++) {
          it=items[k]
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", it)
          gsub(/^"|"$/, "", it)
          gsub(/^'"'"'|'"'"'$/, "", it)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", it)
          if (length(it) > 0) print sid "\t" it
        }
        inblock=0
        next
      }
      /^trigger_probes:[[:space:]]*$/ { inblock=1; next }
      # Indented "- item" list entries belong to the block.
      inblock && /^[[:space:]]+-[[:space:]]+/ {
        line=$0
        sub(/^[[:space:]]+-[[:space:]]+/, "", line)
        gsub(/^"|"$/, "", line)
        gsub(/^'"'"'|'"'"'$/, "", line)
        if (length(line) > 0) print sid "\t" line
        next
      }
      # Any other non-blank, non-list line ends the block.
      inblock && /[^[:space:]]/ { inblock=0 }
    ' "$skill_md"
  done | sort -u
)

if [[ "${#pairs[@]}" -eq 0 ]]; then
  echo "::notice::skill-probe-i0: no \`trigger_probes:\` phrases declared in ${SKILLS_DIR}/*/SKILL.md — I0 receipt lane has nothing to probe (this is expected until skills opt in)."
  exit 0
fi

nondeterministic=0
count=0
for pair in "${pairs[@]}"; do
  sid="${pair%%$'\t'*}"
  phrase="${pair#*$'\t'}"
  count=$((count + 1))

  receipt="${RECEIPT_DIR}/${sid}.json"
  tmp_a="$(mktemp)"
  tmp_b="$(mktemp)"

  # Run the deterministic probe twice. `set +e` — the probe's own exit code
  # (rank-#1 pass=0 / rank-drop=1 / usage=2) is NOT a gate here; it is recorded
  # in the receipt JSON's `declarer_is_top`. Only a byte-DIFF between the two
  # runs is a finding worth surfacing in the I0 lane.
  set +e
  python3 "$SCANNER" "$SKILLS_DIR" --probe "$phrase" --json >"$tmp_a" 2>/dev/null
  python3 "$SCANNER" "$SKILLS_DIR" --probe "$phrase" --json >"$tmp_b" 2>/dev/null
  set -e

  if ! cmp -s "$tmp_a" "$tmp_b"; then
    echo "::warning::skill-probe-i0: NON-DETERMINISTIC probe output for skill '${sid}', phrase '${phrase}' (byte-diff across two runs). This is exactly the failure the determinism assertion exists to catch BEFORE the probe is promoted to a blocking gate."
    nondeterministic=1
  fi

  cp "$tmp_a" "$receipt"
  echo "skill-probe-i0: wrote receipt ${receipt} (phrase: '${phrase}')"
  rm -f "$tmp_a" "$tmp_b"
done

echo "skill-probe-i0: produced ${count} receipt(s) under ${RECEIPT_DIR}/ (I0-informational; not a PR check)."

# I0 lane: surface a non-deterministic probe via non-zero so the CI step can
# annotate it, but the CI step pins continue-on-error so it never blocks.
exit "$nondeterministic"
