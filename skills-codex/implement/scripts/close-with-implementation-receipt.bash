#!/bin/bash
# Internal implementation. The public .sh launcher scrubs shell startup hooks
# before starting this file with a pinned, non-login Bash.
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFY="$SKILL_DIR/scripts/verify-implementation-receipt.sh"
die() { echo "implementation-close: $*" >&2; exit 1; }
sha_file() { if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'; else sha256sum "$1" | awk '{print $1}'; fi; }
trusted_ao() { local p h o u; h="$(/usr/bin/python3 -c 'import os,pwd; print(pwd.getpwuid(os.getuid()).pw_dir)')" || return 1; u="$(id -u)"; for p in "$h/go/bin/ao" /usr/local/bin/ao /opt/homebrew/bin/ao; do [[ -f "$p" && -x "$p" && ! -L "$p" ]] || continue; o="$(stat -f %u "$p" 2>/dev/null || stat -c %u "$p" 2>/dev/null)"; [[ "$o" == "$u" || ( "$p" == /usr/local/bin/ao || "$p" == /opt/homebrew/bin/ao ) && "$o" == "0" ]] || continue; printf '%s\n' "$p"; return; done; return 1; }
safe_rel() { [[ -n "$1" && "$1" != /* && "$1" != *$'\n'* && "$1" != "." && "$1" != ".." && "$1" != ../* && "$1" != */../* && "$1" != */.. ]]; }

ISSUE=""; RECEIPT=""
while [[ $# -gt 0 ]]; do
  case "$1" in --issue) ISSUE="${2:-}"; shift 2 ;; --receipt) RECEIPT="${2:-}"; shift 2 ;; *) die "unknown argument: $1" ;; esac
done
[[ -n "$ISSUE" && -n "$RECEIPT" ]] || die "usage: $0 --issue ID --receipt PATH"
[[ -z "${AGENTOPS_TRACKER+x}" && -z "${BEADS_DIR+x}" && -z "${AGENTOPS_REPO_ROOT+x}" && -z "${AGENTOPS_CONFIG+x}" && -z "${PYTHONPATH+x}" && -z "${PYTHONHOME+x}" && -z "${PYTHONSTARTUP+x}" && -z "${PYTHONUSERBASE+x}" && -z "${VIRTUAL_ENV+x}" && -z "${BASH_ENV+x}" && -z "${ENV+x}" && -z "${CDPATH+x}" ]] || die "tracker/config/repo-root/Python/shell environment overrides are forbidden"
REPO="$(git rev-parse --show-toplevel)"; HEAD_BEFORE="$(git -C "$REPO" rev-parse HEAD)"
[[ -z "$(git -C "$REPO" status --porcelain --untracked-files=no)" ]] || die "tracked worktree is dirty"
SOURCE="$REPO/.agents/pawl-verdicts/$ISSUE.json"; [[ -f "$SOURCE" && -s "$SOURCE" && ! -L "$SOURCE" ]] || die "canonical pawl verdict missing"
ROOT="$REPO/.agents/evidence/implement/$ISSUE/$HEAD_BEFORE"; EXPECTED="$ROOT/$ISSUE-$HEAD_BEFORE-receipt.json"
[[ "$(cd "$(dirname "$RECEIPT")" && pwd -P)/$(basename "$RECEIPT")" == "$EXPECTED" ]] || die "noncanonical receipt path"
PR="$(jq -er .pr "$SOURCE")"; DISP="$(jq -er .disposition "$SOURCE")"; PAWL="$REPO/scripts/pawl-verdict.sh"
[[ -x "$PAWL" && ! -L "$PAWL" ]] || die "pinned canonical pawl verifier unavailable"
env -u AGENTOPS_REPO_ROOT -u PAWL_UNTRUSTED_REPO PAWL_AUTOBIND=0 /bin/bash "$PAWL" check "$ISSUE" "$PR" --verdict-file "$SOURCE" --head "$HEAD_BEFORE" >/dev/null || die "canonical pawl rejected before archive mutation"
mkdir -p "$ROOT/evidence"; STAGE="$ROOT/.archive.$$"; mkdir -p "$STAGE/evidence"
SOURCE_HASH="$(sha_file "$SOURCE")"; cp "$SOURCE" "$STAGE/evidence/pawl-verdict.json"
REVIEW_JSON='[]'
while IFS= read -r rel; do
  safe_rel "$rel" || die "unsafe canonical review evidence path"
  src="$REPO/$rel"; dst="$STAGE/$rel"; [[ -f "$src" && -s "$src" && ! -L "$src" ]] || die "canonical review evidence missing"
  mkdir -p "$(dirname "$dst")"; cp "$src" "$dst"; digest="$(sha_file "$dst")"
  REVIEW_JSON="$(jq -c --arg p "$rel" --arg d "$digest" '. + [{path:$p,sha256:$d}]' <<<"$REVIEW_JSON")"
