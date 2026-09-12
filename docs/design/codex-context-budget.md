# Codex context-budget design and evidence

Status: implementation and live proof in progress. Cutoff: 2026-09-12.
Acceptance is the two-job caller request recorded in private BD `age-z25n`.
The exact e32e88c verdict was written before this job began and posted at
https://github.com/boshu2/agentops/pull/1137#issuecomment-5648513520.

## Installed-runtime premises

(a) Refusal exists. `codex --version` returned `codex-cli 0.154.0`;
`codex features list` returned `hooks stable true` and `multi_agent stable true`.
The original Desktop session's native session metadata reports 0.153.4,
OpenAI `gpt-6-astra`, context `01a0974c-6f78-74c2-a43f-f0413b63ca6d`.
The CLI is a local stdin wrapper around the installed OpenAI package; it was
inspected and this work never invokes its noninteractive execution subcommand.

The [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
and [hook contract](https://learn.chatgpt.com/docs/hooks#pretooluse) were fetched
from the official documentation connector. `PreToolUse` is synchronous by
default; exit 2 plus stderr, or a JSON `permissionDecision: deny`, refuses
supported calls. Shell and unified exec use `Bash` / `tool_input.command`.
Canonical common fields include session_id, cwd, hook_event_name, model,
permission_mode and transcript_path; PreToolUse adds turn_id and tool_use_id.
Local hooks are discovered beside active config layers (`~/.codex/hooks.json`
or `<repo>/.codex/hooks.json`), with exact-definition trust via `/hooks`.
Untrusted project layers and untrusted hook definitions do not run. Shell
sandbox/approval policy also restrict operations, but is not a line-budget
predicate. Hosted tools and some specialized paths are not intercepted, and
write_stdin does not repeat PreToolUse for an already-running command.

`codex app-server generate-json-schema --out <scratch>/runtime/schema` also
succeeded, exposing HooksListParams/Response and hook notifications. The live
hook probe below checks actual invocation; the schemas alone do not prove it.

(b) [Custom-agent configuration](https://learn.chatgpt.com/docs/agent-configuration/subagents#custom-agents)
uses one TOML per role in `~/.codex/agents/` or `.codex/agents/`. Required fields
are name, description and developer_instructions. Optional model,
model_reasoning_effort and sandbox_mode configure the spawned session.
The name field identifies the role, not the filename. Spawned work has its own
agent thread and tool transcript. Existing context may be inherited unless the
caller requests a fresh context; this Desktop facade exposes fork_turns=none.
The same facade has no agent_type parameter, so an explicit role-prompt fallback
is documented without claiming implicit role discovery or sandbox application.
`codex debug prompt-input` emits messages without tool schemas; absence of role
names there does not establish whether custom-role discovery works.

(c) `codex debug models` refreshed the following native catalog. Visibility is
reported verbatim; hidden entries are not ordinary selectable task models.
The Desktop spawn schema separately exposes Astra, Sol, Terra, Luna and 5.5;
the app task-creation contract also lists Spark. This work uses no task-creation
API or substitute headless model process.

| Identifier | Catalog visibility | Runtime description |
|---|---|---|
| `gpt-6-astra` | list | Our most capable model for complex, demanding work. |
| `gpt-reserve` | hide | Fast and affordable agentic coding model. |
| `gpt-5.6-sol` | list | Reliable agentic workhorse for everyday tasks. |
| `gpt-5.6-terra` | list | Balanced agentic coding model for everyday work. |
| `gpt-5.6-luna` | list | Fast and affordable agentic coding model. |
| `gpt-5.5` | list | Proven previous-generation model for coding and general work. |
| `gpt-5.3-codex-spark` | list | Ultra-fast coding model. |
| `codex-auto-review` | hide | Automatic approval review model for Codex. |

Selected: `gpt-5.6-luna`, low effort for reading, medium for writing. The catalog
calls it fast and affordable. The fetched [official rate card](https://learn.chatgpt.com/docs/pricing#token-rates)
lists input/cached/output credits per million tokens: Luna 5/0.5/30,
Terra 50/5/300, Sol 100/10/500, Astra 250/25/1250, and 5.5 125/12.5/750.
Spark is a research preview without a comparable rate. Thus Luna is the cheapest
listed candidate with published comparable rates; hidden entries and unpriced
preview models do not establish a cheaper general reader. Adequacy is checked
by the live reader/writer tasks below, not inferred solely from the description.
The catalog itself contains no pricing; no account-specific charge is claimed.

## Three layers

Delegation source: `skills/agent-native/agents/bulk-reader.toml` and
`code-writer.toml`, mirrored by `scripts/regen-all.sh` into the existing Codex
skill bundle. `.codex/agents/` points to those sources for checkout discovery.
`scripts/install-codex-context-agents.sh` copies the generated templates for
personal or project use, preserving changed files with unique backups. The
removed `scripts/install-codex.sh` remains a tombstone. The plugin manifest
continues shipping `./skills-codex`; no new skill or automatic hook wiring.
Source guidance is shared, so the existing parity_only catalog treatment is
retained rather than inventing an override or editing generated twins.

Enforcement: optional native installer and thin Bash adapter reuse the same
read-budget predicate, waiver controls, sentinel behavior and hashed telemetry.
The installer leaves hook trust to Codex. Only verified canonical Bash input is
wired; no invented Read/read_file mapping. Rule scope and known fail-open cases
are the shared shell guard's contract. This can refuse covered shell calls,
but it is not comprehensive protection against arbitrary reads.

Advisory: `skills/agent-native/SKILL.md` and its
`references/context-budget-delegation.md` explain when/how to delegate, slices,
required references, fresh contexts and receipts. Reader read-only and writer
workspace-write are role defaults; parent live overrides can supersede them.
Target-only writes and content-free child replies are role instructions, not
output filtering or dynamic per-file confinement. Check output stays in the
child; the parent receives status only. A dead child means unknown side effects.
Native transcripts persist according to the runtime; no 10–30 second latency
or 90% savings claim is made for this implementation.

## Live proofs

Pending. Commands, native session IDs, context separation and available token
counts will be added after actual execution. Nothing in this section is yet PASS.

## Checks and delivery

Pending final subject validation and integration checks. BD resolves its private
Dolt store with `bd context --json`; `bd create` recorded `age-z25n` with the
original request and acceptance, and `bd update ... --status in_progress` ran.
Private tracker storage is not included in Git.
