#!/usr/bin/env bash
# Create, structurally check, and project one skill exactly once.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
raw_repo="${SKILL_BUILDER_REPO_ROOT:-$(cd "$SCRIPT_DIR/../../.." && pwd -P)}"
[[ "$raw_repo" == /* && -d "$raw_repo" && ! -L "$raw_repo" ]] || {
  echo "skill-builder: repository root must be an absolute non-symlink directory" >&2
  exit 2
}
REPO_ROOT="$(cd "$raw_repo" && pwd -P)"
[[ "$raw_repo" == "$REPO_ROOT" ]] || {
  echo "skill-builder: repository root must use its canonical spelling" >&2
  exit 2
}
source "$SCRIPT_DIR/transaction.sh"

usage() {
  cat >&2 <<EOF
usage:
  build.sh from-scratch <slug>
  build.sh from-template <slug> --like <existing-slug>
  build.sh absorb-external <slug> --external-root <absolute-root> --from <relative-file>
EOF
  exit 2
}

[[ $# -ge 2 ]] || usage
mode="$1"
slug="$2"
shift 2
[[ "$slug" =~ ^[a-z][a-z0-9-]{0,63}$ ]] || {
  echo "skill-builder: slug must be lowercase-hyphen and at most 64 characters" >&2
  exit 2
}

STEP_TIMEOUT="${SKILL_BUILDER_STEP_TIMEOUT:-120}"
[[ "$STEP_TIMEOUT" =~ ^[0-9]+$ && "$STEP_TIMEOUT" -ge 1 && "$STEP_TIMEOUT" -le 300 ]] \
  || { echo "skill-builder: SKILL_BUILDER_STEP_TIMEOUT must be in [1,300]" >&2; exit 2; }
TIMEOUT_BIN=""
for candidate in /usr/bin/timeout /bin/timeout /opt/homebrew/bin/gtimeout /opt/homebrew/bin/timeout /usr/local/bin/gtimeout; do
  if [[ -x "$candidate" ]]; then TIMEOUT_BIN="$candidate"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || { echo "skill-builder: timeout or gtimeout is required" >&2; exit 2; }

for required in \
  "$REPO_ROOT/scripts/generate-skill-mesh.py" \
  "$REPO_ROOT/scripts/codex-sync.sh" \
  "$REPO_ROOT/scripts/regen-codex-hashes.sh"; do
  [[ -f "$required" && ! -L "$required" ]] \
    || { echo "skill-builder: required generator missing or symlinked" >&2; exit 2; }
done

run_step() {
  "$TIMEOUT_BIN" --signal=TERM --kill-after=2s "$STEP_TIMEOUT" "$@"
}

case "$mode" in
  from-scratch) init_mode=--scratch ;;
  from-template) init_mode=--template ;;
  absorb-external) init_mode=--external ;;
  *) usage ;;
esac

sb_txn_begin "$REPO_ROOT" "$slug" 1
run_step bash "$SCRIPT_DIR/init.sh" "$init_mode" "$slug" "$@"

report="$REPO_ROOT/.agents/scratch/skill-builder/${slug}-build.json"
if ! run_step env HEAL_REPO_ROOT="$REPO_ROOT" bash "$REPO_ROOT/skills/skill-builder/scripts/heal.sh" \
  --check --strict "$REPO_ROOT/skills/$slug"; then
  echo "skill-builder: structural check failed" >&2
  exit 1
fi

run_step python3 "$REPO_ROOT/scripts/generate-skill-mesh.py"
run_step bash "$REPO_ROOT/scripts/codex-sync.sh" --only "$slug"
run_step bash "$REPO_ROOT/scripts/regen-codex-hashes.sh" --only "$slug"

python3 - "$report" <<'PY'
import json
import os
from pathlib import Path
import sys
import tempfile

path = Path(sys.argv[1])
if path.is_symlink() or not path.is_file() or path.stat().st_size > 1048576:
    raise SystemExit("skill-builder: build report is unavailable, unsafe, or oversized")
payload = json.loads(path.read_text(encoding="utf-8"))
payload["structure_check_pass"] = True
encoded = json.dumps(payload, indent=2, sort_keys=True) + "\n"
with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=path.parent, prefix=f".{path.name}.", delete=False) as handle:
    temp = Path(handle.name)
    try:
        handle.write(encoded)
        handle.flush()
        os.fsync(handle.fileno())
    except BaseException:
        temp.unlink(missing_ok=True)
        raise
try:
    os.replace(temp, path)
except BaseException:
    temp.unlink(missing_ok=True)
    raise
PY

sb_txn_commit
echo "skill-builder: created and projected $slug"
