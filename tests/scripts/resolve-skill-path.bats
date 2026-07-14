#!/usr/bin/env bats
# Tests for scripts/lib/resolve-skill-path.sh (ag-2vz5v).
#
# resolve_skill_path <path> routes skill paths through the historical:
# section of docs/contracts/skill-dispositions.yaml so validators follow
# ledger folds (merged-into) and cuts instead of hardcoding paths.
# Hermetic: fixture ledgers in temp dirs via the SKILL_DISPOSITIONS_FILE
# env seam; no repo mutation.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    LIB="$REPO_ROOT/scripts/lib/resolve-skill-path.sh"
    TMP_DIR="$(mktemp -d)"
    LEDGER="$TMP_DIR/skill-dispositions.yaml"
    cat > "$LEDGER" <<'EOF'
# fixture ledger — mirrors the real flat shape (historical: above dispositions:)
historical:
  alpha:
    state:        merged-into
    merged-into:  beta
    date:         2026-06-12
    rationale:    "fixture: folded into beta"
  gone:
    state:        cut
    date:         2026-06-12
    rationale:    "fixture: cut"
  plan:
    state:        merged-into
    merged-into:  discovery
    date:         2026-06-12
    rationale:    "fixture: exact-match guard (plan vs plan-foundry)"

dispositions:
  - skill:          beta
    domain:         "BC2 Skills"
EOF
    EMPTY_LEDGER="$TMP_DIR/empty-ledger.yaml"
    cat > "$EMPTY_LEDGER" <<'EOF'
historical:

dispositions:
  - skill:          beta
    domain:         "BC2 Skills"
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

resolve() {
    # resolve <ledger> <path> — run resolve_skill_path in a fresh bash.
    SKILL_DISPOSITIONS_FILE="$1" bash -c \
        'source "$1" && resolve_skill_path "$2"' _ "$LIB" "$2"
}

# --- resolver units -------------------------------------------------------

@test "resolver lib exists and is sourceable" {
    [ -f "$LIB" ]
    run bash -c 'source "$1" && type resolve_skill_path' _ "$LIB"
    [ "$status" -eq 0 ]
}

@test "merged-into rewrites the skills/ slug segment" {
    run resolve "$LEDGER" "skills/alpha/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/beta/SKILL.md" ]
}

@test "merged-into rewrites the skills-codex/ slug segment" {
    run resolve "$LEDGER" "skills-codex/alpha/references/notes.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills-codex/beta/references/notes.md" ]
}

@test "cut prints nothing, returns 0" {
    stdout="$(resolve "$LEDGER" "skills/gone/SKILL.md" 2>/dev/null)"
    [ -z "$stdout" ]
    run resolve "$LEDGER" "skills/gone/SKILL.md"
    [ "$status" -eq 0 ]
}

@test "cut warns on stderr naming the slug and 'cut'" {
    stderr="$(resolve "$LEDGER" "skills/gone/SKILL.md" 2>&1 >/dev/null)"
    [[ "$stderr" == *"gone"* ]]
    [[ "$stderr" == *"cut"* ]]
}

@test "slug with no historical row is byte-identical" {
    run resolve "$LEDGER" "skills/heal-skill/scripts/heal.sh"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/heal-skill/scripts/heal.sh" ]
}

@test "exact slug match only: plan row does not rewrite plan-foundry" {
    run resolve "$LEDGER" "skills/plan-foundry/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/plan-foundry/SKILL.md" ]
    # ...while the exact slug IS rewritten
    run resolve "$LEDGER" "skills/plan/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/discovery/SKILL.md" ]
}

@test "non-skill paths pass through unchanged" {
    run resolve "$LEDGER" "AGENTS.md"
    [ "$status" -eq 0 ]
    [ "$output" = "AGENTS.md" ]
    run resolve "$LEDGER" "skills-codex-overrides/catalog.json"
    [ "$status" -eq 0 ]
    [ "$output" = "skills-codex-overrides/catalog.json" ]
}

@test "trailing-prefix path (skills/<slug>/dir/) resolves the slug segment" {
    run resolve "$LEDGER" "skills/alpha/references/"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/beta/references/" ]
}

@test "missing ledger file degrades to identity" {
    run resolve "$TMP_DIR/does-not-exist.yaml" "skills/alpha/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/alpha/SKILL.md" ]
}

@test "empty historical section is identity" {
    run resolve "$EMPTY_LEDGER" "skills/alpha/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/alpha/SKILL.md" ]
}

@test "dispositions rows never leak into historical lookup" {
    # 'beta' exists only under dispositions:, not historical: — identity.
    run resolve "$LEDGER" "skills/beta/SKILL.md"
    [ "$status" -eq 0 ]
    [ "$output" = "skills/beta/SKILL.md" ]
}

# --- validator integration: validate-codex-rpi-contract.sh ----------------

