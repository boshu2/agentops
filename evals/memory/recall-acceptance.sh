#!/usr/bin/env bash
# recall-smoke.sh — `ao recall` TOPIC-RECALL SMOKE (report-only; NOT a ship gate)
#
# Runs `ao recall` over a 25-query set and REPORTS, echo-free, whether a memory
# whose path/snippet mentions each topic appears in the top-5 (matched against the
# JSON hits — which carry NO query header, so the query text cannot false-pass).
#
# THIS CERTIFIES NOTHING. It is a coarse TOPIC-presence smoke + the ripgrep
# contrast, used to drive the MISS LOG. It deliberately does NOT implement
# memory-v1.md done-criteria #3 — "the top-5 contains the RIGHT cited memory"
# (known-item retrieval, one exact memory per query). The tokens here are broad
# (e.g. "graph", "skill"), so a topic-relevant-but-not-exact hit counts: that is
# adequate for a smoke, INADEQUATE for certification, and intentionally so.
#
# The real known-item ship gate is bead age-unified-agent-memory-nyfq.8, which is
# OPEN and earned as the curated corpus is enriched (.5: ingest the Claude silos) —
# today many topics live in memory CONTENT, not in path-distinctive curated files.
# This script exits 0 regardless of the count; it reports, it does not gate.
# Read-only; pure bash.
set -u
AO="${AO:-ao}"
LIMIT=5
pass=0; total=0; declare -a MISSES

# run <query> <expected-token> — the expected token must appear in the RETRIEVED
# RESULTS (cited path OR snippet content), never in the echoed query. We match the
# JSON output, which contains ONLY the hits array (no query header), so an empty or
# wrong recall cannot false-pass on the query text. Honest recall@5: did recall
# surface the right memory in the top 5? Misses are LOGGED (not tuned away).
run() {
  local q="$1" expect="$2"; total=$((total+1))
  # JSON = the hits array only (path+snippet); no query echo to game the match.
  local results; results="$("$AO" recall "$q" --limit $LIMIT --output json 2>/dev/null)"
  if printf '%s\n' "$results" | grep -qiF "$expect"; then
    pass=$((pass+1)); printf '  ok   [%s] -> %s\n' "$q" "$expect"
  else
    MISSES+=("$q :: expected $expect"); printf '  MISS [%s] -> %s\n' "$q" "$expect"
  fi
}

echo "== ao recall acceptance (top-$LIMIT cited) — 25-query gate (memory-v1.md #3) =="
run "tool freshness rot coverage hole uca"             "tool-freshness-rot"
run "re-baseline before scoping new work"              "re-baseline-before-scoping"
run "ao recall is the unified memory SOT"              "memory-v1-ao-recall"
run "hot checkout concurrent lane discipline worktree" "hot-checkout-concurrent"
run "pawl run foreground not background reaped"        "foreground"
run "execution packet schema validator unify"         "55qz"
run "membrane maximal adversarial scope grind"        "membrane"
run "cross-family verification catch reachable stranger" "a9iv"
run "evolve autonomous improvement loop dormancy"     "evolve"
run "pawl good bar calibrated merge gate"             "good-bar"
run "codex approval fanout judge"                     "codex-approval"
run "workpacket hardening operating loop"             "workpacket"
run "omnigent dispatch command land"                  "omnigent"
run "beads tracker br ready offline"                  "beads"
run "crank epic wave execution swarm"                 "crank"
run "discovery dense execution packet intent"         "discovery"
run "validate eval substrate architecture"            "substrate"
run "tmux socket isolation test collision"            "tmux"
run "branch worktree rationalization cleanup"         "worktree"
run "cass session mining transcript search"           "cass"
run "council decision one way door consensus"         "council"
run "provenance ledger verdict commit binding"        "provenance"
run "skill registry valuable when to use"             "skill"
run "freshness decay maturity weight retrieval"       "decay"
run "knowledge graph rejected dense retrieval deferred" "graph"

echo
echo "== ripgrep baseline (.4): same NL queries, head-to-head =="
# Fair comparison: feed rg the SAME natural-language query (OR of its words) and
# check whether the expected file appears at all. rg has no ranking, no tier, no
# decay/maturity — so even when it matches it cannot CITE the right memory the way
# recall does. This is the .4 "recall must beat rg" check.
rg_hits=0; rg_total=0
while IFS=$'\t' read -r q expect; do
  [ -z "$q" ] && continue; rg_total=$((rg_total+1))
  pat="$(printf '%s' "$q" | tr ' ' '|')"
  rg -l -i -e "$pat" .agents/ 2>/dev/null | grep -qiF "$expect" && rg_hits=$((rg_hits+1))
done <<'PAIRS'
tool freshness rot coverage hole uca	tool-freshness-rot
re-baseline before scoping new work	re-baseline-before-scoping
ao recall is the unified memory SOT	memory-v1-ao-recall
pawl run foreground not background reaped	foreground
membrane maximal adversarial scope grind	membrane
PAIRS
echo "rg surfaced the expected file (unranked, no citation) for $rg_hits/$rg_total; recall returns RANKED, tier-cited hits rg cannot."

echo
echo "== TOPIC-RECALL SMOKE (report-only, NOT a certification): $pass/$total topics surfaced in top-$LIMIT =="
if [ "${#MISSES[@]}" -gt 0 ]; then
  echo "-- MISS LOG (drives corpus enrichment .5 + dense retrieval; feeds the known-item gate .8) --"
  printf '   %s\n' "${MISSES[@]}"
fi
echo "NOTE: this is a smoke, not memory-v1.md done-criteria #3 (the known-item ship gate is bead .8, OPEN)."
# Report-only: exit 0 regardless of the count. This script does not gate.
exit 0
