#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/validate-codex-generated-artifacts.sh"
MANIFEST_SCRIPT="$ROOT/scripts/validate-codex-generated-manifest.sh"
AUDIT_SCRIPT="$ROOT/scripts/audit-codex-parity.sh"
AUDIT_IMPL="$ROOT/scripts/audit-codex-parity.py"

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

setup_repo() {
  local repo="$1"

  mkdir -p "$repo/scripts" "$repo/skills/example" "$repo/skills-codex/example" "$repo/skills-codex-overrides"
  cp "$SCRIPT" "$repo/scripts/validate-codex-generated-artifacts.sh"
  cp "$MANIFEST_SCRIPT" "$repo/scripts/validate-codex-generated-manifest.sh"
  cp "$AUDIT_SCRIPT" "$repo/scripts/audit-codex-parity.sh"
  cp "$AUDIT_IMPL" "$repo/scripts/audit-codex-parity.py"
  chmod +x "$repo/scripts/validate-codex-generated-artifacts.sh"
  chmod +x "$repo/scripts/validate-codex-generated-manifest.sh"
  chmod +x "$repo/scripts/audit-codex-parity.sh"
  chmod +x "$repo/scripts/audit-codex-parity.py"

  cat > "$repo/skills/example/SKILL.md" <<'EOF'
---
name: example
description: fixture
---
EOF

  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
---
name: example
description: fixture
---
EOF

  cat > "$repo/skills-codex-overrides/catalog.json" <<'EOF'
{
  "version": 1,
  "waves": [
    {"id": "fixture", "description": "fixture"}
  ],
  "skills": [
    {"name": "example", "treatment": "bespoke", "wave": "fixture", "reason": "fixture"}
  ]
}
EOF

  export FIXTURE_ROOT="$repo"
  python3 - <<'PY'
import hashlib
import json
import os
from pathlib import Path

repo = Path(os.environ["FIXTURE_ROOT"])
skills_root = repo / "skills-codex"
skill_dir = skills_root / "example"

def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()

def sha256_file(path: Path) -> str:
    return sha256_bytes(path.read_bytes())

def hash_tree(root: Path) -> str:
    rows = []
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        if path.name in {".agentops-manifest.json", ".agentops-generated.json"}:
            continue
        rows.append(f"{path.relative_to(root).as_posix()}\t{sha256_file(path)}\n")
    return sha256_bytes("".join(rows).encode("utf-8"))

generated_hash = hash_tree(skill_dir)
source_hash = sha256_bytes(b"fixture-source")
marker = {
    "generator": "manual-maintained",
    "source_skill": "skills/example",
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
            "name": "example",
            "source_skill": "skills/example",
            "source_hash": source_hash,
            "generated_hash": generated_hash,
        }
    ],
}
(skills_root / ".agentops-manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
PY

  git -C "$repo" init -q
  git -C "$repo" config user.email "test@example.com"
  git -C "$repo" config user.name "Test"
  git -C "$repo" add .
  git -C "$repo" commit -qm "fixture"
}

test_passes_when_markers_exist_and_no_changes() {
  local repo="$TMP_DIR/pass"
  setup_repo "$repo"

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope head >/dev/null); then
    pass "passes with manifest and per-skill markers present"
  else
    fail "should pass with generated markers present"
  fi
}

test_fails_on_missing_marker() {
  local repo="$TMP_DIR/missing-marker"
  setup_repo "$repo"
  rm -f "$repo/skills-codex/example/.agentops-generated.json"

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope worktree >/dev/null 2>&1); then
    fail "should fail when per-skill marker is missing"
  else
    pass "fails when per-skill marker is missing"
  fi
}

test_fails_on_codex_only_edits() {
  local repo="$TMP_DIR/codex-only"
  setup_repo "$repo"
  echo "# direct edit" >> "$repo/skills-codex/example/SKILL.md"

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope worktree >/dev/null 2>&1); then
    fail "should fail on codex-only edits"
  else
    pass "fails when skills-codex changes without source edits"
  fi
}

test_fails_when_source_changes_without_regen() {
  local repo="$TMP_DIR/source-only"
  setup_repo "$repo"
  echo "# source edit" >> "$repo/skills/example/SKILL.md"

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope worktree >/dev/null 2>&1); then
    fail "should fail when source changes without regenerated codex output"
  else
    pass "fails when source changes are missing regenerated codex output"
  fi
}

test_fails_when_changed_skill_has_codex_semantic_drift() {
  local repo="$TMP_DIR/semantic-drift"
  setup_repo "$repo"
  cat >> "$repo/skills/example/SKILL.md" <<'EOF'
# source edit
EOF
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
---
name: example
description: fixture
---

TaskCreate(subject="broken")
EOF

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope worktree >/dev/null 2>&1); then
    fail "should fail when changed Codex skill still has semantic drift"
  else
    pass "fails when changed Codex skill still has semantic drift"
  fi
}

test_fails_when_skill_has_non_codex_frontmatter() {
  local repo="$TMP_DIR/frontmatter-drift"
  setup_repo "$repo"
  cat > "$repo/skills-codex/example/SKILL.md" <<'EOF'
---
name: example
description: fixture
metadata:
  tier: meta
---
EOF

  if (cd "$repo" && bash scripts/validate-codex-generated-artifacts.sh --scope worktree >/dev/null 2>&1); then
    fail "should fail when Codex skill retains non-Codex frontmatter fields"
  else
    pass "fails when Codex skill retains non-Codex frontmatter fields"
  fi
}

echo "== test-codex-generated-artifacts =="
test_passes_when_markers_exist_and_no_changes
test_fails_on_missing_marker
test_fails_on_codex_only_edits
test_fails_when_source_changes_without_regen
test_fails_when_changed_skill_has_codex_semantic_drift
test_fails_when_skill_has_non_codex_frontmatter

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
