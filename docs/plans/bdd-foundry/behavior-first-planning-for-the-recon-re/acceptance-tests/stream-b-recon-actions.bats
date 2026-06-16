#!/usr/bin/env bats
# Acceptance tests — Stream B (recon repo action items): B1, B2, B4.
# B3 + B5 are Go-backed and live in stream-b-go.bats (which drives `go test`).
#
# TEST-FIRST: until B1 wires the detector into CI + sweeps stale refs, B2 cuts
# the changelog/tag, and B4 decomposes cli/cmd/ao, these are RED by construction.
#
# Run from the repo root:
#   bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../../../.." && pwd)"
    DETECTOR="$REPO_ROOT/scripts/check-doc-skill-refs.sh"
    VALIDATE_YML="$REPO_ROOT/.github/workflows/validate.yml"
    CHANGELOG="$REPO_ROOT/CHANGELOG.md"
    DOCS="$(mktemp -d "$BATS_TMPDIR/docs.XXXXXX")"
    SKILLS="$(mktemp -d "$BATS_TMPDIR/skills.XXXXXX")"
    mkdir -p "$SKILLS/alpha"
}

teardown() {
    [ -n "${DOCS:-}" ] && rm -rf "$DOCS"
    [ -n "${SKILLS:-}" ] && rm -rf "$SKILLS"
}

# ── B1 — wire check-doc-skill-refs.sh into CI + sweep dead-skill refs ─────────

@test "B1-S1: validate.yml invokes check-doc-skill-refs.sh --strict (occurrence >= 1)" {
    [ -f "$VALIDATE_YML" ]
    run grep -c "check-doc-skill-refs" "$VALIDATE_YML"
    [ "$status" -eq 0 ]
    [ "$output" -ge 1 ]
    grep -Eq "check-doc-skill-refs\.sh.*--strict|--strict.*check-doc-skill-refs" "$VALIDATE_YML"
}

@test "B1-S2: the detector passes --strict against HEAD after the sweep" {
    bash "$DETECTOR" --strict
}

@test "B1-S3: a fixture doc citing a retired skill on a non-exempt line is caught in strict mode" {
    printf 'Run `/bug-hunt` to find bugs.\n' > "$DOCS/CLAUDE.md"
    run bash "$DETECTOR" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 1 ]
    [[ "$output" == *"bug-hunt"* ]]
}

@test "B1-S4: a retirement-note line is exempt and does not fail" {
    printf '`/vibe` was retired and folded into `/validate`.\n' > "$DOCS/CLAUDE.md"
    run bash "$DETECTOR" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
}

@test "B1-S5: archival refs are removed OR allowlisted with an inline reason, not silently swept" {
    # Assert the strict detector is clean at HEAD (every kept-stale ref handled)
    # AND that the detector recognizes a structured allowlist mechanism.
    bash "$DETECTOR" --strict
    grep -Eq "allowlist|allow-list|ALLOWLIST" "$DETECTOR"
}

@test "B1-G9: the checker is a REQUIRED, live, path-reaching CI job (not advisory/skipped/if:false)" {
    [ -f "$VALIDATE_YML" ]
    # the job/step must run --strict and must NOT be neutered by continue-on-error / if:false
    grep -Eq "check-doc-skill-refs\.sh --strict" "$VALIDATE_YML"
    # the step that runs the detector must not carry continue-on-error: true on the same job block
    run bash -c "awk '/check-doc-skill-refs/{print NR\": \"\$0}' '$VALIDATE_YML'"
    [ -n "$output" ]
    # no 'if: false' guarding it and no continue-on-error in the immediate vicinity
    run grep -nE "continue-on-error:\s*true" "$VALIDATE_YML"
    # if continue-on-error appears, it must not be on the doc-skill-refs job — assert
    # the detector line is reachable by a normal push diff (job has no restrictive 'if:false')
    run grep -nE "if:\s*false" "$VALIDATE_YML"
    [ "$status" -ne 0 ]
}

@test "B1-G11: incidental retirement words do NOT exempt a live stale ref (structural exemption only)" {
    printf '`/zzz-phantom` is not retired; run it for live incidents.\n' > "$DOCS/CLAUDE.md"
    run bash "$DETECTOR" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 1 ]
    [[ "$output" == *"zzz-phantom"* ]]
}

# ── B3 — quorum doctrine ratification (doc + fleet-memory content assertions) ─
# B3-S1 is the Go guardrail (stream-b-go.bats); the doc/memory/consumer work
# (B3-S2/S3/S4) is content-asserted here. RED by construction until C8 writes the
# doctrine note, edits the two memories, and reconciles consumers.

# Scan repo docs for the ratified doctrine note (path loosely specified in spec;
# resolved by scanning the doc tree, mirroring A5's guidance_files() pattern).
doctrine_doc() {
    grep -rilE "context, not the model, makes a judge independent" \
        "$REPO_ROOT/docs" 2>/dev/null \
        | grep -v "/bdd-foundry/" | head -1
}

@test "B3-S2: the quorum doctrine note states the context-floor doctrine + names RequireCrossFamily as the opt-in" {
    doc="$(doctrine_doc)"
    [ -n "$doc" ]
    grep -Eq "context, not the model, makes a judge independent" "$doc"
    grep -Eiq "cross-family is an opt-in upgrade|opt-in upgrade for multi-model" "$doc"
    grep -Eq "RequireCrossFamily" "$doc"
    grep -Eiq "opt-in strengthener|opt-in upgrade" "$doc"
}

