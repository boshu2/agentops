#!/usr/bin/env bash
# check-frontdoor-admission.sh — the M5 governance front-door admission guard
# (age-d16-self-hosting-route-nkr.6).
#
# WHAT: a kind-unified ADMISSION GUARD. A newly-ADDED skill / workflow / loop
# implementation cannot merge unless its front-door evidence shows all three facts:
#   1. bounded-context FOUND   — the unit is placed in a BC (BC1..BC6)
#   2. role ASSIGNED           — the unit declares a hexagonal_role
#   3. acceptance RUN          — the unit carries a runnable acceptance
# Missing any fact for any added unit => FAIL-CLOSED (exit 1): the gate blocks
# the merge. This is ENFORCEMENT at the door (wired into the pre-push gate +
# scripts/reconcile-pr.sh); scripts/intake.sh stays the ADVISORY early-warning.
#
# SCOPE = added-only. The guard governs NEW units (git --diff-filter=A vs the
# base), never retroactively re-judging existing skills/workflows — so a routine
# edit elsewhere is never blocked by a legacy unit that predates the guard.
#
# Evidence sources (committed, self-contained — no regeneration here):
#   - skills:    role  <- skills/<name>/SKILL.md frontmatter `hexagonal_role:`
#                BC    <- docs/reference/agentops-skill-domain-map.md row (BC<n>)
#                accept<- skills/<name>/references/<name>.feature OR `## Scenarios`
#   - workflows: BC+role <- docs/contracts/skill-dispositions.yaml `workflows:`
#                           entry (`domain: "BC.."` + `hexagonal_role:`)
#                accept  <- a registered ledger entry (`kind:` present) — the
#                           .js<->ledger bijection IS the workflow's acceptance.
#
# Non-goal: re-litigate the intake schema; regenerate the maps; judge existing
# units. Law 0: only git/grep/sed/awk — never `claude -p`.
#
# Exit codes: 0 admitted (or nothing new to admit) · 1 BLOCKED (missing
# evidence) · 2 usage error.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="origin/main"
SKILLS_ROOT="$REPO_ROOT/skills"
DOMAIN_MAP="$REPO_ROOT/docs/reference/agentops-skill-domain-map.md"
DISPOSITIONS="$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
JSON=0
ADDED=()

usage() {
  cat >&2 <<'EOF'
Usage: check-frontdoor-admission.sh [--base <ref>] [--added <path>]... [opts]

  --base <ref>         git ref to diff against for ADDED units (default: origin/main)
  --added <path>       treat this path as added (repeatable); bypasses git — for
                       tests / explicit invocation. When given, --base is ignored.
  --skills-root <dir>  skills/ root (default: <repo>/skills)
  --domain-map <path>  skill->BC table (default: docs/reference/agentops-skill-domain-map.md)
  --dispositions <p>   workflows ledger (default: docs/contracts/skill-dispositions.yaml)
  --json               machine-readable summary line only
  -h|--help            this help

FAIL-CLOSED: any added skill/workflow/loop missing bounded-context, role, or a
runnable acceptance blocks the merge (exit 1). Emits one terminal summary line.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --base) BASE="${2:-}"; shift 2 ;;
    --added) ADDED+=("${2:-}"); shift 2 ;;
    --skills-root) SKILLS_ROOT="${2:-}"; shift 2 ;;
    --domain-map) DOMAIN_MAP="${2:-}"; shift 2 ;;
    --dispositions) DISPOSITIONS="${2:-}"; shift 2 ;;
    --json) JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "frontdoor-admission: unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

