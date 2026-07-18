#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
ROOT="$(resolve_repo_root)"
SCOPE="auto"
SKILLS_ROOT="$ROOT/skills-codex"
MANIFEST_FILE="$SKILLS_ROOT/.agentops-manifest.json"
MARKER_FILE_NAME=".agentops-generated.json"
MANIFEST_VALIDATOR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/validate-codex-generated-manifest.sh"
AUDIT_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/audit-codex-parity.sh"

usage() {
  cat <<'EOF'
Usage: bash scripts/validate-codex-generated-artifacts.sh [repo-root] [--scope auto|upstream|staged|worktree|head]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      SCOPE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      ROOT="$1"
      if [[ "$ROOT" != /* ]]; then
        ROOT="$(cd "$ROOT" && pwd)"
      fi
      SKILLS_ROOT="$ROOT/skills-codex"
      MANIFEST_FILE="$SKILLS_ROOT/.agentops-manifest.json"
      shift
      ;;
  esac
done

case "$SCOPE" in
  auto|upstream|staged|worktree|head) ;;
  *)
    echo "Invalid --scope: $SCOPE" >&2
    exit 2
    ;;
esac

failures=0
warnings=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

warn() {
  echo "WARN: $1" >&2
  warnings=$((warnings + 1))
}

collect_changed_files() {
  local scope="$1"
  local ahead_files=""

  if ! git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    return 0
  fi

  case "$scope" in
    upstream)
      if git -C "$ROOT" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' >/dev/null 2>&1; then
        git -C "$ROOT" diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
      fi
      ;;
    staged)
      git -C "$ROOT" diff --name-only --cached 2>/dev/null || true
      ;;
    worktree)
      git -C "$ROOT" diff --name-only --cached 2>/dev/null || true
      git -C "$ROOT" diff --name-only 2>/dev/null || true
      git -C "$ROOT" ls-files --others --exclude-standard 2>/dev/null || true
      ;;
    head)
      git -C "$ROOT" show --name-only --pretty=format: HEAD 2>/dev/null || true
      ;;
    auto)
      if git -C "$ROOT" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' >/dev/null 2>&1; then
        ahead_files="$(git -C "$ROOT" diff --name-only '@{upstream}...HEAD' 2>/dev/null || true)"
        if [[ -n "$ahead_files" ]]; then
          printf '%s\n' "$ahead_files"
          return 0
        fi
      fi
      git -C "$ROOT" diff --name-only --cached 2>/dev/null || true
      git -C "$ROOT" diff --name-only 2>/dev/null || true
      git -C "$ROOT" ls-files --others --exclude-standard 2>/dev/null || true
      git -C "$ROOT" show --name-only --pretty=format: HEAD 2>/dev/null || true
      ;;
  esac
}

# strip_skill_frontmatter reads a SKILL.md on stdin and prints only the body
# (everything after the leading --- ... --- frontmatter block). The Codex twin
# frontmatter is name+description only, so a frontmatter-only source edit (e.g.
# hex-wiring fields the twin does not carry) needs no twin change — the body is
# the surface that must stay mirrored (age-j1g).
strip_skill_frontmatter() {
  awk 'seen<2 { if ($0 == "---") seen++; next } { print }'
}

# source_skill_body_changed returns 0 (true) when the source SKILL.md *body* for
# a skill changed under the active scope, 1 (false) when only frontmatter changed
# or the body is identical. Conservative: any ambiguity (new file, unresolved
# base ref, read failure) returns 0 so a real divergence is never silently
# skipped. base/target mirror collect_changed_files' per-scope diff semantics.
source_skill_body_changed() {
  local scope="$1" skill="$2"
  local file="skills/$skill/SKILL.md"
  local base target
  case "$scope" in
    head)     base="HEAD~1";      target="HEAD" ;;
    staged)   base="HEAD";        target=":0" ;;
    upstream) base="@{upstream}"; target="HEAD" ;;
    *)        base="HEAD";        target="" ;;  # worktree/auto: working tree vs HEAD
  esac
  # No base version (new file, unborn/unresolved base ref) → conservatively changed.
  git -C "$ROOT" cat-file -e "$base:$file" 2>/dev/null || return 0
  local base_body target_body
  base_body="$(git -C "$ROOT" show "$base:$file" 2>/dev/null | strip_skill_frontmatter)"
  if [[ -z "$target" ]]; then
    [[ -f "$ROOT/$file" ]] || return 0
    target_body="$(strip_skill_frontmatter < "$ROOT/$file")"
  else
    git -C "$ROOT" cat-file -e "$target:$file" 2>/dev/null || return 0
    target_body="$(git -C "$ROOT" show "$target:$file" 2>/dev/null | strip_skill_frontmatter)"
  fi
  [[ "$base_body" == "$target_body" ]] && return 1
  return 0
}

echo "=== Codex artifact metadata validation ==="

[[ -d "$SKILLS_ROOT" ]] || {
  echo "Missing skills-codex root: $SKILLS_ROOT" >&2
  exit 1
}
[[ -f "$MANIFEST_FILE" ]] || {
  echo "Missing Codex artifact manifest: $MANIFEST_FILE" >&2
  exit 1
}
if [[ -x "$MANIFEST_VALIDATOR" ]]; then
  bash "$MANIFEST_VALIDATOR" "$SKILLS_ROOT" >/dev/null
fi

while IFS= read -r skill_dir; do
  [[ -f "$skill_dir/SKILL.md" ]] || continue
  skill_name="$(basename "$skill_dir")"
  [[ -f "$skill_dir/$MARKER_FILE_NAME" ]] || fail "missing Codex artifact marker: ${skill_dir#"$ROOT"/}/$MARKER_FILE_NAME"
  if grep -qE "^description:[[:space:]]*['\"]?[>|]['\"]?[[:space:]]*$" "$skill_dir/SKILL.md"; then
    fail "malformed generated description frontmatter: ${skill_dir#"$ROOT"/}/SKILL.md"
  fi
done < <(find "$SKILLS_ROOT" -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort)

# parity_only twins are GENERATED by codex-sync and verified by its byte-exact
# drift gate (codex-sync --check, run in regen-all ahead of this gate); content +
# divergence are re-checked only on BESPOKE (hand-authored) twins.
CATALOG_JSON="$(dirname "$SKILLS_ROOT")/skills-codex-overrides/catalog.json"
BESPOKE_SKILLS="$(python3 -c "import json; d=json.load(open('$CATALOG_JSON')); print(chr(10).join(e['name'] for e in d.get('skills',[]) if e.get('treatment')=='bespoke'))" 2>/dev/null || true)"
is_bespoke() { grep -qxF "$1" <<<"$BESPOKE_SKILLS"; }

# A POINTER twin (skills-codex/<skill>/SKILL.md frontmatter `parity_policy: pointer`)
# deliberately carries NO mirrored prose — it defers to the source skill as the canonical
# body ("the source skill is the source of truth — read it first") plus a short Codex
# Runtime Contract. A source body/references edit therefore has nothing to mirror into it,
# so the content-divergence + reference-counterpart gates would be pure churn. Pointer twins
# are exempt from those gates ONLY; their own content is still covered by the source->codex
# existence check and the manifest/hash audit. (Default — full-mirror — twins are unaffected.)
twin_is_pointer() {
  local twin="$SKILLS_ROOT/$1/SKILL.md"
  [[ -f "$twin" ]] || return 1
  awk 'NR==1 && /^---/{f=1; next} f && /^---/{exit} f && /^parity_policy:[[:space:]]*pointer([[:space:]]+#.*|[[:space:]]*)$/{found=1} END{exit !found}' "$twin"
}

# --- Frontmatter completeness check ---
for skill_md in "$SKILLS_ROOT"/*/SKILL.md; do
  [[ -f "$skill_md" ]] || continue
  skill_name=$(basename "$(dirname "$skill_md")")
  is_bespoke "$skill_name" || continue  # parity twins are generator/drift-verified
  frontmatter_fields=""

  # Extract only the leading frontmatter block.
  frontmatter=$(awk 'NR==1 && /^---$/{in_fm=1; print; next} in_fm && /^---$/{print; exit} in_fm{print}' "$skill_md")
  frontmatter_fields="$(printf '%s\n' "$frontmatter" | grep -oE '^[a-z_-]+:' | sed 's/:$//' || true)"

  if ! echo "$frontmatter" | grep -q '^name:'; then
    fail "$skill_name missing 'name' in frontmatter"
  fi
  if ! echo "$frontmatter" | grep -q '^description:'; then
    fail "$skill_name missing 'description' in frontmatter"
  fi
  # `parity_policy: pointer` is an allowed twin marker — it declares the twin defers to the
  # source skill body (exempting it from the source-divergence gates below), so a source-only
  # prose edit needs no twin churn. See twin_is_pointer().
  extra_fields="$(printf '%s\n' "$frontmatter_fields" | grep -vE '^(name|description|parity_policy)$' || true)"
  if [[ -n "$extra_fields" ]]; then
    fail "$skill_name has non-Codex frontmatter fields: $(printf '%s' "$extra_fields" | tr '\n' ',' | sed 's/,$//')"
  fi
done

# --- Wrong-directory cross-reference check ---
for skill_md in "$SKILLS_ROOT"/*/SKILL.md; do
  [[ -f "$skill_md" ]] || continue
  skill_name=$(basename "$(dirname "$skill_md")")
  is_bespoke "$skill_name" || continue  # parity twins are generator/drift-verified
  # Ignore code blocks by checking only non-fenced lines
  if grep -v '^\s*```' "$skill_md" | grep -v '^\s*`' | grep -qE '\]\(skills/' ; then
    warn "$skill_name contains ](skills/ cross-ref (should use relative paths)"
  fi
done

mapfile -t changed_files < <(collect_changed_files "$SCOPE" | sed '/^[[:space:]]*$/d' | sort -u)

if [[ "${#changed_files[@]}" -gt 0 ]]; then
  declare -A changed_source_skills=()
  declare -A changed_codex_skills=()
  declare -A changed_source_refs=()
  declare -A changed_source_skillmd=()
  declare -A changed_source_content=()
  declare -A changed_codex_content=()

  for changed_file in "${changed_files[@]}"; do
    case "$changed_file" in
      skills/_*/*)
        # Leading-underscore scaffolding under skills/ is not real skill
        # source and has no Codex twin under skills-codex/.
        ;;
      skills/*/*)
        skill_name="${changed_file#skills/}"
        skill_name="${skill_name%%/*}"
        changed_source_skills["$skill_name"]=1
        # references/** is mirrored near-verbatim into the Codex twin, so a
        # source references edit MUST be accompanied by a twin content change.
        case "$changed_file" in
          skills/*/references/*) changed_source_refs["$skill_name"]=1; changed_source_content["$skill_name"]=1 ;;
          skills/*/SKILL.md)     changed_source_skillmd["$skill_name"]=1 ;;
          *)                     changed_source_content["$skill_name"]=1 ;;
        esac
        ;;
      skills-codex/*/*)
        skill_name="${changed_file#skills-codex/}"
        skill_name="${skill_name%%/*}"
        changed_codex_skills["$skill_name"]=1
        # Only a REAL twin content change counts. The generated bookkeeping
        # files (.agentops-generated.json marker, .agentops-manifest.json) are
        # refreshed by regen-codex-hashes.sh to be self-consistent with the
        # CURRENT (possibly stale) twin, so they must not satisfy the
        # content-mirror requirement below (age-yxl).
        case "$changed_file" in
          */.agentops-generated.json|*/.agentops-manifest.json) ;;
          *) changed_codex_content["$skill_name"]=1 ;;
        esac
        ;;
    esac
  done

  for skill_name in "${!changed_source_skills[@]}"; do
    is_bespoke "$skill_name" || continue  # parity: codex-sync regenerates + drift-gates
    needs_twin_update=0
    if [[ -n "${changed_source_content[$skill_name]+x}" ]]; then
      needs_twin_update=1
    elif [[ -n "${changed_source_skillmd[$skill_name]+x}" ]] && source_skill_body_changed "$SCOPE" "$skill_name"; then
      needs_twin_update=1
    fi
    if [[ "$needs_twin_update" -eq 1 && -z "${changed_codex_skills[$skill_name]+x}" ]]; then
      fail "source skill changed without matching checked-in Codex update: skills/$skill_name -> skills-codex/$skill_name"
    fi
  done

  # Codex-twin content-divergence gate (age-yxl). regen-all only refreshes the
  # twin's hash record, NOT its prose: editing skills/<skill>/references/** and
  # running regen makes the marker self-consistent with the STALE twin, so the
  # source->codex check above is satisfied by a hash bump alone and a divergent
  # twin ships silently. Require a real twin content change to mirror the source
  # references edit; a marker-only codex change does not count.
  for skill_name in "${!changed_source_refs[@]}"; do
    is_bespoke "$skill_name" || continue  # parity: codex-sync regenerates + drift-gates
    if [[ -z "${changed_codex_content[$skill_name]+x}" ]] && ! twin_is_pointer "$skill_name"; then
      fail "Codex twin content divergence: skills/$skill_name/references/ changed but skills-codex/$skill_name has no matching content update (only generated hashes changed). regen-all refreshes hashes, not twin prose — manually mirror the edit into skills-codex/$skill_name/references/, then run scripts/regen-codex-hashes.sh --only $skill_name. (A pointer twin may instead declare \`parity_policy: pointer\` in its frontmatter to defer to the source.)"
    fi
  done

  # Codex-twin SKILL.md body-divergence gate (age-j1g). Same silent-staleness as
  # references, for the SKILL.md BODY: a source SKILL.md body edit with a stale
  # twin is masked by a regen hash bump. Scoped to the body on purpose —
  # frontmatter-only edits (hex-wiring fields the twin does not carry:
  # consumes/produces/hexagonal_role/context_rel/...) need no twin change and are
  # NOT flagged, so legitimate frontmatter-only pushes stay green.
  for skill_name in "${!changed_source_skillmd[@]}"; do
    is_bespoke "$skill_name" || continue  # parity: codex-sync regenerates + drift-gates
    if source_skill_body_changed "$SCOPE" "$skill_name" && [[ -z "${changed_codex_content[$skill_name]+x}" ]] && ! twin_is_pointer "$skill_name"; then
      fail "Codex twin content divergence: skills/$skill_name/SKILL.md body changed but skills-codex/$skill_name has no matching content update (only generated hashes changed). regen-all refreshes hashes, not twin prose — manually mirror the body edit into skills-codex/$skill_name/SKILL.md, then run scripts/regen-codex-hashes.sh --only $skill_name. (Frontmatter-only edits need no twin change; a pointer twin may declare \`parity_policy: pointer\` to defer to the source.)"
    fi
  done
