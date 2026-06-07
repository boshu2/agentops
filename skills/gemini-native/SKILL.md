---
name: gemini-native
description: |
  Drive the AgentOps claim->work->validate->close->persist loop natively on
  Gemini CLI, using `gemini -p`, worktrees, skills, MCP, hooks, and structured
  output instead of Claude or Codex runtime wrappers.

  Triggers: "gemini native", "Gemini CLI image", "run the loop on Gemini",
  "Gemini worker", "Gemini validator", "gemini -p", "gemini worktree",
  "Gemini headless tick", "make Gemini AgentOps-native", "Google-family lane".
practices:
- continuous-delivery
- design-by-contract
hexagonal_role: driving-adapter
consumes:
- operating-loop-skill
- beads-br
produces:
- gemini-run-evidence
- scoped-commit
context_rel:
- kind: customer-of
  with: operating-loop-skill
skill_api_version: 1
user-invocable: false
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: cross-vendor
  stability: experimental
  dependencies: [beads-br, dcg, agent-mail]
output_contract: "A Gemini-native loop turn: claimed bead, isolated worker run, independent validator verdict, evidence artifact, scoped commit, and persisted handoff."
---

# gemini-native

Run the AgentOps operating loop on the **Gemini CLI image**. This is distinct
from AGY/Antigravity: AGY is its own harness, while `gemini` is the local Gemini
CLI with native skills, extensions, hooks, MCP, worktrees, sandboxing, sessions,
and headless execution.

Ground truth on this host: `gemini --version` returned `0.45.2`; `gemini --help`
shows `-p/--prompt`, `--worktree`, `--sandbox`, `--approval-mode`, `--resume`,
`--output-format`, `skills`, `extensions`, `hooks`, and `mcp`.

## Critical Constraints

- **Use Gemini primitives, not another runtime's worker path.** Dispatch with
  `gemini -p` or an interactive `gemini` session. Do not shell out to a Claude
  or Codex worker to call this a Gemini lane. **Why:** the point is a real
  Google-family turnout with its own failure modes and proof surface.
- **Author and judge are separate Gemini contexts.** A worker that edits cannot
  close its own bead. Run the validator with a fresh session, separate worktree,
  or read-only approval mode. **Why:** self-grade is the false-close path.
- **Prefer `--worktree` for write lanes.** Let Gemini create an isolated git
  worktree for author runs, or provide an already isolated repo dir. **Why:**
  shared working trees are what made the previous large skill wave expensive.
- **Evidence is a file, not chat.** Capture output format, exit code, run
  directory, diff, and validator verdict. **Why:** later agents need durable
  evidence, not a live terminal transcript.
- **Keep approval explicit.** Use `--approval-mode plan` for read-only
  validation, `auto_edit` only for scoped file edits, and `yolo` only inside an
  externally sandboxed host. **Why:** automatic tool approval is a blast-radius
  choice.

## Quick Start

```bash
gemini --version
gemini -p "Read the repo and summarize the next safe action." \
  --approval-mode plan \
  --output-format json
```

For a worker:

```bash
gemini -p "Claim bead <id>, implement only its scope, run the gate, report evidence." \
  --worktree gemini-<id> \
  --approval-mode auto_edit \
  --output-format stream-json
```

For a validator:

```bash
gemini -p "Validate bead <id> read-only. Return VERDICT: PASS or FAIL with evidence." \
  --approval-mode plan \
  --output-format json
```

## Workflow

### Phase 1: Verify the image

Run:

```bash
command -v gemini
gemini --version
gemini skills list --all
gemini mcp list
```

Checkpoint: the binary resolves, skills can be listed, and the needed MCP
servers are visible or explicitly not required.

### Phase 2: Claim and isolate

Pick one bead or one bounded task. Record the write scope before launching a
worker. For edits, use `--worktree <name>` or start Gemini in a pre-created
worktree. Pass `--include-directories` only for extra read context that the task
actually needs.

Checkpoint: no other active lane writes the same paths.

### Phase 3: Run the author

Launch the author with the specific objective, write scope, and done condition.
Use `--approval-mode auto_edit` for scoped edits. Capture stream JSON when the
run is part of a factory loop.

Checkpoint: the author reports changed files, commands run, exit codes, and the
bead state it believes it satisfied.

### Phase 4: Run the judge

Launch a separate Gemini context with `--approval-mode plan`. It reads the diff,
reruns the relevant gate, checks scope, and emits a verdict artifact. The author
does not write this verdict.

Checkpoint: close only if the judge verdict says PASS and cites command output.

### Phase 5: Persist and close

Commit the scoped change, attach the evidence to the bead, and write any useful
learning to the shared corpus. If the validator fails, leave the bead open with
the failing evidence and next action.

## Output Specification

Write a run record under the repo or a temp evidence dir:

- `events.jsonl` or `response.json`
- `exit-code`
- `changed-files.txt`
- `verdict.md`
- bead update or handoff note

## Quality Rubric

- The worker and validator are separate contexts.
- The write scope is explicit and respected.
- Every close cites a command exit code or durable artifact.
- The Gemini lane uses Gemini CLI primitives directly.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Gemini asks for every tool call | Approval mode is default | Use `--approval-mode auto_edit` for authors or `plan` for validators |
| Validator edits files | Wrong approval mode | Rerun with `--approval-mode plan` |
| Output is hard to parse | Text output selected | Use `--output-format json` or `stream-json` |
| Parallel runs collide | Shared worktree | Use `--worktree` or separate checkout |

## See Also

- [gemini-headless-evidence](../gemini-headless-evidence/SKILL.md)
- [gemini-skills-extensions](../gemini-skills-extensions/SKILL.md)
- [gemini-mcp-hooks](../gemini-mcp-hooks/SKILL.md)