# --- resolve the ADDED set ---------------------------------------------------
if [ "${#ADDED[@]}" -eq 0 ]; then
  # Derive added files from git via the three-dot merge-base diff (what THIS
  # branch adds vs the base). A governance guard must NEVER admit-on-
  # can't-determine: if the base ref is unresolvable (shallow CI clone, fresh
  # worktree before fetch, detached HEAD) we FAIL-CLOSED — we do NOT silently
  # degrade to a narrower base like HEAD~1, which would answer a different
  # question and hide an added unit (a fail-open). The caller can always inject
  # the exact added set via --added to bypass git entirely.
  if ! git -C "$REPO_ROOT" rev-parse --verify --quiet "${BASE}^{commit}" >/dev/null 2>&1; then
    [ "$JSON" -eq 0 ] && echo "FRONTDOOR-ADMISSION: BLOCKED — cannot resolve admission base '$BASE' (fetch origin/main, or pass --added/--base). Refusing to admit on an undetermined diff (fail-closed)." >&2
    printf '{"guard":"frontdoor-admission","new_units":0,"admitted":0,"blocked":0,"error":"unresolvable-base"}\n'
    exit 1
  fi
  if ! added_raw="$(git -C "$REPO_ROOT" diff --diff-filter=A --name-only "${BASE}...HEAD" 2>/dev/null)"; then
    [ "$JSON" -eq 0 ] && echo "FRONTDOOR-ADMISSION: BLOCKED — git diff against '$BASE' failed; refusing to admit on an undetermined diff (fail-closed)." >&2
    printf '{"guard":"frontdoor-admission","new_units":0,"admitted":0,"blocked":0,"error":"diff-failed"}\n'
    exit 1
  fi
  if [ -n "$added_raw" ]; then
    while IFS= read -r f; do [ -n "$f" ] && ADDED+=("$f"); done <<EOF
$added_raw
EOF
  fi
fi

