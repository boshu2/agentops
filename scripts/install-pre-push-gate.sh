#!/usr/bin/env bash
# install-pre-push-gate.sh — wire the AgentOps cockpit pre-push gate (P1.2 / ag-qidx.2).
#
# Idempotent. Installs scripts/hooks/pre-push.local into the SHARED git hooks dir
# (git-common-dir, so it covers the main checkout and every linked worktree at
# once) and chains it from the beads-managed pre-push hook — appended AFTER the
# beads END marker so beads' managed-section rewrites never clobber it.
#
# Re-run anytime (e.g. after a fresh clone). Bypass at push time is the audited
# AGENTOPS_GATE_DISABLED=1 only.
set -euo pipefail

# Resolve repo_root from the script's own location, NOT `git rev-parse
# --show-toplevel` (age-4st3): --show-toplevel fails from a linked worktree when
# the shared config carries a core.bare pollution ("fatal: this operation must be
# run in a work tree"), which left the installer unrunnable from worktrees — the
# exact place the concurrency-safe workflow runs. Script location is robust to it.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
common="$(git rev-parse --git-common-dir)"
case "$common" in
    /*) ;;
    *) common="$(cd "$common" && pwd)" ;;
esac
hooks_dir="${common}/hooks"
mkdir -p "$hooks_dir"

src="${repo_root}/scripts/hooks/pre-push.local"
dst="${hooks_dir}/pre-push.local"
if [[ ! -f "$src" ]]; then
    echo "ERROR: source hook ${src} missing" >&2
    exit 1
fi
install -m 0755 "$src" "$dst"
echo "✓ installed ${dst}"

hook="${hooks_dir}/pre-push"
marker="# --- AGENTOPS PRE-PUSH GATE (managed by install-pre-push-gate.sh) ---"
end_marker="# --- END AGENTOPS PRE-PUSH GATE ---"
chain_block="${marker}
_agentops_common=\"\$(git rev-parse --git-common-dir)\"
_agentops_local=\"\${_agentops_common}/hooks/pre-push.local\"
# Self-heal (age-4st3): a landed gate fix is inert until the installed copy is
# refreshed, and NOTHING re-installs it automatically — so a stale gate runs
# indefinitely. Before gating, refresh pre-push.local from TRUNK (origin/main)
# when it drifts. Trust the trunk gate, never the pushed branch's working tree
# (a branch must not weaken — or, off an old base, silently downgrade — its own
# push gate). Best-effort: if trunk is unavailable the existing copy still runs.
_agentops_trunk=\"\$(git show origin/main:scripts/hooks/pre-push.local 2>/dev/null)\"
if [ -n \"\$_agentops_trunk\" ] && ! printf '%s' \"\$_agentops_trunk\" | cmp -s - \"\$_agentops_local\" 2>/dev/null; then
  printf '%s' \"\$_agentops_trunk\" > \"\$_agentops_local\" && chmod +x \"\$_agentops_local\"
fi
if [ -x \"\$_agentops_local\" ]; then
  \"\$_agentops_local\" \"\$@\" || exit \$?
fi
${end_marker}"

if [[ ! -f "$hook" ]]; then
    printf '#!/usr/bin/env sh\n%s\n' "$chain_block" > "$hook"
    chmod +x "$hook"
    echo "✓ created ${hook} with gate chain"
elif grep -qF "$marker" "$hook"; then
    # Replace the existing block (between markers) so a stale chainer — e.g. one
    # without the self-heal above — is refreshed on reinstall instead of a no-op.
    # Python (not awk -v: that can't take a multi-line replacement value).
    HOOK="$hook" MARKER="$marker" END_MARKER="$end_marker" CHAIN_BLOCK="$chain_block" \
        python3 - <<'PY'
import os, re
hook = os.environ["HOOK"]
text = open(hook, encoding="utf-8").read()
pattern = re.compile(re.escape(os.environ["MARKER"]) + r".*?" + re.escape(os.environ["END_MARKER"]), re.DOTALL)
new, n = pattern.subn(os.environ["CHAIN_BLOCK"], text, count=1)
if n:
    open(hook, "w", encoding="utf-8").write(new)
PY
    chmod +x "$hook"
    echo "✓ refreshed gate chain in ${hook} (replaced managed block)"
else
    printf '\n%s\n' "$chain_block" >> "$hook"
    echo "✓ appended gate chain to existing ${hook} (after beads section)"
fi

# Runtime-file rebase ergonomics (age-uqj). This repo tracks runtime audit logs
# (.agents/rpi/next-work.jsonl, .agents/findings/registry.jsonl, the provenance
# ledger) that tooling dirties mid-session, so a plain `git pull --rebase` fails
# with "cannot rebase: you have unstaged changes". autoStash makes rebase
# transparently stash the dirty tree and reapply it afterward. These logs are
# intentionally NOT union-merged (they are mutated in place / hash-chained — see
# .gitattributes), so a genuine divergence still surfaces as a real conflict on
# reapply rather than being silently merged. Repo-local only; never touches global.
git config --local rebase.autoStash true
echo "✓ set rebase.autoStash=true (local) — pull --rebase no longer blocks on dirty runtime logs"

# Heal core.bare pollution (age-4st3). A test that flips core.bare=true and fails
# to reset it (the gate's own "open core.bare pollution at shuffle=2") leaves the
# SHARED config bare=true, which makes every git op in the main checkout AND all
# linked worktrees fail with "fatal: this operation must be run in a work tree" —
# silently freezing worktree commits/pushes (the concurrency-safe workflow). This
# repo is never bare, so force it false. (The leaking test still needs fixing — a
# separate bead — this just stops the pollution from bricking worktrees.)
if [[ "$(git config --local --get core.bare 2>/dev/null)" == "true" ]]; then
    git config --local core.bare false
    echo "✓ healed core.bare=true → false (was bricking worktree git ops)"
fi

echo ""
echo "Cockpit pre-push gate active for this repo (all worktrees)."
echo "Audited bypass (logged): AGENTOPS_GATE_DISABLED=1 git push ..."
