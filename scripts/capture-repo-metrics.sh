#!/usr/bin/env bash
# capture-repo-metrics.sh — append one dated snapshot of GitHub repo metrics to
# docs/metrics/traffic.jsonl, so usage signal ACCUMULATES into a trend.
#
# WHY: GitHub's traffic API is a 14-day rolling window, top-10 only — every read
# evaporates. Without a periodic capture, "what do people use?" is forever a thin
# 14-day guess. This script is the harness: run it on a schedule (cron / launchd
# / the out-of-session substrate — AgentOps ships no daemon, ADR-0009) and the
# JSONL grows into a longitudinal record you can actually trend.
#
# CAVEAT baked into the record (clones_note): the clone count is NOT users — it
# is dominated by CI, package mirrors, and scrapers (e.g. 43k clones / ~1k views
# in one 14d window). Rank human interest by unique PATH views + fork velocity,
# never by clones.
#
# Usage:
#   scripts/capture-repo-metrics.sh [<owner/repo>] [--dry-run]
#     <owner/repo>   default: boshu2/agentops
#     --dry-run      print the record to stdout; do NOT append
#
# Requires: gh (authenticated), python3. Fails CLOSED (no partial record) if gh
# is missing/unauthenticated or any endpoint cannot be fetched.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
slug="boshu2/agentops"
dry_run=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    */*) slug="$arg" ;;
    *) echo "unrecognized argument: $arg (expected owner/repo or --dry-run)" >&2; exit 2 ;;
  esac
done

if ! command -v gh >/dev/null 2>&1; then
  echo "FAIL: gh CLI not found — cannot capture metrics." >&2
  exit 1
fi
if ! gh auth status >/dev/null 2>&1; then
  echo "FAIL: gh is not authenticated (gh auth login) — cannot capture metrics." >&2
  exit 1
fi

# Fetch each endpoint; any failure aborts before writing (fail-closed, no partial).
fetch() {
  local out
  if ! out="$(gh api "$1" 2>/dev/null)"; then
    echo "FAIL: could not fetch $1 (token scope/permissions? traffic needs push access)." >&2
    exit 1
  fi
  printf '%s' "$out"
}
repo_json="$(fetch "repos/${slug}")"
views_json="$(fetch "repos/${slug}/traffic/views")"
clones_json="$(fetch "repos/${slug}/traffic/clones")"
paths_json="$(fetch "repos/${slug}/traffic/popular/paths")"
refs_json="$(fetch "repos/${slug}/traffic/popular/referrers")"

captured_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

record="$(SLUG="$slug" CAPTURED_AT="$captured_at" \
  REPO_JSON="$repo_json" VIEWS_JSON="$views_json" CLONES_JSON="$clones_json" \
  PATHS_JSON="$paths_json" REFS_JSON="$refs_json" python3 - <<'PY'
import json, os
repo   = json.loads(os.environ["REPO_JSON"])
views  = json.loads(os.environ["VIEWS_JSON"])
clones = json.loads(os.environ["CLONES_JSON"])
paths  = json.loads(os.environ["PATHS_JSON"])
refs   = json.loads(os.environ["REFS_JSON"])

rec = {
    "captured_at": os.environ["CAPTURED_AT"],
    "repo": os.environ["SLUG"],
    "stars": repo.get("stargazers_count"),
    "forks": repo.get("forks_count"),
    "watchers": repo.get("subscribers_count"),
    "open_issues": repo.get("open_issues_count"),
    "views_14d": views.get("count"),
    "views_14d_unique": views.get("uniques"),
    "clones_14d": clones.get("count"),
    "clones_14d_unique": clones.get("uniques"),
    "clones_note": "clones are NOT users — dominated by CI/mirrors/scrapers; rank interest by unique path views + fork velocity",
    "top_paths": [
        {"path": p.get("path"), "count": p.get("count"), "uniques": p.get("uniques")}
        for p in paths[:10]
    ],
    "top_referrers": [
        {"referrer": r.get("referrer"), "count": r.get("count"), "uniques": r.get("uniques")}
        for r in refs[:10]
    ],
}
# Fail CLOSED if ANY core scalar came back null — a garbled or permission-limited
# payload must never append a junk record with null counts. (Empty top_paths /
# top_referrers are NOT failures: a genuinely quiet 14-day window has zero of
# each, and a wrong-typed payload already raises above on the list slice.)
required = [
    "stars", "forks", "watchers", "open_issues",
    "views_14d", "views_14d_unique", "clones_14d", "clones_14d_unique",
]
missing = [k for k in required if rec[k] is None]
if missing:
    raise SystemExit(f"FAIL: metrics payload missing core field(s) {missing} — not writing.")
print(json.dumps(rec, separators=(",", ":")))
PY
)"

if [ "$dry_run" -eq 1 ]; then
  printf '%s\n' "$record"
  echo "(dry-run — not appended)" >&2
  exit 0
fi

out_dir="$repo_root/docs/metrics"
mkdir -p "$out_dir"
printf '%s\n' "$record" >> "$out_dir/traffic.jsonl"
echo "appended snapshot for ${slug} @ ${captured_at} -> docs/metrics/traffic.jsonl"
