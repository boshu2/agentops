# Codex Skill API Contract

> Source of truth for what the Codex runtime actually supports. All converter output and validation must conform to this contract.
> Orientation contract: [`AGENTS.md`](../../AGENTS.md).

**Official docs:**
- [Codex Skills](https://developers.openai.com/codex/skills/)
- [Codex Multi-Agent](https://developers.openai.com/codex/multi-agent/)
- [Codex CLI Features](https://developers.openai.com/codex/cli/features)

---

## SKILL.md Frontmatter

Codex recognizes **only** these frontmatter fields:

```yaml
---
name: skill-name
description: 'Explain when this skill triggers and when it does not.'
---
```

**Required:** `name`, `description`
**Everything else is ignored.** Fields like `skill_api_version`, `context`, `metadata`, `allowed-tools`, `model`, `user-invocable`, and `output_contract` are AgentOps-internal and must be stripped from Codex output.

---

## Optional: agents/openai.yaml

Codex skills may include `agents/openai.yaml` for display metadata and policy:

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

---

## Skill Discovery Paths

Codex scans these directories (in order):

| Scope | Path | Use Case |
|-------|------|----------|
| Repo (nearest) | `.agents/skills/` from CWD | Folder-specific workflows |
| Repo (parent) | `../.agents/skills/` | Nested repo organization |
| Repo (root) | `$REPO_ROOT/.agents/skills/` | Organization-wide skills |
| User | `$HOME/.agents/skills/` | Personal skill collection |
| Admin | `/etc/codex/skills/` | System-wide defaults |
| System | Bundled with Codex | Built-in skills |

**NOT:** `~/.claude/skills/` or `~/.codex/skills/` — these are Claude Code paths.

---

## Skill Invocation

| Method | Syntax | Description |
|--------|--------|-------------|
| Explicit | `$skill-name` or `/skills` menu | User directly requests the skill |
| Implicit | Automatic | Codex matches task to skill description |

Skills are loaded via **progressive disclosure**: metadata first (name, description), full SKILL.md only when activated.

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

### Claude → Codex Primitive Mapping

| Claude Code | Codex Equivalent | Converter Action |
|-------------|-----------------|------------------|
| `Read` tool | `read_file` | Map |
| `Edit` tool | `apply_patch` | Map |
| `Grep` tool | `rg` | Map |
| `Glob` tool | `glob_file_search` | Map |
| `Agent(subagent_type="Explore")` | Explorer agent role | Map |
| `Skill(skill="name")` | `$name` invocation | Map |
| `TaskCreate` / `TaskList` / `TaskUpdate` | No equivalent (`todo_write`/`update_plan` not available — empirically verified) | Strip |
| `TeamCreate` / `TeamDelete` | No equivalent | Strip |
| `SendMessage` | `send_input` for brief follow-up only | Rewrite or strip |
| `EnterPlanMode` / `ExitPlanMode` | No equivalent | Strip |
| `EnterWorktree` | No equivalent | Strip |
| `context.window` | No equivalent | Strip from frontmatter |
| `context.sections.exclude` | No equivalent | Strip from frontmatter |
| `context.intel_scope` | Intelligence scoping | Does not exist |

Skills referencing these primitives produce **broken instructions** in Codex.

---

## Converter Requirements

When generating Codex skills from source skills:

1. **Strip all non-Codex frontmatter** — emit only `name` + `description`
2. **Map Claude tools to Codex tools** — Read→read_file, Edit→apply_patch, Grep→rg, Glob→glob_file_search
3. **Rewrite `Skill(skill="X")` to `$X`** — Codex uses dollar-prefix invocation
4. **Strip ALL task/team primitives** — TaskCreate, TaskList, TeamCreate, SendMessage (none have working Codex equivalents as direct tool calls — `todo_write`/`update_plan` empirically unavailable, and `send_input` is follow-up-only)
5. **Fix paths** — `~/.claude/skills/` → `~/.agents/skills/` (Codex discovery path)
6. **Rewrite reference files** — `.md` files in references/ pass through `codex_rewrite_text()` during copy
7. **Preserve skill body** — the SKILL.md body (instructions) is the skill's value; keep it functional

---

## Validation Criteria

A Codex-conformant skill must:

1. Have frontmatter with only `name` and `description`
2. Contain no Claude-only primitive names (TaskCreate, TeamCreate, SendMessage, etc.)
3. Contain no Claude-specific paths (`~/.claude/`, `~/.codex/`)
4. Have valid `agents/openai.yaml` if present
5. Not reference non-existent Codex features (context controls, plan mode, etc.)

---

## CLI Skill-Map Refresh

After changing `ao` command usage in any of these locations, refresh [`docs/cli-skills-map.md`](../cli-skills-map.md):

- `skills/*/SKILL.md`
- `skills-codex/*/SKILL.md`

Process:

1. Update the map from current sources.
2. Run `bash tests/docs/validate-doc-release.sh` and `bash tests/docs/validate-skill-count.sh` before pushing.

---

## Codex Skill Maintenance

Codex is a first-class runtime in this repo.

- `skills/<name>/SKILL.md` is the canonical behavior contract.
- `skills-codex-overrides/<name>/` is the Codex-specific tailoring layer.
- `skills-codex-overrides/catalog.json` is the machine-readable treatment map for the full catalog.
- `skills-codex/<name>/` is the checked-in Codex runtime artifact. It is manually maintained; legacy manifest/marker files remain part of the validation contract.

**Editing an EXISTING parity skill regenerates its Codex twin — not just hashes.**
`make regen-all` / `scripts/codex-sync.sh` refresh parity-only twins from
`skills/<name>/` whenever their generated body, prompt, or mirrored references
drift. `scripts/regen-codex-hashes.sh` remains the bookkeeping step after
content is current. Manual edits under `skills-codex/<name>/` are reserved for
bespoke skills or deliberate Codex-only divergence recorded in
`skills-codex-overrides/catalog.json`; otherwise fix the source skill or the
codex-sync transform/template and regenerate.

**Bespoke twins are HAND-MAINTAINED in full — body AND references (age-0js4).**
A `treatment: bespoke` twin (catalog.json — `council`, `crank`, `evolve`,
`plan`, `premortem`, `research`, `rpi`, … 19 total) is skipped ENTIRELY by
codex-sync, **including `--force`**. Its `SKILL.md` body and everything under
`references/`/`scripts/` are authored by hand: many bespoke references are
deliberate Codex-condensed rewrites of source (e.g. `research/references/
data-flow-from-entry-points.md` is a substantial hand-rewrite — 85 source lines
deleted, 56 added), so a source edit does **NOT** auto-propagate, and
`codex-sync --force --only <bespoke>` reporting "nothing to generate" is
CORRECT, not a bug. Refreshing a bespoke twin after a source change is a
deliberate human edit of the twin. **Do not** auto-mirror source over a bespoke
twin — it would clobber the hand-authored Codex copy. (Auto-refresh was
evaluated under age-0js4 and rejected: dozens of the bespoke reference files are
genuine hand-rewrites; only an explicit per-file tracked/bespoke manifest could
refresh safely, which is disproportionate to the low-frequency cost. *Accidental*
drift — a twin that should have tracked source but didn't — is the divergence
gate's job, tracked under age-odv to add an explicit bespoke exemption, not
codex-sync's.)

**Pointer twins are exempt from the mirror requirement (`parity_policy: pointer`).**
Distinct from bespoke: some twins are deliberately THIN POINTERS — they carry no
mirrored prose, just "the source skill is the source of truth — read it first"
plus a short Codex Runtime Contract (e.g. `pawl-review`, `agent-mail`,
`ntm`; ~16 of them). For these there is nothing to mirror, so a source-only prose
edit must NOT demand twin churn. Declare it once in the twin's frontmatter:

```yaml
parity_policy: pointer   # twin defers to the source body; exempt from source-divergence
```

`validate-codex-generated-artifacts.sh` (`twin_is_pointer`) then skips the
SKILL.md-body and references divergence gates for that twin. Use this ONLY for a
genuine pointer — a twin that duplicates source prose must stay a full mirror and
keep the marker off, so its divergence gate still fires. The twin's own content
(incl. its Codex Runtime Contract) is still validated by the source→codex
existence check and the manifest/hash audit. Marking the existing ~16 pointer
twins is tracked under age-backfill-pointer-twin-markers-uco.

When a skill change affects Codex behavior, phrasing, orchestration, or UX:

1. Update the source skill under `skills/` when the shared contract changes.
2. For parity-only skills, update source or the codex-sync transform/template and regenerate. Update `skills-codex/<name>/SKILL.md` directly only when the Codex runtime copy is bespoke, or update `skills-codex-overrides/<name>/` when the Codex experience should differ from Claude.
   - Prompt/operator-layer changes belong in `skills-codex-overrides/<name>/prompt.md`.
   - Durable Codex-only body rewrites belong in `skills-codex-overrides/<name>/SKILL.md`.
3. Run the semantic audit if the checked-in Codex body looks suspicious:

   ```bash
   bash scripts/audit-codex-parity.sh
   # or target one skill
   bash scripts/audit-codex-parity.sh --skill <name>
   ```

4. Validate the checked-in Codex artifacts:

   ```bash
   bash scripts/audit-codex-parity.sh
   bash scripts/validate-codex-override-coverage.sh
   bash scripts/validate-codex-generated-artifacts.sh --scope worktree
   bash scripts/validate-codex-backbone-prompts.sh
   bash scripts/validate-codex-rpi-contract.sh
   bash scripts/validate-codex-lifecycle-guards.sh
   bash scripts/validate-headless-runtime-skills.sh
   ```

Think of `skills/` as the shared contract, `skills-codex-overrides/` as the durable Codex-only tailoring layer, and `skills-codex/` as the checked-in Codex artifact shipped to users.
