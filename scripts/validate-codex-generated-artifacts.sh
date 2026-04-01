#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
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
  [[ -f "$skill_dir/$MARKER_FILE_NAME" ]] || fail "missing Codex artifact marker: ${skill_dir#"$ROOT"/}/$MARKER_FILE_NAME"
  if grep -qE "^description:[[:space:]]*['\"]?[>|]['\"]?[[:space:]]*$" "$skill_dir/SKILL.md"; then
    fail "malformed generated description frontmatter: ${skill_dir#"$ROOT"/}/SKILL.md"
  fi
done < <(find "$SKILLS_ROOT" -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort)

# --- Frontmatter completeness check ---
for skill_md in "$SKILLS_ROOT"/*/SKILL.md; do
  [[ -f "$skill_md" ]] || continue
  skill_name=$(basename "$(dirname "$skill_md")")
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
  extra_fields="$(printf '%s\n' "$frontmatter_fields" | grep -vE '^(name|description)$' || true)"
  if [[ -n "$extra_fields" ]]; then
    fail "$skill_name has non-Codex frontmatter fields: $(printf '%s' "$extra_fields" | tr '\n' ',' | sed 's/,$//')"
  fi
done

# --- Wrong-directory cross-reference check ---
for skill_md in "$SKILLS_ROOT"/*/SKILL.md; do
  [[ -f "$skill_md" ]] || continue
  skill_name=$(basename "$(dirname "$skill_md")")
  # Ignore code blocks by checking only non-fenced lines
  if grep -v '^\s*```' "$skill_md" | grep -v '^\s*`' | grep -qE '\]\(skills/' ; then
    warn "$skill_name contains ](skills/ cross-ref (should use relative paths)"
  fi
done

mapfile -t changed_files < <(collect_changed_files "$SCOPE" | sed '/^[[:space:]]*$/d' | sort -u)

if [[ "${#changed_files[@]}" -gt 0 ]]; then
  declare -A changed_source_skills=()
  declare -A changed_codex_skills=()

  for changed_file in "${changed_files[@]}"; do
    case "$changed_file" in
      skills/*/*)
        skill_name="${changed_file#skills/}"
        skill_name="${skill_name%%/*}"
        changed_source_skills["$skill_name"]=1
        ;;
      skills-codex/*/*)
        skill_name="${changed_file#skills-codex/}"
        skill_name="${skill_name%%/*}"
        changed_codex_skills["$skill_name"]=1
        ;;
    esac
  done

  for skill_name in "${!changed_source_skills[@]}"; do
    if [[ -z "${changed_codex_skills[$skill_name]+x}" ]]; then
      fail "source skill changed without matching checked-in Codex update: skills/$skill_name -> skills-codex/$skill_name"
    fi
  done
fi

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
