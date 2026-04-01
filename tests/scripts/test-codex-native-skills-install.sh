#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NATIVE_SCRIPT="$ROOT/scripts/install-codex-native-skills.sh"
PLUGIN_SCRIPT="$ROOT/scripts/install-codex-plugin.sh"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

if [[ ! -f "$NATIVE_SCRIPT" ]]; then
  echo "FAIL: missing script: $NATIVE_SCRIPT" >&2
  exit 1
fi
if [[ ! -f "$PLUGIN_SCRIPT" ]]; then
  echo "FAIL: missing script: $PLUGIN_SCRIPT" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

setup_fixture() {
  local fixture="$1"

  mkdir -p \
    "$fixture/scripts" \
    "$fixture/.codex-plugin" \
    "$fixture/.agents/plugins" \
    "$fixture/skills-codex/source-skill"

  cp "$NATIVE_SCRIPT" "$fixture/scripts/install-codex-native-skills.sh"
  cp "$PLUGIN_SCRIPT" "$fixture/scripts/install-codex-plugin.sh"
  chmod +x \
    "$fixture/scripts/install-codex-native-skills.sh" \
    "$fixture/scripts/install-codex-plugin.sh"

  cat > "$fixture/scripts/export-claude-skills-to-codex.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
SRC=""
DST=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --src)
      SRC="${2:-}"
      shift 2
      ;;
    --dst)
      DST="${2:-}"
      shift 2
      ;;
    --backup|--only)
      shift 2
      ;;
    --dry-run)
      shift
      ;;
    *)
      shift
      ;;
  esac
done
[[ -n "$SRC" && -n "$DST" ]] || exit 1
mkdir -p "$DST"
cp -R "$SRC/." "$DST/"
EOF
  chmod +x "$fixture/scripts/export-claude-skills-to-codex.sh"

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

  cat > "$fixture/skills-codex/source-skill/SKILL.md" <<'EOF'
---
name: source-skill
description: fixture
---
EOF

  cat > "$fixture/skills-codex/source-skill/prompt.md" <<'EOF'
# source-skill
EOF

  cat > "$fixture/skills-codex/source-skill/.agentops-generated.json" <<'EOF'
{
  "generator": "manual-maintained",
  "source_skill": "skills/source-skill",
  "layout": "modular",
  "source_hash": "fixture-source",
  "generated_hash": "fixture-generated"
}
EOF

  cat > "$fixture/skills-codex/.agentops-manifest.json" <<'EOF'
{
  "generator": "manual-maintained",
  "source_root": "skills",
  "layout": "modular",
  "skills": [
    {
      "name": "source-skill",
      "source_skill": "skills/source-skill",
      "source_hash": "fixture-source",
      "generated_hash": "fixture-generated"
    }
  ]
}
EOF
}

test_native_plugin_metadata_refreshes_plugin_cache() {
  local fixture="$TMP_DIR/plugin-mode"
  local home_dir="$TMP_DIR/home-plugin"
  local plugin_root="$home_dir/.codex/plugins/cache/agentops-marketplace/agentops/local"
  local user_skills_root="$home_dir/.agents/skills"

  setup_fixture "$fixture"
  mkdir -p "$home_dir/.codex"
  cat > "$home_dir/.codex/.agentops-codex-install.json" <<EOF
{
  "install_mode": "native-plugin",
  "plugin_root": "$plugin_root"
}
EOF

  if ! HOME="$home_dir" bash "$fixture/scripts/install-codex-native-skills.sh" >/dev/null; then
    fail "native-plugin refresh should succeed"
    return
  fi

  if [[ ! -f "$plugin_root/skills-codex/source-skill/SKILL.md" ]]; then
    fail "native-plugin refresh should write skills into plugin cache"
    return
  fi
  if [[ -e "$user_skills_root/source-skill/SKILL.md" ]]; then
    fail "native-plugin refresh should not leave a raw ~/.agents/skills mirror"
    return
  fi
  if [[ -e "$home_dir/.codex/skills/source-skill/SKILL.md" ]]; then
    fail "native-plugin refresh should not recreate ~/.codex/skills AgentOps mirror"
    return
  fi
  if rg -q '"source": "install-codex-plugin.sh"' "$home_dir/.codex/.agentops-codex-install.json" && \
    rg -q '"install_mode": "native-plugin"' "$home_dir/.codex/.agentops-codex-install.json"; then
    pass "native-plugin metadata refreshes plugin cache without leaving ~/.agents/skills overlap"
  else
    fail "native-plugin refresh should preserve plugin install metadata"
  fi
}

test_default_raw_install_targets_home_agents_skills() {
  local fixture="$TMP_DIR/default-raw"
  local home_dir="$TMP_DIR/home-default"
  local raw_dst="$home_dir/.agents/skills"

  setup_fixture "$fixture"

  if ! HOME="$home_dir" bash "$fixture/scripts/install-codex-native-skills.sh" >/dev/null; then
    fail "default raw install should succeed"
    return
  fi

  if [[ ! -f "$raw_dst/source-skill/SKILL.md" ]]; then
    fail "default raw install should target ~/.agents/skills"
    return
  fi
  if [[ -e "$home_dir/.codex/skills/source-skill/SKILL.md" ]]; then
    fail "default raw install should not target ~/.codex/skills"
    return
  fi
  if rg -q '"install_mode": "raw-skills"' "$home_dir/.codex/.agentops-codex-install.json" && \
    rg -q "\"skills_root\": \"$raw_dst\"" "$home_dir/.codex/.agentops-codex-install.json"; then
    pass "default raw install targets ~/.agents/skills"
  else
    fail "default raw install should record ~/.agents/skills metadata"
  fi
}

test_explicit_dest_keeps_raw_skill_install_behavior() {
  local fixture="$TMP_DIR/raw-mode"
  local home_dir="$TMP_DIR/home-raw"
  local raw_dst="$home_dir/custom-skills"
  local plugin_root="$home_dir/.codex/plugins/cache/agentops-marketplace/agentops/local"

  setup_fixture "$fixture"
  mkdir -p "$home_dir/.codex"
  cat > "$home_dir/.codex/.agentops-codex-install.json" <<EOF
{
  "install_mode": "native-plugin",
  "plugin_root": "$plugin_root"
}
EOF

  if ! HOME="$home_dir" bash "$fixture/scripts/install-codex-native-skills.sh" --dest "$raw_dst" >/dev/null; then
    fail "explicit --dest raw install should succeed"
    return
  fi

  if [[ ! -f "$raw_dst/source-skill/SKILL.md" ]]; then
    fail "explicit --dest should install raw skills into requested destination"
    return
  fi
  if [[ ! -f "$raw_dst/.agentops-codex-state.json" ]]; then
    fail "explicit --dest should write raw install state file"
    return
  fi
  if [[ -e "$plugin_root/skills-codex/source-skill/SKILL.md" ]]; then
    fail "explicit --dest should not refresh plugin cache"
    return
  fi
  if rg -q '"source": "install-codex-native-skills.sh"' "$home_dir/.codex/.agentops-codex-install.json" && \
    rg -q '"manifest_hash"' "$home_dir/.codex/.agentops-codex-install.json"; then
    pass "explicit --dest preserves raw skill install behavior"
  else
    fail "explicit --dest should record raw skill install metadata"
  fi
}

echo "== test-codex-native-skills-install =="
test_native_plugin_metadata_refreshes_plugin_cache
test_default_raw_install_targets_home_agents_skills
test_explicit_dest_keeps_raw_skill_install_behavior

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
