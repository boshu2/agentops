#!/usr/bin/env bash
# tests/e2e/membrane-bead-knowledge-queryable.sh
#   E0.TEST — bead age-membrane-memory-arch-tz2s.1.4
#
# Proves E0's thin foundation (bead age-...tz2s.1.1): a bead is a QUERYABLE
# KNOWLEDGE NODE, not a junk-drawer of prose. Domain/provenance knowledge stamped
# into a bead's structured `agent_context` (intent_domain + provenance_links) is
# retrievable by a GRAPH QUERY over the ledger — and crucially NOT by grepping the
# bead's freeform text, which is the failure mode E0 exists to fix.
#
# Fixture fidelity: knowledge is stamped via the PRODUCTION path
# (`br update --agent-context <json>`), never a hand-built JSONL row. Beads are
# given titles that DO NOT mention the domain, so a grep over their text finds
# nothing — only the structured query retrieves them. That contrast IS the proof.
#
# Real br/bv, isolated sandbox, mock nothing (tests/e2e/README.md contract).
# bv's offline embedding provider does not rank-discriminate, so retrieval is
# asserted via the DETERMINISTIC structured `br list --json` graph filter; bv is
# asserted only to PARTICIPATE (indexes + returns the node in structured form).
#
# HARNESS EXCEPTION (explicit, per README.md §"The contract"): the shared
# tests/lib/e2e-*.sh factory/guards are `ao`-binary-centric (e2e_factory_ao_bin /
# e2e_guard_ao_bin) and do not fit a test whose system-under-test is the EXTERNAL
# beads_rust pair (br/bv), not the repo-built ao. So, like the sibling scripts
# (rpi-phased-domain.sh, goals-measure-scenarios.sh) which also hand-roll mktemp,
# this applies the harness's ISOLATION PRINCIPLE by hand and in full: a mktemp
# sandbox, a sandboxed HOME (so br/bv config/index can never touch the real ~),
# an isolated BEADS_DIR (never this repo's _beads), and a chmod+rm trap cleanup.
#
# RUN MODEL: local/manual (`bash tests/e2e/membrane-bead-knowledge-queryable.sh`).
# DELIBERATELY NOT wired into validate.yml: this is the only e2e that depends on
# the EXTERNAL beads_rust tools (br/bv), which the CI image does not install — a
# CI step here would skip-green and FALSE-PASS without proving anything (worse
# than no coverage). Wire it in only once br/bv are provisioned in CI. The skip
# guard below keeps a br/bv-less checkout from a spurious failure; the skip line
# is loud so a manual runner sees the capability was not exercised.
#
# Scope: the THIN E0 capability that shipped (intent_domain + provenance_links,
# 1.1). Richer queries — "beads that ESCAPED" (needs escape_domain, EM.2 .2.2)
# and "DECISIONS backing a BC" (decisions field, deferred in 1.1) — are gated on
# those fields and are out of scope here.
set -euo pipefail

log()  { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
pass() { printf 'PASS: %s\n' "$*"; }

# br/bv are external tools (beads_rust), not built from this repo. If absent, this
# e2e cannot run — skip cleanly rather than fail a checkout that lacks them.
for tool in br bv python3 git; do
  command -v "$tool" >/dev/null 2>&1 || { log "SKIP: required tool '$tool' not on PATH"; exit 0; }
done

# ── isolated sandbox (never touches this repo's _beads or the real HOME) ──────
WORK="$(mktemp -d)"
# chmod first so a read-only artifact can't pin the dir (README §4); then wipe.
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
export HOME="$WORK/home"          # any br/bv config/index write lands in-sandbox
mkdir -p "$HOME"
cd "$WORK"
git init -q
git config user.email "e2e@agentops.test"
git config user.name "agentops-e2e"
export BEADS_DIR="$WORK/.beads"
br init >/dev/null 2>&1 || fail "br init failed"
log "isolated ledger: $BEADS_DIR"

# ── fixture: two beads whose TITLES do not name the domain ───────────────────
DOMAIN="Photosynthesis"           # a token that appears in NO bead title
ART_ID="art-nightly-summary-2026-06-21"
mk() { br create "$1" -t task --json 2>/dev/null | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])'; }
ID_KNOWN="$(mk "Widget refactor task")"
ID_OTHER="$(mk "Cache eviction tuning")"
[[ -n "$ID_KNOWN" && -n "$ID_OTHER" ]] || fail "bead creation did not return ids"
log "beads: known=$ID_KNOWN other=$ID_OTHER"

