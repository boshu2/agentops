# Agent workflow reference

[AGENTS.md](../AGENTS.md) directs the agent to own the authorized outcome through
finish. Zero AgentOps skills are required. Use the native agent and
shell; the tracker, Git and runtime keep their existing authority.

1. Use the existing intent. Plan only for missing intent or consequential
   uncertainty. A trivial change needs no Plan, Recall or Learn worksheet.
2. Implement the smallest useful change, repair known defects directly, and
   revise an approach when evidence disproves its assumption within unchanged
   outcome and scope. Changing acceptance needs caller authority.
3. Run cheap discriminating checks while editing, then required integration
   checks. For an ordinary change these and CI are the gate: finish.
4. Obtain one fresh author-distinct read only when the caller asks, a mistake
   cannot be cheaply undone after it lands (a published release or instructions
   users will follow, a security boundary, destroying data or tracker state,
   deleting a check that protects the product), or no deterministic check
   covers the changed behavior. Same-family is default; cross-model is opt-in.
   The reviewer answers one question written in advance and does not re-run
   the checks.
5. Repair what fails the accepted behavior or would mislead a user, break
   install or the CLI, or remove protection for the product. Confirm each
   repair with a check; a repair does not start another review. A genuine
   causal stall admits at most one bounded helper, never a chain. Report the
   outcome, what was checked and what was not when complete or actually stopped.

[Memory](../skills/memory/SKILL.md) owns on-demand find/recall, capture/mine/learn
and curate operations over caller-selected reviewed project `.context/` or
external topic pages. Mining is separately budgeted. Read cleared project pages
with ordinary filesystem tools without BD or AO; existing docs, ADRs and code
retain authority. Other roles use Memory's operation pointers.
BD owns work/status/handoffs, Git content, and native/CASS sources episodes.
Learning can update, qualify or remove a rule; no-change is valid. Only later
work establishes benefit. This lean route uses public or already-cleared inputs
and claims no native restricted-source enforcement.

Specialists, anti-ceremony audits, factories and outer-goal guidance are optional.
The [RPI charter](../skills/rpi/SKILL.md) is an explicitly selected workflow;
native goals and direct coding do not require it. A skill earns its context
cost by resolving a task-specific need, not by occupying a phase in a sequence.
No new scheduler, command or process ledger is needed. The grandfathered pure
fixed-dispatch adapter is separately described in its own reference.
Optional context-budget tooling (an opt-in read-budget hook plus bulk-read /
code-write delegation) is described in
[context-budget delegation](https://github.com/boshu2/agentops/blob/main/skills/agent-native/references/context-budget-delegation.md).

For exact intent snapshots, subject manifests, evidence storage, scope and
freshness, use [RPI traversal](architecture/rpi-traversal.md) and
[Validate mechanics](../skills/validate/references/mechanics.md). New proof goes
to caller-selected protected external non-Git storage; missing routing does not
permit a workspace fallback. Legacy `.agents/` evidence remains preserved.
