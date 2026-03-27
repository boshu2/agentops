# Goals

The local DevOps layer for coding agents — skills, CLI, and knowledge flywheel that make every session smarter than the last.

## North Stars

- Skills work identically across Claude Code, Codex CLI, Cursor, and OpenCode
- Knowledge captured in one session is retrieved and applied in the next
- A new user goes from install to first validated workflow in under 5 minutes

## Anti Stars

- Product promises with no automated verification
- Goals that measure code metrics instead of user outcomes
- Quarantined tests that hide real regression risk

## Directives

### 1. Close the multi-runtime promise gap

README and PRODUCT.md promise skills work across 4 runtimes, but runtime-specific tests are quarantined (Claude Code, Codex, OpenCode all disabled in `tests/_quarantine/`). Only one cross-runtime test exists (`tests/codex/test-skill-cross-runtime.sh`). Ship at least 2 more runtime-specific smoke tests and promote them to CI.

**Steer:** increase (runtime coverage count)

### 2. Gate the install path

Three install scripts (`install.sh`, `install-codex.sh`, `install-opencode.sh`) have zero automated testing. A broken install is the fastest way to lose a user. Add install-path smoke tests that verify each script produces a working skill set.

**Steer:** increase (install scripts with smoke tests)

### 3. Resurrect quarantined E2E tests

8 test directories sit disabled in `tests/_quarantine/` — RPI pipeline, skill triggering, native teams, runtime-specific tests. Each represents a real user workflow with no regression protection. Triage each: fix and promote, or delete if obsolete.

**Steer:** decrease (quarantined test count)

### 4. Verify knowledge lifecycle end-to-end

The flywheel-compounding gate proves σρ > δ (escape velocity). But the full lifecycle — capture quality, injection correctness, citation in downstream work — has no gate. Add a gate that traces one learning from extraction through injection to retrieval.

**Steer:** increase (lifecycle stages gated)

### 5. Keep complexity regressions at zero

CC 20 ceiling was achieved. Gate enforces the threshold — the directive is to maintain zero violations and prevent future regressions via pre-commit checks.

**Steer:** decrease (functions exceeding CC 20)

### 6. Maintain competitive awareness

Competitive analysis docs (`docs/comparisons/vs-*.md`) must stay fresh. GSD, Compound Engineer, and sdd are actively iterating — stale analysis means blind spots. Refresh comparisons within 45 days of last update. `/evolve` picks this up automatically when other goals pass.

**Steer:** decrease (stale comparison doc count)

### 7. Enforce codex parity proactively

CI catches codex drift at push time, but 40% of fix commits in the March 2026 integration were codex parity issues caught too late. The PreToolUse hook warns during editing; the goal gate blocks push if drift exists.

**Steer:** decrease (codex parity findings count)

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| flywheel-compounding | `bash -c 'cd cli && go build -o /tmp/ao-fw-check ./cmd/ao && cd .. && /tmp/ao-fw-check flywheel status --json 2>/dev/null \| jq -e ".escape_velocity_compounding == true"'` | 8 | Knowledge flywheel above escape velocity (σρ > δ), a necessary but not sufficient condition for true compounding |
| flywheel-proof | `bash scripts/proof-run.sh` | 7 | Flywheel compounds across sessions (automated proof) |
| skill-frontmatter | `bash -c 'for f in skills/*/SKILL.md; do head -5 "$f" \| grep -q "^---" && head -10 "$f" \| grep -q "^name:" && head -10 "$f" \| grep -q "^description:" \|\| { echo FAIL:$f; exit 1; }; done'` | 6 | Every skill has valid YAML frontmatter |
| hook-preflight | `timeout 60 ./scripts/validate-hook-preflight.sh` | 6 | All hooks pass safety checks |
| go-cli-builds | `cd cli && go build -o /dev/null ./cmd/ao` | 8 | Go CLI compiles without errors |
| go-cli-tests | `cd cli && timeout 120 go test -race ./...` | 8 | All Go tests pass with race detector |
| go-vet-clean | `cd cli && go vet ./...` | 5 | No common bugs detected by vet |
| go-complexity-ceiling | `timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/ --threshold 20 && timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/internal/ --threshold 18` | 6 | No Go function exceeds CC thresholds (cli/: 20, cli/internal/: 18) |
| security-gate | `test -x scripts/security-gate.sh && timeout 60 bash tests/scripts/test-security-gate.sh` | 6 | Security toolchain gate is executable and passes |
| manifest-versions-match | `test "$(jq -r '.metadata.version' .claude-plugin/marketplace.json)" = "$(jq -r '.version' .claude-plugin/plugin.json)"` | 5 | Plugin and marketplace versions in sync |
| wiring-closure | `timeout 60 bash scripts/check-wiring-closure.sh` | 7 | All scripts, skills, and hooks referenced by registries exist |
| contract-compatibility | `timeout 60 bash scripts/check-contract-compatibility.sh` | 5 | Contract schemas and references exist on disk |
| release-cadence | `bash scripts/release-cadence-check.sh` | 3 | Release cadence policy enforced (warn <7d, block <1d) |
| goals-validate | `bash -c 'cd cli && go build -o /tmp/ao-goals-val ./cmd/ao && cd .. && /tmp/ao-goals-val goals validate --json 2>/dev/null \| jq -e ".valid == true"'` | 5 | GOALS.md parses and validates without structural errors |
| athena-freshness | `bash scripts/check-athena-health.sh` | 4 | Athena defrag report is fresh and stale learnings are low |
| athena-no-oscillation | `bash -c 'test -f .agents/defrag/latest.json && jq -e "(.oscillation.oscillating_goals // []) \| length == 0" .agents/defrag/latest.json'` | 4 | No evolve goals oscillating across consecutive cycles |
| competitive-freshness | `bash scripts/check-competitive-freshness.sh` | 3 | Competitive analysis docs updated within 45 days |
| codex-parity-drift | `bash scripts/check-codex-parity-drift.sh` | 5 | No codex parity findings from audit |
