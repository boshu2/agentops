#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 0 ]]; then
  echo "usage: $0" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
fixtures="${MORTEM_COMPAT_FIXTURES_DIR:-$repo_root/tests/fixtures/mortem-compatibility}"
if [[ "$fixtures" != /* ]]; then
  fixtures="$repo_root/$fixtures"
fi

required=(
  legacy-directory/pre-mortem-check.json
  directory-conflict/pre-mortem-check.json
  directory-conflict/premortem-check.json
  explicit-skill-redirect.yaml
)
for relative in "${required[@]}"; do
  [[ -f "$fixtures/$relative" ]] || {
    echo "missing compatibility fixture: tests/fixtures/mortem-compatibility/$relative" >&2
    exit 1
  }
done

python3 - "$fixtures" <<'PY'
import json
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])

def load_json(relative):
    path = root / relative
    try:
        return json.loads(path.read_bytes())
    except Exception as exc:
        raise SystemExit(f"{relative}: invalid JSON fixture: {exc}")

legacy_path = "directory-conflict/pre-mortem-check.json"
canonical_path = "directory-conflict/premortem-check.json"
legacy = load_json(legacy_path)
canonical = load_json(canonical_path)
if legacy.get("id") != canonical.get("id") or legacy == canonical:
    raise SystemExit(
        f"{legacy_path} and {canonical_path}: expected different content for the same ID"
    )

redirect_path = "explicit-skill-redirect.yaml"
redirect_text = (root / redirect_path).read_text(encoding="utf-8")
for source, target in (
    ("pre-mortem", "premortem"), ("post-mortem", "postmortem"),
    ("pre_mortem", "premortem"), ("post_mortem", "postmortem"),
):
    block = re.search(
        rf"^  {re.escape(source)}:\n((?:    .*\n?)*)", redirect_text, re.MULTILINE
    )
    if (
        not block
        or "state: merged-into" not in block.group(1)
        or f"merged-into: {target}" not in block.group(1)
    ):
        actual = block.group(1).strip() if block else "missing block"
        raise SystemExit(f"{redirect_path}: {source} must redirect to {target}; got {actual}")
PY

(
  cd "$repo_root/cli"
  export MORTEM_COMPAT_FIXTURES_DIR="$fixtures"
  go test ./internal/domain/packet -run 'TestExecutionPacketPremortemContract' -count=1
  go test ./internal/adapters/storage_fs -run 'TestRepo_PremortemContract' -count=1
  go test ./internal/ports -run 'Mortem|Premortem' -count=1
  go test ./cmd/ao \
    -run 'TestProductionFindingCompiler_PremortemAliases|TestCanonicalMortemRatchetStepsRemainRegistered|TestStigmergicScorecard_EmitsCanonicalMortem|TestStatusFlywheel_EmitsCanonicalPremortem|TestPremortemDirectoryReader_|TestMortemCompatibilityFixtures_Directory|TestCollectContextExplainHealth_UsesCanonicalReconciledPremortemCount|TestRunContextExplain_RejectsConflictingPremortemDirectories|TestMortemJSONLChainLoad_' \
    -count=1
)

source "$repo_root/scripts/lib/resolve-skill-path.sh"
export SKILL_DISPOSITIONS_FILE="$fixtures/explicit-skill-redirect.yaml"
[[ "$(resolve_skill_path skills/pre-mortem/SKILL.md)" == "skills/premortem/SKILL.md" ]]
[[ "$(resolve_skill_path skills-codex/post-mortem/SKILL.md)" == "skills-codex/postmortem/SKILL.md" ]]
[[ "$(resolve_skill_path skills/pre_mortem/SKILL.md)" == "skills/premortem/SKILL.md" ]]
[[ "$(resolve_skill_path skills-codex/post_mortem/SKILL.md)" == "skills-codex/postmortem/SKILL.md" ]]
[[ "$(resolve_skill_path skills/premortem/SKILL.md)" == "skills/premortem/SKILL.md" ]]
[[ "$(resolve_skill_path skills-codex/postmortem/SKILL.md)" == "skills-codex/postmortem/SKILL.md" ]]

echo "mortem compatibility: PASS (canonical packet contract; non-packet directory and skill redirects)"
