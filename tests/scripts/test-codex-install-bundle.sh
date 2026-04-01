#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-codex-install-bundle.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

if [[ ! -f "$SCRIPT" ]]; then
  echo "FAIL: missing script: $SCRIPT" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"
  local skill_body="$2"

  mkdir -p \
    "$fixture/.codex-plugin" \
    "$fixture/.agents/plugins" \
    "$fixture/scripts" \
    "$fixture/skills-codex/source-skill"

  cp "$SCRIPT" "$fixture/scripts/validate-codex-install-bundle.sh"
  cp "$ROOT/scripts/validate-codex-generated-manifest.sh" "$fixture/scripts/validate-codex-generated-manifest.sh"
  cp "$ROOT/scripts/validate-codex-generated-artifacts.sh" "$fixture/scripts/validate-codex-generated-artifacts.sh"
  cp "$ROOT/scripts/audit-codex-parity.sh" "$fixture/scripts/audit-codex-parity.sh"
  cp "$ROOT/scripts/audit-codex-parity.py" "$fixture/scripts/audit-codex-parity.py"
  chmod +x \
    "$fixture/scripts/validate-codex-install-bundle.sh" \
    "$fixture/scripts/validate-codex-generated-manifest.sh" \
    "$fixture/scripts/validate-codex-generated-artifacts.sh" \
    "$fixture/scripts/audit-codex-parity.sh" \
    "$fixture/scripts/audit-codex-parity.py"

  cat > "$fixture/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "agentops",
  "skills": "./skills-codex"
}
EOF

  cat > "$fixture/.agents/plugins/marketplace.json" <<'EOF'
{
  "name": "agentops-marketplace",
  "plugins": [
    {
      "name": "agentops",
      "source": {
        "source": "local",
        "path": "./"
      }
    }
  ]
}
EOF

  cat > "$fixture/skills-codex/source-skill/SKILL.md" <<EOF
$skill_body
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

  (
    cd "$fixture"
    git init >/dev/null 2>&1
    git config user.name "Codex Test"
    git config user.email "codex@example.com"
    git add .
    git commit -m "fixture" >/dev/null 2>&1
  )
}

run_fixture() {
  local fixture="$1"
  local out_file="$2"

  (
    cd "$fixture"
    bash scripts/validate-codex-install-bundle.sh
  ) > "$out_file" 2>&1
}

test_pass_with_consistent_bundle() {
  local fixture="$TMP_DIR/pass"
  local out="$fixture/out.txt"
  local body='---
name: source-skill
description: generated
---

# Source Skill

Bundle metadata and files are internally consistent.'

  setup_fixture "$fixture" "$body"

  if run_fixture "$fixture" "$out"; then
    pass "passes when archived bundle is internally consistent"
  else
    fail "should pass when archived bundle is internally consistent"
    sed 's/^/  /' "$out"
  fi
}

test_fail_with_manifest_drift() {
  local fixture="$TMP_DIR/fail"
  local out="$fixture/out.txt"
  local body='---
name: source-skill
description: current
---

# Source Skill

This bundle starts consistent.'

  setup_fixture "$fixture" "$body"
  echo "drift" >> "$fixture/skills-codex/source-skill/SKILL.md"

  if run_fixture "$fixture" "$out"; then
    fail "should fail when archived bundle metadata drifts"
    return
  fi

  if grep -q "generated_hash drift detected" "$out"; then
    pass "fails when archived bundle metadata drifts"
  else
    fail "missing bundle drift error"
    sed 's/^/  /' "$out"
  fi
}

echo "== test-codex-install-bundle =="
test_pass_with_consistent_bundle
test_fail_with_manifest_drift

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