setup_rpi_fake_repo() {
    FAKE_REPO="$TMP_DIR/rpi-repo"
    mkdir -p "$FAKE_REPO/scripts/lib" "$FAKE_REPO/docs/contracts"
    /bin/cp "$REPO_ROOT/scripts/validate-codex-rpi-contract.sh" "$FAKE_REPO/scripts/"
    /bin/cp "$LIB" "$FAKE_REPO/scripts/lib/"
    chmod +x "$FAKE_REPO/scripts/validate-codex-rpi-contract.sh"
    mkdir -p "$FAKE_REPO/skills-codex" "$FAKE_REPO/skills-codex-overrides/research"
    # Padding dirs for the fake repo — the fold under test is on `rpi`. Copy every
    # slug the four-umbrella contract validator now touches so the fold is the ONLY
    # variable between the control (fails) and retarget (passes) cases. The
    # contract grew past rpi+crank (learn/evolve/premortem checks + the rpi Python
    # sub-validator) — the fixture tracks it. `research` is a currently-present
    # codex skill + override; the rpi/crank/evolve/premortem overrides stay absent
    # (require_absent), which the real repo already satisfies.
    for slug in rpi crank learn evolve premortem discovery validate research; do
        /bin/cp -R "$REPO_ROOT/skills-codex/$slug" "$FAKE_REPO/skills-codex/$slug"
    done
    /bin/cp "$REPO_ROOT/skills-codex-overrides/research/prompt.md" \
        "$FAKE_REPO/skills-codex-overrides/research/prompt.md"
    # rpi's validate-execution-packet.py resolves the repo root by walking up for
    # schemas/execution-packet.schema.json — provide it at the fake-repo root.
    mkdir -p "$FAKE_REPO/schemas"
    /bin/cp "$REPO_ROOT/schemas/execution-packet.schema.json" "$FAKE_REPO/schemas/"
    # Simulate a fold: rpi's dir moved to its merge target, path now absent.
    mv "$FAKE_REPO/skills-codex/rpi" "$FAKE_REPO/skills-codex/rpi-target"
    RPI_LEDGER="$TMP_DIR/rpi-ledger.yaml"
    cat > "$RPI_LEDGER" <<'EOF'
historical:
  rpi:
    state:        merged-into
    merged-into:  rpi-target
    date:         2026-06-12
    rationale:    "fixture: integration fold"

dispositions:
EOF
}

@test "rpi-contract validator fails when a listed skill dir is absent (control)" {
    setup_rpi_fake_repo
    run env SKILL_DISPOSITIONS_FILE="$EMPTY_LEDGER" \
        bash "$FAKE_REPO/scripts/validate-codex-rpi-contract.sh"
    [ "$status" -ne 0 ]
}

@test "rpi-contract validator passes when the ledger retargets the absent slug" {
    setup_rpi_fake_repo
    run env SKILL_DISPOSITIONS_FILE="$RPI_LEDGER" \
        bash "$FAKE_REPO/scripts/validate-codex-rpi-contract.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"passed"* ]]
}

# --- validator integration: check-hookless-cold-start.sh ------------------

setup_coldstart_fake_repo() {
    FAKE_REPO="$TMP_DIR/coldstart-repo"
    mkdir -p "$FAKE_REPO/scripts/lib" "$FAKE_REPO/skills/status2"
    /bin/cp "$REPO_ROOT/scripts/check-hookless-cold-start.sh" "$FAKE_REPO/scripts/"
    /bin/cp "$LIB" "$FAKE_REPO/scripts/lib/"
    chmod +x "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    # Clean non-skill surface so 'scanned' stays > 0 in every case.
    echo "Run ao session bootstrap explicitly." > "$FAKE_REPO/AGENTS.md"
    COLD_LEDGER="$TMP_DIR/coldstart-ledger.yaml"
    cat > "$COLD_LEDGER" <<'EOF'
historical:
  status:
    state:        merged-into
    merged-into:  status2
    date:         2026-06-12
    rationale:    "fixture: integration fold"

dispositions:
EOF
}

@test "cold-start validator scans the ledger-retargeted file" {
    setup_coldstart_fake_repo
    # Violation lives in the retarget; only the ledger can route the scan there.
    cat > "$FAKE_REPO/skills/status2/SKILL.md" <<'EOF'
The SessionStart hook loads repo context for every worker.
EOF
    run env SKILL_DISPOSITIONS_FILE="$COLD_LEDGER" \
        bash "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -ne 0 ]
    [[ "$output" == *"skills/status2/SKILL.md"* ]]
}

@test "cold-start validator skips cut slugs visibly and still passes" {
    setup_coldstart_fake_repo
    cat > "$TMP_DIR/coldstart-cut-ledger.yaml" <<'EOF'
historical:
  status:
    state:        cut
    date:         2026-06-12
    rationale:    "fixture: cut"

dispositions:
EOF
    # A violation in the CUT slug's old path must not be scanned.
    mkdir -p "$FAKE_REPO/skills/status"
    cat > "$FAKE_REPO/skills/status/SKILL.md" <<'EOF'
The SessionStart hook loads repo context for every worker.
EOF
    run env SKILL_DISPOSITIONS_FILE="$TMP_DIR/coldstart-cut-ledger.yaml" \
        bash "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"cut"* ]]
}
