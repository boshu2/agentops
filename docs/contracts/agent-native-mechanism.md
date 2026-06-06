# Contract: Agent-Native Mechanism (Managed Agents + Agent SDK → hookless waist)

> **Status: decision doc (ag-uphk9 spike).** Nails the mechanism by which an
> out-of-session Claude/Codex loop stays under AgentOps guardrails **without
> hooks**, before the managed-dispatch lane is built. Companion to the
> [`/agent-native`](https://github.com/boshu2/agentops/blob/main/skills/agent-native/SKILL.md) skill (the how-to) and
> the [Eval Verdict Pipeline](eval-verdict-pipeline.md) / [Outcomes Rubric
> Projection](outcomes-rubric-projection.md) contracts.

## The reframe

The instinct — "port the ~50 marketplace hooks into Managed Agents / the Agent
SDK" — is **wrong for AgentOps 3.0**. Guardrails come from three pillars, never
hooks:

1. **Skills** — `skills/<name>/SKILL.md` progressive-disclosure contracts.
2. **The `ao` CLI** — the deterministic tool surface (`ao session bootstrap`,
   `ao inject`, `ao corpus inject --query`, `ao validate`, `ao goals measure`),
   reachable in two ways: shell (Codex/NTM) or **MCP** (`ao mcp serve`, hosted/SDK).
3. **CI as the authoritative gate** — `.github/workflows/validate.yml` runs the
   standards/scenario/holdout checks as CI jobs, **not** as PreToolUse hooks.

A runtime-specific Agent definition is emitted by `ao agent bundle` (`--managed`
| `--codex-ntm`) and graded by the **same** CI gate as interactive work
(`.github/workflows/agent-output-validate.yml`).

## 1. Hook-intent → hookless equivalent (zero hooks required)

| Old hook *intent* | Claude (Managed Agent / SDK / MCP-`ao`) | Codex / NTM (shell-`ao` / tmux / ssh bushido) | CI gate (authoritative) |
|---|---|---|---|
| **Orientation** (SessionStart) | `ao session bootstrap` + `session-bootstrap` skill in instructions | `ssh bushido 'cd repo && ao session bootstrap'` | n/a (read-path) |
| **Standards injection** (Edit) | `standards` skill loaded into instructions; `ao validate` | same skill text via `ao agent bundle --codex-ntm` | `validate.yml` standards/scenario jobs |
| **Scope guard** (edit-scope) | skill discipline + `ao validate`; PR diff is the unit | same; `ssh bushido` worktree-per-bead | `changes` + `process-hygiene` jobs |
| **Commit / output review** (commit-review) | the PR + `agent-output-validate.yml` | same (Codex output bundled, then validated) | `claude-review` + `agent-output-validate.yml` |
| **Holdout isolation** (holdout-isolation-gate) | rubric projection strips ground truth *by construction*; payloads never carry holdout | same — the Outcomes/local score is the only thing that crosses | `Outcomes holdout-leak gate` (`check-outcomes-holdout-leak.sh`, deny-by-default) |

**Conclusion: zero hooks are required.** Every old-hook intent maps onto a skill,
an `ao` subcommand, and a CI job. The CI gate is the enforcement boundary; the
skill/`ao` layer is advisory-by-design (an agent that ignores them simply fails CI).

## 2. Managed Agents Agent definition + self-hosted sandbox

**Agent shape** (`ao agent bundle --managed` emits this):

- `model` — the Claude model id.
- `instructions` — the resolved standards + the task; **never** the holdout
  corpus or `~/.agents/evals/SCHEMA.md` (Managed Agents are not ZDR).
- `tools` — the curated `ao` MCP tool surface (`ao mcp serve --print-tools`).
- `skills` — the progressive-disclosure SKILL.md set the loop may load.

**Self-hosted sandbox (bushido).** A Managed Agent bound to a self-hosted
Environment runs *inside the boundary*, so it can reach private services without
shipping anything to the cloud:

- **`ao` over MCP** — `ao mcp serve` (tailnet-bound) gives the in-boundary loop
  the deterministic tool surface. Hosted Claude reaches it via the sandbox's
  private MCP, never the public internet.
- **Dolt (bd spine)** — `100.109.17.108:3306` over tailnet (per the fleet hub);
  the sandbox writes bead/provenance state to the same shared server interactive
  work uses. LAN-firewalled; tailnet ACL is the boundary.
- **Holdout stays on-box** — the sandbox may read `~/.agents/evals/` locally, but
  the *Agent bundle* sent to the hosted control plane excludes it.

**Codex / NTM column.** The same bushido sandbox is reached by shell: `ssh
bushido 'cd repo && ao …'`, a tmux-pane swarm for parallelism, and agent-mail /
`ntm` for cross-agent coordination. No MCP; `ao` is shell-called. The Agent
definition for this runtime is `ao agent bundle --codex-ntm`.

## 3. Agent SDK hooks — the OPTIONAL adapter

The Agent SDK exposes `PreToolUse` / `PostToolUse` / `Stop` / `SessionStart`
surfaces. AgentOps treats these strictly as an **optional adapter** — the wiring
lives in [`skills/agent-native/references/sdk-hook-adapter.md`](https://github.com/boshu2/agentops/blob/main/skills/agent-native/references/sdk-hook-adapter.md),
never as the primary guardrail.

**Why CI is the default gate, not these hooks:** a hook runs *inside* the agent's
own process — the same actor it is meant to police — so it is advisory and
bypassable, and it drifts per-runtime (Claude SDK ≠ Codex ≠ Managed Agent). CI
runs *after* the agent, on a neutral host, against the PR — one gate, every
runtime, non-bypassable. The SDK hook adapter is a convenience (fail fast in the
loop); the CI gate is the contract.

## 4. Non-goals (explicit)

- **No marketplace-hook port.** The ~50 marketplace hooks are not re-implemented
  in any new runtime. Their *intents* are covered per §1.
- **Eval substrate untouched.** `~/.agents/evals/SCHEMA.md` is LOCKED and only
  EXTENDED (the Outcomes projection), never relitigated.
- **Holdout / eval corpus excluded from hosted bundles.** Managed Agents are not
  ZDR; an Agent bundle never carries holdout target/ground_truth/PII. The only
  thing that crosses the boundary is a holdout-stripped rubric and a score.

## See also

- [`/agent-native` skill](https://github.com/boshu2/agentops/blob/main/skills/agent-native/SKILL.md) — the how-to (this doc is the why/mechanism).
- `cli/cmd/ao/agent.go` (`ao agent bundle`), `cli/cmd/ao/mcp_serve.go` (`ao mcp serve`).
- `.github/workflows/agent-output-validate.yml` — the authoritative output gate.
- Fleet topology (bushido sandbox, tailnet `100.109.17.108`, Dolt 3306): operator hub.
- Open follow-ons: the managed-dispatch CLI (`ao managed dispatch`) and the
  self-hosted bushido Managed-Agents sandbox registration are the code/ops slices
  this doc unblocks.
