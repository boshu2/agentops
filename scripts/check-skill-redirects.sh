#!/usr/bin/env bash
# check-skill-redirects.sh — folded-skill redirect-validity gate (age-rhlx).
#
# A folded skill is a redirect: `state: merged-into` in
# docs/contracts/skill-dispositions.yaml means the skill's content moved to the
# `merged-into:` target. People still arrive at the old skill's path (GitHub
# search, cached links, our own docs). If a LATER rename/prune deletes the fold
# TARGET, the redirect silently points at a skill that no longer exists and the
# inbound visitor dead-ends with no breadcrumb.
#
# This gate keeps the folded-skill map's targets real: every `merged-into`
# disposition MUST resolve — following the merged-into chain — to a live skill
# (skills/<name>/SKILL.md), with no cycles. `cut`/`retired`/`removed` skills
# have no successor and need no target.
#
# It does NOT try to make a deleted GitHub blob URL resolve (GitHub has no
# per-file redirect — that is structurally impossible in-repo). It guarantees
# the one thing the repo CAN: the redirect target always exists, so the
# disposition ledger is a valid map from any folded skill to a real one.
#
# Exit 0: every merged-into target resolves to a live skill; no cycles.
# Exit 1: a fold target does not resolve to a live skill, or a chain cycles.
#
# Repo root from cwd git. No hardcoded paths. python3 for parsing (matches
# check-registry-drift.sh); no GNU-only shell constructs.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
disp_yaml="$repo_root/docs/contracts/skill-dispositions.yaml"

if [ ! -f "$disp_yaml" ]; then
  echo "FAIL: ledger not found: $disp_yaml" >&2
  exit 1
fi

REPO_ROOT="$repo_root" DISP_YAML="$disp_yaml" python3 - <<'PY'
import os, re, sys

repo = os.environ["REPO_ROOT"]
disp = os.environ["DISP_YAML"]

# Parse every 2-space-indented `name:` block that carries a `state:` key. Only
# the `historical:` section uses `state:`; the `dispositions:` list uses
# `- skill:` / `disposition:` and the `workflows:` section uses `kind:`, so
# neither is picked up. This also matches a fixture that nests the blocks under
# a `skills:` parent — the discriminator is the `state:` field, not the parent.
folded = {}   # name -> {"state":..., "target":...}
cur = None
for ln in open(disp, encoding="utf-8").read().splitlines():
    m = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", ln)
    if m:
        cur = m.group(1)
        continue
    if cur is None:
        continue
    ms = re.match(r"^    state:\s*(\S+)", ln)
    if ms:
        folded.setdefault(cur, {})["state"] = ms.group(1)
        continue
    mt = re.match(r"^    merged-into:\s*(\S+)", ln)
    if mt:
        folded.setdefault(cur, {})["target"] = mt.group(1).strip('"')

def is_live(name):
    return os.path.isfile(os.path.join(repo, "skills", name, "SKILL.md"))

merged = {n: d.get("target") for n, d in folded.items()
          if d.get("state") == "merged-into"}
terminal = {n for n, d in folded.items()
            if d.get("state") in ("cut", "retired", "removed")}

broken = []   # (name, reason)
cycles = []   # name where the chain loops

def resolve(name):
    """Follow merged-into from `name`'s target to a live skill. Return True if a
    live skill is reached, else False. Records a cycle if the chain loops."""
    seen = set()
    cur = merged.get(name)
    while True:
        if cur is None:
            return False
        if cur in seen:
            cycles.append(name)
            return False
        seen.add(cur)
        if cur in merged:          # cur is itself folded (declared gone) — it is
            cur = merged[cur]      # NEVER a valid terminal, even if a stale
            continue               # skills/<cur>/SKILL.md lingers. Chasing its
                                   # target is what makes a cycle among folded
                                   # nodes always revisit a node (-> detected)
                                   # instead of short-circuiting on a live file.
        if is_live(cur):           # genuine terminal: a live, non-folded skill
            return True
        return False               # neither a live terminal nor a known fold

for name, target in sorted(merged.items()):
    if not target:
        broken.append((name, "no merged-into target"))
        continue
    if not resolve(name):
        if name in cycles:
            broken.append((name, f"merged-into chain cycles (starts at -> {target})"))
        else:
            broken.append((name, f"fold target '{target}' does not resolve to a live skill (skills/{target}/SKILL.md missing)"))

print(f"check-skill-redirects: {len(merged)} merged-into, {len(terminal)} cut/retired, "
      f"{len(broken)} broken")

if broken:
    print("\nFAIL: folded skill(s) whose redirect target is a dead end:", file=sys.stderr)
    for n, why in broken:
        print(f"  - {n}: {why}", file=sys.stderr)
    print("\nFix: repoint the disposition's merged-into to a live skill, or restore the "
          "target skill. The ledger must map every folded skill to a real successor.",
          file=sys.stderr)
    sys.exit(1)

print("OK: every merged-into skill resolves to a live skill; no cycles.")
PY