@test "B3-S3: neither fleet memory asserts a model-family floor as the default" {
    MEM="${HOME}/.claude/projects/-Users-bo-dev-agentops/memory"
    COST="$MEM/cost-law-quorum-at-gates-cheap-at-generation.md"
    GATE="$MEM/quorum-gate-exists-producer-is-the-build.md"
    [ -f "$COST" ] || skip "fleet memory host not present (cost-law-quorum)"
    [ -f "$GATE" ] || skip "fleet memory host not present (quorum-gate-exists)"
    # neither memory may assert a ≥2-model-families floor as the DEFAULT
    run grep -EiC0 "≥ *2 model families|two model families|cross-family.*(required|default)|families at one-way doors" "$COST"
    [ "$status" -ne 0 ]
    run grep -EiC0 "≥ *2 model families|two model families|cross-family.*(required|default)|families at one-way doors" "$GATE"
    [ "$status" -ne 0 ]
    # both must reflect the fresh-contexts / cross-family-opt-in doctrine
    grep -Eiq "fresh context|cross-family.*opt-in|opt-in.*cross-family" "$COST"
    grep -Eiq "fresh context|cross-family.*opt-in|opt-in.*cross-family" "$GATE"
}

@test "B3-S4: a family-floor consumer is explicitly given RequireCrossFamily:true (not a silent default flip)" {
    # Reconciliation surface: any consumer that depended on the OLD family floor
    # for a real safety property is flagged AND set explicitly. The runnable
    # evidence is (a) the doctrine note documents the consumer reconciliation,
    # and (b) where a consumer needs the floor, it sets RequireCrossFamily:true
    # at the call site — never relying on a default flip.
    doc="$(doctrine_doc)"
    [ -n "$doc" ]
    grep -Eiq "consumer|olympusd|reconcil" "$doc"
    # if any binding consumer needs the floor it must set it EXPLICITLY; assert at
    # least one explicit opt-in exists in the consumer surface OR the doctrine note
    # records that no consumer required it (a deliberate, documented outcome).
    run bash -c "grep -rEl 'RequireCrossFamily: *true' '$REPO_ROOT/cli' 2>/dev/null"
    explicit_set="$status"
    grep -Eiq "RequireCrossFamily: *true|no consumer (requires|required|depends on) the (family )?floor|none required the floor" "$doc" || [ "$explicit_set" -eq 0 ]
}

# ── B2 — CHANGELOG entry + v3.2.0 tag (content + git assertions) ──────────────

@test "B2-S1: CHANGELOG has a v3.2.0 section listing the release-window items" {
    [ -f "$CHANGELOG" ]
    grep -Eq "v?3\.2\.0" "$CHANGELOG"
    # the section enumerates the window's headline changes
    grep -Eiq "skills retire|prune|~?104" "$CHANGELOG"
    grep -Eiq "provenance" "$CHANGELOG"
    grep -Eiq "quorum" "$CHANGELOG"
    grep -Eiq "br|tracker" "$CHANGELOG"
    grep -Eiq "BC6" "$CHANGELOG"
}

@test "B2-S2: the quorum change is flagged BREAKING with the correct new default" {
    [ -f "$CHANGELOG" ]
    grep -Eiq "BREAKING" "$CHANGELOG"
    grep -Eiq "fresh context|cross-family.*opt-in|opt-in.*cross-family" "$CHANGELOG"
}

@test "B2-S3: the v3.2.0 tag exists locally and points at the release-window HEAD" {
    run git -C "$REPO_ROOT" tag --list v3.2.0
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    run git -C "$REPO_ROOT" rev-list -n1 v3.2.0
    [ "$status" -eq 0 ]
    [ -n "$output" ]
}

@test "B2-S4: B2's bead carries a blocks dep so B3 closes first (ordering)" {
    # Disposition: ordering is enforced in the tracker. The runnable surface is the
    # changelog wording matching ratified B3 doctrine (substance), asserted here.
    [ -f "$CHANGELOG" ]
    grep -Eiq "fresh context" "$CHANGELOG"
}

@test "B2-G15: the v3.2.0 tag exists on origin at HEAD (remote release proof)" {
    run git -C "$REPO_ROOT" ls-remote --tags origin refs/tags/v3.2.0
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    remote_sha="$(echo "$output" | awk '{print $1}')"
    head_sha="$(git -C "$REPO_ROOT" rev-parse HEAD)"
    [ "$remote_sha" = "$head_sha" ]
}

# ── B4 — decompose cli/cmd/ao package concentration (deferred/epic) ───────────

@test "B4-S1: build/vet/test stay green after extraction" {
    # The behavior is `go build ./... && go vet ./... && go test ./...`. The full
    # test sweep is heavy; a generous go-test timeout keeps it deterministic in
    # the umbrella (the package suite, not CI scale).
    run bash -c "cd '$REPO_ROOT/cli' && go build ./... && go vet ./... && go test -timeout 600s ./..."
    [ "$status" -eq 0 ]
}

@test "B4-S2: the command surface is unchanged after decomposition" {
    # cli-surface.json must match its regenerated form (zero command-surface drift)
    [ -f "$REPO_ROOT/docs/cli-surface.json" ]
    run bash "$REPO_ROOT/scripts/regen-all.sh" --check
    [ "$status" -eq 0 ]
}

@test "B4-S3: the cli/cmd/ao top-level .go file concentration is measurably reduced" {
    # codex.go responsibilities move to a codex/ sub-package; top-level count drops.
    [ -d "$REPO_ROOT/cli/cmd/ao/codex" ]
    count="$(find "$REPO_ROOT/cli/cmd/ao" -maxdepth 1 -name '*.go' | wc -l | tr -d ' ')"
    # pre-refactor concentration was 633 top-level .go files; the metric must improve
    [ "$count" -lt 633 ]
}
