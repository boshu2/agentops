# AgentOps migration

AgentOps now owns one small product boundary:

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

(The 3.0 through 3.6 releases stopped after one validation; ADR-0017 added the
bounded repair phase.)

Known defects can be repaired directly and an approach can change within the
accepted outcome, scope and real allowance. The caller owns changes to that
acceptance, new allowances, scheduling, trackers, Git and delivery. Deterministic repository checks stay
under `ao gate check`; semantic judgment belongs to a fresh reviewer.

## Native default — 2026-09-10

The default requires zero AgentOps skills. `ao quick-start` and `ao demo` give
read-only native guidance. `ao demo --rpi` retains the explicit workflow example.
`ao init` remains optional evidence setup. No session bootstrap is required.

The skill sources, explicit full plugins and no-selector `ao skills link` are
preserved. From a checkout, repeat `--skill NAME` to link selected skills only.
Selection does not uninstall existing links or rewrite user configuration.
The optional RPI and Validate skills retain their contracts; native work uses
the same exact-content, author-distinct judgment bar without invoking them.

## Removed responsibilities

| Removed command or surface | Surviving alternative |
|---|---|
| `ao pawl` | Invoke the Validate skill for one independent semantic verdict. |
| `ao plan-pawl` | Invoke `premortem` when the caller wants an advisory plan challenge. |
| `ao validate` | Use the Validate skill; use `ao gate check` only for deterministic checks. |
| `ao land` | Use the repository's Git or CI delivery process. |
| `ao done`, `ao close` | Report the result to the caller; AgentOps does not close work. |
| `ao governor`, `ao converge` | The caller decides whether to start a new invocation or revision. |
| `ao yield` | Observe throughput in the selected runtime or external system. |
| `ao claim`, `ao next-work` | Use the caller's tracker or substrate directly. |
| `ao state`, `ao reconcile` | Inspect the bead or caller intent, derived subject manifest, verdict, and generic provenance as read-only evidence. |
| `ao worktree` | Use Git directly. |
| `ao membrane` | Record observations as Validate findings or generic provenance. |
| `ao crank` | Call an executor directly or use the optional `dispatch_once` adapter. |
| `ao constraint` | Encode accepted mechanical policy in repository-owned linters or checks; AgentOps no longer promotes findings into blocking state. |
| `ao skills edit` | Edit canonical `skills/<slug>/` sources directly; use normal repository Git policy outside `ao`. |
| `ao goals trace` | Inspect current goal/scenario artifacts directly; the retired directive-to-bead lifecycle chain has no replacement. |
| `ao inject` | Use optional Memory recall over caller-owned, authorized context sources; there is no replacement CLI retrieval controller. |
| `ao session memory` | Use caller-authored `ao session handoff` evidence or maintain repository memory through the caller's own policy. |
| `ao config models` | Model-tier configuration was removed; nothing consumed it. Model choice belongs to the caller's runtime. Existing `models:` config sections still parse and are ignored. |
| `ao verify` | Use the Validate skill for semantic judgment and `ao gate check` for deterministic checks. Delete any `ao verify init` pre-push ratchet from `.git/hooks/pre-push` (restore `pre-push.agentops-orig` if one was set aside); `ao verify init --remove` no longer exists, and `git push --no-verify` bypasses a stale hook once. |
| `ao flywheel` | The CLI surface remains retired; existing `flywheel:` config sections still parse and are ignored. The optional Memory skill can review useful episodes and curate topic pages. They do not automatically compute compounding or claim benefit without later work. |
| `ao eval` | The offline eval surface was retired unconsumed (no gate, workflow, or script ran it); use a repository-selected evaluator and record the result as generic `ao provenance` evidence. |
| `ao redact` | Its only declared caller (the compile skill's render-write) never existed. Use owner-authorized disclosure review before storage; removing or replacing this command does not authorize reading restricted sources. |

These names are no longer registered commands. Invoking one fails as an
unknown command (exit 1) and prints the matching replacement pointer from the
table above; nothing forwards to old code or mutates old state.

Other 3.2 bookkeeping and knowledge verbs (`ao beads`, `ao agents`, `ao canon`,
`ao ci`, `ao citation`, `ao findings`, `ao forge`, `ao knowledge`,
`ao mcp`, `ao metrics`, `ao notebook`, `ao patterns`, `ao pool`, `ao ratchet`,
`ao registry`, `ao scope`, `ao sessions`, `ao wiki`) were pruned from the
default build without tombstones. They have no replacement inside AgentOps; use
the caller's own tools, `ao gate check` for deterministic checks, or generic
`ao provenance` records.

## Config file location

`~/.agentops/config.yaml` and `./.agentops/config.yaml` moved to
`~/.agents/ao/config.yaml` and `./.agents/ao/config.yaml`. The legacy paths are
still read as a fallback for this release (with a deprecation warning on
stderr) when no file exists at the new path; move the file to silence the
warning:

```bash
mkdir -p ~/.agents/ao && mv ~/.agentops/config.yaml ~/.agents/ao/config.yaml
```

## Upgrade from 3.6 to 3.7

Version 3.7 removes published commands and skill entry points. Update explicit
invocations before upgrading automation; the new names are not compatibility
aliases. Ordinary native coding requires no replacement invocation.

- Replace `ao eval` calls with the evaluator selected by your repository. Replace
  `ao redact` calls with the owner's disclosure-review process. Both command
  names now fail with a migration pointer. Generic `ao provenance` records can
  retain the resulting facts, but do not run either retired service.
- Replace scripted `workflows/rpi.js` invocations with native execution, or invoke
  the optional RPI skill explicitly. The skill retains the outcome-to-judgment
  contract; it does not recreate the old script's retry controller.
- Existing `.agents/` evidence stays where its owner placed it. New CDLC proof
  requires a selected protected external non-Git destination; a missing route
  must not silently write new proof into the repository.
- The build toolchain is Go 1.27.1 (`cli/go.mod` still declares Go 1.26.0 as its
  language floor). An older local toolchain may download the selected toolchain
  or fail according to `GOTOOLCHAIN`.

## Skills

The release menu has 34 skills, compared with 52 in 3.6.0. Twenty former roots
were retired; `memory` and `skill-eval` are new relative to that release. Use
[the current menu](SKILL-ROUTER.md) to choose guidance for the actual task.

| Retired 3.6 skill | Current owner or migration |
|---|---|
| `learn`, `toil-mining` | `memory` for explicitly requested recall, mining or curation. |
| `codebase-recon`, `pattern-mining` | `research` for cited local questions, recon packs and pattern evidence. |
| `bootstrap`, `handoff` | `doc` for requested missing documents and factual continuity handoffs; native work needs no bootstrap. |
| `standards` | `domain` for repository conventions and their existing owners. |
| `converter`, `operationalize` | `skill-builder` for exports and supported expertise proposals. |
| `swarm` | `agent-native` for explicitly selected delegation; use the native runtime for ordinary execution. |
| `fitness`, `status` | `reality-check` for a requested comparison of claims with evidence; CLI status remains available. |
| `scope`, `product` | `plan` for missing acceptance/scope; `domain` for vocabulary; `doc` for a requested product document. |
| `goals` | Use the native goal/tracker; `craft-goal` remains optional guidance for an explicitly selected persistent-goal workflow. |
| `scaffold`, `workflow-builder` | Implement the requested repository change natively; use `skill-builder` only when the output is a skill. No generic workflow generator replaces these names. |
| `anti-ceremony`, `automation-shape-routing`, `shared` | No standalone invocation. The operating contract retains the artifact-creation boundary, runtime choice stays with the caller, and surviving skills link their needed references. |

### Unified discovery batch

This batch keeps all current skill names and the 34-skill inventory. Use Plan
for resumable discovery of the next complete slice; focused entrypoints remain
available when their narrower method is the requested work.

| Existing entrypoint or method | Current owner and disposition in this batch |
|---|---|
| `plan`, including former `scope`/`product` intent shaping | Plan owns uncertainty routing, compact native resumption and the next complete slice. Existing caller intent remains the output owner. |
| `research` | Keep as the cited investigation, tracing and pattern-evidence specialist; Plan links it only for a source question. |
| `domain` | Keep as the vocabulary, bounded-context and standards specialist; settled terms return to Plan without a new interview. |
| Plan ground-truth routing and optional prototype | Plan's existing ground-truth reference owns a question-driven disposable probe. Mandatory stock controls and deviation ledgers are removed; useful integration comparisons remain. |
| `premortem` and `council` | Keep as caller-selected strategies. Plan owns the common optional challenge exchange; Premortem retains its evidence-shape, reversibility and defeat methods, and Council retains its selected broader review. Neither supplies acceptance. |

No new discovery root, mandatory stage, work ledger or automatic context
admission is introduced. Other skill dispositions are unchanged by this batch.
A compact plan or native handoff can carry assignment references and the next
discriminator; status and ownership remain authoritative only in the native
tracker/runtime. Existing Plan/Implement/Validate authority and fresh exact-content
judgment are unchanged. This contract change makes no comparative benefit claim.

For source-linked installations, inspect old links before removing them: a
retired name may still be visible as a dangling link after updating the checkout.
Use the install's owned unlink path and relink the selected surviving names;
never remove a real directory or another tool's link just to match the count.
Managed plugin upgrades should use the runtime's update mechanism. Avoid loading
both a plugin copy and source links for the same skill.

The new context-budget roles are optional. Claude's plugin includes the
`agentops:bulk-reader` and `agentops:code-writer` subagent definitions. Codex's
plugin includes their generated TOML resources, but role activation is separate:
from a source checkout run `bash scripts/install-codex-context-agents.sh`, or add
`--project` for the current project, then restart Codex. The installer preserves
unrelated configuration and makes backups when replacing owned values.

Read-budget hooks are also separate opt-ins. Use
`scripts/install-read-budget-guard.sh` for Claude or
`scripts/install-codex-read-budget-guard.sh` for Codex. Codex requires native
hook review/trust. Do not infer that installing a skill or role activates a hook;
see [the Codex runtime contract](design/codex-context-budget.md) for discovery,
linked-worktree restrictions and the sandbox limitation.

## Verdicts and identity

When persistence is requested, `verdict.v2` binds acceptance and a deterministic
`subject-manifest.v1` to distinct declared author and validator context
identities. Freshness is an attested trust fact, not cryptographic proof of
process isolation. New CDLC proof uses caller-selected protected external
non-Git storage, with atomic content-addressed verdict writes when requested.
Preserve existing `.agents/` evidence under owner policy. Missing destination
routing is not permission to fall back to repository storage (ADR-0016).

Historical Pawl, queue, claim, landing, and lifecycle artifacts remain inert
evidence. They no longer influence phase sequencing, verdict validity, or CLI
outcomes.

## Install migration

AgentOps supports three optional skill install paths: `npx skills@latest add
boshu2/agentops --all -g` (universal across coding agents), runtime plugins for
Claude Code and Codex (managed bundles that update with the release), and one
canonical checkout plus source symlinks for users who edit skills or
contribute:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --dry-run
ao skills link
```

The 3.x curl installers (`scripts/install.sh`, `install-claude.sh`,
`install-codex.sh`, `install-agy.sh`, `install-opencode.sh`, and
`install-codex.ps1`) are tombstones: they refuse to install and print the
supported paths. Internal helpers (`install-codex-plugin.sh`,
`install-codex-native-skills.sh`) were deleted.

If you switch from a plugin to source links, remove the runtime plugin through
that runtime before linking the checkout so only one corpus is visible:

- Claude Code: `claude plugin uninstall agentops@agentops-marketplace`, then
  `claude plugin marketplace remove agentops-marketplace`.
- Codex: `codex plugin remove agentops@agentops-marketplace`, then
  `codex plugin marketplace remove agentops-marketplace`. (Older Codex without
  the plugin verb: remove `~/.codex/plugins/cache/agentops-marketplace` and
  `~/.codex/.agentops-codex-install.json`, then remove the AgentOps plugin
  enable entry from `~/.codex/config.toml`.)
- Gemini/Antigravity: `agy plugin disable agentops-core-gemini`, then
  `agy plugin uninstall agentops-core-gemini`.

`ao skills link` refuses to replace real directories and foreign links. Resolve
each reported conflict deliberately; never delete a user-owned skill merely to
make the counts match. Use `ao skills unlink` to remove only links that point
into the current checkout.

## Optional runtimes

NTM, Agent Mail, Gas City, councils, and model-specific executors remain
caller-selected adapters or strategies. None is a hard dependency of RPI,
Plan, Implement, or Validate, and none may translate its own attempts, leases,
queues, or delivery state into AgentOps correctness state.
