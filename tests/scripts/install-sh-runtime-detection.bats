#!/usr/bin/env bats
# Regression tests for scripts/install.sh runtime-detection block (soc-vuu6.31).
#
# We can't run install.sh end-to-end in CI — it expects network access and
# rewrites ~/.claude. Instead we stub `claude`, `codex`, and `curl` on PATH
# and run the real install.sh; the script exits at the curl call once it
# moves past the detection block, so we capture stdout up to that point.
#
# What we assert: the script prints runtime-by-runtime detection lines and
# the correct mode line for each combination (both / claude-only /
# codex-only / neither).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  INSTALL_SH="$REPO_ROOT/scripts/install.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin"
  # Stub curl so the script exits after the detection block without
  # touching the network. Returning 1 causes set -e + pipefail to bail.
  cat >"$TMP/bin/curl" <<'EOF'
#!/usr/bin/env bash
echo "[stubbed-curl] $*" >&2
exit 1
EOF
  chmod +x "$TMP/bin/curl"
  # Keep coreutils on PATH (sed/awk/cat/etc). We add stubs at the front.
  COREUTILS_PATH="$ORIG_PATH"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

stub_runtime() {
  local name="$1"
  cat >"$TMP/bin/$name" <<EOF
#!/usr/bin/env bash
echo "[stubbed-$name] \$*"
exit 0
EOF
  chmod +x "$TMP/bin/$name"
}

run_install_with_path() {
  cd "$TMP"
  export PATH="$1"
  # The script's `set -e` will exit non-zero at curl; that's fine.
  run bash "$INSTALL_SH"
}

@test "both claude and codex present: mixed-model mode" {
  stub_runtime claude
  stub_runtime codex
  run_install_with_path "$TMP/bin:$COREUTILS_PATH"
  [[ "$output" == *"Detected Claude Code: yes"* ]]
  [[ "$output" == *"Detected Codex CLI: yes"* ]]
  [[ "$output" == *"mixed-model mode"* ]]
}

@test "claude only: single-runtime + Codex install suggestion" {
  stub_runtime claude
  # Do NOT stub codex; absent in $TMP/bin → also absent in PATH after we
  # restrict to $TMP/bin + a coreutils-only shim.
  mkdir -p "$TMP/coreutils-only"
  for cmd in bash sh sed awk grep tr sort cat mkdir mv rm cp ls wc dirname basename head tail printf jq mktemp tar id chmod; do
    full="$(command -v "$cmd" 2>/dev/null || true)"
    [ -n "$full" ] && ln -sf "$full" "$TMP/coreutils-only/$cmd"
  done
  run_install_with_path "$TMP/bin:$TMP/coreutils-only"
  [[ "$output" == *"Detected Claude Code: yes"* ]]
  [[ "$output" == *"Detected Codex CLI: no"* ]]
  [[ "$output" == *"single-runtime mode (Claude Code only)"* ]]
  [[ "$output" == *"install Codex CLI: https://github.com/openai/codex"* ]]
}

@test "codex only: single-runtime + Claude install suggestion" {
  stub_runtime codex
  mkdir -p "$TMP/coreutils-only"
  for cmd in bash sh sed awk grep tr sort cat mkdir mv rm cp ls wc dirname basename head tail printf jq mktemp tar id chmod; do
    full="$(command -v "$cmd" 2>/dev/null || true)"
    [ -n "$full" ] && ln -sf "$full" "$TMP/coreutils-only/$cmd"
  done
  run_install_with_path "$TMP/bin:$TMP/coreutils-only"
  [[ "$output" == *"Detected Claude Code: no"* ]]
  [[ "$output" == *"Detected Codex CLI: yes"* ]]
  [[ "$output" == *"single-runtime mode (Codex CLI only)"* ]]
  [[ "$output" == *"install Claude Code: https://docs.anthropic.com/en/docs/claude-code"* ]]
}

@test "neither present: warning + both install links" {
  # No runtime stubs; PATH points only at coreutils + curl stub.
  mkdir -p "$TMP/coreutils-only"
  for cmd in bash sh sed awk grep tr sort cat mkdir mv rm cp ls wc dirname basename head tail printf jq mktemp tar id chmod; do
    full="$(command -v "$cmd" 2>/dev/null || true)"
    [ -n "$full" ] && ln -sf "$full" "$TMP/coreutils-only/$cmd"
  done
  run_install_with_path "$TMP/bin:$TMP/coreutils-only"
  [[ "$output" == *"Detected Claude Code: no"* ]]
  [[ "$output" == *"Detected Codex CLI: no"* ]]
  [[ "$output" == *"No supported coding agent found"* ]]
  [[ "$output" == *"docs.anthropic.com/en/docs/claude-code"* ]]
  [[ "$output" == *"github.com/openai/codex"* ]]
  [[ "$output" == *"Continuing anyway"* ]]
}

@test "detection lines appear before any 'Step 1' install banner" {
  stub_runtime claude
  stub_runtime codex
  run_install_with_path "$TMP/bin:$COREUTILS_PATH"
  detect_line="$(printf '%s\n' "$output" | grep -n 'Detected Claude Code' | head -1 | cut -d: -f1)"
  step_line="$(printf '%s\n' "$output" | grep -n 'Step 1' | head -1 | cut -d: -f1)"
  [ -n "$detect_line" ]
  [ -n "$step_line" ]
  [ "$detect_line" -lt "$step_line" ]
}
