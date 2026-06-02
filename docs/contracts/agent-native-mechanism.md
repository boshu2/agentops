# Contract: Agent-Native Mechanism (NTM background agents → hookless waist)

> **Status: decision doc (ag-uphk9 spike).** Nails the mechanism by which an
> out-of-session Claude/Codex background session stays under AgentOps guardrails **without
> hooks**, before the NTM background-agent lane is built. Companion to the
> `/agent-native` skill (`skills/agent-native/SKILL.md`, the how-to) and
> the [Eval Verdict Pipeline](eval-verdict-pipeline.md) / [Outcomes Rubric
> Projection](outcomes-rubric-projection.md) contracts.

## The reframe

The instinct — "port the ~50 marketplace hooks into a new runtime" or "make
Anthropic Managed Agents the default background runner" — is **wrong for
AgentOps 3.0**. Guardrails come from three pillars, never hooks:

1. **Skills** — `skills/<name>/SKILL.md` progressive-disclosure contracts.
2. **The `ao` CLI** — the deterministic tool surface (`ao session bootstrap`,
   `ao inject`, `ao corpus inject --query`, `ao validate`, `ao goals measure`),
   reachable in two ways: shell (Claude/Codex NTM sessions) or **MCP** (`ao mcp serve`).
3. **CI as the authoritative gate** — `.github/workflows/validate.yml` runs the
   standards/scenario/holdout checks as CI jobs, **not** as PreToolUse hooks.

A runtime-specific background-session profile is emitted by `ao agent bundle`
(`--codex-ntm` today, Claude NTM profile in the background-agent roster work)
and graded by the **same** CI gate as interactive work
(`.github/workflows/agent-output-validate.yml`).

## 1. Hook-intent → hookless equivalent (zero hooks required)

| Old hook *intent* | Claude NTM background session | Codex NTM background session | CI gate (authoritative) |
|---|---|---|---|
| **Orientation** (SessionStart) | `ao session bootstrap` + `session-bootstrap` skill in instructions | same | n/a (read-path) |
| **Standards injection** (Edit) | `standards` skill loaded into instructions; `ao validate` | same skill text via `ao agent bundle --codex-ntm` | `validate.yml` standards/scenario jobs |
| **Scope guard** (edit-scope) | skill discipline + mcp-agent-mail file reservation + `ao validate`; PR diff is the unit | same | `changes` + `process-hygiene` jobs |
| **Commit / output review** (commit-review) | the PR + `agent-output-validate.yml` | same (Codex output bundled, then validated) | `claude-review` + `agent-output-validate.yml` |
| **Holdout isolation** (holdout-isolation-gate) | eligibility excludes holdout/evaluator/PII work from background sessions unless explicitly allowed | same | `Outcomes holdout-leak gate` (`check-outcomes-holdout-leak.sh`, deny-by-default) |

**Conclusion: zero hooks are required.** Every old-hook intent maps onto a skill,
an `ao` subcommand, and a CI job. The CI gate is the enforcement boundary; the
skill/`ao` layer is advisory-by-design (an agent that ignores them simply fails CI).

## 2. NTM background-session profile

**Session profile shape** (`ao agent bundle --codex-ntm` emits the first checked-in form):

- `runtime` — Claude or Codex under NTM.
- `instructions` — the resolved standards + the task; **never** holdout
  target/ground-truth/PII.
- `mailbox` — the mcp-agent-mail identity the worker uses for assignments,
  reservations, check-ins, and handoff.
- `worktree_policy` — one worktree per bead/slice; no shared-checkout edits.
- `tools` — the curated `ao` shell/MCP tool surface (`ao mcp serve --print-tools` when MCP is used).
- `skills` — the progressive-disclosure SKILL.md set the loop may load.

**NTM + mcp-agent-mail.** A background agent is a real Claude/Codex session in
a tmux pane, not a hosted Managed Agent:

- **NTM** starts/stops/attaches/restarts the pane.
- **mcp-agent-mail** carries the assignment thread, file reservations, status
  check-ins, and result handoff.
- **`ao` over shell or MCP** gives the worker bootstrap, inject, validation, and
  provenance tools.
- **bd/Dolt** remains the durable work spine; the worker claims/updates/closes
  beads like any other agent.

## 3. Agent SDK / hosted hooks — optional adapters, not the default

The Agent SDK and hosted agent products expose `PreToolUse` / `PostToolUse` /
`Stop` / `SessionStart`-style surfaces. AgentOps treats these strictly as
**optional adapters** — the wiring lives in
`skills/agent-native/references/sdk-hook-adapter.md`,
never as the primary guardrail or the default background-agent runner.

**Why CI is the default gate, not these hooks:** a hook runs *inside* the agent's
own process — the same actor it is meant to police — so it is advisory and
bypassable, and it drifts per-runtime (Claude SDK ≠ Codex ≠ Managed Agent). CI
runs *after* the agent, on a neutral host, against the PR — one gate, every
runtime, non-bypassable. The SDK/hosted hook adapter is a convenience (fail fast
inside a specific runtime); the CI gate is the contract.

## 4. Non-goals (explicit)

- **No marketplace-hook port.** The ~50 marketplace hooks are not re-implemented
  in any new runtime. Their *intents* are covered per §1.
- **Eval substrate untouched.** `~/.agents/evals/SCHEMA.md` is LOCKED and only
  EXTENDED (the Outcomes projection), never relitigated.
- **Holdout / eval corpus excluded from background profiles and hosted bundles.**
  A session profile never carries holdout target/ground_truth/PII. Hosted/cloud
  adapters remain non-ZDR unless proven otherwise.

## See also

- `/agent-native` skill (`skills/agent-native/SKILL.md`) — the how-to (this doc is the why/mechanism).
- `cli/cmd/ao/agent.go` (`ao agent bundle`), `cli/cmd/ao/mcp_serve.go` (`ao mcp serve`).
- `.github/workflows/agent-output-validate.yml` — the authoritative output gate.
- Fleet topology (bushido sandbox, tailnet `100.109.17.108`, Dolt 3306): operator hub.
- Open follow-ons: the NTM background-agent roster/session commands and the
  provenance/writeback slices this doc unblocks.