fi

# --- Static reference-counterpart assertion (age-odv) ---
# The diff-scoped divergence gates above only catch a CHANGED source reference; they
# MISS the missing-counterpart case — a source skill that ships a references/*.md the
# parity twin never mirrored (e.g. a twin hand-trimmed to a pointer but not marked).
# codex-sync mirrors source references/** into parity twins, but the gate must not RELY
# on codex-sync having run. Assert STATICALLY (every push, full-repo) that each source
# references/*.md has a twin counterpart. Exemptions: BESPOKE twins (age-0js4 —
# hand-maintained; refs deliberately diverge/omit) and `parity_policy: pointer` twins
# (age-k2ag — defer to the source body). A twin that does not exist at all is the
# separate source->codex existence check's job, so skip when the twin dir is absent.
while IFS= read -r src_ref; do
  [[ -n "$src_ref" ]] || continue
  rel="${src_ref#"$ROOT"/skills/}"     # <skill>/references/<file>
  ref_skill="${rel%%/*}"
  ref_rel="${rel#*/}"                   # references/<file>
  is_bespoke "$ref_skill" && continue
  [[ -d "$SKILLS_ROOT/$ref_skill" ]] || continue
  twin_is_pointer "$ref_skill" && continue
  if [[ ! -f "$SKILLS_ROOT/$ref_skill/$ref_rel" ]]; then
    fail "Codex twin missing source reference: skills/$ref_skill/$ref_rel has no counterpart at skills-codex/$ref_skill/$ref_rel. Parity twins must mirror every source references/ file — run scripts/codex-sync.sh --only $ref_skill (or --force) to regenerate. If this twin should NOT mirror prose, declare \`parity_policy: pointer\` in its frontmatter; if it is bespoke, register it in skills-codex-overrides/catalog.json."
  fi
done < <(find "$ROOT/skills" -mindepth 3 -path '*/references/*' -type f -name '*.md' 2>/dev/null)

# --- Invoke codex parity audit ---
if [[ -x "$AUDIT_SCRIPT" ]]; then
  echo "--- Running codex parity audit ---"
  if ! bash "$AUDIT_SCRIPT"; then
    fail "Codex parity audit failed"
  fi
fi

if [[ "$warnings" -gt 0 ]]; then
  echo "Codex artifact metadata validation: $warnings warning(s)." >&2
fi

if [[ "$failures" -gt 0 ]]; then
  echo "Repair flow: bash scripts/refresh-codex-artifacts.sh --scope $SCOPE" >&2
  echo "Codex artifact metadata validation FAILED ($failures finding(s))." >&2
  exit 1
fi

echo "Codex artifact metadata validation passed."
exit 0
