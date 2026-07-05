#!/usr/bin/env bash
# scaffold-quest.sh — deterministically instantiate quests/<slug>/ from
# quests/_template/. This is the MECHANICAL half of planner intake (operating-loop
# move 1: shape intent as a BDD acceptance contract): the planner invokes it to
# get a well-formed, default-FAIL quest skeleton, then authors CONTRACT.md's
# numbered acceptance clauses from the one-line ask.
#
# It is scaffold-ONLY and fail-closed:
#   * it NEVER edits an existing quest's impl code — it refuses (BLOCKED) if the
#     destination already exists. Creating a fresh skeleton is the planner's ONE
#     write surface; this tool is that surface made mechanical.
#   * gc's harness permission_mode is coarse (plan | auto-edit | unrestricted —
#     no path-scoped write allowlist; see README §Honest gaps + gascity
#     internal/worker/builtin/profiles.go). So the ENFORCED RBAC boundary is this
#     tool being the sole write path, not a harness flag.
#
# Deterministic + dependency-light: bash + (optional) git only, no sed/awk/mv, so
# it is cheaply unit-provable (tests/intake.bats).
#
# Usage:
#   scaffold-quest.sh <slug> [--root <quests-dir>] [--ask <one-line ask>] [--no-git]
# Exit codes:
#   0 = SCAFFOLDED   2 = usage/internal error   3 = BLOCKED (bad slug / exists)
set -euo pipefail

usage() {
  echo "usage: scaffold-quest.sh <slug> [--root <quests-dir>] [--ask <text>] [--no-git]" >&2
  exit 2
}

SLUG=""; ROOT=""; ASK=""; DO_GIT=1
while [ $# -gt 0 ]; do
  case "$1" in
    --root)    ROOT="${2:?--root needs a value}"; shift 2 ;;
    --ask)     ASK="${2:?--ask needs a value}"; shift 2 ;;
    --no-git)  DO_GIT=0; shift ;;
    -h|--help) usage ;;
    --) shift; break ;;
    -*) echo "scaffold: unknown flag: $1" >&2; usage ;;
    *)  if [ -z "$SLUG" ]; then SLUG="$1"; shift; else echo "scaffold: unexpected arg: $1" >&2; usage; fi ;;
  esac
done
[ -n "$SLUG" ] || usage

# --- slug guard: same pattern as membrane-quest.toml [vars.quest] -------------
if ! printf '%s' "$SLUG" | grep -Eq '^[a-z0-9][a-z0-9-]*$'; then
  echo "BLOCKED reason=bad_slug slug='$SLUG' (must match ^[a-z0-9][a-z0-9-]*\$)" >&2
  exit 3
fi
if [ "$SLUG" = "_template" ]; then
  echo "BLOCKED reason=reserved_slug slug='_template' (that is the template itself)" >&2
  exit 3
fi

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # .../packs/agentops-membrane/membrane
PACK="$(cd "$HERE/.." && pwd)"                          # .../packs/agentops-membrane
TEMPLATE="$PACK/quests/_template"
ROOT="${ROOT:-$PACK/quests}"
DEST="$ROOT/$SLUG"

[ -d "$TEMPLATE" ] || { echo "scaffold: template missing at $TEMPLATE" >&2; exit 2; }

# --- fail-closed: never overwrite / edit an existing quest --------------------
if [ -e "$DEST" ]; then
  echo "BLOCKED reason=quest_exists dest='$DEST' (scaffold never overwrites or edits existing impl)" >&2
  exit 3
fi

mkdir -p "$DEST"
# copy the three build files; the template's own README explains the shape and is
# NOT carried into instantiated quests.
cp "$TEMPLATE/CONTRACT.md" "$DEST/CONTRACT.md"
cp "$TEMPLATE/test.sh"     "$DEST/test.sh"
cp "$TEMPLATE/impl.sh"     "$DEST/impl.sh"
chmod +x "$DEST/test.sh" "$DEST/impl.sh"

# --- deterministic substitution (bash-native; no sed/awk/mv, & and \ literal) -
ASK_TEXT="${ASK:-<one-line ask — planner fills this from the request>}"
subst() { # <basename>
  local f="$DEST/$1" c
  c="$(cat "$f")"
  c="${c//\{\{QUEST\}\}/$SLUG}"
  c="${c//\{\{SLUG\}\}/$SLUG}"
  c="${c//\{\{ASK\}\}/$ASK_TEXT}"
  printf '%s\n' "$c" > "$f"
}
subst CONTRACT.md
subst test.sh
subst impl.sh
chmod +x "$DEST/test.sh" "$DEST/impl.sh"

# --- init the quest git repo on `main` (the ruler branch the close gate reads) -
# close-gate.sh reads `git show main:CONTRACT.md`; the builder worktrees a
# quest/<slug> branch off main. All local — no remote, no network.
if [ "$DO_GIT" = "1" ] && command -v git >/dev/null 2>&1; then
  git -C "$DEST" init -q
  git -C "$DEST" add -A
  git -C "$DEST" -c user.email=scaffold@membrane -c user.name=membrane-scaffold \
    -c commit.gpgsign=false commit -q -m "scaffold quest $SLUG from _template (default-FAIL)"
  git -C "$DEST" branch -M main 2>/dev/null || true
fi

echo "SCAFFOLDED dest='$DEST' branch=main (CONTRACT.md default-FAIL; ./test.sh exits nonzero until implemented)"
exit 0
