---
name: discovery
description: 'Full discovery phase orchestrator. Brainstorm + ao search + research + plan + pre-mortem gate. Produces epic-id and execution-packet for $crank. Triggers: "discovery", "discover", "explore and plan", "research and plan", "discovery phase".'
---

# $discovery — Full Discovery Phase Orchestrator

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), ensure startup context
before entering the discovery DAG:

```bash
ao codex ensure-start 2>/dev/null || true
```

`ao codex ensure-start` is the single startup guard for Codex skills. It records
startup once per thread and skips duplicate startup automatically. Leave
`ao codex ensure-stop` to closeout skills; discovery owns the startup path.

## DAG — Execute This Sequentially

```
mkdir -p .agents/rpi
detect bd and ao CLI availability
```

**Run every step in order. Do not stop between steps.**

```
STEP 1  ──  if not --skip-brainstorm AND goal is vague (<50 chars or vague keywords):
              $brainstorm <goal>
              Use refined goal for subsequent steps if produced.

STEP 2  ──  if ao available:
              ao search "<goal keywords>" 2>/dev/null || true
              ao lookup --query "<goal keywords>" --limit 5 2>/dev/null || true
              Assemble ranked packet: compiled planning rules + active findings
              + unconsumed high-severity next-work items. Carry forward as context.

STEP 3  ──  $research <goal> [--auto]
              Pass --auto unless --interactive. Output lands in .agents/research/.
              After: identify applicable test levels (L0-L3) for downstream $plan.

STEP 4  ──  $plan <goal> [--auto]
              Pass --auto unless --interactive.
              After: extract epic-id, auto-detect complexity from issue count
              (1-2 → fast, 3-6 → standard, 7+ → full) unless --complexity override.

STEP 5  ──  $pre-mortem [--quick]
              Use --quick for fast/standard. Full council for full.
              PASS/WARN? → continue to STEP 6
              FAIL?      → re-plan with findings, re-run pre-mortem (max 3 total)
                           Still FAIL after 3? → output <promise>BLOCKED</promise>, stop

STEP 6  ──  Write execution-packet.json + phase summary to .agents/rpi/
              Include test_levels, ranked packet, epic-id, complexity.
              ao ratchet record discovery 2>/dev/null || true
              Output <promise>DONE</promise>
```

**That's it.** Steps 1→2→3→4→5→6. No stopping between steps.

---

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
if bd ready --json >/dev/null 2>&1 && bd list --type epic --status open --json >/dev/null 2>&1; then
  TRACKING_MODE="beads"
else
  TRACKING_MODE="tasklist"
fi
if command -v ao &>/dev/null; then AO_AVAILABLE=true; else AO_AVAILABLE=false; fi
```

## Gate Detail

**STEP 5 (pre-mortem) is the only gate.** Max 3 attempts with plan→pre-mortem retry loop.

- **PASS/WARN:** Store verdict, proceed to STEP 6.
- **FAIL:** Log `"Pre-mortem: FAIL (attempt N/3) -- retrying plan with feedback"`. Re-invoke `$plan` with findings context, then re-invoke `$pre-mortem`. After 3 total failures: output `<promise>BLOCKED</promise>`, stop.

## Step Detail

**STEP 1 (brainstorm):** Skip if `--skip-brainstorm`, or goal >50 chars with no vague keywords (`improve`, `better`, `something`, `somehow`, `maybe`), or brainstorm artifact already exists in `.agents/brainstorm/`.

**STEP 2 (search history):** Ranked packet assembly — match compiled planning rules, active findings from `.agents/findings/*.md`, and unconsumed high-severity items from `.agents/rpi/next-work.jsonl`. Rank by goal-text overlap → issue-type overlap → file-path overlap.

**STEP 3.1 (test levels):** After research, determine L0-L3 applicability. External APIs/I/O → L0+L1+L2 min. Cross-module → add L2. Full subsystem → add L3. Record in `discovery_state.test_levels`.

**STEP 4 (plan):** After plan, keep `tracker_mode` honest. If tracker probes are healthy, extract epic-id via `bd list --type epic --status open`. If tracker probes are degraded, keep the objective + plan file in `.agents/rpi/execution-packet.json` and continue in `tasklist` mode without inventing an epic.

**STEP 6 (output):** Write execution packet and phase summary per `references/output-templates.md`. Include `test_levels` and ranked packet in the execution packet for `$crank` consumption.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--interactive` | off | Human gates in research and plan |
| `--skip-brainstorm` | auto | Skip brainstorm step |
| `--complexity=<level>` | auto | Force complexity level (fast/standard/full) |
| `--no-budget` | off | Disable phase time budgets |

## Quick Start

```bash
$discovery "add user authentication"              # full discovery
$discovery --interactive "refactor payment module" # human gates in research + plan
$discovery --skip-brainstorm "fix login bug"       # skip brainstorm for specific goals
$discovery --complexity=full "migrate to v2 API"   # force full council ceremony
```

## Completion Markers

```
<promise>DONE</promise>      # Discovery complete, epic-id + execution-packet ready
<promise>BLOCKED</promise>   # Pre-mortem failed 3x, manual intervention needed
```

## Troubleshooting

Read `references/troubleshooting.md` for common problems and solutions.

## Reference Documents

- [references/complexity-auto-detect.md](references/complexity-auto-detect.md) — precedence contract for keyword vs issue-count classification
- [references/idempotency-and-resume.md](references/idempotency-and-resume.md) — re-run safety and resume behavior
- [references/phase-budgets.md](references/phase-budgets.md) — time budgets per complexity level
- [references/troubleshooting.md](references/troubleshooting.md) — common problems and solutions
- [references/output-templates.md](references/output-templates.md) — execution packet and phase summary formats

**See also:** [brainstorm](../brainstorm/SKILL.md), [research](../research/SKILL.md), [plan](../plan/SKILL.md), [pre-mortem](../pre-mortem/SKILL.md), [crank](../crank/SKILL.md), [rpi](../rpi/SKILL.md)

## Local Resources

### references/

- [references/complexity-auto-detect.md](references/complexity-auto-detect.md)
- [references/idempotency-and-resume.md](references/idempotency-and-resume.md)
- [references/output-templates.md](references/output-templates.md)
- [references/phase-budgets.md](references/phase-budgets.md)
- [references/troubleshooting.md](references/troubleshooting.md)

<!-- Lifecycle integration wired: 2026-03-28. See skills/discovery/SKILL.md for canonical -->
