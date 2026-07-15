---
name: bootstrap
description: 'Initialize minimal AgentOps documentation and verdict storage without taking over repository workflow.'
practices:
- hermetic-builds
- code-complete
hexagonal_role: driving-adapter
consumes:
- goals
- product
- doc
- shared
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
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  graph_root: true
  tier: session
  dependencies: []
output_contract: minimal project docs and .agentops/verdicts directory
---
# Bootstrap — minimal project setup

Bootstrap fills only missing AgentOps entry documents and the default durable
verdict directory. It does not initialize Git, install hooks, create tracker
state, start runtimes, or impose a delivery workflow.

## Procedure

1. Inspect the target directory and report which canonical files already exist.
2. Ask the caller for missing product intent or goal content when it cannot be
   inferred safely.
3. Create only missing, explicitly requested files. Never overwrite an existing
   document.
4. Create `.agentops/verdicts/sha256/` when durable local verdict storage is
   requested.
5. Validate filesystem existence and report created, skipped, and failed paths.
6. Stop.

Typical documents are `PRODUCT.md`, `GOALS.md`, `AGENTS.md`, and a README section
that explains the one-pass loop. Repositories remain free to use their own Git,
CI, tracker, release, and deployment policies.

## Non-goals

- installing or invoking `ao`, `br`, `bd`, NTM, Agent Mail, or another runtime;
- creating `.git`, worktrees, branches, commits, hooks, or CI workflows;
- choosing work or claiming that repository setup is complete beyond the paths
  actually inspected;
- running RPI automatically.

## Output

Return target path, requested files, created files, existing files left intact,
failed writes, and validation observations. Do not include a next action.

## References

- [Goals](../goals/SKILL.md)
- [Product](../product/SKILL.md)
- [Documentation](../doc/SKILL.md)
- [Examples](references/examples.md)
