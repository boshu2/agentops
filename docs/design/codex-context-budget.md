# Codex context-budget design and evidence

Status: implemented; observed runtime behavior and remaining limits below.
Evidence cutoff: 2026-09-12. Final gate results are recorded below.
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
Local hooks use `~/.codex/hooks.json` or the primary checkout's
`.codex/hooks.json`, with exact-definition trust via `/hooks`.
Untrusted project layers and untrusted hook definitions do not run. Shell
sandbox/approval policy also restrict operations, but is not a line-budget
predicate. Hosted tools and some specialized paths are not intercepted, and
write_stdin does not repeat PreToolUse for an already-running command.

Independent credentials-free `config/read` and `hooks/list` probes confirmed
personal discovery and trusted ordinary-project discovery. Project trust must
be persisted in the isolated `CODEX_HOME/config.toml`; a command-line trust
value alone did not enable that layer. On 0.154, a linked worktree reads hooks
from the primary checkout even though it reads agent config from the linked
worktree. Installing hooks in both an isolated main and linked checkout returned
only the main hook's `sourcePath`. Evidence:
`runtime/hooks-discovery-audit.61iv_4je/linked-both-configs/result.json` under
the external scratch directory. The installer therefore rejects `--project`
in a linked worktree before writing anything and recommends personal install
or running in the primary checkout. An explicit `CODEX_HOOKS_FILE` remains a
caller-selected destination. Discovery does not itself establish hook trust.

`codex app-server generate-json-schema --out <scratch>/runtime/schema` also
succeeded, exposing HooksListParams/Response and hook notifications. The live
hook probe below checks actual invocation; the schemas alone do not prove it.

