---
name: cli-agent-ux-audit
description: |
  Score and aggressively improve CLI ergonomics for AI agents — predictable flags,
  greppable --help, machine-readable (JSON/NDJSON) output modes, stable exit codes,
  and low-friction robot surfaces — then apply the fixes in-tree.
  Triggers: "make this CLI agent-friendly", "agent ergonomics", "robot mode for my CLI",
  "JSON output mode", "NDJSON streaming output", "stable exit codes", "audit CLI for agents",
  "design --help for agents", "machine-readable output", "CLI affordances for AI", "--robot flag".
practices:
- hexagonal-architecture
- pragmatic-programmer
- design-for-the-machine
hexagonal_role: supporting
consumes: []
produces:
- ergonomics-scorecard
- remediation-plan
context_rel:
- kind: complements
  with: installer-workmanship
- kind: complements
  with: readme-writing
skill_api_version: 1
context:
  window: inherit
  intent:
    mode: task
  intel_scope: none
metadata:
  tier: judgment
  stability: experimental
  dependencies: []
output_contract: 'an ergonomics scorecard (0–100 across 8 axes) plus a prioritized, in-tree remediation plan with concrete flag/output/exit-code changes'
user-invocable: false
allowed-tools:
- Read
- Edit
- Bash
- Grep
- Glob
---

# Agent Ergonomics & Intuitiveness Maximization for CLI Tools

> Make a CLI a tool an **AI agent** can drive blind — without a human in the loop, without
> trial-and-error, without a screen-scraping parser. The agent reads `--help`, predicts the
> right invocation, runs it, parses structured output, and branches on the exit code.

## ⚠️ Critical Constraints

- **Default human output and machine output are SEPARATE surfaces. Never make an agent parse pretty text.**
  **Why:** color codes, box-drawing, spinners, and right-aligned columns are unparseable; a `--json`
  flag (or `NO_COLOR`/`--robot`) gives the agent a stable contract while humans keep the pretty view.
  - WRONG: `print(f"✓ Done in {green(elapsed)}s ({count} items)")` as the only output.
  - CORRECT: human default above **plus** `--json` → `{"status":"ok","elapsed_s":1.2,"count":42}`.
- **Exit codes MUST be stable, documented, and meaningful. `exit 1` for everything is a defect.**
  **Why:** an agent branches on the exit code before it parses anything; collapsing all failures to `1`
  forces brittle string-matching on stderr. Reserve `0` success, `1` generic, `2` usage error, and
  named codes for the failures the agent must distinguish.
- **`--help` is the agent's API surface. If it is incomplete or non-greppable, the tool is invisible.**
  **Why:** the agent's first move is to read help, not docs. Every flag, every subcommand, every
  accepted value, and the exit-code table must appear in `--help`/`<cmd> help` output.
- **Side-effecting commands MUST offer `--dry-run` and read confirmation from a flag, never a TTY prompt.**
  **Why:** an interactive `Are you sure? [y/N]` prompt hangs a non-interactive agent forever. Use
  `--yes`/`--force` and let `--dry-run` preview the effect.

## Why This Exists

CLIs are historically designed for a human at a terminal: ANSI color, interactive prompts, output
tuned for the eye, and "the error message tells you what went wrong." An AI agent has none of those
affordances. It cannot see color, cannot answer a prompt mid-run, cannot infer a flag from a man page
it didn't read, and cannot recover from an exit code that means nothing. A CLI that is *delightful for
humans* can be *unusable for agents* — and the inverse, a tool built robot-first, is usually better
for humans too (predictable, scriptable, testable). This skill turns "feels clunky for agents" into a
measured score and a concrete fix list.

## Quick Start

```bash
# 1. Self-check this skill is intact.
bash {baseDir}/scripts/validate.sh

# 2. Probe the CLI under audit (substitute your binary for mytool):
mytool --help            ; echo "help_exit=$?"
mytool --version         ; echo "ver_exit=$?"
mytool --json 2>/dev/null| head -c 200; echo
mytool definitely-bogus  ; echo "bad_exit=$?"   # expect a usage/2 code, not 1
NO_COLOR=1 mytool status | cat -v | head        # any ESC[ sequences = leaking color
```

Then write the scorecard + remediation plan (see Output Specification) and apply fixes in-tree.

## Methodology — Audit → Score → Remediate → Verify

**Phase 1 — Audit (read the surfaces, do not guess).**
Run `--help`, every subcommand's `--help`, `--version`, a known-good and a known-bad invocation.
Capture exit codes. Grep the source for `print`/`fmt.Print`/`console.log`/color libraries and for the
exit-code constants. The 8 axes are detailed in [the rubric reference](references/ergonomics-rubric.md).

→ **Checkpoint:** you can name, for each axis, exactly what the tool does today and the evidence.

**Phase 2 — Score (0–100, 8 axes × ~12.5).**
Score each axis 0/partial/full per [references/ergonomics-rubric.md](references/ergonomics-rubric.md).
Record the number AND the failing line/file so the fix is unambiguous.

