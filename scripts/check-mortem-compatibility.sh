#!/usr/bin/env bash
set -euo pipefail

writer=""
legacy_readback=false
for arg in "$@"; do
  case "$arg" in
    --writer=legacy-v2|--writer=canonical-v3) writer="${arg#--writer=}" ;;
    --legacy-readback) legacy_readback=true ;;
    *) echo "usage: $0 --writer=legacy-v2 | --writer=canonical-v3 --legacy-readback" >&2; exit 2 ;;
  esac
done
if [[ "$writer" == "canonical-v3" ]]; then
  [[ "$legacy_readback" == true ]] || {
    echo "canonical-v3 requires --legacy-readback" >&2
    exit 2
  }
  echo "canonical-v3 writer is reserved for the cross-family-approved S8 cutover" >&2
  exit 1
fi
[[ "$writer" == "legacy-v2" && "$legacy_readback" == false ]] || {
  echo "usage: $0 --writer=legacy-v2 | --writer=canonical-v3 --legacy-readback" >&2
  exit 2
}

repo_root="$(git rev-parse --show-toplevel)"
fixtures="${MORTEM_COMPAT_FIXTURES_DIR:-$repo_root/tests/fixtures/mortem-compatibility}"
if [[ "$fixtures" != /* ]]; then
  fixtures="$repo_root/$fixtures"
fi

required=(
  v1-old-only.json v2-old-only.json v3-new-only.json
  v1-new-only-invalid.json v2-new-only-invalid.json v3-old-only-invalid.json
  unknown-version.json both-equal.json both-conflicting.json
  neither-optional.json neither-required.json
  legacy-directory/pre-mortem-check.json
  directory-conflict/pre-mortem-check.json
  directory-conflict/premortem-check.json
  explicit-skill-redirect.yaml
  legacy-readback/v1-old-only.json
  legacy-readback/v2-old-only.json
  writer-legacy-v2.json
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

expected_packets = {
    "v1-old-only.json": {"schema_version": 1, "required": True, "pre_mortem_verdict": "PASS"},
    "v2-old-only.json": {"schema_version": 2, "required": True, "pre_mortem_verdict": "WARN"},
    "v3-new-only.json": {"schema_version": 3, "required": True, "premortem_verdict": "PASS"},
    "v1-new-only-invalid.json": {"schema_version": 1, "required": True, "premortem_verdict": "PASS"},
    "v2-new-only-invalid.json": {"schema_version": 2, "required": True, "premortem_verdict": "PASS"},
    "v3-old-only-invalid.json": {"schema_version": 3, "required": True, "pre_mortem_verdict": "PASS"},
    "unknown-version.json": {"schema_version": 4, "required": True, "premortem_verdict": "PASS"},
    "both-equal.json": {"schema_version": 3, "required": True, "pre_mortem_verdict": "PASS", "premortem_verdict": "PASS"},
    "both-conflicting.json": {"schema_version": 3, "required": True, "pre_mortem_verdict": "PASS", "premortem_verdict": "FAIL"},
    "neither-optional.json": {"schema_version": 2, "required": False},
    "neither-required.json": {"schema_version": 2, "required": True},
}
for relative, expected in expected_packets.items():
    actual = load_json(relative)
    if actual != expected:
        raise SystemExit(f"{relative}: packet fixture contract mismatch: {actual!r}")

for relative, version in (
    ("legacy-readback/v1-old-only.json", 1),
    ("legacy-readback/v2-old-only.json", 2),
):
    actual = load_json(relative)
    expected = {"schema_version": version, "required": True, "pre_mortem_verdict": "PASS"}
    if actual != expected:
        raise SystemExit(f"{relative}: legacy-readback fixture contract mismatch: {actual!r}")

writer_path = "writer-legacy-v2.json"
writer = load_json(writer_path)
if writer.get("schema_version") != 2 or writer.get("packet_fields") != {"pre_mortem_verdict": "PASS"}:
    raise SystemExit(f"{writer_path}: legacy-v2 schema_version/packet_fields contract mismatch: {writer!r}")
if writer.get("runtime_paths") != [".agents/pre-mortem-checks/current.md"]:
    raise SystemExit(f"{writer_path}: legacy-v2 runtime_paths contract mismatch: {writer!r}")
if writer.get("ratchet_steps") != ["premortem", "postmortem"]:
    raise SystemExit(f"{writer_path}: canonical ratchet_steps contract mismatch: {writer!r}")

legacy_path = "directory-conflict/pre-mortem-check.json"
canonical_path = "directory-conflict/premortem-check.json"
legacy = load_json(legacy_path)
canonical = load_json(canonical_path)
if legacy.get("id") != canonical.get("id") or legacy == canonical:
    raise SystemExit(f"{legacy_path} and {canonical_path}: expected different content for the same ID")

redirect_path = "explicit-skill-redirect.yaml"
redirect_text = (root / redirect_path).read_text(encoding="utf-8")
for source, target in (
    ("pre-mortem", "premortem"), ("post-mortem", "postmortem"),
    ("pre_mortem", "premortem"), ("post_mortem", "postmortem"),
):
    block = re.search(rf"^  {re.escape(source)}:\n((?:    .*\n?)*)", redirect_text, re.MULTILINE)
    if not block or "state: merged-into" not in block.group(1) or f"merged-into: {target}" not in block.group(1):
        actual = block.group(1).strip() if block else "missing block"
        raise SystemExit(f"{redirect_path}: {source} must redirect to {target}; got {actual}")
PY

(
  cd "$repo_root/cli"
  export MORTEM_COMPAT_FIXTURES_DIR="$fixtures"
  go test ./internal/domain/packet \
    -run 'TestExecutionPacketDecodeJSON_MortemSchemaOwnership|TestExecutionPacketPublishedSchema_MortemOwnership|TestExecutionPacketDecodeJSON_MortemArtifactPathOwnershipMatchesSchemaVersion|TestExecutionPacketDecodeJSON_MortemEqualAndConflictRules|TestMortemCompatibilityFixtures_|TestExecutionPacketMarshal_MortemWriterRemainsLegacyV2OnlyThroughS7' \
    -count=1
  go test ./internal/ports -run 'Mortem|Premortem' -count=1
  go test ./internal/adapters/storage_fs -count=1
  go test ./cmd/ao \
    -run 'TestProductionFindingCompiler_PremortemAliases|TestMortemCompatibilityFixture_|TestStigmergicScorecard_EmitsCanonicalMortem|TestStatusFlywheel_EmitsCanonicalPremortem|TestPremortemDirectoryReader_|TestMortemCompatibilityFixtures_Directory|TestCollectContextExplainHealth_UsesCanonicalReconciledPremortemCount|TestRunContextExplain_RejectsConflictingPremortemDirectories|TestMortemJSONLChainLoad_' \
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

echo "mortem compatibility: PASS (version-owned readers, legacy-v2 writer, permanent redirects)"
