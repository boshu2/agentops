#!/usr/bin/env bash
# Atomically create one metadata-complete canonical skill source package.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
raw_repo="${SKILL_BUILDER_REPO_ROOT:-$(cd "$SCRIPT_DIR/../../.." && pwd -P)}"
[[ "$raw_repo" == /* && -d "$raw_repo" && ! -L "$raw_repo" ]] || {
  echo "init.sh: repository root must be an absolute non-symlink directory" >&2
  exit 2
}
REPO_ROOT="$(cd "$raw_repo" && pwd -P)"
[[ "$raw_repo" == "$REPO_ROOT" ]] || {
  echo "init.sh: repository root must use its canonical spelling" >&2
  exit 2
}
[[ -d "$REPO_ROOT/skills" && ! -L "$REPO_ROOT/skills" ]] || {
  echo "init.sh: canonical skills directory is unavailable" >&2
  exit 2
}

usage() {
  cat >&2 <<'EOF'
usage:
  init.sh --scratch <slug>
  init.sh --template <slug> --like <slug>
  init.sh --external <slug> --external-root <absolute-root> --from <relative-file>
EOF
  exit 2
}

[[ $# -ge 2 ]] || usage
mode="$1"
slug="$2"
shift 2

[[ "$slug" =~ ^[a-z][a-z0-9-]{0,63}$ ]] || {
  echo "init.sh: slug must be lowercase-hyphen and at most 64 characters" >&2
  exit 2
}

source_hint=""
case "$mode" in
  --scratch)
    [[ $# -eq 0 ]] || usage
    ;;
  --template)
    [[ $# -eq 2 && "$1" == "--like" ]] || usage
    template_slug="$2"
    [[ "$template_slug" =~ ^[a-z][a-z0-9-]{0,63}$ ]] || {
      echo "init.sh: invalid template slug" >&2
      exit 2
    }
    template_dir="$REPO_ROOT/skills/$template_slug"
    template_file="$template_dir/SKILL.md"
    [[ -d "$template_dir" && ! -L "$template_dir" && -f "$template_file" && ! -L "$template_file" ]] || {
      echo "init.sh: template skill is unavailable or unsafe" >&2
      exit 2
    }
    template_size="$(stat -f '%z' "$template_file" 2>/dev/null || stat -c '%s' "$template_file")"
    (( template_size <= 1048576 )) || {
      echo "init.sh: template skill exceeds 1048576 bytes" >&2
      exit 2
    }
    source_hint="template:$template_slug"
    ;;
  --external)
    [[ $# -eq 4 && "$1" == "--external-root" && "$3" == "--from" ]] || usage
    external_root="$2"
    external_from="$4"
    if ! source_digest="$(python3 - "$external_root" "$external_from" <<'PY'
import hashlib
import os
import stat
import sys

root, relative = sys.argv[1:]
if not os.path.isabs(root) or not os.path.isdir(root) or os.path.islink(root):
    raise SystemExit("init.sh: external root must be an absolute non-symlink directory")
if os.path.abspath(root) != os.path.realpath(root):
    raise SystemExit("init.sh: external root must use its canonical spelling")
parts = relative.split("/")
if not relative or os.path.isabs(relative) or any(part in {"", ".", ".."} for part in parts):
    raise SystemExit("init.sh: external source must be a canonical relative file")
candidate = os.path.join(root, *parts)
cursor = root
for part in parts:
    cursor = os.path.join(cursor, part)
    if os.path.islink(cursor):
        raise SystemExit("init.sh: external source may not traverse symlinks")
try:
    info = os.stat(candidate, follow_symlinks=False)
except OSError:
    raise SystemExit("init.sh: external source is unavailable")
if not stat.S_ISREG(info.st_mode):
    raise SystemExit("init.sh: external source must be a regular file")
if info.st_size > 1048576:
    raise SystemExit("init.sh: external source exceeds 1048576 bytes")
digest = hashlib.sha256()
with open(candidate, "rb") as handle:
    while chunk := handle.read(65536):
        digest.update(chunk)
print(digest.hexdigest())
PY
)"; then
      exit 2
    fi
    source_hint="external-sha256:$source_digest"
    ;;
  *) usage ;;
esac

target="$REPO_ROOT/skills/$slug"
[[ ! -e "$target" && ! -L "$target" ]] || {
  echo "init.sh: target already exists" >&2
  exit 1
}

tier="${SKILL_TIER:-execution}"
[[ "$tier" =~ ^[a-z][a-z0-9_-]{0,31}$ ]] || {
  echo "init.sh: SKILL_TIER must be a bounded identifier" >&2
  exit 2
}

normalize_list() {
  local value="$1"
  (( ${#value} <= 8192 )) || {
    echo "init.sh: metadata JSON exceeds 8192 characters" >&2
    return 2
  }
  python3 - "$value" <<'PY'
import json
import sys

try:
    parsed = json.loads(sys.argv[1])
except (TypeError, ValueError):
    raise SystemExit("init.sh: metadata values must be valid JSON")
if not isinstance(parsed, list) or len(parsed) > 64:
    raise SystemExit("init.sh: metadata values must be arrays of at most 64 strings")
if not all(isinstance(item, str) and len(item.encode("utf-8")) <= 256 for item in parsed):
    raise SystemExit("init.sh: metadata items must be strings of at most 256 bytes")
print(json.dumps(parsed, ensure_ascii=True, separators=(",", ":")))
PY
}

if ! dependencies="$(normalize_list "${SKILL_DEPENDENCIES:-[]}")"; then exit 2; fi
if ! capabilities="$(normalize_list "${SKILL_CAPABILITIES:-[\"${slug//-/_}\"]}")"; then exit 2; fi
if ! effects="$(normalize_list "${SKILL_EFFECTS:-[]}")"; then exit 2; fi

# Reject a redirected report path before creating anything. Missing directories
# are created only during the commit and are removed if that commit fails.
report_dir="$REPO_ROOT/.agents/scratch/skill-builder"
for parent in "$REPO_ROOT/.agents" "$REPO_ROOT/.agents/scratch" "$report_dir"; do
  if [[ -e "$parent" || -L "$parent" ]]; then
    [[ -d "$parent" && ! -L "$parent" ]] || {
      echo "init.sh: build report directory is unavailable or unsafe" >&2
      exit 2
    }
  fi
done
report="$report_dir/${slug}-build.json"
[[ ! -L "$report" && ( ! -e "$report" || -f "$report" ) ]] || {
  echo "init.sh: build report target is unavailable or unsafe" >&2
  exit 2
}

staging_root="$(mktemp -d "$REPO_ROOT/skills/.skill-init.${slug}.XXXXXX")"
package_stage="$staging_root/package"
report_stage="$staging_root/build-report.json"
created_dirs=()
target_installed=0
report_installed=0
cleanup() {
  local rc=$? index
  trap - EXIT INT TERM
  if [[ "$target_installed" -eq 1 && "$report_installed" -eq 0 ]]; then
    rm -rf -- "$target"
  fi
  rm -rf -- "$staging_root"
  if [[ "$report_installed" -eq 0 ]]; then
    for ((index=${#created_dirs[@]}-1; index>=0; index--)); do
      rmdir -- "${created_dirs[$index]}" 2>/dev/null || true
    done
  fi
  exit "$rc"
}
trap cleanup EXIT INT TERM

mkdir -p "$package_stage/scripts"
cat >"$package_stage/SKILL.md" <<EOF
---
name: $slug
description: 'TODO: state the behavior and concrete trigger phrases for $slug.'
practices: []
skill_api_version: 1
hexagonal_role: supporting
consumes: []
produces: []
context_rel: []
user-invocable: true
metadata:
  tier: $tier
  dependencies: $dependencies
  capabilities: $capabilities
  effects: $effects
  canonical_status: canonical
  disposition: keep_specialist
  stability: experimental
---

# /$slug

TODO: Explain the bounded behavior this skill provides.

<!-- craft:trigger-rich-description Put Triggers:/Use when phrases callers actually say in the frontmatter description. -->
<!-- craft:causal-insight-line State the one causal insight (Insight:/Why:/a-because-clause) that makes this skill work. -->
<!-- craft:named-failure-mode Name the concrete failure mode this skill exists to prevent. -->
<!-- craft:router-shape Map trigger phrases to modes/entry points in a routing table when the skill has modes. -->

## Inputs

TODO: List required inputs and explicit non-goals.

<!-- craft:negative-space State what this skill is NOT for (non-goals / not-for / do-not-use-when). -->

## Procedure

1. TODO: Perform one bounded operation.
2. TODO: Check the output against the stated contract.
3. Report the result and stop.

<!-- craft:named-loop-stop-condition If any step iterates, name the loop and give a checkable stop condition in the same section. -->
<!-- craft:quantified-rules Quantify at least one rule with a number and unit. -->
<!-- craft:anti-pattern-with-corrective Pair each anti-pattern with its corrective in the same section. -->
<!-- craft:frozen-prompts Provide any reusable prompt as a fenced block marked copy-paste-only. -->
<!-- craft:runnable-commands Include at least one fenced block with runnable commands. -->

## Output

TODO: Define the artifact or response shape and how a caller checks it.

<!-- craft:measurable-done Give a machine-checkable done signal (done-when phrase, exit 0, validator command). -->

## Checks

- The output satisfies the declared behavior.
- No undeclared side effect occurred.

<!-- craft:provenance-citation Cite at least one resolvable repo path or .agents/ao verdict/intent digest grounding this skill. -->

## Failure behavior

Report the concrete failure and stop. The caller owns any revision.
EOF

cat >"$package_stage/scripts/validate.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
exec bash "$REPO_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SKILL_DIR"
EOF
chmod +x "$package_stage/scripts/validate.sh"

python3 - "$report_stage" "$mode" "$slug" "$source_hint" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
mode = {"--scratch": "from-scratch", "--template": "from-template", "--external": "absorb-external"}[sys.argv[2]]
payload = {
    "mode": mode,
    "skill_name": sys.argv[3],
    "files_created": [f"skills/{sys.argv[3]}/SKILL.md", f"skills/{sys.argv[3]}/scripts/validate.sh"],
    "structure_check_pass": False,
}
if sys.argv[4]:
    payload["source_hint"] = sys.argv[4]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

for parent in "$REPO_ROOT/.agents" "$REPO_ROOT/.agents/scratch" "$report_dir"; do
  if [[ ! -d "$parent" ]]; then
    mkdir -- "$parent"
    created_dirs+=("$parent")
  fi
done
mv -- "$package_stage" "$target"
target_installed=1
mv -f -- "$report_stage" "$report"
report_installed=1

echo "init.sh: created skills/$slug"