done < <(jq -r '[.refuters[]?.evidence // empty, .council_artifact // empty] | .[]' "$SOURCE")
[[ "$(jq length <<<"$REVIEW_JSON")" -gt 0 ]] || die "pawl verdict names no review evidence"
cp "$STAGE/evidence/pawl-verdict.json" "$ROOT/evidence/pawl-verdict.json"
while IFS= read -r rel; do mkdir -p "$(dirname "$ROOT/$rel")"; cp "$STAGE/$rel" "$ROOT/$rel"; done < <(jq -r '.[].path' <<<"$REVIEW_JSON")
VERDICT_HASH="$(sha_file "$ROOT/evidence/pawl-verdict.json")"
TMP="$RECEIPT.tmp.$$"
jq --arg disposition "$DISP" --argjson pr "$PR" --arg source ".agents/pawl-verdicts/$ISSUE.json" --arg vd "$VERDICT_HASH" --argjson reviews "$REVIEW_JSON" '.independent_validation={disposition:$disposition,pr:$pr,source_verdict_path:$source,copied_verdict:{path:"evidence/pawl-verdict.json",sha256:$vd},review_evidence:$reviews}' "$RECEIPT" >"$TMP"
mv "$TMP" "$RECEIPT"
RECEIPT_HASH="$(sha_file "$RECEIPT")"
MANIFEST="$ROOT/.close-manifest.$$"; : >"$MANIFEST"
printf '%s\t%s\n' "$(sha_file "$SOURCE")" "$SOURCE" >>"$MANIFEST"; printf '%s\t%s\n' "$(sha_file "$ROOT/evidence/pawl-verdict.json")" "$ROOT/evidence/pawl-verdict.json" >>"$MANIFEST"
while IFS= read -r rel; do printf '%s\t%s\n' "$(sha_file "$REPO/$rel")" "$REPO/$rel" >>"$MANIFEST"; printf '%s\t%s\n' "$(sha_file "$ROOT/$rel")" "$ROOT/$rel" >>"$MANIFEST"; done < <(jq -r '.[].path' <<<"$REVIEW_JSON")
"$VERIFY" --issue "$ISSUE" --receipt "$RECEIPT"
[[ "$(git -C "$REPO" rev-parse HEAD)" == "$HEAD_BEFORE" ]] || die "HEAD changed during verification"
[[ "$(sha_file "$SOURCE")" == "$SOURCE_HASH" && "$(sha_file "$RECEIPT")" == "$RECEIPT_HASH" ]] || die "verdict or receipt changed during verification"
while IFS=$'\t' read -r expected_hash manifest_path; do [[ "$(sha_file "$manifest_path")" == "$expected_hash" ]] || die "close manifest changed: $manifest_path"; done <"$MANIFEST"
[[ -z "$(git -C "$REPO" status --porcelain --untracked-files=no)" ]] || die "verification dirtied tracked files"
AO="$(trusted_ao)" || die "trusted ao binary unavailable"; ACCOUNT_HOME="$(/usr/bin/python3 -c 'import os,pwd; print(pwd.getpwuid(os.getuid()).pw_dir)')"
env -u AGENTOPS_TRACKER -u BEADS_DIR -u AGENTOPS_CONFIG HOME="$ACCOUNT_HOME" PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin "$AO" beads exec close "$ISSUE" -r "commit:$HEAD_BEFORE receipt:$RECEIPT receipt_sha256:$RECEIPT_HASH"
