# Codex Skill API Contract

> Source of truth for what the Codex runtime reads from an AgentOps skill.
> Orientation contract: [`AGENTS.md`](../../AGENTS.md).

**Official docs:**
- [Codex Skills](https://developers.openai.com/codex/skills/)
- [Build skills and invocation policy](https://learn.chatgpt.com/docs/build-skills)
- [Codex Multi-Agent](https://developers.openai.com/codex/multi-agent/)
- [Codex CLI Features](https://developers.openai.com/codex/cli/features)

---

## One skills tree

Codex loads the canonical `skills/<name>/` packages directly. The plugin
manifest, [.codex-plugin/plugin.json](https://github.com/boshu2/agentops/blob/main/.codex-plugin/plugin.json),
ships `./skills`, and `ao skills link` links the same directories into
`~/.codex/skills`. No Codex copy of the skills is generated, and nothing under
`skills/` is rewritten for Codex.

The facts below were observed from the Codex loader (`skills/list` on
codex-cli 0.156.1) and are held by
[validate-codex-api-conformance.sh](https://github.com/boshu2/agentops/blob/main/scripts/validate-codex-api-conformance.sh),
which runs in `scripts/regen-all.sh --check`.

---

## SKILL.md Frontmatter

Codex uses two fields for discovery:

```yaml
---
name: skill-name
description: 'Explain when this skill triggers and when it does not.'
---
```

Canonical `skills/` frontmatter is host-extended: it also carries AgentOps
fields such as `skill_api_version`, `metadata`, `practices`, `user-invocable`
and `disable-model-invocation`. Codex ignores every field it does not know, so
they load without change. They are not portable Agent Skills frontmatter, and
this repository does not claim they are.

Codex refuses to load a skill when:

- the frontmatter is not a YAML mapping, or repeats a key;
- `description` is missing or empty;
- `name` is longer than 64 characters.

A missing `name` falls back to the directory name. The loader does not bound
the description length, but it may shorten the discovery list to fit its
context budget, so keep descriptions concise and front-load the key use case.

Codex walks the whole tree under a skill root. A `SKILL.md` nested below
`skills/<name>/` is loaded as a skill of its own, so fixtures and scaffolds
live outside `skills/`.

---

## Optional: agents/openai.yaml

A skill may include `agents/openai.yaml` for display metadata and policy:

```yaml
interface:
  display_name: "User-facing name"
  short_description: "Brief description"
  icon_small: "./assets/small-logo.svg"
  icon_large: "./assets/large-logo.png"
  brand_color: "#3B82F6"
  default_prompt: "Optional surrounding prompt"

policy:
  allow_implicit_invocation: false

dependencies:
  tools:
    - type: "mcp"
      value: "toolName"
      description: "Tool description"
      transport: "streamable_http"
      url: "https://example.com"
```

| Field | Purpose |
|-------|---------|
| `interface.display_name` | User-visible name in Codex UI |
| `interface.short_description` | Brief description for skill browser |
| `policy.allow_implicit_invocation` | `false` prevents auto-activation (explicit `$skill` only) |
| `dependencies.tools` | MCP server dependencies |

The Codex default for `policy.allow_implicit_invocation` is `true`. Setting it
to `false` prevents implicit activation while preserving explicit `$skill`
invocation.

Codex reads the invocation policy only from this file. It does not read
`disable-model-invocation` from `SKILL.md`. A skill marked
`disable-model-invocation: true` therefore carries the matching policy in its
own `skills/<name>/agents/openai.yaml`. The file is hand-maintained in the
source skill; nothing derives it. Without it, or when Codex cannot use it, Codex
selects the skill implicitly. The conformance check fails when the file is
missing, is not valid YAML, or lacks `policy.allow_implicit_invocation: false`.
It does not catch every file Codex drops: a non-object `interface` or
`dependencies.tools`, or the YAML 1.1 spelling `no` for the boolean, passes the
check and is ignored by Codex 0.156.1.

---

## Skill Discovery And Installation

The portable Codex skill root is `.agents/skills/` at repository or parent
scope and `$HOME/.agents/skills/` at user scope. Current Codex desktop installs
may also index `$HOME/.codex/skills/`. Inspect the actual package at each root:
a development symlink, a copied package and a plugin cache can coexist. A root
name alone does not establish which bytes the host loaded.

| Scope | Path | Use Case |
|-------|------|----------|
| Repo (nearest) | `.agents/skills/` from CWD | Folder-specific workflows |
| Repo (parent) | `../.agents/skills/` | Nested repo organization |
| Repo (root) | `$REPO_ROOT/.agents/skills/` | Organization-wide skills |
| User | `$HOME/.agents/skills/` | Portable personal skill collection |
| Host compatibility | `$HOME/.codex/skills/` | Codex-host symlink root when configured |
| Admin | `/etc/codex/skills/` | System-wide defaults |
| System | Bundled with Codex | Built-in skills |

A plugin install and a source link both resolve to the same `skills/<name>/`
content, but they are distinct installations. Host checks must record the
resolved loaded path, exact root/reference bytes, invocation policy and native
resource-read evidence for each tested installation. A catalog listing alone
proves neither invocation nor required-resource use. Separate explicit
invocation from normal catalog selection and leave unobserved routing or
loading unproven. Plugin caches are neither source nor an installation target
and may be deleted without affecting the canonical repository skills.

---

## Skill Invocation

| Method | Syntax | Description |
|--------|--------|-------------|
| Explicit | `$skill-name` or `/skills` menu | User directly requests the skill |
| Implicit | Automatic | Codex matches task to skill description |

Skills are loaded via **progressive disclosure**: metadata first (name,
description), full SKILL.md only when activated. The description is the
implicit-routing surface, and Codex reads the source description as written.

---

## Multi-Agent (Sub-Agents)

Current Codex releases enable subagent workflows by default. Codex only spawns
subagents when the operator or parent agent explicitly asks it to do so.
Subagents inherit the parent sandbox policy and live runtime overrides.

### Agent Roles

Codex ships with these built-in roles:

| Role | Purpose |
|------|---------|
| `default` | General-purpose fallback |
| `worker` | Execution-focused implementation |
| `explorer` | Read-heavy codebase exploration |

Custom agents live as standalone TOML files under `$HOME/.codex/agents/` for
personal agents or `.codex/agents/` for project-scoped agents. Each custom
agent file must define `name`, `description`, and `developer_instructions`.
Optional fields such as `nickname_candidates`, `model`,
`model_reasoning_effort`, `sandbox_mode`, `mcp_servers`, and `skills.config`
inherit from the parent session when omitted.

Global subagent limits stay in the `[agents]` section of `config.toml`:

```toml
[agents]
max_threads = 6
max_depth = 1
job_max_runtime_seconds = 1800

[agents.reviewer]
description = "Code review specialist"
config_file = "codex-reviewer.toml"
```

`agents.max_threads` defaults to `6`; `agents.max_depth` defaults to `1`,
which allows direct child agents but prevents deeper recursive fan-out.

### Batch Processing

`spawn_agents_on_csv` processes batches of similar tasks:

| Parameter | Description |
|-----------|-------------|
| `csv_path` | Source CSV file |
| `instruction` | Worker prompt template with `{column_name}` placeholders |
| `id_column` | Stable identifiers |
| `output_schema` | Fixed JSON structure for worker results |
| `output_csv_path` | Destination CSV containing row metadata and results |
| `max_concurrency` | Parallel worker limit |
| `max_runtime_seconds` | Worker timeout |

Workers call `report_agent_job_result` exactly once.

`sqlite_home` controls where Codex stores the SQLite-backed state used for
agent jobs and exported results.

### Codex Built-in Tools

Tools available inside a Codex agent session:

| Tool | Purpose |
|------|---------|
| `read_file` | Read file contents |
| `list_dir` | List directory contents |
| `glob_file_search` | Find files by pattern |
| `apply_patch` | Apply file edits (diff-based) |
| `rg` | Ripgrep search |
| `git` | Git operations |
| Shell/terminal tool | Shell command execution |
| `spawn_agent` | Create a focused sub-agent |
| `send_input` | Send follow-up input to a sub-agent |
| `wait_agent` | Wait for one or more sub-agents |
| `close_agent` | Stop a stuck or no-longer-needed sub-agent |

### Claude and Codex primitives

One skill body serves both hosts, so a body that depends on one host's
primitives is broken on the other. This table is for authors deciding how to
word a step.

| Claude Code | Codex | Write it as |
|-------------|-------|-------------|
| `Read` tool | `read_file` | "read the file" |
| `Edit` tool | `apply_patch` | "edit the file" |
| `Grep` tool | `rg` | "search for" |
| `Glob` tool | `glob_file_search` | "find files matching" |
| `Agent(subagent_type="Explore")` | Explorer agent role | "use a read-only helper" |
| `Skill(skill="name")` or `/name` | `$name` | the skill's name or a link to it |
| `TaskCreate` / `TaskList` / `TaskUpdate` | No equivalent (`todo_write`/`update_plan` not available, empirically verified) | native work tracking, named generically |
| `TeamCreate` / `TeamDelete` | No equivalent | omit |
| `SendMessage` | `send_input` for brief follow-up only | "send a follow-up" |
| `EnterPlanMode` / `ExitPlanMode` | No equivalent | omit |
| `EnterWorktree` | No equivalent | omit |
| `disable-model-invocation: true` | `agents/openai.yaml` with `policy.allow_implicit_invocation: false` | set both |

---

## Generated inventories

`skills/<name>/SKILL.md` metadata owns the skill inventory and dependency graph.
Run `scripts/regen-all.sh` after changing it. The CLI command surface is a
separate executable projection generated from Cobra; no handwritten CLI-to-skill
map is maintained.

---

## Codex Skill Maintenance

Codex is a first-class runtime in this repo.

- `skills/<name>/SKILL.md` is the behavior contract for every runtime.
- `skills/<name>/agents/openai.yaml` holds the Codex-only display metadata and
  invocation policy.
- There is no Codex-specific copy, override layer or treatment catalog.

When a skill change affects Codex behavior, phrasing, orchestration, or UX:

1. Change the source skill under `skills/`. Word the body so it holds on both
   hosts; see [Codex parity](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/references/codex-parity.md).
2. If the skill is explicit-only, keep its `agents/openai.yaml` policy in step
   with `disable-model-invocation`.
3. Validate:

   ```bash
   bash scripts/validate-codex-api-conformance.sh
   bash scripts/regen-all.sh --check
   ```

These checks establish that Codex can load the package and that the invocation
policy is declared. They do not establish that Codex selected or followed the
skill. That needs a session on the host, such as
`bash scripts/validate-headless-runtime-skills.sh`, which makes live model
requests.
