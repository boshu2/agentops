# Goals

The operational layer for coding agents — repo-native bookkeeping, validation, primitives, and flows that make every session smarter than the last.

## North Stars

- The knowledge flywheel is the product — every session makes the next session smarter
- Skills work identically across Claude Code, Codex CLI, Cursor, and OpenCode
- Knowledge captured in one session is retrieved and applied in the next
- The flywheel runs autonomously between sessions (dream cycle), not just on-demand
- A new user goes from install to first validated flow in under 5 minutes

## Anti Stars

- Product promises with no automated verification
- Goals that measure code metrics instead of user outcomes
- Quarantined tests that hide real regression risk

## Directives

### 1. Close the multi-runtime promise gap

README and PRODUCT.md promise skills work across 4 runtimes. The current contract is tiered: Tier S structural/install proof must stay green in CI, Tier I live inventory proof may skip when external CLIs/auth are absent unless strict mode is enabled, and Tier E live execution proof remains opt-in/nightly. Keep the Tier S gates green for Claude Code, Codex, Cursor, and OpenCode, and expand Tier I/E only where the runtime can be provisioned reliably.

**Progress:** Tier S is active in CI through `tests/smoke-test.sh`: `tests/skills/test-runtime-claude-code-smoke.sh`, `tests/skills/test-runtime-codex-smoke.sh`, `tests/skills/test-runtime-cursor-smoke.sh`, and `tests/skills/test-runtime-opencode-smoke.sh`. `tests/scripts/test-headless-runtime-skills.sh` exercises the Claude/Codex headless validator contract with mocked runtimes, while `scripts/validate-headless-runtime-skills.sh` performs live Tier I inventory proof when local CLIs/auth are available. Remaining gap: live hosted-runtime execution proof is not a default CI gate.

**Steer:** increase (runtime coverage count)

### 2. Gate the install path

Three install scripts (`install.sh`, `install-codex.sh`, `install-opencode.sh`) have zero automated testing. A broken install is the fastest way to lose a user. Add install-path smoke tests that verify each script produces a working skill set.

**Progress:** `install-smoke` gate added (`tests/install/test-install-smoke.sh`, weight 5) — validates syntax and structure of all install scripts. Gate is active in CI. Runtime execution tests added: when a local `cli/bin/ao` binary exists, the gate now verifies `ao --version`, `ao help`, and that `flywheel`, `goals`, and `inject` subcommands are registered. Remaining gap: end-to-end install execution (running `scripts/install.sh` against a clean environment) requires a sandboxed CI environment with network access — documented as out-of-scope for local gate.

**Steer:** increase (install scripts with smoke tests)

### 3. Resurrect quarantined E2E tests

`tests/_quarantine/` currently has zero active quarantined suites. Keep it empty: newly disabled workflow tests must either be promoted back to CI, deleted as obsolete, or tracked as explicit follow-up work before they can remain quarantined.

**Steer:** decrease (quarantined test count)

### 4. Verify knowledge lifecycle end-to-end

The flywheel-compounding gate proves σρ > δ (escape velocity). But the full lifecycle — capture quality, injection correctness, citation in downstream work — has no gate. Add a gate that traces one learning from extraction through injection to retrieval.

**Progress:** `flywheel-lifecycle` gate now traces 5 stages: capture → retrieval → inject → round-trip → citation (`scripts/check-flywheel-lifecycle.sh`). Stage 5 (citation) checks for cross-citations between learnings, briefings directory population, and corpus density. Citation checks are soft-fail on sparse corpus (structurally valid but no accumulated sessions yet) — they hard-fail only if the corpus is populated and citations are structurally absent. Gate is active in CI.

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

### 8. Automate the dream cycle (nightly flywheel consolidation)

Today harvest/forge/inject are on-demand — an operator runs them when they remember to. Anthropic's "dream cycle" concept validates what we've known: consolidation should happen automatically between sessions. Ship a GitHub Action (or scheduled Claude task) that runs nightly: harvest new learnings from recent sessions, forge patterns from accumulated learnings, defrag stale knowledge, and report flywheel health. The dream cycle is what turns the flywheel from "useful when invoked" to "always compounding."

**Progress:** Implemented in nightly CI. `.github/workflows/nightly.yml` now runs a dedicated dream-cycle proof job (`harvest -> forge -> close-loop -> defrag -> metrics health`) against the checked-in knowledge corpus, uploads the full report artifact, and updates a rolling GitHub issue with a visible compounding summary.

**Steer:** increase (automated consolidation runs per week)

### 9. Build the pattern-to-skill pipeline (self-programming)

When the same pattern appears across 3+ sessions — a debugging technique, a validation sequence, a refactoring approach — the system should propose a new skill. Today skills are hand-authored. The next step is semi-automated: `/compile` or `/forge` detects recurring patterns, drafts a skill skeleton (SKILL.md + frontmatter), and presents it for human review before promotion. This is Anthropic's "Skillify" concept — compound growth without manual authoring.

**Progress:** Prototype implemented. `ao flywheel close-loop` now generates review-only draft skills under `.agents/skill-drafts/` when a pattern has evidence across 3+ session artifacts. The remaining gap is promotion polish: richer section synthesis, stronger tier heuristics, and a cleaner review/publish path from draft to shipped skill.

