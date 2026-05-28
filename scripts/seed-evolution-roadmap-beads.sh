#!/usr/bin/env bash
# Seed evolution road-map beads from docs/plans/2026-05-11-evolution-roadmap.md.
# Idempotent: skips if a bead with the same title already exists.
# Usage:
#   bash scripts/seed-evolution-roadmap-beads.sh           # create
#   bash scripts/seed-evolution-roadmap-beads.sh --dry-run # preview only
set -euo pipefail

DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

ROADMAP="docs/plans/2026-05-11-evolution-roadmap.md"

# bd_create_if_missing TITLE DESCRIPTION TYPE PRIORITY LABELS [PARENT_ID]
bd_create_if_missing() {
  local title="$1"
  local desc="$2"
  local type="$3"
  local priority="$4"
  local labels="$5"
  local parent="${6:-}"

  # Idempotency: check if an open issue with the exact title exists
  if bd list --status=open --limit 0 --json 2>/dev/null | jq -e --arg t "$title" '.[] | select(.title == $t)' >/dev/null 2>&1; then
    echo "skip: $title (exists)"
    return 0
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "would-create: [$type/P$priority] $title (parent=${parent:-none})"
    return 0
  fi

  local cmd=(bd create
    --title="$title"
    --description="$desc"
    --type="$type"
    --priority="$priority"
    --labels="$labels"
  )
  [[ -n "$parent" ]] && cmd+=(--parent="$parent")

  local id
  id="$("${cmd[@]}" --json 2>/dev/null | jq -r '.id // empty')"
  if [[ -n "$id" ]]; then
    echo "created: $id $title"
    eval "$2_ID='$id'" 2>/dev/null || true
  else
    echo "FAILED: $title"
    return 1
  fi
}

# Simpler: capture created ID for parent linkage
bd_mk() {
  # Args: VAR TITLE DESC TYPE PRIORITY LABELS [PARENT]
  local var="$1"
  shift
  local title="$1" desc="$2" type="$3" priority="$4" labels="$5" parent="${6:-}"

  if existing="$(bd list --status=open --limit 0 --json 2>/dev/null | jq -r --arg t "$title" '.[] | select(.title == $t) | .id' | head -1)"; then
    if [[ -n "$existing" ]]; then
      echo "skip: $existing $title"
      printf -v "$var" '%s' "$existing"
      return 0
    fi
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "would-create: [$type/P$priority] $title"
    printf -v "$var" '%s' "dry-run-id"
    return 0
  fi

  local cmd=(bd create
    --title="$title"
    --description="$desc"
    --type="$type"
    --priority="$priority"
    --labels="$labels"
  )
  [[ -n "$parent" ]] && cmd+=(--parent="$parent")

  local out id
  out="$("${cmd[@]}" --json 2>/dev/null)"
  id="$(echo "$out" | jq -r '.id // empty')"
  if [[ -z "$id" ]]; then
    echo "FAILED: $title"
    echo "$out" >&2
    return 1
  fi
  echo "created: $id $title"
  printf -v "$var" '%s' "$id"
}

ROAD_REF="See $ROADMAP"

# =========================================================================
# EPIC E1: Directive Closure
# =========================================================================
bd_mk E1 \
  "[epic] Evolution E1: Close GOALS.md directive gaps (11 directives)" \
  "Each of the 11 directives in GOALS.md has a Progress line. Where progress is incomplete, a child bead closes the gap. $ROAD_REF (section E1)." \
  task 1 "evolution-roadmap,directives"

bd_mk D1 \
  "D1: Multi-runtime live execution proof (Tier E)" \
  "Tier S structural is green; Tier E live execution is not a default CI gate. Either build CI lanes that exercise real Claude/Codex/Cursor/OpenCode runtimes, OR document Tier E as opt-in in docs/contracts/multi-runtime-tier-charter.md. Acceptance: explicit charter doc OR tests/skills/test-runtime-*-live.sh runs in CI." \
  feature 1 "evolution-roadmap,directive,multi-runtime" "$E1"

bd_mk D2 \
  "D2: End-to-end install execution in sandboxed CI" \
  "tests/install/test-install-smoke.sh validates syntax/structure but real install execution against a clean env is out-of-scope. Build .github/workflows/install-e2e.yml that runs install.sh against ubuntu+macos containers and verifies ao --version post-install." \
  feature 1 "evolution-roadmap,directive,install" "$E1"

