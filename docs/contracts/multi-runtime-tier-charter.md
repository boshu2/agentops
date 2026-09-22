# Multi-runtime tier charter

This contract owns the host/install surface mapping and the meaning of its
validation tiers. [Install and day-2 operations](../install-day2-ops.md) owns
operator steps; [migration](../MIGRATION.md) owns old-name dispositions.
Claude Code and Codex remain first-class installs. Cursor and OpenCode retain
their declared structural coverage; Gemini/Antigravity retains its packaged
compatibility surface. Missing live evidence does not retire a promised journey.

## Evidence tiers

| Tier | What establishes it | What does not establish it |
|---|---|---|
| S: structural / install smoke | Source files, manifests, generated bundles, exports and isolated filesystem installation checks agree with their owners. | A file on disk does not show that a host discovered or loaded it. |
| I: actual inventory / load | A real host session exposes the intended inventory and loads the selected installed skill and its required references from the identified source. | CLI `--help`, manifest validation, package-manager listings and a successful link operation alone. |
| E: live execution | An authorized real host completes the supported journey against frozen acceptance, with actual results and fresh author-distinct judgment. | Startup, inventory, model self-report, exit status or an output envelope alone. |

The blocking check roster is maintained in [CI](../../.github/workflows/validate.yml),
the [install CI workflow](../../.github/workflows/install-e2e.yml) and the
[gate registry](../../cli/internal/gates/checks/seed.go), explained in
[CI/CD](../CI-CD.md). Structural checks need no model request or authentication.
The standalone smoke scripts below are available checks, not a claim that each
is wired into every CI run. No live execution tier is a default CI gate.
Live inventory and execution require explicit authority, available host access
and real runtime bounds; missing access is missing evidence, never a pass.

## Host and install surface mapping

This is the single mapping of retained install consumers. The canonical catalog
comes from [skills](../../skills/catalog.json); metadata-owned bundles are
regenerated through [regen-all](../../scripts/regen-all.sh).

| Consumer | Retained surface and owner | Structural checks | Actual load / execution obligation |
|---|---|---|---|
| Claude Code, first-class | Managed plugin: [.claude-plugin](../../.claude-plugin/plugin.json), canonical `skills/`, [agents](../../agents/) and [policy dispatcher](../../hooks/hooks.json). Source links: detected `~/.claude/skills`. | [Claude smoke](../../tests/skills/test-runtime-claude-code-smoke.sh), [manifest validation](../../scripts/validate-manifests.sh). | A fresh session must discover the chosen installation, load selected guidance and complete its accepted journey. Plugin inventory alone is insufficient. Optional hooks have separate activation/effect proof. |
| Codex, first-class | Managed plugin: [.codex-plugin](../../.codex-plugin/plugin.json) points at generated `skills-codex/` under the [Codex API contract](codex-skill-api.md). Source links expose canonical `skills/` in detected `~/.codex/skills`. [Native roles](../../scripts/install-codex-context-agents.sh) and [read-budget hook](../../scripts/install-codex-read-budget-guard.sh) are separate opt-ins. | [Codex smoke](../../tests/skills/test-runtime-codex-smoke.sh), [bundle check](../../scripts/validate-codex-install-bundle.sh), generated parity checks in `regen-all.sh --check`. | Qualify the chosen plugin or source-link path independently. Confirm actual loaded content and names; plugin names use `agentops:`, source links use their catalog names. Copied roles and trusted hooks need separate upgrade checks. |
| Cursor, retained structural coverage | [Converter](../../skills/skill-builder/scripts/converter/convert.sh) exports `.mdc` rules; source linking also detects `~/.cursor/skills`. | [Cursor export smoke](../../tests/skills/test-runtime-cursor-smoke.sh); source-link tests below cover destination mechanics. | No maintained automated inventory/execution lane is declared here. An authorized native session must establish discovery, selected loading and any claimed execution; export success proves only S. |
| OpenCode, retained structural coverage | Canonical skills through portable `~/.agents/skills` or explicit `--dest ~/.config/opencode/skills`; [OpenCode install guide](../../.opencode/INSTALL.md) also describes optional plugin hooks. Automatic fan-out does not detect its dedicated config root. | [OpenCode smoke](../../tests/skills/test-runtime-opencode-smoke.sh), including explicit-destination installation and protection of existing entries. | No maintained automated inventory/execution lane is declared here. Qualify actual discovery and execution separately, including optional hooks when selected. |
| Gemini / Antigravity, retained compatibility package and export | [Gemini image](../../images/gemini/README.md), generated [plugin manifest](../../images/gemini/plugin.json), bundled skills, agents, rules, hooks and optional Agent Mail configuration. The wrapper is migration-only compatibility under that owner. Source linking detects `~/.gemini/skills`. | [Image verification](../../images/gemini/verify.sh) checks inventory and byte identity; available `agy plugin validate` checks package shape. | Package validation is not Gemini or Antigravity load proof. Each claimed host journey, optional dependency and migration needs its own native evidence. |
| Pi and portable source-link consumers | Existing [destination resolver](../../cli/internal/skillsapp/roots.go) always includes `~/.agents/skills` and detects `~/.pi/skills`; `--dest` selects one explicit root for other consumers. | [Source-link tests](../../cli/internal/skillsapp/link_test.go) and [unlink tests](../../cli/internal/skillsapp/unlink_test.go). | These are existing filesystem consumers, not a new claim of live host qualification. Host discovery and behavior remain separately unproven until exercised. |
| `npx skills` consumers | External Skills installer reads this repository's skills. `npx skills@latest add boshu2/agentops --all -g` requests all skills and all agents supported by that installer; it does not install the runtime plugin, AO, roles or hooks. | Repository catalog/frontmatter checks cover the input. The external installer's current `--help` owns its selection, link/copy and update semantics. | Record installer version, selected agents and installed source. External installation success does not establish any host's I or E tier. |

