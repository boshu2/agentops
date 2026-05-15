# Discovery DAG and step detail

> Procedure body extracted from SKILL.md to keep the billboard within the meta-tier line cap. SKILL.md owns the quick-ref, flags, and pointers; this file owns the executable workflow.

## DAG — Execute This Sequentially

```
mkdir -p .agents/rpi
detect bd and ao CLI availability
```

**Run every step in order. Do not stop between steps.**

```
STEP 1  ──  if not --skip-brainstorm AND goal is vague (<50 chars or vague keywords):
              Skill(skill="brainstorm", args="<goal>")
              Use refined goal for subsequent steps if produced.

STEP 1.5 ── if PRODUCT.md exists in repo root
              AND goal appears to be a feature or capability
              (not a bug fix, chore, or docs task — i.e., goal does NOT start with
               "fix", "chore", "docs", "typo", "bump", "update dep", "lint", "format"):
                Skill(skill="design", args="<goal> [--quick]")
                FAIL verdict? → output <promise>BLOCKED</promise>, stop (design is a blocking product-alignment gate).
              Skip silently if PRODUCT.md does not exist or goal is non-feature.

STEP 2  ──  if ao available:
              ao search "<goal keywords>" 2>/dev/null || true
              ao lookup --query "<goal keywords>" --limit 5 2>/dev/null || true
              Assemble ranked packet: compiled planning rules + active findings
              + unconsumed high-severity next-work items. Carry forward as context.
              For each returned learning, check applicability to the goal. If applicable,
              cite by filename and record: ao metrics cite "<path>" --type applied 2>/dev/null || true
              (orchestrator-owned: this knowledge retrieval is intentionally inline CLI,
               not a Skill() delegation. Do NOT expand into a separate /research call.)

STEP 3  ──  Skill(skill="research", args="<goal> [--auto]")
              Pass --auto unless --interactive. Output lands in .agents/research/.
              After: identify applicable test levels (L0-L3) for downstream /plan.

STEP 4  ──  Skill(skill="plan", args="<goal> [--auto]")
              Pass --auto unless --interactive.
              After: extract epic-id, auto-detect complexity from issue count
              (1-2 → fast, 3-6 → standard, 7+ → full) unless --complexity override.
              The plan output MUST include `acceptance_criteria` fenced YAML
              blocks at TWO levels: per-epic (parent epic body) AND per-bead
              (each issue body). Criterion contract documented under
              "Acceptance Criteria Contract" below; canonical machine-readable
              shape lives in `schemas/execution-packet.schema.json` (#/$defs/Criterion).

STEP 4.5 ── if --no-scaffold is NOT set (alias: --no-lifecycle, deprecated)
              AND plan output contains new project/module creation
              (keywords: scaffold, new project, bootstrap, init, create module,
               new package, new service):
                detect language from plan context or existing project files
                Skill(skill="scaffold", args="<detected-language> <project-name>")
                Scaffold output becomes input context for pre-mortem.
              Skip if: --no-scaffold flag (or deprecated --no-lifecycle), no new project/module detected in plan.

STEP 5  ──  Skill(skill="pre-mortem", args="<plan-path> [--quick]")
              Use --quick for fast/standard. Full council for full.
              PASS/WARN? → continue to STEP 6
              FAIL?      → re-plan with findings, re-run pre-mortem (max 3 total)
                           Still FAIL after 3? → output <promise>BLOCKED</promise>, stop

STEP 6  ──  Write execution-packet.json (latest alias) + per-run packet archive
              to .agents/rpi/ and .agents/rpi/runs/<run-id>/ when run_id exists.
              Include plan_path, test_levels, ranked_packet_path, epic-id, complexity.
              Record the criteria fences from STEP 4's plan into the packet under
              `epic_criteria` (array) and `bead_criteria` (object keyed by bead ID).
              Canonical shape: `schemas/execution-packet.schema.json`.
              ao ratchet record discovery 2>/dev/null || true
              Output <promise>DONE</promise>
```

**That's it.** Steps 1→1.5→2→3→4→5→6. No stopping between steps.

## Setup Detail

**State:**
```
discovery_state = {
  goal: "<goal string>",
  interactive: <true if --interactive>,
  complexity: <fast|standard|full or null for auto-detect>,
  skip_brainstorm: <true if --skip-brainstorm or goal is >50 chars and specific>,
  epic_id: null,
  attempt: 1,
  verdict: null
}
```

**CLI dependency detection:**
```bash
if command -v bd &>/dev/null; then TRACKING_MODE="beads"; else TRACKING_MODE="tasklist"; fi
if command -v ao &>/dev/null; then AO_AVAILABLE=true; else AO_AVAILABLE=false; fi
```