bd_mk D3 \
  "D3: Quarantine-empty enforcement gate" \
  "tests/_quarantine/ currently has zero suites. Add a goals-validate gate that fails when find tests/_quarantine -name '*.sh' -o -name '*.bats' | wc -l > 0. Weight 4. Acceptance: new gate row in GOALS.md, blocks push when quarantine populated without explicit override label." \
  feature 1 "evolution-roadmap,directive,gates" "$E1"

bd_mk D4 \
  "D4: Flywheel-lifecycle citation hard-fail mode" \
  "scripts/check-flywheel-lifecycle.sh Stage 5 (citation) is soft-fail on sparse corpus. Add --strict flag that hard-fails when corpus has >= 100 learnings AND citation density < threshold. Wire as opt-in initially." \
  feature 2 "evolution-roadmap,directive,flywheel" "$E1"

bd_mk D5 \
  "D5: Complexity regression ratchet to CC 18 for new code" \
  "CC 20 ceiling is green. Add a pre-commit-only stricter threshold (CC 18) for new functions in cli/internal/. Existing functions stay grandfathered. Acceptance: hooks/go-complexity-precommit.sh accepts a --new-code-threshold=18 flag." \
  feature 2 "evolution-roadmap,directive,complexity" "$E1"

bd_mk D6 \
  "D6: Competitive freshness sweep (docs/comparisons)" \
  "scripts/check-competitive-freshness.sh enforces 45-day window. Audit current docs/comparisons/vs-*.md last_reviewed dates, refresh any drifting, and ensure the gate is currently green." \
  task 2 "evolution-roadmap,directive,docs" "$E1"

bd_mk D7 \
  "D7: Codex parity drift to zero" \
  "scripts/check-codex-parity-drift.sh exists. Run it, classify findings, resolve each. Acceptance: bash scripts/check-codex-parity-drift.sh returns 0 findings; gate is green in CI without any --warn-only escape." \
  task 1 "evolution-roadmap,directive,codex-parity" "$E1"

bd_mk D8 \
  "D8: Dream end-user dogfood validation" \
  ".agents/schedule.yaml.example exists. Add tests/install/test-dream-dogfood.sh that runs ao init --with-schedule in a temp dir and verifies the resulting .agents/schedule.yaml parses + has real-bodied job types (dream.run, wiki.forge, not stub bodies)." \
  feature 2 "evolution-roadmap,directive,dream" "$E1"

bd_mk D9 \
  "D9: Pattern-to-skill synthesis (v2)" \
  "Detection layer is v1 (ao flywheel close-loop drafts skills under .agents/skill-drafts/). Synthesis v2 writes full SKILL.md bodies (not just frontmatter). Acceptance: a pattern with 3+ session evidence produces a draft skill that passes skill-frontmatter gate AND skill-lint dry-run." \
  feature 2 "evolution-roadmap,directive,skills,synthesis" "$E1"

bd_mk D10 \
  "D10: Behavioral eval as default blocking gate" \
  "Workbench + A/B + scoring exist. eval-skill-delta CI gate is structural-only. Upgrade eval-workbench-verify to also fail when make -C evals/workbench head-to-head produces a regression delta. Acceptance: PR that introduces a skill regression has its CI fail with a delta scorecard artifact." \
  feature 1 "evolution-roadmap,directive,eval" "$E1"

bd_mk D11 \
  "D11: Corpus durability snapshot/restore (soc-rv5p)" \
  "Routine cleanup wipes most of .agents/. Build ao corpus snapshot writing to configurable durable path, ao corpus restore rehydrating from latest, and a corpus-freshness gate firing if snapshot > 7 days old. Cross-link to existing bd soc-rv5p." \
  feature 1 "evolution-roadmap,directive,corpus,durability" "$E1"

# =========================================================================
# EPIC E2: Roadmap Gate Promotion
# =========================================================================
bd_mk E2 \
  "[epic] Evolution E2: Promote Roadmap gates to CI-blocking" \
  "GOALS.md three-gap contract surface lists 5 gates as 'Roadmap (declared, not yet enforced)'. Each child bead moves one gate left. $ROAD_REF (section E2)." \
  task 1 "evolution-roadmap,gates"

