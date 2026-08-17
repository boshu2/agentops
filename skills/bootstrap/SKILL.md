---
name: bootstrap
description: 'Initialize explicitly requested, missing AgentOps documentation and optional verdict storage without taking over repository workflow. Triggers: "bootstrap AgentOps", "initialize AgentOps docs".'
practices:
- hermetic-builds
- code-complete
hexagonal_role: driving-adapter
consumes:
- fitness
- product
- doc
produces: []
context_rel: []
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  intel_scope: none
metadata:
  capabilities: [bootstrap]
  effects: [read_requested_project_paths, write_missing_project_docs, optionally_create_verdict_storage_directory]
  canonical_status: canonical
  disposition: keep_specialist
  graph_root: true
  tier: session
  dependencies: []
output_contract: explicitly requested missing project docs plus optional declared .agents/ao/verdicts/sha256 directory, with created/existing/failed paths
---
# Bootstrap — minimal project setup

Bootstrap fills only explicitly requested, missing AgentOps entry documents and,
when requested, the durable verdict directory. It does not initialize Git,
install hooks, create tracker state, start runtimes, or impose a delivery
workflow.

Never-overwrite is what makes bootstrap safe to run on any repository: a setup
step that can only add is idempotent by construction, while one that can
replace must first prove it understands what it is replacing.

Named failure mode — **scaffold sprawl**: creating files the caller never
requested because a "complete" setup feels more helpful than a minimal one.

Anti-pattern: inferring product intent from directory names and READMEs to
avoid asking the caller. Corrective: ask for the missing content; a wrong
PRODUCT.md written confidently is worse than a question.

## Procedure

1. Inspect the target directory and report which canonical files already exist.
2. Ask the caller for missing product intent or goal content when it cannot be
   inferred safely.
3. Create only missing, explicitly requested files. Never overwrite an existing
   document.
4. Create `.agents/ao/verdicts/sha256/` when durable local verdict storage is
   requested.
5. Validate filesystem existence and report created, skipped, and failed paths.
6. Stop.

Typical documents are `PRODUCT.md`, `GOALS.md`, `AGENTS.md`, and a README section
that explains the RPI traversal. Generated product copy starts from the
operations-layer category and preserves the ownership boundary. Repositories
remain free to use their own Git, CI, tracker, release, and deployment
policies.

**Naming.** Three surfaces share the word "bootstrap"; they are distinct. This
skill authors missing entry documents. `ao init` is the CLI command that creates
the local evidence and verdict directories (`.agents/ao/**`). `ao session
bootstrap` is a read-only session command that reports which local orientation
files are present. This skill invokes neither.

## Constraints

- **Why target bounds matter.** Resolve one caller-declared physical target root and a literal allowlist of
  requested document paths before any write. Reject traversal, symlink escapes,
  `/`, the user's home as the target itself, and every unrequested path.
- **Why optional storage stays separate.** Durable verdict storage is a separate optional effect. Create exactly
  `<target>/.agents/ao/verdicts/sha256/` only when the caller explicitly requests
  it; record that directory in requested, created/existing, and failed outputs.
  A docs-only bootstrap never creates `.agents/ao/**`.
- Recheck nonexistence immediately before each create and use exclusive create
  semantics. A collision or partial directory failure stops that path without
  overwriting, deleting, chmodding, or repairing existing content.
- Completion means every requested path is classified exactly once as created,
  existing-and-untouched, or failed, and all created paths resolve under the
  target root. No unclassified or extra filesystem effect is permitted.

## Quality checks

- Every requested path has exactly one created, existing-and-untouched, or
  failed classification, and the reported write set equals observed writes.
- A docs-only fixture leaves `.agents/ao/**` absent; a storage fixture creates
  only the declared verdict directory under the physical target root.
- A collision, traversal, symlink escape, or existing destination fails before
  overwrite and preserves the pre-run bytes and mode.

## Non-goals

- installing or invoking `ao`, `br`, `bd`, NTM, Agent Mail, or another runtime;
- creating `.git`, worktrees, branches, commits, hooks, or CI workflows;
- choosing work or claiming that repository setup is complete beyond the paths
  actually inspected;
- running RPI automatically.

## Output

Return target path, requested document paths, whether verdict storage was
requested, requested/created/existing/failed directories, created/existing
documents, failed writes, and validation observations. Report `writes: []` for
inspection-only or all-existing runs. Serialize those facts as
`bootstrap-result.v1` and validate them with
`bash skills/bootstrap/scripts/validate-output.sh <result.json>`. Do not include
a next action.

## References

- [Fitness](../fitness/SKILL.md)
- [Product](../product/SKILL.md)
- [Documentation](../doc/SKILL.md)
- [Examples](references/examples.md)
