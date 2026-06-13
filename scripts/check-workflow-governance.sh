#!/usr/bin/env bash
# check-workflow-governance.sh — kind-aware workflow drift/governance gate
# (ag-km74w; bidirectional + DDD-identity extension ag-jy8gj).
#
# Asserts a BIDIRECTIONAL identity match between the repo-tracked Claude
# workflows (.claude/workflows/*.js) and the top-level `workflows:` section of
# docs/contracts/skill-dispositions.yaml, plus the DDD identity triple on each
# ledger row.
#
#   FORWARD  — every workflow .js has a `workflows.<id>` ledger row that carries
#              kind: workflow + a Bounded Context (domain) + a hexagonal_role.
#   REVERSE  — every `workflows.<id>` ledger row (kind: workflow) has a matching
#              repo-tracked .js (else the row is STALE and the gate FAILS).
#
# Workflows are Claude-only (ag-jy8gj): they live only in .claude/workflows/
# with no skills-codex twin, so this gate checks Claude-runtime presence and
# never requires a Codex twin or trips audit-codex-parity.
#
# This is the thin workflow-only sliver of the S0 artifact-dispositions schema:
# it gives Claude Workflow scripts a governed home in the ledger WITHOUT teaching
# the `- skill:` line-parsers (sku_catalog.py / generate-skill-domain-map.sh) to
# miscount them as skills (the `workflows:` mapping is top-level, skipped like
# `historical:`).
#
# Exit 0: the .js set and the kind: workflow ledger rows are in bijection and
#         every row carries kind: workflow + domain (BC) + hexagonal_role.
# Exit 1: a workflow .js has no ledger row; a row lacks the identity triple; or a
#         kind: workflow row has no matching .js (stale).
#
# Repo root from cwd git. No hardcoded user paths.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
disp_yaml="$repo_root/docs/contracts/skill-dispositions.yaml"

if [ ! -f "$disp_yaml" ]; then
  echo "FAIL: ledger not found: $disp_yaml" >&2
  exit 1
fi

status=0

# For each tracked workflow .js, derive its id from meta.name (authoritative),
# then assert the ledger has a `workflows.<id>` entry with kind: workflow.
while IFS= read -r tracked; do
  rel="$tracked"
  abs="$repo_root/$tracked"
  [ -f "$abs" ] || continue
  # id = meta.name literal from the .js (single- or double-quoted).
  id="$(grep -oE "name:[[:space:]]*['\"][^'\"]+['\"]" "$abs" | head -1 | sed -E "s/.*['\"]([^'\"]+)['\"].*/\1/")"
  if [ -z "$id" ]; then
    echo "FAIL: $rel has no parseable meta.name (cannot match to ledger)" >&2
    status=1
    continue
  fi
  # Verify the ledger has this workflow id with the DDD identity triple:
  # kind: workflow + a Bounded Context (domain) + a hexagonal_role.
  if ! python3 - "$disp_yaml" "$id" "$rel" <<'PY'
import sys
import yaml
disp_yaml, wf_id, rel = sys.argv[1], sys.argv[2], sys.argv[3]
data = yaml.safe_load(open(disp_yaml, encoding="utf-8")) or {}
workflows = data.get("workflows") or {}
entry = workflows.get(wf_id)
if entry is None:
    print(f"FAIL: workflow '{wf_id}' ({rel}) has no `workflows:` ledger entry", file=sys.stderr)
    sys.exit(1)
if entry.get("kind") != "workflow":
    print(f"FAIL: workflow '{wf_id}' ledger entry kind={entry.get('kind')!r}, expected 'workflow'", file=sys.stderr)
    sys.exit(1)
# DDD identity: a Bounded Context (domain) and a hexagonal_role are required.
if not (entry.get("domain") or "").strip():
    print(f"FAIL: workflow '{wf_id}' ({rel}) ledger row has no `domain:` (Bounded Context)", file=sys.stderr)
    sys.exit(1)
if not (entry.get("hexagonal_role") or "").strip():
    print(f"FAIL: workflow '{wf_id}' ({rel}) ledger row has no `hexagonal_role:`", file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
  then
    status=1
    continue
  fi
  echo "OK: $id ($rel) registered with kind: workflow + BC + role"
done < <(git -C "$repo_root" ls-files '.claude/workflows/*.js')

# REVERSE: every `workflows.<id>` ledger row with kind: workflow must have a
# matching repo-tracked .js. A row without one is STALE — fail naming it.
# (The id list is computed once from the .js set, parsing meta.name the same way
# the forward pass does, so a renamed-but-not-removed row is caught.)
present_ids="$(
  while IFS= read -r tracked; do
    abs="$repo_root/$tracked"
    [ -f "$abs" ] || continue
    grep -oE "name:[[:space:]]*['\"][^'\"]+['\"]" "$abs" | head -1 | sed -E "s/.*['\"]([^'\"]+)['\"].*/\1/"
  done < <(git -C "$repo_root" ls-files '.claude/workflows/*.js')
)"
if ! python3 - "$disp_yaml" "$present_ids" <<'PY'
import sys
import yaml
disp_yaml, present_blob = sys.argv[1], sys.argv[2]
present = {ln.strip() for ln in present_blob.splitlines() if ln.strip()}
data = yaml.safe_load(open(disp_yaml, encoding="utf-8")) or {}
workflows = data.get("workflows") or {}
stale = []
for wf_id, entry in workflows.items():
    entry = entry or {}
    if entry.get("kind") != "workflow":
        continue
    if wf_id not in present:
        stale.append(wf_id)
if stale:
    for wf_id in stale:
        print(f"FAIL: ledger workflow '{wf_id}' is STALE — kind: workflow row with no matching .claude/workflows/*.js", file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
then
  status=1
fi

if [ "$status" -eq 0 ]; then
  echo "OK: all repo-tracked .claude/workflows/*.js are governed (kind: workflow + BC + role), and no ledger workflow rows are stale"
fi
exit "$status"
