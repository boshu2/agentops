# Goals

A wiki for your agents — repo-native, version-controlled, mechanically maintained — that turns your context into the durable moat under any model or harness.

## North Stars

<!-- agentops:claim:AOP-CLAIM-GOALS-DREAM-VALIDATED -->
- The knowledge flywheel is the product — every session makes the next session smarter
- The wiki maintains itself: every session contributes to `.agents/` by default
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

**Progress:** cli/ threshold (20) is green. cli/internal/ threshold (18) is green. Previously `validateRoutingLaneGates` was CC 19; refactored into `validateYieldGate` and `validateLaneAuthority` helpers (2026-05-04).

**Steer:** decrease (functions exceeding CC 20)

### 6. Maintain competitive awareness

Competitive analysis docs (`docs/comparisons/vs-*.md` and `docs/comparisons/competitive-radar.md`) must stay fresh. GSD, Compound Engineer, and sdd are actively iterating — stale analysis means blind spots. Refresh comparisons within 45 days of last update. `/evolve` picks this up automatically when other goals pass.

**Steer:** decrease (stale comparison doc count)

### 7. Enforce codex parity proactively

CI catches codex drift at push time, but 40% of fix commits in the March 2026 integration were codex parity issues caught too late. The PreToolUse hook warns during editing; the goal gate blocks push if drift exists.

**Steer:** decrease (codex parity findings count)

### 8. Automate the dream cycle (nightly flywheel consolidation)

Today harvest/forge/inject are on-demand — an operator runs them when they remember to. Anthropic's "dream cycle" concept validates what we've known: consolidation should happen automatically between sessions. Ship a GitHub Action (or scheduled Claude task) that runs nightly: harvest new learnings from recent sessions, forge patterns from accumulated learnings, defrag stale knowledge, and report flywheel health. The dream cycle is what turns the flywheel from "useful when invoked" to "always compounding."

**Progress:** Implemented in nightly CI. `.github/workflows/nightly.yml` now runs a dedicated dream-cycle proof job (`harvest -> forge -> close-loop -> defrag -> metrics health`) against the checked-in knowledge corpus, uploads the full report artifact, and updates a rolling GitHub issue with a visible compounding summary. v1.0+: end-user repos can run the same loop locally via `ao daemon run --schedule-file .agents/schedule.yaml`. Substrate via soc-8inr (recurrence + JobTypeLLMWikiLoop + scheduling primitives, shipped 2026-05-01); operator-facing dogfood via soc-hxnr (stock .agents/schedule.yaml.example + ao init --with-schedule + operator runtime templates).

**Steer:** increase (automated consolidation runs per week)

### 9. Build the pattern-to-skill pipeline (self-programming)

When the same pattern appears across 3+ sessions — a debugging technique, a validation sequence, a refactoring approach — the system should propose a new skill. Today skills are hand-authored. The next step is semi-automated: `/compile` or `/forge` detects recurring patterns, drafts a skill skeleton (SKILL.md + frontmatter), and presents it for human review before promotion. This is Anthropic's "Skillify" concept — compound growth without manual authoring.

**Progress:** Prototype implemented. `ao flywheel close-loop` now generates review-only draft skills under `.agents/skill-drafts/` when a pattern has evidence across 3+ session artifacts. The remaining gap is promotion polish: richer section synthesis, stronger tier heuristics, and a cleaner review/publish path from draft to shipped skill.

**Steer:** increase (auto-proposed skill drafts)

### 10. Measure skill value through real-task evaluation

The existing eval suites are CI canaries (contract checks). None answers "did this skill change make agents better?" Ship a behavioral eval system with a known-good workbench project, task definitions with golden solutions, and scoring scripts that measure correctness, safety, and process adherence. The eval engine already supports A/B comparison via `--baseline-mode=both` and statistical verdict — the gap is eval content, not infrastructure.

