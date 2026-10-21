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

## Choose methods for the problem

Start with the user's outcome and use the smallest set of methods that can
change a decision. The skill menu is a set of tools, not a sequence to run in
full. An incoming report with an unverified expectation belongs to
[Triage](../skills/triage/SKILL.md); accepted work does not need another intake
pass. For example, consider adding a batch import that users can safely retry:

- If "safe to retry" is ambiguous, use [Interview](../skills/interview/SKILL.md)
  to settle duplicate handling and partial success. Keep settled answers; ask
  the next question only when its prerequisites are known.
- If the boundary is uncertain, use [Plan](../skills/plan/SKILL.md) to compare
  caller examples for the plausible interfaces and choose a thin end-to-end
  slice. Use [Refactor](../skills/refactor/SKILL.md) when existing coupling makes
  that slice expensive, preserving the behavior and tests users depend on.
  [Architecture](../skills/architecture/SKILL.md) can investigate structural
  friction and compare boundaries when the cause itself is uncertain; a small
  known transformation does not need that investigation.
- Implement the slice and exercise its actual boundaries. For this import,
  useful examples include a malformed row, a retry after partial success, and
  a response lost after the server accepted the batch. [Test](../skills/test/SKILL.md)
  helps choose checks that distinguish these outcomes; [Review](../skills/review/SKILL.md)
  helps examine consequential gaps when a fresh read is owed or requested.
  If the question is whether an agent can discover and recover through the
  interface, [Ergonomics](../skills/ergonomics/SKILL.md) examines that named
  surface. Its findings can inform Test's assertions; it adds no permission to
  run production mutations or forward access.
- When a failure exposes reusable guidance, [Postmortem](../skills/postmortem/SKILL.md)
  can trace what happened and [Memory](../skills/memory/SKILL.md) can maintain a
  supported, scoped lesson. Preserve why a proposed explanation was rejected.
  Reuse on later work establishes benefit; writing the lesson does not.

A task with clear intent and adequate checks can go directly to implementation.
Choose a method because it resolves a concrete uncertainty, and leave it when
that uncertainty is settled. No separate worksheet or skill invocation is owed
for each example above.

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
No new scheduler, command or process ledger is needed.
Optional context-budget tooling (an opt-in read-budget hook plus bulk-read /
code-write delegation) is described in
[context-budget delegation](https://github.com/boshu2/agentops/blob/main/skills/agent-native/references/context-budget-delegation.md).

For exact intent snapshots, subject manifests, evidence storage, scope and
freshness, use [RPI traversal](architecture/rpi-traversal.md) and
[Validate mechanics](../skills/validate/references/mechanics.md). New proof goes
to caller-selected protected external non-Git storage; missing routing does not
permit a workspace fallback. Legacy `.agents/` evidence remains preserved.
