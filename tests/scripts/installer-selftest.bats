#!/usr/bin/env bats
# Regression tests for the installer self-test tails (age-txfnl / FU7).
#
# Each installer must verify its own claims before printing success: the skill
# directory count, the manifest count, and the recorded metadata count must all
# agree (so `ao doctor` will agree too), the config-enable entry must be
# present, and a sentinel skill must be readable. On any mismatch the installer
# must exit nonzero naming the delta — never print success over a broken state.
#
# The historical bug these guard: install-codex.sh reported "Skills available:
# 66" and success, but the very next `ao doctor` flagged "install metadata says
# 66 skills but manifest says 62" because doctor reads the manifest's
# package_count (falling back to the smaller skills[] count when it is absent)
# while the installer counted on-disk directories.
#
# All installs run offline against the worktree via AGENTOPS_BUNDLE_ROOT, with
# HOME/CODEX_HOME pointed at a temp dir — never the real $HOME.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  command -v jq >/dev/null 2>&1 || skip "jq required to seed manifest-mismatch fixtures"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# make_codex_bundle builds a lightweight AgentOps bundle: everything the Codex
# installer reads is symlinked from the worktree except skills-codex/, which is
# copied so a test can mutate its manifest without touching the real checkout.
make_codex_bundle() {
  local bundle="$TMP/bundle"
  mkdir -p "$bundle"
  ln -s "$REPO_ROOT/.codex-plugin" "$bundle/.codex-plugin"
  ln -s "$REPO_ROOT/plugins" "$bundle/plugins"
  ln -s "$REPO_ROOT/scripts" "$bundle/scripts"
  [ -f "$REPO_ROOT/.mcp.json" ] && ln -s "$REPO_ROOT/.mcp.json" "$bundle/.mcp.json"
  [ -f "$REPO_ROOT/.app.json" ] && ln -s "$REPO_ROOT/.app.json" "$bundle/.app.json"
  cp -R "$REPO_ROOT/skills-codex" "$bundle/skills-codex"
  echo "$bundle"
}

run_codex_install() {
  local bundle="$1"
  local home="$TMP/home"
  mkdir -p "$home"
  run env HOME="$home" AGENTOPS_BUNDLE_ROOT="$bundle" bash "$REPO_ROOT/scripts/install-codex.sh"
}

# ─────────────────────────── install-codex.sh ───────────────────────────

@test "install-codex.sh: clean bundle passes self-test and reports consistent counts" {
  bundle="$(make_codex_bundle)"
  run_codex_install "$bundle"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Self-test passed"* ]]
  [[ "$output" == *"Skills available:"* ]]
  [[ "$output" == *"Verify it worked"* ]]
  # The printed count and the recorded metadata count must match.
  home="$TMP/home"
  disk="$(find "$home/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex" -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l | tr -d ' ')"
  meta="$(jq -r '.skill_count' "$home/.codex/.agentops-codex-install.json")"
  pkg="$(jq -r '.package_count' "$bundle/skills-codex/.agentops-manifest.json")"
  [ "$disk" = "$meta" ]
  [ "$disk" = "$pkg" ]
}

@test "install-codex.sh: stale manifest (no package_count, skills[] < disk dirs) fails — the 66-vs-62 bug" {
  bundle="$(make_codex_bundle)"
  # Strip package_count so doctor (and our self-test) fall back to len(skills[]),
  # which on the real bundle (62 implementation rows) is < the 66 on-disk dirs —
  # exactly the audit's 66-vs-62 divergence.
  local man="$bundle/skills-codex/.agentops-manifest.json"
  local skills_len disk_dirs
  skills_len="$(jq '.skills | length' "$man")"
  disk_dirs="$(find "$bundle/skills-codex" -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l | tr -d ' ')"
  # Precondition: the fallback count must actually differ from the disk count,
  # otherwise the test proves nothing.
  [ "$skills_len" != "$disk_dirs" ]
  jq 'del(.package_count)' "$man" > "$man.tmp" && mv "$man.tmp" "$man"

  run_codex_install "$bundle"
  [ "$status" -ne 0 ]
  [[ "$output" == *"!= manifest skill count ($skills_len)"* ]]
  [[ "$output" == *"refusing to report success"* ]]
  # Must NOT have declared success.
  [[ "$output" != *"Native Codex plugin installed"* ]]
  [[ "$output" != *"Verify it worked"* ]]
}