bd_mk G1 \
  "G1: Make flywheel-compounding CI-blocking via corpus-state snapshot" \
  "Gate is long-cycle, corpus-state. Design a corpus-state evidence snapshot in .agents/proof/flywheel-compounding-<date>.json that CI can validate without running multi-session work. Acceptance: gate moves from Roadmap to Currently enforcing column with documented snapshot protocol." \
  feature 1 "evolution-roadmap,gates,flywheel" "$E2"

bd_mk G2 \
  "G2: Wire flywheel-proof gate as CI-blocking" \
  "scripts/proof-run.sh exists but is not invoked from blocking automation. Wire into .github/workflows/validate.yml (and pre-push if cheap enough). Acceptance: every push to main runs flywheel-proof; failure blocks merge." \
  feature 1 "evolution-roadmap,gates,flywheel" "$E2"

bd_mk G3 \
  "G3: Make compile-freshness CI-blocking via runtime-artifact mode" \
  "Gate depends on .agents/defrag/latest.json. Design either CI step that generates the artifact, or stages a pre-computed one with hash check. Acceptance: gate no longer skipped in CI." \
  feature 2 "evolution-roadmap,gates,compile" "$E2"

bd_mk G4 \
  "G4: Wire goals-validate as CI-blocking" \
  "goals-validate runs ao goals validate --json | jq -e '.valid == true' but is currently not blocking. Wire into pre-push and validate.yml. Acceptance: any push that breaks GOALS.md validity is blocked at gate." \
  feature 1 "evolution-roadmap,gates" "$E2"

bd_mk G5 \
  "G5: Wire wiring-closure as CI-blocking" \
  "scripts/check-wiring-closure.sh exists. Wire as blocking in CI. Acceptance: orphan scripts/skills/hooks block push." \
  feature 1 "evolution-roadmap,gates" "$E2"

# =========================================================================
# EPIC E3: Known Product Gap Closure
# =========================================================================
bd_mk E3 \
  "[epic] Evolution E3: Close PRODUCT.md Known Product Gaps (11 gaps)" \
  "PRODUCT.md 'Known Product Gaps' table enumerates 11 gaps with current status. Each child bead drives one gap toward closure or explicit 'won't fix' disposition. $ROAD_REF (section E3)." \
  task 1 "evolution-roadmap,product-gaps"

bd_mk PG1 \
  "PG1: First-value path 5-minute install→validated-flow journey" \
  "PRODUCT.md gap: first-value path too diffuse for 3.0 PMF wedge. Build measurable 5-minute journey: install → first /rpi → validated artifact. Surface: README quickstart, ao quickstart CLI polish, install UX, first /rpi experience. Acceptance: tests/install/test-five-minute-journey.sh measures end-to-end time + artifact existence." \
  feature 1 "evolution-roadmap,product-gap,onboarding" "$E3"

bd_mk PG2 \
  "PG2: 3.0 PMF scenario exported evidence (soc-m6v5.8)" \
  "PMF scenario spec exists in soc-m6v5.8 but no exported proof yet. Define scenario, control path, run, export to docs/releases/v3.0/pmf-scenario.md or evals/workbench/results/. Public launch claims about PMF stay gated until this lands." \
  feature 1 "evolution-roadmap,product-gap,release,pmf" "$E3"

bd_mk PG3 \
  "PG3: /validate + /curate release-train consolidation (soc-m6v5.9)" \
  "Resolve epic soc-m6v5.9 (AgentOps 3.0 polished release train). Skill-count, registry, codex artifact gates must pass. Cross-link to existing epic." \
  feature 1 "evolution-roadmap,product-gap,release" "$E3"

bd_mk PG4 \
  "PG4: Public launch claims need exported proof under docs/releases/" \
  "Audit all AOP-CLAIM-* markers in README + landing pages, link each to docs/releases/<version>/<claim-id>.md evidence file. Local .agents/ notes are not enough for public claims. Cross-link to A1 audit." \
  task 1 "evolution-roadmap,product-gap,claims,evidence" "$E3"