(b) [Custom-agent configuration](https://learn.chatgpt.com/docs/agent-configuration/subagents#custom-agents)
uses one TOML per role in `~/.codex/agents/` or `.codex/agents/`. Required fields
are name, description and developer_instructions. Optional model,
model_reasoning_effort and sandbox_mode configure the spawned session.
The name field identifies the role, not the filename. On this installed CLI,
standalone project files did not make the name available in two live attempts
(`01a09758-99ee-7291-9e5d-ebb4fa19560c` and
`01a0975a-f888-73a0-bab9-d709800798fb`). Explicit
`[agents.bulk-reader]` / `[agents.code-writer]` entries with `description` and
`config_file` resolved the name successfully. The checkout therefore includes
`.codex/config.toml` registrations and the installer registers both names using
the installed runtime's `config/batchWrite` TOML editor in an isolated staging
home. It preserves unrelated configuration and publishes only after success;
it does not start a model session. Invoke a registered role with
`spawn_agent(agent_type="bulk-reader", fork_turns="none", ...)`; the native
reader proof below confirms `agent_role: bulk-reader`.

Spawned work has its own
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
skill bundle. `.codex/agents/` points to those sources, and `.codex/config.toml`
registers both names for checkout use.
`scripts/install-codex-context-agents.sh` copies the generated templates for
personal or project use, preserving changed roles and config with unique
backups. It requires Node and an installed Codex with `config/batchWrite`. The
removed `scripts/install-codex.sh` remains a tombstone. The plugin manifest
continues shipping `./skills-codex`; no new skill or automatic hook wiring.
Source guidance is shared, so the existing parity_only catalog treatment is
retained rather than inventing an override or editing generated twins.

Enforcement: `scripts/install-codex-read-budget-guard.sh` and the
opt-in `skills/cc-hooks/hooks/codex-read-budget-guard.sh` adapter reuse the same
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

Evidence files below live under the external scratch directory
`/tmp/agentops-context-budget.TWbdWl/live-proof`; native transcripts remain under
`~/.codex/sessions/2026/09/12/`. Raw transcripts and private tracker data are not
copied into Git. Quoted receipts contain summaries and metadata only.

### Reader: observed parent/child separation

Interactive command, with the concrete checkout path substituted for `$REPO`:

```sh
codex --no-alt-screen \
  -c 'agents.bulk-reader.description="Read one large file in slices; return findings only"' \
  -c 'agents.bulk-reader.config_file="/tmp/agentops-context-budget.TWbdWl/native/skills/agent-native/agents/bulk-reader.toml"' \
  -C "$REPO" '<delegate a question about tests/scripts/probe-skill.bats; parent must not read it>'
```

This was an actual interactive CLI session. The prompt asked which behaviors
`tests/scripts/probe-skill.bats` verifies, required six separate tool invocations
with at most 350 lines each, and prohibited parent reads. Parent transcript
`01a09776-004f-79a0-af85-c75b472a1e68` contains only `spawn_agent` and two
`wait_agent` calls. Spawn requested `agent_type: bulk-reader`, `gpt-5.6-luna`
and `fork_turns: none`; returned identity `/root/probe_skill_reader`.

Child native metadata `01a09776-3c38-7281-b582-01fef5101f17` records that parent,
`agent_role: bulk-reader`, model `gpt-5.6-luna`, effort `low`. Its six tool calls
contain respectively the `sed -n` ranges `1,350p`, `351,700p`, `701,1050p`,
`1051,1400p`, `1401,1750p`, `1751,1772p`. Transcript inspection found zero
truncation markers in all six responses. Its reply contained five `{ref,text}`
findings and `"lines_covered":1772,"complete":true`; no file dump entered
the parent. Example: `tests/scripts/probe-skill.bats:200-407` — verifies replay
provenance, immutable metadata, transcript integrity and drift handling.

Native cumulative token accounting (input includes cached input):

| Session | Input | Cached input | Output | Total | Duration |
|---|---:|---:|---:|---:|---:|
| Reader parent | 105,760 | 91,520 | 897 | 106,657 | 59,608 ms |
| Reader child | 293,281 | 249,088 | 840 | 294,121 | 27,856 ms |

These are cumulative request tokens including instruction/bootstrap overhead,
not unique context occupancy or measured savings. Native context-window capacity
was 258,400. No direct-read control run or cost reduction is claimed.
Two earlier reader attempts are excluded from the successful coverage proof:
the facade attempt ended with a 372-line slice; registered child
`01a09761-5d5e-7ff0-8471-bcee884315fd` combined six valid slices into one response
that was truncated without a complete reread. The latter also self-reported an
incorrect generic model identity. Those failures motivated the source role's
one-slice-per-invocation instruction. Model claims above use native metadata,
not self-report; parent/child separation alone never proves full coverage.

### Writer: observed receipt-only completion

The interactive CLI was started with explicit `agents.code-writer.description`
and `agents.code-writer.config_file` pointing to the source template, just as
in the reader invocation. The prompt required exactly one native child with
`agent_type: code-writer`, Luna, fresh context, one test target and a required
reference. Parent `01a09771-8908-7a00-b101-919b558cf8c1` called only `spawn_agent`
and `wait_agent`; it never read a file. Required reference:
`tests/scripts/codex-context-agents.bats`. Task: create a standalone Bats test
that checks `printf` produces the exact string `native-ready`.

Child `01a09771-bbbf-7232-add4-cc9ec55ad759` records that parent,
`agent_role: code-writer`, observed `gpt-5.6-luna`, effort `medium`.
It wrote the target, ran `bats` once and returned this receipt:

```json
{"target":"/tmp/agentops-context-budget.TWbdWl/live-proof/registered-smoke.bats","written":true,"lines":7,"check_ran":true,"check_ok":true,"summary":"Created standalone Bats smoke test matching reference conventions; bats check passed."}
```

The coordinating Desktop parent independently ran `bats <target> >
<scratch>/registered-parent-check.log 2>&1`, exit 0, without reading the target.
Transcript extraction inspected call names, accounting and the final receipt;
it did not import child write payloads or generated source into either parent.

| Session | Input | Cached input | Output | Total | Duration |
|---|---:|---:|---:|---:|---:|
| Writer parent | 78,424 | 58,496 | 358 | 78,782 | 47,974 ms |
| Writer child | 155,441 | 127,744 | 1,097 | 156,538 | 26,124 ms |

These totals have the same cumulative-accounting limitation as the reader table.
The earlier Desktop facade fallback also completed a seven-line Bats file and
passed its check (`01a0975d-e448-70a2-8d24-18d6c188dd26`, Luna/medium,
`agent_role: null`); it is not used to claim named-role discovery.

### Hook: actual synchronous refusal

After `bash scripts/install-codex-read-budget-guard.sh --project`, a temporary
wrapper captured the native payload for one controlled synthetic file and
forwarded it to the installed adapter. The interactive CLI was launched with
an explicit session `hooks.PreToolUse` definition matching `^Bash$`, command
`bash <scratch>/capture-hook.sh`, timeout 10. Its exact definition was reviewed
and trusted in `/hooks`; existing user hook settings were not enabled or disabled.
Native parent: `01a09769-14d2-7c22-9b7d-50847de07c90`.

The actual captured input included `hook_event_name: PreToolUse`,
`tool_name: Bash`, `tool_input.command`, `cwd`, `model: gpt-6-astra`,
`session_id`, `turn_id` and `tool_use_id`. A 400-line fixture was used:
`cat <scratch>/large-proof.txt` was refused before execution;
`sed -n '1,3p' <scratch>/large-proof.txt` succeeded with exit 0 and three lines.
No retry or waiver was used. The initial turn was
`01a0976a-5f04-7c13-ae51-704b1f783399`. The final shared guard was reinstalled
and the same pair rerun after repairs in turn
`01a09771-25f2-7830-830b-498d7ca1945e` (again denied / exit 0);
its source and installed SHA-256 both
were `c5a4c73598c7eed1e2b058534c309f96b7c77dd4e6baf4409b2f62e5724f6590`.

The deny ledger contains one hashed entry per denied call: policy
`core.context:unbounded-read`, `tool: Bash`, `lines: 400`, `budget: 350`,
`mode: deny`, `decision: deny`, a SHA-256 path, timestamp and session ID.
It contains no raw path or command; allowed slices add no line. The separate
explicit proof capture contains synthetic command metadata solely to verify
the real runtime shape and is not the product telemetry stream.

## Checks and delivery

The repair PR is [#1139](https://github.com/boshu2/agentops/pull/1139), separate
from the Codex-native branch. Its final exact-content review of `60779f5bc`
found no remaining reproduced major defect; 33 gates and 196 tests passed.
Its live Claude criteria remain NOT_PROVEN.

`bash tests/run-all.sh` exited 0: 10 passed, 0 failed, 1 skipped (optional OL
integration directory absent). This is the repository's default static tier;
it does not establish live Claude integration. Final native routed gates and
fresh review are recorded with the PR once complete.

BD resolves its private
Dolt store with `bd context --json`; `bd create` recorded `age-z25n` with the
original request and acceptance, and `bd update ... --status in_progress` ran.
Private tracker storage is not included in Git.