# --- classify added units ----------------------------------------------------
declare -a SKILL_NAMES=() WORKFLOW_NAMES=()
seen_skill=" " seen_wf=" "
for f in "${ADDED[@]:-}"; do
  [ -n "$f" ] || continue
  case "$f" in
    skills/*/SKILL.md)
      name="${f#skills/}"; name="${name%/SKILL.md}"
      # Compatibility redirects are loadable aliases for an existing skill,
      # not new implementations entering through the front door. Their compact
      # contract is enforced by check-skill-redirects.sh.
      skill_file="$SKILLS_ROOT/$name/SKILL.md"
      if [ -f "$skill_file" ] && grep -Eq '^implementation:[[:space:]]+false([[:space:]]|$)' "$skill_file"; then
        continue
      fi
      case "$seen_skill" in (*" $name "*) : ;; (*) SKILL_NAMES+=("$name"); seen_skill="$seen_skill$name " ;; esac
      ;;
    .claude/workflows/*.js)
      name="${f#.claude/workflows/}"; name="${name%.js}"
      case "$seen_wf" in (*" $name "*) : ;; (*) WORKFLOW_NAMES+=("$name"); seen_wf="$seen_wf$name " ;; esac
      ;;
  esac
done

VIOLATIONS=()
ADMITTED=0

# has_bc_skill / has_role_skill / has_acceptance_skill — each one checkable fact.
bc_found_skill() {
  local n="$1"
  [ -f "$DOMAIN_MAP" ] || return 1
  grep -Eq "\`$n\`[^\`]*BC[1-6]" "$DOMAIN_MAP"
}
role_assigned_skill() {
  local n="$1" sk="$SKILLS_ROOT/$1/SKILL.md"
  [ -f "$sk" ] || return 1
  grep -Eq '^hexagonal_role:[[:space:]]*[A-Za-z]' "$sk"
}
# A RUNNABLE acceptance, not merely a present file: a non-empty .feature that
# actually carries a Scenario/Given, OR a SKILL.md `## Scenarios` block with at
# least one Given/When/Then line. An empty .feature or a bare `## Scenarios`
# header (no steps) does NOT satisfy "acceptance run".
acceptance_skill() {
  local n="$1" sk="$SKILLS_ROOT/$1/SKILL.md" feat="$SKILLS_ROOT/$1/references/$1.feature"
  if [ -s "$feat" ] && grep -Eqi 'scenario|given' "$feat"; then return 0; fi
  if [ -f "$sk" ] && awk '
      /^## Scenarios/ { f=1; next }
      f && /^## / { f=0 }
      f && /[Gg]iven|[Ww]hen|[Tt]hen/ { found=1 }
      END { exit(found?0:1) }' "$sk"; then return 0; fi
  return 1
}

# Workflow facts read from the `workflows:` block of the dispositions ledger.
# wf_field <name> <field> -> the field value (empty if absent). awk walks the
# named block under `workflows:` and prints the requested key once.
wf_field() {
  local n="$1" key="$2"
  [ -f "$DISPOSITIONS" ] || return 0
  awk -v want="$n" -v key="$key" '
    /^workflows:/ { inwf=1; next }
    inwf && /^[a-zA-Z]/ { inwf=0 }              # left the workflows: section
    inwf && /^  [a-zA-Z0-9_-]+:/ {
      cur=$1; sub(/:$/,"",cur); inblk=(cur==want); next
    }
    inwf && inblk && $1==key":" {
      v=$0; sub(/^[[:space:]]*[a-zA-Z0-9_-]+:[[:space:]]*/,"",v)
      # Strip a trailing inline `# comment` BEFORE it can leak into the value:
      # an empty/bogus domain or role with a comment containing a BC token (or
      # any text) would otherwise false-satisfy the non-empty/BC[1-6] checks.
      # Honor a quoted value first (the comment lives outside the quotes); else
      # cut at the first ` #`.
      if (v ~ /^"/) { sub(/^"/,"",v); sub(/".*$/,"",v) }
      else if (v ~ /^'\''/) { sub(/^'\''/,"",v); sub(/'\''.*$/,"",v) }
      else { sub(/[[:space:]]*#.*$/,"",v) }
      sub(/[[:space:]]+$/,"",v)          # trailing whitespace
      print v; exit
    }
  ' "$DISPOSITIONS"
}

for n in "${SKILL_NAMES[@]:-}"; do
  [ -n "$n" ] || continue
  miss=""
  bc_found_skill "$n"      || miss="$miss bounded-context"
  role_assigned_skill "$n" || miss="$miss role"
  acceptance_skill "$n"    || miss="$miss acceptance"
  if [ -n "$miss" ]; then
    VIOLATIONS+=("skill '$n' missing:$miss")
  else
    ADMITTED=$((ADMITTED + 1))
  fi
done

for n in "${WORKFLOW_NAMES[@]:-}"; do
  [ -n "$n" ] || continue
  miss=""
  # BC must be a REAL bounded context (BC1..BC6), symmetric with skills — a
  # non-empty-but-bogus domain ("not-a-bc") does not count as "bounded-context found".
  printf '%s' "$(wf_field "$n" domain)" | grep -Eq 'BC[1-6]' || miss="$miss bounded-context"
  [ -n "$(wf_field "$n" hexagonal_role)" ] || miss="$miss role"
  [ -n "$(wf_field "$n" kind)" ] || miss="$miss acceptance"
  if [ -n "$miss" ]; then
    VIOLATIONS+=("workflow '$n' missing:$miss")
  else
    ADMITTED=$((ADMITTED + 1))
  fi
done

# --- terminal verdict (one crisp summary line; never a silent pass) ----------
new_units=$(( ${#SKILL_NAMES[@]} + ${#WORKFLOW_NAMES[@]} ))
blocked=${#VIOLATIONS[@]}

summary() {
  printf '{"guard":"frontdoor-admission","new_units":%s,"admitted":%s,"blocked":%s}\n' \
    "$new_units" "$ADMITTED" "$blocked"
}

if [ "$blocked" -gt 0 ]; then
  if [ "$JSON" -eq 0 ]; then
    echo "FRONTDOOR-ADMISSION: BLOCKED — $blocked new unit(s) lack front-door evidence:" >&2
    for v in "${VIOLATIONS[@]}"; do echo "  - $v" >&2; done
    echo "  Required: bounded-context (BC1..BC6) + hexagonal_role + a runnable acceptance (references/<name>.feature or ## Scenarios)." >&2
  fi
  summary
  exit 1
fi

if [ "$JSON" -eq 0 ]; then
  if [ "$new_units" -eq 0 ]; then
    echo "FRONTDOOR-ADMISSION: PASS — no new skill/workflow/loop in this change." >&2
  else
    echo "FRONTDOOR-ADMISSION: PASS — $ADMITTED new unit(s) carry full front-door evidence." >&2
  fi
fi
summary
exit 0