→ **Checkpoint:** a scorecard table with a number per axis and a one-line justification.

**Phase 3 — Remediate (apply in-tree, smallest changes first).**
Order fixes by leverage: machine output mode and exit-code discipline first (they unblock the agent
most), then `--help` completeness, then prompt/TTY removal, then the polish axes. Apply with `Edit`.
Patterns for each language and surface live in
[references/robot-mode-patterns.md](references/robot-mode-patterns.md).

→ **Checkpoint:** each fix has a diff and a re-run command proving the behavior changed.

**Phase 4 — Verify (re-probe, prove the delta).**
Re-run the Phase-1 probes; the exit codes and output shape must now match the contract. For JSON
modes, pipe through `jq .` (must parse). For NDJSON, every line must independently parse.

→ **Checkpoint:** before/after probe transcript attached to the scorecard; score recomputed.

## Output Specification

Produce two artifacts (Markdown; print to chat or write under the target repo's `docs/`):

1. **`cli-ergonomics-scorecard.md`** — a table: `| Axis | Score | Evidence | Fix |`, the total /100,
   and the before/after probe transcript.
2. **`cli-ergonomics-remediation.md`** — the prioritized fix list, each item: surface, current
   behavior, target behavior, file:line, and the verification command.

## Robot Mode (what "agent-friendly" output looks like)

The contract you are designing *toward*. Every audited tool should expose one of these:

```bash
mytool list --json          # one JSON document (arrays/objects), stable keys
mytool watch --ndjson       # one JSON object per line, flushed per event (streaming)
mytool status --json | jq -e '.healthy'   # branchable without string-matching
```

Rules: stable key names (never localized), nulls explicit (no present-vs-omitted ambiguity), machine
output to stdout / logs+progress to stderr, and `--json`/`--ndjson`/`--robot` documented in `--help`.
Language-specific implementations: [references/robot-mode-patterns.md](references/robot-mode-patterns.md).

## Exit Codes (the contract you audit for, and this skill's own)

A well-behaved CLI publishes a table like this in `--help`; absence of one is itself a finding.

| Code | Meaning | Agent action |
|------|---------|--------------|
| 0 | Success | proceed |
| 1 | Generic/unexpected failure | read stderr, may retry once |
| 2 | Usage error (bad flag/arg) | fix the invocation, do not retry as-is |
| 3 | Not found (resource/target) | resolve the target first |
| 4 | Precondition unmet (auth, config) | satisfy the precondition |
| 5 | Partial success | inspect machine output for which items failed |

This skill's `scripts/validate.sh`: `0` = intact, `1` = structural finding.

## Quality Rubric

- **Machine output verified by parse, not by eye:** every claimed `--json` output pipes cleanly through
  `jq .` and every `--ndjson` line parses independently — checked, not assumed.
- **Exit codes proven by invocation:** the scorecard cites the actual code returned by a good and a bad
  run (`echo $?`), not what the docs claim.
- **`--help` is greppable and complete:** every flag/subcommand referenced in the remediation appears in
  `--help` output, and no documented flag is missing from help.
- **No interactive trap remains:** a `--dry-run` and non-TTY confirmation path exists for every
  side-effecting command; piping the command non-interactively never hangs.

## Examples

- **Go tool prints colored status only.** Add a `--json` flag (also honor `NO_COLOR`); emit
  `json.NewEncoder(os.Stdout).Encode(result)`; keep the human view as default. The machine-output axis
  goes 0 → full.
- **Python CLI returns `sys.exit(1)` on both "bad flag" and "network down".** Let argparse's usage
  error stay `2` (stop overriding it) and add a named `4` for the precondition. The agent can now branch.
- **Node CLI prompts `readline.question("Delete? ")`.** Replace with a `--force` flag and a `--dry-run`
  that logs the plan; the prompt fires only when stdin is a TTY (`process.stdin.isTTY`).

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Agent "can't tell if it worked" | all paths exit 0, or all exit 1 | Give failures distinct, documented codes |
| `--json` output won't `jq` | progress/log lines mixed into stdout | Send logs+progress to stderr; only data to stdout |
| Agent hangs on a command | interactive prompt waiting on TTY | Add `--yes`/`--force`; gate prompt on `isTTY` |
| Agent picks wrong flag | flag absent or misdescribed in `--help` | Make `--help` the complete, greppable contract |
| Color codes in piped output | unconditional ANSI | Honor `NO_COLOR` + detect non-TTY; strip when piped |

## See Also

| I need to… | Use |
|------------|-----|
| Detailed scoring criteria per axis | [references/ergonomics-rubric.md](references/ergonomics-rubric.md) |
| Concrete robot-mode code per language | [references/robot-mode-patterns.md](references/robot-mode-patterns.md) |
| A self-diagnosing `doctor` subcommand | a CLI doctor-mode skill, if present in your corpus |
| A curl\|bash installer for the tool | the `installer-workmanship` skill |
