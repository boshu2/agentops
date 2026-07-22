#!/usr/bin/env bash
set -euo pipefail

die() { printf 'agentops-refine: %s\n' "$*" >&2; exit 1; }
usage() { printf '%s\n' 'Usage: refine.sh --worktree PATH --bead ID [--base-ref NAME] [--mode auto|manual] [--gate PATH]'; }
worktree=""; bead=""; base_ref="main"; mode="auto"; gate="${AGENTOPS_GC_GATE:-}"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --worktree) worktree="${2:?--worktree requires a path}"; shift 2 ;;
    --bead) bead="${2:?--bead requires an id}"; shift 2 ;;
    --base-ref) base_ref="${2:?--base-ref requires a name}"; shift 2 ;;
    --mode) mode="${2:?--mode requires a value}"; shift 2 ;;
    --gate) gate="${2:?--gate requires a path}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$worktree" ] && [ -n "$bead" ] || die "worktree and bead are required"
case "$mode" in auto|manual) ;; *) die "mode must be auto or manual" ;; esac
[[ "$bead" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || die "unsafe bead id"
[[ "$base_ref" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$ ]] || die "unsafe base ref"
git_bin="${AGENTOPS_GC_GIT_BIN:-git}"
gh_bin="${AGENTOPS_GC_GH_BIN:-gh}"
worktree="$(python3 - "$worktree" <<'PY'
import os,sys
print(os.path.realpath(sys.argv[1]))
PY
)"
"$git_bin" -C "$worktree" rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a Git worktree"
[ -z "$("$git_bin" -C "$worktree" status --porcelain)" ] || die "candidate worktree is dirty"
branch="$("$git_bin" -C "$worktree" branch --show-current)"
[ -n "$branch" ] && [ "$branch" != "$base_ref" ] || die "candidate branch is invalid"
validated_head="$("$git_bin" -C "$worktree" rev-parse HEAD)"
validated_base="$("$git_bin" -C "$worktree" merge-base "origin/$base_ref" HEAD)"
candidate_digest() {
  python3 - "$git_bin" "$worktree" "$1" <<'PY'
import hashlib, subprocess, sys
git, worktree, base = sys.argv[1:]
names = subprocess.check_output([git, "-C", worktree, "diff", "--name-only", "-z", f"{base}..HEAD"]).split(b"\0")
h = hashlib.sha256()
for name in sorted(value for value in names if value):
    entry = subprocess.check_output([git, "-C", worktree, "ls-tree", "-z", "HEAD", "--", name])
    h.update(name + b"\0" + (entry if entry else b"DELETED\0"))
print(h.hexdigest())
PY
}
validated_candidate="$(candidate_digest "$validated_base")"
rebases=0

sync_base() {
  local after
  "$git_bin" -C "$worktree" fetch origin "$base_ref" --quiet
  if "$git_bin" -C "$worktree" merge-base --is-ancestor "origin/$base_ref" HEAD; then return 0; fi
  [ "$rebases" -eq 0 ] || return 3
  if ! "$git_bin" -C "$worktree" rebase "origin/$base_ref"; then
    "$git_bin" -C "$worktree" rebase --abort >/dev/null 2>&1 || true
    return 3
  fi
  after="$(candidate_digest "$("$git_bin" -C "$worktree" merge-base "origin/$base_ref" HEAD)")"
  [ "$after" = "$validated_candidate" ] || die "rebase changed validated candidate paths or content"
  rebases=1
}
sync_base || exit $?
if [ -n "$gate" ]; then
  [ -x "$gate" ] || die "gate is not executable: $gate"
  (cd "$worktree" && "$gate")
fi
head_oid="$("$git_bin" -C "$worktree" rev-parse HEAD)"
"$git_bin" -C "$worktree" push --quiet --force-with-lease -u origin "$branch"

pr_json="$("$gh_bin" pr list --head "$branch" --state open --limit 1 --json number,url,headRefOid)"
pr_number="$(python3 - "$pr_json" <<'PY'
import json,sys
rows=json.loads(sys.argv[1]); print(rows[0]["number"] if rows else "")
PY
)"
if [ -z "$pr_number" ]; then
  pr_url="$("$gh_bin" pr create --base "$base_ref" --head "$branch" --title "[$bead] validated AgentOps candidate" --body "Validated candidate for bead $bead.")"
  pr_json="$("$gh_bin" pr view "$pr_url" --json number,url,headRefOid)"
else
  pr_json="$("$gh_bin" pr view "$pr_number" --json number,url,headRefOid)"
fi
read -r pr_number pr_url remote_head < <(python3 - "$pr_json" <<'PY'
import json,sys
p=json.loads(sys.argv[1]); print(p["number"],p["url"],p["headRefOid"])
PY
)
[ "$remote_head" = "$head_oid" ] || die "PR head differs from validated candidate"

python3 - "$gh_bin" "$pr_number" <<'PY'
import subprocess,sys
try:
    result=subprocess.run([sys.argv[1],"pr","checks",sys.argv[2],"--watch","--fail-fast"],timeout=900)
except subprocess.TimeoutExpired:
    raise SystemExit("hosted checks timed out")
raise SystemExit(result.returncode)
PY
sync_base || exit $?
new_head="$("$git_bin" -C "$worktree" rev-parse HEAD)"
if [ "$new_head" != "$head_oid" ]; then
  head_oid="$new_head"
  "$git_bin" -C "$worktree" push --quiet --force-with-lease origin "$branch"
  python3 - "$gh_bin" "$pr_number" <<'PY'
import subprocess,sys
try: result=subprocess.run([sys.argv[1],"pr","checks",sys.argv[2],"--watch","--fail-fast"],timeout=900)
except subprocess.TimeoutExpired: raise SystemExit("hosted checks timed out")
raise SystemExit(result.returncode)
PY
fi
pr_json="$("$gh_bin" pr view "$pr_number" --json number,url,headRefOid,state)"
remote_head="$(python3 - "$pr_json" <<'PY'
import json,sys
print(json.loads(sys.argv[1])["headRefOid"])
PY
)"
[ "$remote_head" = "$head_oid" ] || die "PR head moved after hosted checks"
if [ "$mode" = "auto" ]; then
  "$gh_bin" pr merge "$pr_number" --auto --squash --delete-branch
  python3 - "$gh_bin" "$pr_number" <<'PY'
import json, subprocess, sys, time
deadline = time.monotonic() + 900
while time.monotonic() < deadline:
    result = subprocess.run(
        [sys.argv[1], "pr", "view", sys.argv[2], "--json", "state,mergeStateStatus"],
        check=False, capture_output=True, text=True,
    )
    if result.returncode:
        raise SystemExit((result.stderr or result.stdout).strip())
    state = json.loads(result.stdout)
    if state.get("state") == "MERGED":
        break
    if state.get("state") == "CLOSED":
        raise SystemExit("PR closed without merging")
    time.sleep(2)
else:
    raise SystemExit("PR did not merge before timeout")
PY
fi
python3 - "$pr_json" "$mode" "$rebases" "$validated_head" "$validated_candidate" <<'PY'
import json,sys
p=json.loads(sys.argv[1]); print(json.dumps({"mode":sys.argv[2],"pr":p["number"],"url":p["url"],"head":p["headRefOid"],"rebases":int(sys.argv[3]),"validated_head":sys.argv[4],"candidate_digest":sys.argv[5]},sort_keys=True,separators=(",",":")))
PY