bd_mk PG5 \
  "PG5: Dream full-loop autonomy" \
  "/dream + ao overnight + nightly.yml exist. Remaining work: full-loop autonomy without operator intervention, calibration, onboarding polish. Acceptance: a scheduled dream run executes harvest → forge → close-loop → defrag → report end-to-end with no manual steps." \
  feature 1 "evolution-roadmap,product-gap,dream" "$E3"

bd_mk PG8 \
  "PG8: Worker context packets carry prevention/finding info" \
  "Workers spawned by /crank should receive cited learnings + planning rules + finding registry, not just spec text. Audit ao context assemble output for worker phases, add prevention/finding sections to worker packets. Acceptance: a worker packet for an issue with related findings has those findings inline." \
  feature 1 "evolution-roadmap,product-gap,workers,context" "$E3"

bd_mk PG10 \
  "PG10: High-assurance profile control mapping" \
  "Extend docs/assurance-profile.md with redaction, evidence export, supply-chain inputs, program-specific control mapping. Goal: a constrained-environment operator can read assurance-profile.md and identify which controls AgentOps satisfies vs which need program-specific work." \
  task 2 "evolution-roadmap,product-gap,assurance" "$E3"

bd_mk PG11 \
  "PG11: Context-compiler messaging sweep" \
  "CDLC framing landed in Mission/Strategic Bet/README/mkdocs hero. Remaining: downstream comparison docs and skill-page intros still use older framing. Sweep docs/comparisons/*.md and skills/*/SKILL.md intros for consistency." \
  task 2 "evolution-roadmap,product-gap,messaging" "$E3"

# =========================================================================
# EPIC E4: Four-Layer Polish
# =========================================================================
bd_mk E4 \
  "[epic] Evolution E4: Polish one capability per product layer" \
  "Each of the four PRODUCT.md layers (Bookkeeping, Context Compiler, Validation Gates, Knowledge Flywheel) has one highest-impact missing capability. Four child beads close them. $ROAD_REF (section E4)." \
  task 1 "evolution-roadmap,four-layer"

bd_mk L1 \
  "L1: Improve citation signal-to-noise via follow_up_action field" \
  "Citation log (.agents/ao/citations.jsonl) has ~3,867 entries but utility scoring is weak. Differentiate cited-then-followed (agent acted on cite) from cited-then-ignored. Add follow_up_action field to citation events; weight in retrieval scoring." \
  feature 2 "evolution-roadmap,layer-bookkeeping,citations" "$E4"

bd_mk L2 \
  "L2: Phase-scoped context assembly test coverage" \
  "PRODUCT.md claims phase-scoped context packets but no test verifies ao context assemble produces different packets per RPI phase. Add tests/scripts/test-context-phase-scoping.bats that asserts research vs implement vs validate packets differ in expected ways." \
  task 2 "evolution-roadmap,layer-compiler,tests" "$E4"

bd_mk L3 \
  "L3: Council planted-bug detection fixture test" \
  "No test exists for council judges' detection rate against known-bad inputs. Build tests/council/test-planted-bug-detection.bats with N planted bugs in fixtures; council must catch >= floor%. Acceptance: floor configurable, currently set to 70%." \
  feature 2 "evolution-roadmap,layer-gates,council,tests" "$E4"

bd_mk L4 \
  "L4: ao flywheel dashboard (operator-facing trend view)" \
  "Dream cycle reports compounding metrics but no operator-facing dashboard shows trends. Build ao flywheel dashboard: single-screen markdown view of sigma-rho, delta, citation density, learning count over time." \
  feature 2 "evolution-roadmap,layer-flywheel,dashboard" "$E4"

# =========================================================================
# EPIC E5: Three-Gap Contract Surface integration
# =========================================================================
bd_mk E5 \
  "[epic] Evolution E5: Three-gap contract surface super-gates" \
  "GOALS.md three-gap table separates 'Currently enforcing' from 'Roadmap'. Each of the three gaps gets a super-gate that combines its component gates. $ROAD_REF (section E5)." \
  task 2 "evolution-roadmap,three-gap-contract"

bd_mk TG1 \
  "TG1: Gap-1 council-coverage super-gate" \
  "Add council-coverage gate that verifies every PR-bound commit has either a /pre-mortem or /vibe verdict in .agents/council/. Acceptance: new gate in GOALS.md, blocks PR merge if either verdict is missing for any commit in the PR diff." \
  feature 2 "evolution-roadmap,three-gap-contract,council" "$E5"

