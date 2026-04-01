#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-codex-generated-manifest.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

[[ -x "$SCRIPT" ]] || {
  echo "FAIL: missing script: $SCRIPT" >&2
  exit 1
}
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"
  mkdir -p "$fixture/skills/source-skill" "$fixture/skills-codex/source-skill"
  cat > "$fixture/skills/source-skill/SKILL.md" <<'EOF'
---
name: source-skill
description: fixture
---
EOF
  cat > "$fixture/skills-codex/source-skill/SKILL.md" <<'EOF'
---
name: source-skill
description: fixture
---
EOF
  cat > "$fixture/skills-codex/source-skill/prompt.md" <<'EOF'
# source-skill
EOF
  export FIXTURE_ROOT="$fixture"
  python3 - <<'PY'
import hashlib
import json
import os
from pathlib import Path

fixture = Path(os.environ["FIXTURE_ROOT"])
skills_root = fixture / "skills-codex"
skill_dir = skills_root / "source-skill"

def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()

def sha256_file(path: Path) -> str:
    return sha256_bytes(path.read_bytes())

def hash_tree(root: Path) -> str:
    rows = []
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        if path.name in {".agentops-manifest.json", ".agentops-generated.json", ".DS_Store"}:
            continue
        if "__pycache__" in path.parts:
            continue
        if path.suffix == ".pyc":
            continue
        rows.append(f"{path.relative_to(root).as_posix()}\t{sha256_file(path)}\n")
    return sha256_bytes("".join(rows).encode("utf-8"))

generated_hash = hash_tree(skill_dir)
source_hash = sha256_bytes(b"fixture-source")
marker = {
    "generator": "manual-maintained",
    "source_skill": "skills/source-skill",
    "layout": "modular",
    "source_hash": source_hash,
    "generated_hash": generated_hash,
}
(skill_dir / ".agentops-generated.json").write_text(json.dumps(marker), encoding="utf-8")
manifest = {
    "generator": "manual-maintained",
    "source_root": "skills",
    "layout": "modular",
    "skills": [
        {
            "name": "source-skill",
            "source_skill": "skills/source-skill",
            "source_hash": source_hash,
            "generated_hash": generated_hash,
        }
    ],
}
(skills_root / ".agentops-manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
PY
}

test_passes_with_matching_manifest() {
  local fixture="$TMP_DIR/pass"
  setup_fixture "$fixture"

  if (cd "$fixture" && bash "$SCRIPT" skills-codex >/dev/null); then
    pass "passes when codex manifest matches tree"
  else
    fail "should pass with matching codex manifest"
  fi
}

test_fails_on_drift() {
  local fixture="$TMP_DIR/fail"
  setup_fixture "$fixture"
  echo "drift" >> "$fixture/skills-codex/source-skill/prompt.md"

  if (cd "$fixture" && bash "$SCRIPT" skills-codex >/dev/null 2>&1); then
    fail "should fail when codex manifest drifts"
  else
    pass "fails when codex manifest drifts"
  fi
}

test_ignores_cache_artifacts() {
  local fixture="$TMP_DIR/cache"
  setup_fixture "$fixture"
  mkdir -p "$fixture/skills-codex/source-skill/__pycache__"
  printf 'cache' > "$fixture/skills-codex/source-skill/__pycache__/temp.cpython-314.pyc"
  printf 'junk' > "$fixture/skills-codex/source-skill/.DS_Store"

  if (cd "$fixture" && bash "$SCRIPT" skills-codex >/dev/null); then
    pass "ignores cache artifacts when validating manifests"
  else
    fail "should ignore cache artifacts when validating manifests"
  fi
}

echo "== test-codex-generated-manifest =="
test_passes_with_matching_manifest
test_fails_on_drift
test_ignores_cache_artifacts

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
