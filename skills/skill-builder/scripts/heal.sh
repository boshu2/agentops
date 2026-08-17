#!/usr/bin/env bash
# One-pass structural audit for source skill packages.
set -euo pipefail

MODE=check
STRICT=0
TARGETS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) MODE=check ;;
    --fix) MODE=fix ;;
    --strict) STRICT=1 ;;
    -h|--help)
      echo "usage: heal.sh [--check|--fix] [--strict] [skills/<slug> ...]"
      exit 0
      ;;
    *) TARGETS+=("$1") ;;
  esac
  shift
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
raw_repo="${HEAL_REPO_ROOT:-$(cd "$SCRIPT_DIR/../../.." && pwd -P)}"
[[ "$raw_repo" == /* && -d "$raw_repo" && ! -L "$raw_repo" ]] || {
  echo "heal.sh: repository root must be an absolute non-symlink directory" >&2
  exit 2
}
REPO_ROOT="$(cd "$raw_repo" && pwd -P)"
[[ "$raw_repo" == "$REPO_ROOT" ]] || {
  echo "heal.sh: repository root must use its canonical spelling" >&2
  exit 2
}
source "$SCRIPT_DIR/transaction.sh"

STEP_TIMEOUT="${SKILL_BUILDER_STEP_TIMEOUT:-120}"
[[ "$STEP_TIMEOUT" =~ ^[0-9]+$ && "$STEP_TIMEOUT" -ge 1 && "$STEP_TIMEOUT" -le 300 ]] || {
  echo "heal.sh: SKILL_BUILDER_STEP_TIMEOUT must be in [1,300]" >&2
  exit 2
}
TIMEOUT_BIN=""
for candidate in /usr/bin/timeout /bin/timeout /opt/homebrew/bin/gtimeout /opt/homebrew/bin/timeout /usr/local/bin/gtimeout; do
  if [[ -x "$candidate" ]]; then TIMEOUT_BIN="$candidate"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || { echo "heal.sh: timeout or gtimeout is required" >&2; exit 2; }
run_step() { "$TIMEOUT_BIN" --signal=TERM --kill-after=2s "$STEP_TIMEOUT" "$@"; }

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  for path in "$REPO_ROOT/skills"/*; do
    [[ -d "$path" && -f "$path/SKILL.md" ]] && TARGETS+=("$path")
  done
fi
(( ${#TARGETS[@]} >= 1 && ${#TARGETS[@]} <= 64 )) || {
  echo "heal.sh: target count must be in [1,64]" >&2
  exit 2
}

normalized=()
slugs=()
for target in "${TARGETS[@]}"; do
  if [[ "$target" = /* ]]; then
    [[ "$target" != *"/../"* && "$target" != */.. && "$target" != *"/./"* && "$target" != */. ]] || {
      echo "heal.sh: non-canonical target spelling is not accepted" >&2
      exit 2
    }
  else
    [[ "$target" =~ ^(\./)?skills(-codex)?/[a-z][a-z0-9-]{0,63}$ ]] || {
      echo "heal.sh: relative targets must name one direct skill package" >&2
      exit 2
    }
    target="$REPO_ROOT/${target#./}"
  fi
  [[ -d "$target" ]] || { echo "heal.sh: target does not exist: $target" >&2; exit 2; }
  [[ ! -L "$target" ]] || { echo "heal.sh: symlink targets are not accepted: $target" >&2; exit 2; }
  resolved="$(cd "$target" && pwd -P)"
  case "$(dirname "$resolved")" in
    "$REPO_ROOT/skills"|"$REPO_ROOT/skills-codex") ;;
    *) echo "heal.sh: target is not a direct skill package: $target" >&2; exit 2 ;;
  esac
  [[ "$target" == "$resolved" ]] || {
    echo "heal.sh: target must use its canonical spelling" >&2
    exit 2
  }
  normalized+=("$resolved")
  slugs+=("$(basename "$resolved")")
done

set +e
run_step python3 - "$REPO_ROOT" "${normalized[@]}" <<'PY'
from pathlib import Path
import os
import re
import stat
import sys
import yaml

repo = Path(sys.argv[1])
findings = []
for root in map(Path, sys.argv[2:]):
    skill = root / "SKILL.md"
    rel = root.relative_to(repo).as_posix()
    try:
        info = skill.lstat()
    except OSError:
        findings.append(("MISSING_SKILL", rel, "SKILL.md is missing"))
        continue
    if not stat.S_ISREG(info.st_mode) or skill.is_symlink():
        findings.append(("UNSAFE_SKILL", rel, "SKILL.md must be a non-symlink regular file"))
        continue
    if info.st_size > 1048576:
        findings.append(("OVERSIZED_SKILL", rel, "SKILL.md exceeds 1048576 bytes"))
        continue
    text = skill.read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) != 3:
        findings.append(("INVALID_FRONTMATTER", rel, "leading YAML frontmatter is missing"))
        continue
    try:
        data = yaml.safe_load(parts[1]) or {}
    except yaml.YAMLError as exc:
        findings.append(("INVALID_FRONTMATTER", rel, str(exc).splitlines()[0]))
        continue
    slug = root.name
    if data.get("name") != slug:
        findings.append(("NAME_MISMATCH", rel, f"name must be {slug!r}"))
    if not isinstance(data.get("description"), str) or not data["description"].strip():
        findings.append(("MISSING_DESC", rel, "description must be nonempty"))
    if data.get("skill_api_version") != 1:
        findings.append(("MISSING_API_VERSION", rel, "skill_api_version must be 1"))
    metadata = data.get("metadata")
    if not isinstance(metadata, dict) or not isinstance(metadata.get("disposition"), str) or not metadata["disposition"]:
        findings.append(("MISSING_DISPOSITION", rel, "metadata.disposition must be nonempty"))
    body = parts[2]
    for match in re.finditer(r"\]\((references|scripts)/([^\s)#?]+)", body):
        linked = root / match.group(1) / match.group(2)
        if not linked.exists() or linked.is_symlink():
            findings.append(("DEAD_REF", rel, f"missing {linked.relative_to(root)}"))

for code, path, message in findings:
    print(f"[{code}] {path}: {message}")
sys.exit(1 if findings else 0)
PY
rc=$?
set -e

if [[ "$MODE" == fix ]]; then
  # Source behavior remains human-authored. Repair only owned projections.
  [[ $rc -eq 0 ]] || exit 1
  for required in "$REPO_ROOT/scripts/generate-skill-mesh.py" "$REPO_ROOT/scripts/codex-sync.sh"; do
    [[ -f "$required" && ! -L "$required" ]] || {
      echo "heal.sh: required generator missing or symlinked" >&2
      exit 2
    }
  done
  unique_slugs=()
  names=""
  for slug in "${slugs[@]}"; do
    case ",$names," in
      *",$slug,"*) ;;
      *) unique_slugs+=("$slug"); names="${names:+$names,}$slug" ;;
    esac
  done
  sb_txn_begin_projections "$REPO_ROOT" "${unique_slugs[@]}"
  run_step python3 "$REPO_ROOT/scripts/generate-skill-mesh.py"
  run_step bash "$REPO_ROOT/scripts/codex-sync.sh" --force --only "$names"
  sb_txn_commit
fi

if [[ $rc -ne 0 && ( $STRICT -eq 1 || "$MODE" == fix ) ]]; then
  exit 1
fi
exit 0