# Stamp domain + provenance KNOWLEDGE via the production agent_context path.
br update "$ID_KNOWN" --agent-context \
  "{\"enabled\":true,\"intent_domain\":{\"bc\":\"BC2\",\"name\":\"$DOMAIN\",\"source\":\"docs/architecture/component-map.md\"},\"provenance_links\":[{\"id\":\"$ART_ID\",\"path\":\"/x\",\"kind\":\"automation\"}]}" \
  >/dev/null 2>&1 || fail "agent_context stamp failed"

# ── assertion (a): br show --json exposes the structured knowledge node ───────
br show "$ID_KNOWN" --json 2>/dev/null | DOMAIN="$DOMAIN" ART_ID="$ART_ID" python3 -c '
import sys, json, os
b = json.load(sys.stdin); b = b[0] if isinstance(b, list) else b
ac = b.get("agent_context")
ac = json.loads(ac) if isinstance(ac, str) else (ac or {})
name = (ac.get("intent_domain") or {}).get("name")
provs = [p.get("id") for p in (ac.get("provenance_links") or [])]
want_domain = os.environ["DOMAIN"]; want_art = os.environ["ART_ID"]
assert name == want_domain, "intent_domain.name=%r != %r" % (name, want_domain)
assert want_art in provs, "provenance_links missing artifact edge: %r" % (provs,)
' || fail "(a) br show --json did not expose structured intent_domain + provenance_links"
pass "(a) bead exposes a structured knowledge node (intent_domain + provenance edge)"

# ── assertion (b): a structured GRAPH QUERY retrieves it; GREP does not ───────
LISTJSON="$(br list --json 2>/dev/null)"
echo "$LISTJSON" | DOMAIN="$DOMAIN" ID_KNOWN="$ID_KNOWN" python3 -c '
import sys, json, os
data = json.load(sys.stdin)
items = data if isinstance(data, list) else data.get("issues", data.get("items", []))
def domain_of(b):
    ac = b.get("agent_context"); ac = json.loads(ac) if isinstance(ac, str) else (ac or {})
    return (ac.get("intent_domain") or {}).get("name")
graph_hits = sorted(b["id"] for b in items if domain_of(b) == os.environ["DOMAIN"])
grep_hits  = sorted(b["id"] for b in items if os.environ["DOMAIN"] in (b.get("title","") + b.get("description","")))
assert graph_hits == [os.environ["ID_KNOWN"]], f"structured graph query returned {graph_hits}"
assert grep_hits == [], f"grep over bead text should find nothing but found {grep_hits}"
' || fail "(b) structured graph query did not retrieve exactly the stamped bead (or grep found it)"
pass "(b) structured graph query retrieves by domain field; grep over text finds nothing"

# ── assertion (c): bv participates — indexes the node + returns it structured ─
BVOUT="$(bv --robot-search --search "$DOMAIN domain knowledge" --format json --no-cache 2>/dev/null || true)"
echo "$BVOUT" | ID_KNOWN="$ID_KNOWN" python3 -c '
import sys, json, os
try:
    d = json.load(sys.stdin)
except Exception:
    print("bv produced no JSON"); sys.exit(1)
ids = [r.get("issue_id") for r in d.get("results", [])]
# Offline embedding does not rank-discriminate; only assert bv returns structured
# results (issue_id-bearing) AND the knowledge node is among them — i.e. bv can
# query the bead graph at all (vs grep over markdown files).
assert d.get("results") is not None, "bv returned no results array"
assert os.environ["ID_KNOWN"] in ids, f"bv did not surface the knowledge node: {ids}"
' || fail "(c) bv did not return the knowledge node in structured form"
pass "(c) bv queries the bead knowledge graph and returns the node in structured form"

log "ALL ASSERTIONS PASSED — bead-knowledge is queryable as a graph node, not grep"