**Steer:** increase (auto-proposed skill drafts)

## Three-Gap Contract Proof Surface

AgentOps defines a three-gap contract ([context lifecycle](docs/context-lifecycle.md)) covering the failure modes that persist after prompt construction and agent routing. Every gate below maps to at least one gap. If a gap has no gate, it is an unproven promise.

| Gap | What fails without it | Proving gates | Coverage |
|-----|-----------------------|---------------|----------|
| **1. Judgment validation** — agents ship without risk context | Plans skip architecture fit; implementations pass happy path but miss edge cases | `hook-preflight`, `go-vet-clean`, `go-complexity-ceiling`, `security-gate`, `wiring-closure`, `contract-compatibility` | Mechanically enforced via hooks and static analysis; `/pre-mortem` and `/vibe` supply the non-mechanical judgment layer |
| **2. Durable learning** — solved problems recur | Same auth bug fixed Monday returns Wednesday; agents re-run dead-end investigations | `flywheel-compounding`, `flywheel-proof`, `compile-freshness`, `compile-no-oscillation` | Flywheel escape velocity proves compounding; Compile gates prove curation and freshness |
| **3. Loop closure** — completed work doesn't produce better next work | Sessions end with diffs but no extracted lessons; next session starts cold | `flywheel-proof`, `goals-validate`, `wiring-closure`, `release-cadence` | `flywheel-proof` traces capture-to-retrieval; `goals-validate` ensures findings become directives; `wiring-closure` proves registries stay connected |

**Design rule:** prefer current gates over new scripts unless a true gap is found. New gates are justified only when a gap row shows no proving gate.

**Canonical reference:** `docs/context-lifecycle.md` — evidence map and mechanism inventory for all three gaps.

The three-gap contract is satisfied when the mapped gates below remain green together. `ao goals measure` checks the current set on demand.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| flywheel-compounding | `bash -c 'cd cli && go build -o /tmp/ao-fw-check ./cmd/ao && cd .. && /tmp/ao-fw-check flywheel status --json 2>/dev/null \| jq -e ".escape_velocity_compounding == true"'` | 8 | Knowledge flywheel above escape velocity (σρ > δ), a necessary but not sufficient condition for true compounding |
| flywheel-proof | `bash scripts/proof-run.sh` | 7 | Flywheel compounds across sessions (automated proof) |
| skill-frontmatter | `bash -c 'for f in skills/*/SKILL.md; do head -5 "$f" \| grep -q "^---" && head -10 "$f" \| grep -q "^name:" && head -10 "$f" \| grep -q "^description:" \|\| { echo FAIL:$f; exit 1; }; done'` | 6 | Every skill has valid YAML frontmatter |
| hook-preflight | `timeout 60 ./scripts/validate-hook-preflight.sh` | 6 | All hooks pass safety checks |
| go-cli-builds | `cd cli && go build -o /dev/null ./cmd/ao` | 8 | Go CLI compiles without errors |
| go-cli-tests | `cd cli && timeout 240 go test -race ./...` | 8 | All Go tests pass with race detector |
| go-vet-clean | `cd cli && go vet ./...` | 5 | No common bugs detected by vet |
| go-complexity-ceiling | `timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/ --threshold 20 && timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/internal/ --threshold 18` | 6 | No Go function exceeds CC thresholds (cli/: 20, cli/internal/: 18) |
| security-gate | `test -x scripts/security-gate.sh && timeout 60 bash tests/scripts/test-security-gate.sh` | 6 | Security toolchain gate is executable and passes |
| manifest-versions-match | `test "$(jq -r '.metadata.version' .claude-plugin/marketplace.json)" = "$(jq -r '.version' .claude-plugin/plugin.json)"` | 5 | Plugin and marketplace versions in sync |
| wiring-closure | `timeout 60 bash scripts/check-wiring-closure.sh` | 7 | All scripts, skills, and hooks referenced by registries exist |
| contract-compatibility | `timeout 60 bash scripts/check-contract-compatibility.sh` | 5 | Contract schemas and references exist on disk |
| goals-validate | `bash -c 'cd cli && go build -o /tmp/ao-goals-val ./cmd/ao && cd .. && /tmp/ao-goals-val goals validate --json 2>/dev/null \| jq -e ".valid == true"'` | 5 | GOALS.md parses and validates without structural errors |
| compile-freshness | `bash scripts/check-compile-health.sh` | 4 | Compile defrag report is fresh and stale learnings are low |
| compile-no-oscillation | `bash scripts/check-compile-oscillation.sh` | 4 | No evolve goals oscillating across consecutive cycles |
| competitive-freshness | `bash scripts/check-competitive-freshness.sh` | 3 | Competitive analysis docs updated within 45 days |
| codex-parity-drift | `bash scripts/check-codex-parity-drift.sh` | 5 | No codex parity findings from audit |
| install-smoke | `timeout 30 bash tests/install/test-install-smoke.sh` | 5 | Install scripts pass syntax and structure validation |
| flywheel-lifecycle | `timeout 30 bash scripts/check-flywheel-lifecycle.sh` | 6 | Knowledge lifecycle traces capture → index → inject → retrieval |