Source-link discovery is defined by the resolver, not by detecting running
applications: existing `~/.claude`, `~/.codex`, `~/.gemini`, `~/.cursor` and
`~/.pi` directories select their `skills/` roots. `--dest` overrides this fan-out.
Selection with repeated `--skill` never removes unselected skills and does not
install dependencies. Real files/directories and foreign or wrong links remain
conflicts for explicit owner resolution.

## Existing runtime probe limits

[validate-headless-runtime-skills.sh](../../scripts/validate-headless-runtime-skills.sh)
is a legacy diagnostic with unequal evidence strength:

- Its Claude path runs `claude --plugin-dir <checkout> --help`. That establishes
  CLI availability/argument acceptance, not actual plugin inventory or loading.
- Its Codex path creates source links and requests a model-generated inventory.
  The names comparison is narrower than tracing selected content loading or
  executing a journey. It may copy authentication into an isolated home and make
  live model requests, so it must not run as an unauthorized structural check.
- Its advertised load-check fallback invokes `codex exec --help` and requires a
  legacy install marker that the source-link path does not create. A fallback,
  warning or skip cannot satisfy I or E. `HEADLESS_RUNTIME_SKILL_CODEX_STRICT=1`
  rejects inventory fallback; it does not add content-loading proof.

Do not promote the script's diagnostic output or exit status into a
release qualification. Its behavior is recorded here without changing the
accepted host outcomes.

## Authorized live journeys

Use the installed host's native help before dispatch. For a managed plugin,
select `/agentops:research` inside Claude Code or `$agentops:research` inside
Codex, as shown in the [quickstart](../../README.md#try-one-task). A Codex CLI
prompt is positional, not a `--prompt` flag:

```bash
# Only within an authorized, prepared consumer checkout and installed plugin:
codex exec --sandbox read-only '$agentops:research Trace repository input validation. Cite the loaded skill and relevant source files; change no files.'
```

Use `$research` for the canonical source-linked skill instead. A read-only
research invocation can establish a bounded loading/use observation; it does
not alone prove implementation, independent validation, or upgrade recovery.
Claude's interactive route is supported; do not infer missing headless support
from absence of an AgentOps automation lane. Cursor/OpenCode and
Gemini/Antigravity observations likewise use their native authorized hosts.

Before release qualification, independently exercise the final declared
journeys from reproducible installs, including selective retrieval, cold
resume, final names, stale-copy collisions, upgrades and one recovery from a
failed/partial upgrade with user-owned directories intact. Shared project
context must be discoverable from a clean consumer checkout. Record exact
source/version, install path, host/model, observed loaded content, checks and
remaining gaps. Final G2 qualification owns this live evidence; a contract
repair or earlier-source result does not complete it. Reducing promised
behavior requires a caller decision, not a new caveat in this table.

## Related owners

- [Install and day-2 operations](../install-day2-ops.md) and [migration](../MIGRATION.md).
- [Runtime neutrality](runtime-neutrality.md) and [Codex skill API](codex-skill-api.md).
- [Hook event reference](../../skills/cc-hooks/references/HOOK-EVENTS.md) and
  [Codex context-budget design](../design/codex-context-budget.md).
- [RPI evidence semantics](../architecture/rpi-traversal.md), when that workflow
  is selected; no runtime has a mandatory RPI invocation.
