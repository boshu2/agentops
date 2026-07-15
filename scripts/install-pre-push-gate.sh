#!/usr/bin/env bash
# install-pre-push-gate.sh — wire the ordinary deterministic pre-push checks.
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

# Guard against a hijacked core.hooksPath (recon-2026-07-02 W4 / audit A2). When
# core.hooksPath is set, git runs THAT dir and IGNORES ${common}/hooks entirely —
# so a bd install (or any tool) that redirects core.hooksPath at its own dir (e.g.
# .beads/hooks) leaves the gate we install here INERT: the installer "succeeds"
# but ao gate never runs on push. Detect it and refuse, so the operator fixes the
# redirect instead of trusting a gate that will not fire. Opt out with
# AGENTOPS_ALLOW_HOOKSPATH=1 (installs anyway, knowing the gate stays inert).
hookspath="$(git config --get core.hooksPath 2>/dev/null || true)"
if [[ -n "$hookspath" ]]; then
    case "$hookspath" in
        /*) resolved_hookspath="$hookspath" ;;
        *)  resolved_hookspath="$(cd "$repo_root" 2>/dev/null && cd "$hookspath" 2>/dev/null && pwd || echo "$hookspath")" ;;
    esac
    if [[ "$resolved_hookspath" != "$hooks_dir" ]]; then
        echo "!!  core.hooksPath is set to '${hookspath}' (resolved: ${resolved_hookspath})," >&2
        echo "    but this gate installs into '${hooks_dir}'. git runs the hooksPath dir and" >&2
        echo "    IGNORES ${hooks_dir} — the gate would be installed but INERT on push." >&2
        echo "" >&2
        echo "    Remediation (pick one):" >&2
        echo "      * git config --unset core.hooksPath      # then re-run this installer" >&2
        echo "      * or chain scripts/hooks/pre-push.local from '${resolved_hookspath}/pre-push'" >&2
        echo "    Override (install anyway, gate stays inert): AGENTOPS_ALLOW_HOOKSPATH=1" >&2
        if [[ "${AGENTOPS_ALLOW_HOOKSPATH:-0}" != "1" ]]; then
            echo "ERROR: refusing to install an inert gate. Fix core.hooksPath or set AGENTOPS_ALLOW_HOOKSPATH=1." >&2
            exit 1
        fi
        echo "    AGENTOPS_ALLOW_HOOKSPATH=1 set — installing anyway; the gate will NOT run until core.hooksPath is fixed." >&2
    fi
fi

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
preamble_marker="# --- AGENTOPS PRE-PUSH STDIN SNAPSHOT (managed by install-pre-push-gate.sh) ---"
preamble_end_marker="# --- END AGENTOPS PRE-PUSH STDIN SNAPSHOT ---"
preamble_block="${preamble_marker}
_agentops_pre_push_stdin=\"\"
_agentops_cleanup_pre_push_stdin() {
  [ -n \"\${_agentops_pre_push_stdin:-}\" ] && rm -f \"\$_agentops_pre_push_stdin\" 2>/dev/null || true
}
trap _agentops_cleanup_pre_push_stdin EXIT HUP INT TERM
if [ ! -t 0 ]; then
  _agentops_pre_push_stdin=\"\$(mktemp \"\${TMPDIR:-/tmp}/agentops-installed-prepush-stdin.XXXXXX\" 2>/dev/null)\" || _agentops_pre_push_stdin=\"\"
  if [ -n \"\$_agentops_pre_push_stdin\" ]; then
    cat >\"\$_agentops_pre_push_stdin\" || _agentops_pre_push_stdin=\"\"
    [ -n \"\$_agentops_pre_push_stdin\" ] && exec < \"\$_agentops_pre_push_stdin\"
  fi
fi
${preamble_end_marker}"
marker="# --- AGENTOPS PRE-PUSH GATE (managed by install-pre-push-gate.sh) ---"
end_marker="# --- END AGENTOPS PRE-PUSH GATE ---"
chain_block="${marker}
_agentops_common=\"\$(git rev-parse --git-common-dir)\"
_agentops_local=\"\${_agentops_common}/hooks/pre-push.local\"
# Self-heal (age-4st3): a landed gate fix is inert until the installed copy is
# refreshed, and NOTHING re-installs it automatically — so a stale gate runs
# indefinitely. Before gating, refresh pre-push.local from TRUNK (origin/main)
# when it drifts. Trust the trunk gate by default. The only branch-side
# exception is an explicit fast-forward push to main/master whose installed hook
# already matches the pushed local SHA; that lets hook changes dogfood their own
# landing push without letting stale or uninstalled branch hooks weaken the gate.
# Best-effort: if trunk is unavailable the existing copy still runs.
_agentops_push_local_sha=\"\"
_agentops_push_remote_sha=\"\"
if [ -n \"\${_agentops_pre_push_stdin:-}\" ] && [ -f \"\$_agentops_pre_push_stdin\" ]; then
  _agentops_push_row=\"\$(awk '\$3 ~ /^refs\\/heads\\/(main|master)\$/ { print \$2 \" \" \$4; exit }' \"\$_agentops_pre_push_stdin\" 2>/dev/null)\"
  case \"\$_agentops_push_row\" in
    *' '*)
      _agentops_push_local_sha=\${_agentops_push_row%% *}
      _agentops_push_remote_sha=\${_agentops_push_row#* }
      ;;
  esac
fi
_agentops_trunk_ref=\"origin/main\"
case \"\$_agentops_push_remote_sha\" in
  ''|0000000000000000000000000000000000000000) ;;
  *) _agentops_trunk_ref=\"\$_agentops_push_remote_sha\" ;;
esac
_agentops_candidate_ref=\"\$_agentops_push_local_sha\"
_agentops_trunk_blob=\"\$(git rev-parse \"\${_agentops_trunk_ref}:scripts/hooks/pre-push.local\" 2>/dev/null)\"
_agentops_candidate_blob=\"\"
if [ -n \"\$_agentops_candidate_ref\" ]; then
  _agentops_candidate_blob=\"\$(git rev-parse \"\${_agentops_candidate_ref}:scripts/hooks/pre-push.local\" 2>/dev/null)\"
fi
_agentops_local_blob=\"\"
if [ -f \"\$_agentops_local\" ]; then
  _agentops_local_blob=\"\$(git hash-object \"\$_agentops_local\" 2>/dev/null)\"
fi
_agentops_preserve_candidate_hook=0
if [ -n \"\$_agentops_push_local_sha\" ] && [ -n \"\$_agentops_push_remote_sha\" ] \
  && [ -n \"\$_agentops_trunk_blob\" ] && [ -n \"\$_agentops_candidate_blob\" ] \
  && git merge-base --is-ancestor \"\$_agentops_push_remote_sha\" \"\$_agentops_push_local_sha\" 2>/dev/null \
  && [ \"\$_agentops_local_blob\" = \"\$_agentops_candidate_blob\" ] \
  && [ \"\$_agentops_local_blob\" != \"\$_agentops_trunk_blob\" ]; then
  _agentops_preserve_candidate_hook=1
  echo >&2 \"pre-push: hook-source=local-sha:\$_agentops_push_local_sha\"
fi
if [ \"\$_agentops_preserve_candidate_hook\" != \"1\" ] \
  && [ -n \"\$_agentops_trunk_blob\" ] \
  && [ \"\$_agentops_local_blob\" != \"\$_agentops_trunk_blob\" ]; then
  _agentops_tmp_local=\"\$(mktemp \"\${_agentops_local}.XXXXXX\" 2>/dev/null)\" || _agentops_tmp_local=\"\"
  if [ -n \"\$_agentops_tmp_local\" ]; then
    if git show \"\${_agentops_trunk_ref}:scripts/hooks/pre-push.local\" > \"\$_agentops_tmp_local\" 2>/dev/null && chmod +x \"\$_agentops_tmp_local\"; then
      mv \"\$_agentops_tmp_local\" \"\$_agentops_local\"
      echo >&2 \"pre-push: hook-source=trunk:\$_agentops_trunk_ref\"
    else
      rm -f \"\$_agentops_tmp_local\"
    fi
  fi
fi
if [ -x \"\$_agentops_local\" ]; then
  if [ -n \"\${_agentops_pre_push_stdin:-}\" ] && [ -f \"\$_agentops_pre_push_stdin\" ]; then
    \"\$_agentops_local\" \"\$@\" < \"\$_agentops_pre_push_stdin\" || exit \$?
  else
    \"\$_agentops_local\" \"\$@\" || exit \$?
  fi
fi
${end_marker}"

if [[ ! -f "$hook" ]]; then
    printf '#!/usr/bin/env sh\n%s\n%s\n' "$preamble_block" "$chain_block" > "$hook"
    chmod +x "$hook"
    echo "✓ created ${hook} with gate chain"
else
    if grep -qF "$preamble_marker" "$hook"; then
        HOOK="$hook" MARKER="$preamble_marker" END_MARKER="$preamble_end_marker" CHAIN_BLOCK="$preamble_block" \
            python3 - <<'PY'
import os, re
hook = os.environ["HOOK"]
text = open(hook, encoding="utf-8").read()
pattern = re.compile(re.escape(os.environ["MARKER"]) + r".*?" + re.escape(os.environ["END_MARKER"]), re.DOTALL)
new, n = pattern.subn(os.environ["CHAIN_BLOCK"], text, count=1)
if n:
    open(hook, "w", encoding="utf-8").write(new)
PY
    else
        tmp="$(mktemp "${hook}.XXXXXX")"
        {
            head -n 1 "$hook"
            printf '%s\n' "$preamble_block"
            tail -n +2 "$hook"
        } > "$tmp"
        mv "$tmp" "$hook"
    fi

    if grep -qF "$marker" "$hook"; then
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
    chmod +x "$hook"
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
echo "Deterministic pre-push checks active for this repo (all worktrees)."
echo "Audited bypass (logged): AGENTOPS_GATE_DISABLED=1 git push ..."