@test "install-codex.sh: legacy manifest with matching skills[] and no package_count passes (mirrors doctor)" {
  # A manifest without package_count is fine when len(skills[]) == disk dirs;
  # doctor accepts it, so the self-test must too (no over-strictness).
  bundle="$(make_codex_bundle)"
  local man="$bundle/skills-codex/.agentops-manifest.json"
  local disk_dirs
  disk_dirs="$(find "$bundle/skills-codex" -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l | tr -d ' ')"
  # Rewrite skills[] to have exactly disk_dirs entries and drop package_count.
  jq --argjson n "$disk_dirs" \
    'del(.package_count) | .skills = [range(0;$n) | {name: ("s"+(tostring)), source_skill: ("skills/s"+(tostring))}]' \
    "$man" > "$man.tmp" && mv "$man.tmp" "$man"

  run_codex_install "$bundle"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Self-test passed"* ]]
  [[ "$output" == *"Verify it worked"* ]]
}

@test "install-codex.sh: manifest package_count != disk count fails the self-test" {
  bundle="$(make_codex_bundle)"
  local man="$bundle/skills-codex/.agentops-manifest.json"
  jq '.package_count = 999' "$man" > "$man.tmp" && mv "$man.tmp" "$man"

  run_codex_install "$bundle"
  [ "$status" -ne 0 ]
  [[ "$output" == *"!= manifest skill count (999)"* ]]
  [[ "$output" == *"refusing to report success"* ]]
  [[ "$output" != *"Verify it worked"* ]]
}

# ─────────────────────────── install-opencode.sh ───────────────────────────

@test "install-opencode.sh: clean bundle passes self-test and links skills" {
  home="$TMP/oc-home"
  cfg="$home/.config/opencode"
  mkdir -p "$home"
  run env HOME="$home" OPENCODE_CONFIG_DIR="$cfg" AGENTOPS_BUNDLE_ROOT="$REPO_ROOT" \
    bash "$REPO_ROOT/scripts/install-opencode.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Self-test passed"* ]]
  [[ "$output" == *"Verify it worked"* ]]
  [ -e "$cfg/plugins/agentops.js" ]
  [ -e "$cfg/skills/agentops" ]
}

@test "install-opencode.sh: bundle with no skill files fails the self-test" {
  # Minimal fixture: a plugin file plus a skills/ dir that holds no SKILL.md.
  local bundle="$TMP/oc-bundle"
  mkdir -p "$bundle/.opencode/plugins" "$bundle/skills/not-a-skill"
  echo "// stub" > "$bundle/.opencode/plugins/agentops.js"
  echo "placeholder" > "$bundle/skills/not-a-skill/README.md"

  home="$TMP/oc-home2"
  cfg="$home/.config/opencode"
  mkdir -p "$home"
  run env HOME="$home" OPENCODE_CONFIG_DIR="$cfg" AGENTOPS_BUNDLE_ROOT="$bundle" \
    bash "$REPO_ROOT/scripts/install-opencode.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"self-test"* ]]
  [[ "$output" == *"refusing to report success"* ]]
  [[ "$output" != *"Verify it worked"* ]]
}

# ─────────────────────────── install.sh (orchestrator) ───────────────────────────

@test "install.sh: offline local-source install passes and prints Done" {
  # Stub codex present; restrict PATH so brew/agy/claude are absent (no network).
  local bin="$TMP/bin"
  mkdir -p "$bin"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$bin/codex"
  chmod +x "$bin/codex"
  home="$TMP/is-home"
  mkdir -p "$home"

  run env PATH="$bin:/usr/bin:/bin" HOME="$home" AGENTOPS_BUNDLE_ROOT="$REPO_ROOT" \
    bash "$REPO_ROOT/scripts/install.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Local-source mode"* ]]
  [[ "$output" == *"Self-test passed"* ]]
  [[ "$output" == *"Done!"* ]]
  [[ "$output" == *"Verify it worked"* ]]
  [ -f "$home/.codex/.agentops-codex-install.json" ]
}

@test "install-agy.sh: self-test fails closed when 'agy plugin list' exits nonzero" {
  stub="$BATS_TEST_TMPDIR/stubbin"
  mkdir -p "$stub"
  cat > "$stub/agy" <<'STUB'
#!/usr/bin/env bash
case "$1 $2" in
  "plugin list") exit 1 ;;
  *) exit 0 ;;
esac
STUB
  chmod +x "$stub/agy"
  run env HOME="$BATS_TEST_TMPDIR/home" PATH="$stub:$PATH" \
    AGENTOPS_BUNDLE_ROOT="$REPO_ROOT" bash "$REPO_ROOT/scripts/install-agy.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"agy plugin list"* ]]
}