bd_mk TG2 \
  "TG2: Gap-2 durable-learning super-gate" \
  "Combine flywheel-compounding (G1), flywheel-proof (G2), compile-freshness (G3) into a single durable-learning super-gate that surfaces gap-2 closure status. Acceptance: single ao goals measure --gap=durable-learning command emits a unified PASS/WARN/FAIL." \
  feature 2 "evolution-roadmap,three-gap-contract,flywheel" "$E5"

bd_mk TG3 \
  "TG3: Gap-3 loop-closure super-gate" \
  "Combine release-cadence + flywheel-proof (G2) + goals-validate (G4) + wiring-closure (G5) into a loop-closure super-gate. Acceptance: single ao goals measure --gap=loop-closure emits unified status." \
  feature 2 "evolution-roadmap,three-gap-contract,loop-closure" "$E5"

# =========================================================================
# AUDIT EPICS A1, A2, A3
# =========================================================================
bd_mk A1 \
  "[audit] A1: AOP-CLAIM evidence map (83 claim markers)" \
  "Grep all AOP-CLAIM-* markers and the paragraph that follows each. Classify by category. For each, identify the evidence file or test. Output: .agents/research/2026-05-11-aop-claim-evidence-map.md. Unverified claims spawn child beads of shape 'Verify AOP-CLAIM-<id>'. $ROAD_REF (section A1)." \
  task 1 "evolution-roadmap,audit,aop-claim"

bd_mk A2 \
  "[audit] A2: Contract enforcement matrix (38 contracts)" \
  "For each docs/contracts/<name>.md, search scripts/check-*<name>*.sh and tests/contracts/*<name>*. Classify: enforced, partially-enforced, doc-only. Output: .agents/research/2026-05-11-contract-enforcement-matrix.md. Unenforced contracts spawn child beads 'Enforce <contract-name>'. $ROAD_REF (section A2)." \
  task 1 "evolution-roadmap,audit,contracts"

bd_mk A3 \
  "[audit] A3: Code-map drift report (2 maps)" \
  "For each docs/code-map/*.md, extract claimed file structure and diff against actual repo. Output: .agents/research/2026-05-11-code-map-drift-report.md. Drift items spawn child beads. $ROAD_REF (section A3)." \
  task 2 "evolution-roadmap,audit,code-map"

# =========================================================================
# EPIC LC: Learning Capture Loop
# =========================================================================
bd_mk LC_EPIC \
  "[epic] Evolution LC: Learning capture loop (compound the loop on itself)" \
  "Three-layer self-reflection so each day improves the next: (1) per-cycle 1-line micro-capture to .agents/evolve/daily-learning-log-YYYY-MM-DD.md, (2) every-5th-productive-cycle pattern reflect inline in cycle-history note, (3) end-of-day consolidate via scripts/evolve-capture-daily-learning.sh writing to .agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md and auto-filing evolve-improvement beads for cross-day recurring frictions. $ROAD_REF (section LC)." \
  task 1 "evolution-roadmap,learning-capture"

bd_mk LC1 \
  "LC1: Wire learning-capture protocol into /evolve all-day loop" \
  "Verify the three layers operate end-to-end on a real day: (a) micro-capture appends one line per cycle, (b) every-5th-productive reflect surfaces pattern annotations, (c) end-of-day consolidator runs at hard stop and writes the dated learning file. Acceptance: after one full /evolve day, .agents/evolve/daily-learning-log-YYYY-MM-DD.md has N entries (one per cycle), .agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md exists with counts + ledger + frictions sections, and if any FRICTION tag matches a prior day a LC-followup bead is auto-filed under evolution-roadmap. Cross-link: scripts/evolve-capture-daily-learning.sh, .agents/evolve/daily-learning-log.template.md." \
  feature 1 "evolution-roadmap,learning-capture" "$LC_EPIC"

echo ""
echo "===================="
echo "Seeding complete."
echo "===================="
echo ""
echo "Inspect: bd ready -n 50 | head -30"
echo "Full roadmap doc: $ROADMAP"