<!-- agentops:claim:AOP-CLAIM-GOALS-EVAL-WORKBENCH -->
**Progress:** Workbench built: 3 components (Go CLI, Python FastAPI, DevOps scripts), 12 tasks with setup/score scripts, behavioral eval suite (`workbench-behavioral-v1`) with 12 cases covering bug-fix, feature implementation, security, refactoring, test-writing, and edge-case handling. `make -C evals/workbench verify` passes golden (12/12) and broken detection (12/12). A/B comparison via DeltaScorecard validated. Agent harness script with industry-proven eval patterns shipped. `eval-skill-delta` CI gate added to `validate.yml` (structural, runs on eval file changes). `--two-pass` mode added to pre-push head gate for local skill-delta validation. Remaining gap: expanding eval-skill-delta from structural-only to a default blocking gate with full skill-on vs skill-off execution across the workbench.

**Steer:** increase (behavioral eval tasks with scoring scripts)

### 11. Durability of the corpus across runtime cleanup

On 2026-05-07, routine maintenance wiped most of `.agents/` runtime subdirs (only `.agents/nightly/` is git-tracked); a fresh `scripts/corpus-stats.sh` returns near-zero counts even though the 2026-05-04 stable snapshot recorded ~1,842 learnings, ~186 patterns, ~80 planning rules, and ~3,867 cited decisions. The dogfood receipts claim — and the broader "corpus is the moat" positioning — depends on that asset being durable across cleanup, machine moves, and reinstalls. This directive tracks the design and implementation of a snapshot/restore mechanism: scheduled snapshots of `.agents/` runtime state to durable storage, restore tooling that can rehydrate a fresh checkout, and a freshness/coverage gate so degradation is visible before the receipts go stale. Tracked under bd issue soc-rv5p.

**Steer:** increase (snapshots / restore mechanism)

**Tags:** corpus-state

## Three-Gap Contract Proof Surface

AgentOps defines a three-gap contract ([context lifecycle](docs/context-lifecycle.md)) covering the failure modes that persist after prompt construction and agent routing. Honesty rule: gates only appear in the **Currently enforcing** column when they (a) run in CI/pre-push/release automation AND (b) reliably go green in single-session work. Gates that are declared but not yet enforced — usually because they measure cross-session or corpus-level state — sit in the **Roadmap** column.

| Gap | What fails without it | Currently enforcing | Roadmap (declared, not yet enforced) |
|-----|-----------------------|---------------------|---------------------------------------|
| **1. Judgment validation** — agents ship without risk context | Plans skip architecture fit; implementations pass happy path but miss edge cases | `hook-preflight`, `go-vet-clean`, `go-complexity-ceiling`, `security-gate`, `contract-compatibility`; `/pre-mortem` and `/vibe` supply the non-mechanical judgment layer | — |
| **2. Durable learning** — solved problems recur | Same auth bug fixed Monday returns Wednesday; agents re-run dead-end investigations | `compile-no-oscillation` (defrag stability), `flywheel-proof` (cross-session evidence, soc-45sg.2), `flywheel-compounding-snapshot` (corpus-state evidence, soc-45sg.1) | `flywheel-compounding` (live long-cycle), `compile-freshness` (runtime-artifact dependency) |
| **3. Loop closure** — completed work doesn't produce better next work | Sessions end with diffs but no extracted lessons; next session starts cold | `release-cadence` (where wired), `goals-validate` (soc-45sg.4), `flywheel-proof` (soc-45sg.2), `wiring-closure` (soc-45sg.5) | — |

**Design rule:** prefer current gates over new scripts unless a true gap is found. The Roadmap column is itself a tracked gap — moving a gate left is the work, not adding new gates.

**Canonical reference:** `docs/context-lifecycle.md` — evidence map and mechanism inventory for all three gaps.

**Today's enforcement state:** Gap 1 is mechanically enforced. Gaps 2 and 3 are partial: scripts exist (`scripts/proof-run.sh`, `scripts/check-flywheel-compounding.sh`, `scripts/check-wiring-closure.sh`, etc.) but are not invoked from automation that blocks merges. `flywheel-compounding` is explicitly long-cycle by design — its green path requires multi-session corpus growth, not a single push. The right way to read this table: PRODUCT.md and GOALS.md are allowed to run ahead of the repo because they are desired-state specifications. The Current Proof column is actual state; the Roadmap column is the reconcile queue that `/evolve`, dream, validation gates, and follow-up work drive toward closure.

`ao goals measure` runs every declared gate on demand and is the canonical way to inspect current state, including roadmap gates.

