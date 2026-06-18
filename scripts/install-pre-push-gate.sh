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

repo_root="$(git rev-parse --show-toplevel)"
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
chain_block="${marker}
_agentops_local=\"\$(git rev-parse --git-common-dir)/hooks/pre-push.local\"
if [ -x \"\$_agentops_local\" ]; then
  \"\$_agentops_local\" \"\$@\" || exit \$?
fi
# --- END AGENTOPS PRE-PUSH GATE ---"

if [[ ! -f "$hook" ]]; then
    printf '#!/usr/bin/env sh\n%s\n' "$chain_block" > "$hook"
    chmod +x "$hook"
    echo "✓ created ${hook} with gate chain"
elif grep -qF "$marker" "$hook"; then
    echo "✓ ${hook} already chains the gate (idempotent no-op)"
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

echo ""
echo "Cockpit pre-push gate active for this repo (all worktrees)."
echo "Audited bypass (logged): AGENTOPS_GATE_DISABLED=1 git push ..."