## Gate Detail

Discovery has two blocking gates.

- **STEP 1.5 (design gate):** `FAIL` blocks discovery immediately for feature/capability goals when `PRODUCT.md` exists.
- **STEP 5 (pre-mortem gate):** Max 3 attempts with plan→pre-mortem retry loop.
  - **PASS/WARN:** Store verdict, apply any required pre-mortem hardening back into the plan issues or file-backed task specs, then proceed to STEP 6.
  - **FAIL:** Log `"Pre-mortem: FAIL (attempt N/3) -- retrying plan with feedback"`. Re-invoke `/plan` with findings context, then re-invoke `/pre-mortem`. After 3 total failures: output `<promise>BLOCKED</promise>`, stop.

## Step Detail

- **STEP 1 (brainstorm):** Skip if `--skip-brainstorm`, or goal >50 chars with no vague keywords (`improve`, `better`, `something`, `somehow`, `maybe`), or brainstorm artifact already exists in `.agents/brainstorm/`.
- **STEP 1.5 (design gate):** Optional. Runs `/design` when PRODUCT.md exists at repo root and the goal is a feature or capability (not a bug fix, chore, or docs task). Design verdict `FAIL` blocks discovery; `PASS` or `WARN` continues. Skipped silently when PRODUCT.md is absent.
- **STEP 2 (search history):** Ranked packet assembly — match compiled planning rules, active findings from `.agents/findings/*.md`, and unconsumed high-severity items from `.agents/rpi/next-work.jsonl`. Rank by goal-text overlap → issue-type overlap → file-path overlap.
- **STEP 3.1 (test levels):** After research, determine L0-L3 applicability. External APIs/I/O → L0+L1+L2 min. Cross-module → add L2. Full subsystem → add L3. Record in `discovery_state.test_levels`.
- **STEP 4 (plan):** After plan, record the exact `plan_path` for STEP 5. If tracker probes are healthy, extract epic-id via `bd list --type epic --status open`. If tracker probes are degraded, keep the objective + `plan_path` in `.agents/rpi/execution-packet.json` and continue in `tasklist` mode without inventing an epic. The plan emitted by `/plan` MUST include `acceptance_criteria` fenced YAML at two levels: per-epic (parent epic body) and per-bead (each issue body). Criterion shape is fixed by `schemas/execution-packet.schema.json` (`#/$defs/Criterion`). See Acceptance Criteria Contract below for the YAML form.
- **STEP 5 (pre-mortem):** Pass the recorded `plan_path` into `/pre-mortem`. Do not rely on "most recent" plan/spec selection during discovery retries.
- **STEP 5.5 (pre-mortem fix propagation):** Before STEP 6, copy any required pseudocode fixes from the pre-mortem report into the affected plan issues or file-backed task specs. Workers read issue/task bodies, not the pre-mortem report.
- **STEP 6 (output):** Write execution packet and phase summary per `references/output-templates.md`. Keep `.agents/rpi/execution-packet.json` as the latest alias and archive the same packet to `.agents/rpi/runs/<run-id>/execution-packet.json` when `run_id` exists. Include `plan_path`, `test_levels`, and `ranked_packet_path` in the execution packet for `/crank` and standalone `/validation` consumption. Record the criteria fences from STEP 4's plan into the packet: `epic_criteria` (array, from the epic body) and `bead_criteria` (object keyed by bead ID, one array per bead). Both fields are typed by `#/$defs/Criterion` in `schemas/execution-packet.schema.json` — that schema is the canonical source of truth for criterion shape, not this SKILL.md.

## Acceptance Criteria Contract

Both the epic and each child bead carry an `acceptance_criteria` fenced YAML block. STEP 6 lifts these into the execution packet (`epic_criteria`, `bead_criteria`). Canonical shape: `schemas/execution-packet.schema.json` (`#/$defs/Criterion`).

```yaml
acceptance_criteria:
  - id: ac-<scope>.<n>
    description: "<one-line measurable statement>"
    check_type: test_pass | command_exit_zero | file_exists | grep_match | manual | council_judge | custom_rubric
    check_command: "<shell command or script path>"
    evidence_path: "<glob>"
    evidence_required: true | false
    weight: 0.0–1.0
    optional: true | false
    agent_judge: "<council:name>"  # REQUIRED only when check_type == custom_rubric
```

`agent_judge` is REQUIRED when `check_type == "custom_rubric"` — `custom_rubric` accepts free-text `check_command`, so the judge field names the council/judge that owns the verdict. Enforced by the schema's `if/then` clause; missing it is a packet-write error, not a runtime warning.
