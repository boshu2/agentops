#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: verify-landed.sh <bead-id> [--repo <path>] [--remote <name>]" >&2
}

[[ $# -ge 1 ]] || { usage; exit 2; }
BEAD="$1"
shift
REPO=""
REMOTE="origin"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) [[ $# -ge 2 ]] || { usage; exit 2; }; REPO="$2"; shift 2 ;;
    --remote) [[ $# -ge 2 ]] || { usage; exit 2; }; REMOTE="$2"; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -z "$REPO" ]]; then
  REPO="$(git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "verify-landed: not inside a git worktree" >&2
    exit 2
  }
fi
REPO="$(cd "$REPO" && pwd -P)"
PAWL_VERDICT="$REPO/scripts/pawl-verdict.sh"
VERDICT_DIR="$REPO/.agents/pawl-verdicts"

[[ -x "$PAWL_VERDICT" || -f "$PAWL_VERDICT" ]] || {
  echo "verify-landed: missing $PAWL_VERDICT" >&2
  exit 2
}
git -C "$REPO" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "verify-landed: $REPO is not a git worktree" >&2
  exit 2
}

git -C "$REPO" fetch --quiet "$REMOTE" main || {
  echo "verify-landed: could not fetch $REMOTE main" >&2
  exit 1
}
TIP="$(git -C "$REPO" rev-parse HEAD)"
REMOTE_TIP="$(git -C "$REPO" rev-parse "refs/remotes/$REMOTE/main" 2>/dev/null)" || {
  echo "verify-landed: missing refs/remotes/$REMOTE/main after fetch" >&2
  exit 1
}
if [[ "$TIP" != "$REMOTE_TIP" ]]; then
  echo "verify-landed: stale remote identity: tip=$TIP remote=$REMOTE_TIP" >&2
  exit 1
fi

# The auto-bind commit has one canonical, two-factor signature: the exact
# generated subject for this bead AND a non-empty diff containing only the
# provenance ledger. A marker alone or a provenance-only commit alone never
# changes which SHA was reviewed.
is_canonical_autobind() {
  local sha="$1" subject changed
  subject="$(git -C "$REPO" log -1 --format=%s "$sha")"
  [[ "$subject" == "chore(provenance): bind pawl CONFIRMED verdict for $BEAD #trivial" ]] || return 1
  changed="$(git -C "$REPO" diff-tree --no-commit-id --no-renames --name-only -r "$sha")" || return 1
  [[ "$changed" == "docs/provenance/ledger.jsonl" ]]
}

REVIEWED="$TIP"
if is_canonical_autobind "$TIP"; then
  REVIEWED="$(git -C "$REPO" rev-parse "$TIP^")" || {
    echo "verify-landed: canonical bind tip has no parent: $TIP" >&2
    exit 1
  }
fi

MESSAGE="$(git -C "$REPO" log -1 --format=%B "$REVIEWED")"
CITED="$(grep -oE '[[:alnum:]]+-[[:alnum:]][[:alnum:].-]*' <<<"$MESSAGE" | sed 's/[.,;:]*$//' | sort -u || true)"
if ! grep -Fqx -- "$BEAD" <<<"$CITED"; then
  echo "verify-landed: reviewed commit $REVIEWED does not cite bead $BEAD" >&2
  exit 1
fi

if ! (cd "$REPO" && AGENTOPS_REPO_ROOT="$REPO" PAWL_AUTOBIND=0 \
  bash "$PAWL_VERDICT" check "$BEAD" 0 --dir "$VERDICT_DIR" --head "$REVIEWED"); then
  echo "verify-landed: pawl verdict does not authorize reviewed=$REVIEWED for bead=$BEAD" >&2
  exit 1
fi

printf 'verify-landed: tip=%s reviewed=%s bead=%s\n' "$TIP" "$REVIEWED" "$BEAD"
