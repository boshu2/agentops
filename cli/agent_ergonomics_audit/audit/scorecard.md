# ao CLI — Agent Ergonomics Audit (Pass 1)

Target: `ao` CLI (`cli/cmd/ao/`, 73 top-level commands, Go/Cobra).
Mode: full (audit + apply). Workspace: in-tree, gitignored.

## Baseline (what was already good)

- Global `--json` / `-o json|table|yaml`, `--dry-run`, `--verbose`.
- Errors exit non-zero (1) and go to stderr. `status --json` is pure JSON.
- `ao doctor` already a strong exemplar: `--robot`, `--robot-triage`,
  `doctor capabilities`, `doctor robot-docs`.

## Gaps found → fixes applied

| # | Gap (dimension) | Fix |
|---|-----------------|-----|
| 1 | No top-level introspection (self_documentation) | New `ao capabilities` — JSON contract: command tree, flags, exit codes, env vars, robot surfaces |
| 2 | No agent handbook (self_documentation) | New `ao robot-docs` — paste-ready Markdown handbook |
| 3 | Unknown flags gave opaque "unknown flag: --jsno" (intent_inference) | Levenshtein-≤2 typo suggestion via shared FlagErrorFunc on root + doctor |
| 4 | `required flag(s) not set` was terse (error_pedagogy) | `ExecuteC()` enriches with usage line + example, CLI-wide |
| 5 | Parent commands printed human help under `--json` (output_parseability) | `ao <group> --json` emits a JSON subcommand listing, CLI-wide |
| 6 | `--help` didn't point agents at robot surfaces (self_documentation) | "For AI agents" footer added to root help |
| 7-10 | Errors didn't name the corrective command (error_pedagogy) | Rewrote autodev / claim / citation / constraint errors to name the exact next command |

## Dimensions touched

self_documentation, intent_inference, error_pedagogy, output_parseability.

## Regression coverage (committed Go tests)

`capabilities_test.go`, `flag_suggest_test.go`, `group_json_test.go` —
all assert behavioral correctness. Full `cmd/ao` suite: 10002 pass.

## Deferred (Pass 2 candidates)

- Promote `doctor --robot-triage` shape to a top-level `ao triage` mega-command.
- Audit `--json` fidelity on every read-side leaf command (some still emit
  partial/human output, e.g. `plans list`).
- Unify exit-code dictionary: the doctor extended codes (2-6, 64-74) are not
  shared by other diagnostic commands.