## Gates

The optional `Tags` column lets a gate declare classification metadata that
flows through to `ao goals measure --json` (each measurement carries a `tags`
field). The `long-cycle` and `corpus-state` tags mark gates whose green path
depends on multi-session corpus growth rather than the current commit, so
operator tooling (e.g. /evolve selection) can distinguish "code-actionable"
failures from corpus-bound ones without lowering weights or removing the gate.
The `runtime-artifact` tag marks gates whose green path requires a gitignored
artifact produced by a separate run (e.g. `ao defrag` writing
`.agents/defrag/latest.json`); such flips do not propagate across environments.

| ID | Check | Weight | Description | Tags |
|----|-------|--------|-------------|------|
| flywheel-compounding | `bash scripts/check-flywheel-compounding.sh` | 3 | Knowledge flywheel above escape velocity (σρ > δ); requires multi-session citation activity, not movable by single-session automation — see `.agents/findings/f-2026-04-29-001.md` | long-cycle, corpus-state |
| flywheel-compounding-snapshot | `bash scripts/check-flywheel-compounding-snapshot.sh` | 5 | CI-readable corpus-state evidence: validates `docs/releases/flywheel-compounding-snapshot.json` exists, is < 14 days old, and asserts `escape_velocity_compounding=true`. Operator refresh: `bash scripts/snapshot-flywheel-compounding.sh`. Closes G1 by making the long-cycle gate CI-enforceable without running multi-session work. |  |
| dream-end-user-coverage | `bash scripts/check-schedule-example.sh` | 3 | Stock .agents/schedule.yaml.example exists, parses, and uses real-bodied job types (dream.run, wiki.forge). Closes Directive #8 end-user-repo gap. |  |
| flywheel-proof | `bash scripts/proof-run.sh` | 7 | Flywheel compounds across sessions (automated proof) |  |
| skill-frontmatter | `bash -c 'for f in skills/*/SKILL.md; do head -5 "$f" \| grep -q "^---" && head -10 "$f" \| grep -q "^name:" && head -10 "$f" \| grep -q "^description:" \|\| { echo FAIL:$f; exit 1; }; done'` | 6 | Every skill has valid YAML frontmatter |  |
| hook-preflight | `timeout 60 ./scripts/validate-hook-preflight.sh` | 6 | All hooks pass safety checks |  |
| go-cli-builds | `cd cli && go build -o /dev/null ./cmd/ao` | 8 | Go CLI compiles without errors |  |
| go-cli-tests | `cd cli && timeout 240 go test -race ./...` | 8 | All Go tests pass with race detector |  |
| go-vet-clean | `cd cli && go vet ./...` | 5 | No common bugs detected by vet |  |
| go-complexity-ceiling | `timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/ --threshold 20 && timeout 60 bash scripts/check-go-absolute-complexity.sh --dir cli/internal/ --threshold 18` | 6 | No Go function exceeds CC thresholds (cli/: 20, cli/internal/: 18) |  |
| security-gate | `test -x scripts/security-gate.sh && timeout 60 bash tests/scripts/test-security-gate.sh` | 6 | Security toolchain gate is executable and passes |  |
| manifest-versions-match | `test "$(jq -r '.metadata.version' .claude-plugin/marketplace.json)" = "$(jq -r '.version' .claude-plugin/plugin.json)"` | 5 | Plugin and marketplace versions in sync |  |
| wiring-closure | `timeout 60 bash scripts/check-wiring-closure.sh` | 7 | All scripts, skills, and hooks referenced by registries exist |  |
| contract-compatibility | `timeout 60 bash scripts/check-contract-compatibility.sh` | 5 | Contract schemas and references exist on disk |  |
| goals-validate | `bash -c 'cd cli && go build -o /tmp/ao-goals-val ./cmd/ao && cd .. && /tmp/ao-goals-val goals validate --json 2>/dev/null \| jq -e ".valid == true"'` | 5 | GOALS.md parses and validates without structural errors |  |
| compile-freshness | `bash scripts/check-compile-health.sh` | 4 | Compile defrag report is fresh and stale learnings are low | runtime-artifact |
| compile-no-oscillation | `bash scripts/check-compile-oscillation.sh` | 4 | No evolve goals oscillating across consecutive cycles | runtime-artifact |
| competitive-freshness | `bash scripts/check-competitive-freshness.sh` | 3 | Competitive analysis docs updated within 45 days |  |
| codex-parity-drift | `bash scripts/check-codex-parity-drift.sh` | 5 | No codex parity findings from audit |  |
| quarantine-empty | `bash scripts/check-quarantine-empty.sh` | 4 | tests/_quarantine/ holds zero `.sh`/`.bats` suites (Directive D3). Single-cycle override: set `ALLOW_QUARANTINE=1` when intentionally parking a flaky suite. |  |
| corpus-freshness | `bash scripts/check-corpus-freshness.sh` | 4 | Newest corpus snapshot under `$AGENTOPS_CORPUS_SNAPSHOT_DIR` (default `~/.agentops/corpus-snapshots/`) is within 7 days. Skips cleanly when no snapshots exist. Override: `AGENTOPS_CORPUS_FRESHNESS_SKIP=1`. Companion: `ao corpus snapshot` / `ao corpus restore` (Directive D11). |  |
| factory-yield-ledger | `bash scripts/check-factory-yield-ledger.sh` | 4 | Factory yield-ledger contract enforced: schema + example parse, 25 required correlation+yield fields present, event_type=factory.yield_observation, schema_version=1. Pair to enforced factory-claim-ledger (A2 audit follow-up). |  |
| finding-registry | `bash scripts/check-finding-registry.sh` | 4 | Finding-registry contract enforced: schema is valid JSON, required-field list cross-checks with contract doc, canonical path documented, and live `.agents/findings/registry.jsonl` lines (when present) validate against required fields. CI runs structural-only (no registry); operator boxes also validate live lines (A2 audit follow-up). |  |
| factory-admission | `bash scripts/check-factory-admission.sh` | 4 | Factory-admission contract enforced: wraps the existing `tests/scripts/test-factory-admission-contracts.py` (Python+jsonschema validator) as a blocking gate. Validates both `valid-work-order.json` and `valid-admission-decision.json` fixtures against `schemas/factory-admission.v1.schema.json`. A2 audit follow-up — completes the factory ledger triad (admission/claim/yield). |  |
| contracts-structural-floor | `bash scripts/check-contracts-structural-floor.sh` | 4 | Every `docs/contracts/*.md` (38 contracts) meets the structural floor: top-level # heading, cataloged in `docs/documentation-index.md`, body >= 200 bytes, paired schema (if any) is valid JSON. Floor under the per-contract strong gates (factory-claim, factory-yield, factory-admission, finding-registry, etc.). A2 audit follow-up: covers all 33 partial+doc-only contracts at minimum enforcement level. |  |
| three-gap-supergate | `bash scripts/check-three-gap-supergate.sh --gap=all` | 5 | Three-Gap super-gates (TG1 council-coverage / TG2 durable-learning / TG3 loop-closure) emit unified status. Composes existing gates (flywheel-compounding-snapshot, flywheel-proof, compile-health, goals-validate, wiring-closure) so each Gap's closure status is one command instead of N. Operator surface: `--gap=council-coverage|durable-learning|loop-closure|all`. |  |
| install-smoke | `timeout 30 bash tests/install/test-install-smoke.sh` | 5 | Install scripts pass syntax and structure validation |  |
| flywheel-lifecycle | `timeout 30 bash scripts/check-flywheel-lifecycle.sh` | 6 | Knowledge lifecycle traces capture → index → inject → retrieval |  |
| eval-workbench-verify | `timeout 60 bash scripts/check-eval-workbench.sh` | 6 | Behavioral eval workbench golden state, task scoring, and suite structure verified |  |
| state-path-resolver-coverage | `bash scripts/check-paths-resolver-coverage.sh` | 3 | Tracks executable-code sites that still hardcode `.agents/` paths instead of sourcing the canonical resolver (lib/ao-paths.sh / cli/internal/paths from soc-irg1.1). Warn-only initially per warn-then-fail-ratchet pattern; flip to blocking is a separate follow-up issue under epic soc-irg1 after 2 weeks of baseline data. See `.agents/patterns/2026-05-01-state-path-resolver.md`. | warn-only |
