# Codex Execution Profile — cross-vendor-trust-gate

Read the base skill `../../skills/cross-vendor-trust-gate/SKILL.md` first (the
three-level trust model, the flag interface, what each side validates, the
exit-code table). The base is the source of truth; this is the step-ordered Codex
path. One inviolable rule: **you must RUN the gate — do not eyeball a skill and
declare it trustworthy.** The gate at `~/acfs/skill-pipeline/scripts/trust-gate.sh`
is the authority; this skill operates it, it does not reimplement it.

## Codex tool mapping

- **Run the gate** → `shell_command` invoking
  `bash ~/acfs/skill-pipeline/scripts/trust-gate.sh …`; capture the
  `trust-gate: <name> <level> score=<n>` summary and the exit code.
- **Read the verdict artifact** → `shell_command` with `jq` over
  `skill.trust.json` (drill into `select(.pass==false)` checks); do not guess
  which check failed.
- **Fix a failing twin** → `apply_patch` on the skill's source or
  `skills-codex/<name>/{SKILL.md,prompt.md}` — fix the skill, never the gate.

## Steps

1. **Confirm the tool + `jq`.** `test -f ~/acfs/skill-pipeline/scripts/trust-gate.sh
   && command -v jq`. The gate exits 127 without `jq` — install it first.
2. **Non-blocking grade.** Run WITHOUT `--require-cross` to see where the skill
   stands; this always writes `skill.trust.json` and prints the level on stdout.
3. **Read the artifact, don't guess.** `jq '{trust_level, trust_score, source:
   .source_validation.pass, codex: .codex_validation.pass}'` then drill into
   `.source_validation.checks[] | select(.pass==false)` and the codex equivalent.
4. **Interpret the level.** `fresh` → fix the source side (a `false` check names
   the file), re-run step 2. `single-validated` → author/repair the Codex twin
   (slim `name`+`description` frontmatter; `## Steps` + `## Guardrails` in
   prompt.md), re-run. `cross-validated` → proceed.
5. **Enforce the parity gate.** Re-run WITH `--require-cross`; `echo "exit=$?"`.
   Exit `0` = cross-validated, land it; `1` = below bar, do not land. The exit
   code is the machine-checkable permission, not the prose.

## Guardrails

- **Run the gate; trust the exit code, not self-report.** `--require-cross` exit 0
  is the gate; `skill.trust.json` is the audit trail a manager queries later.
- **Never edit `trust-gate.sh` to make a skill pass** — fix the skill, not the gate.
- **A green `validate.sh` is necessary, not sufficient** — `fresh` can still come
  from a missing spec or absent frontmatter. Read the failing check.
- **`single-validated` is not a landing state when parity is required** — build the
  Codex twin, do not lower the bar.
- **Backstage only.** Never surface this content in client-facing material.
