# Pattern: a three-part agent harness that keeps engineers in the loop

> **Status:** Proposed. A design pattern, not a measured result.
> **Applies to:** a team that builds its own coding-agent setup on a harness
> with an extension API (pi is one example; the split works on any harness
> that lets you add skills, tools and hooks).

A coding agent can write a change faster than a team can understand it. This
pattern splits an agent harness into three parts so that checks are cheap,
judgment is explicit, and the people who own the code still learn it.

## The split

| Part | What it is | What belongs there | Who pays |
| --- | --- | --- | --- |
| **Skills** | Markdown procedures the model reads | Judgment: how to plan, review, test, hand off | Model tokens, every use |
| **A deterministic CLI** | One compiled or scripted tool with stable verbs | Facts: diff checks, impact, lint, codemods, session analysis | Built once, then free |
| **Host extensions** | Code that runs inside the harness process | Capabilities and governance: tools that need credentials or network, hooks on tool calls, sandboxing | Built once, then free |

The split follows one question: who should pay for this step? A skill costs
tokens every time it runs and can be wrong in a new way each time. A tool
costs effort once and gives the same answer every time.

### Where a new rule goes

Ask in this order and stop at the first yes.

1. Does it need credentials, network access or control over the session?
   Put it in a host extension.
2. Must it run on every tool result, every commit or every agent spawn?
   Make it a hook in an extension.
3. Is it a computation over local files with one right answer?
   Make it a CLI verb.
4. Is it a fact about this codebase tied to a file or symbol?
   Put it next to that code, where a check can notice when the code moves.
5. Is it a judgment call or a multi-step procedure?
   Write a skill.

Most teams start by writing everything as a skill. When the same three tool
calls show up in session after session, that is a verb waiting to be built.

## Practices that keep slop out

Each practice names the mechanism that enforces it. A practice with no
mechanism is a hope.

**State the behavior before the code.** One example of what the caller will
observe, plus one thing that must not happen. The implementation and the
review both use that example. Tests written afterward can add evidence but
cannot redefine the promise. *Lives in:* the planning skill.

**Work in small batches.** One behavior per change. Commit to the next slice
only and keep later slices as open questions, because the first slice always
teaches something. *Lives in:* the planning skill, and a CLI check on change
size and scope.

**Declare the write scope.** The change says which paths it may touch. A
write outside that scope blocks. *Lives in:* a CLI check on the diff.

**Let checks be the gate.** Run the declared checks in a sandbox on the
exact commit, and record a receipt: which commit, which checks, which exit
codes, and whether the tree changed while they ran. "All tests pass" in
prose is not a receipt. *Lives in:* an extension that runs the checks and a
CLI verb that verifies the receipt.

**Protect the tests from the agent.** An agent under pressure to go green
will delete an assertion, add a skip, or write a test that cannot fail. Audit
the diff for those shapes, on every commit in the branch and not only the
final state. When a test must change, a named person approves it and the
receipt records that. *Lives in:* a CLI check and the receipt.

**Review where a mistake is costly.** For an ordinary change, the checks
and CI are enough. Ask for one fresh review, by a context that did not write
the change, in three cases: the caller asks for one; the change cannot be
cheaply undone (a release, a security boundary, deleted data, a removed
check); or no deterministic check covers the changed behavior, so green CI
says nothing about it. One round, one question. The author
never grades its own work. AgentOps records why in
[ADR-0017](../adr/ADR-0017-loop-as-control-flow-not-knowledge.md).
*Lives in:* the review skills.

**Stop loops early.** Fingerprint each tool call together with its result.
The same call with the same result, repeated, is a loop; the same call with
a changing result is polling and is fine. On a loop, try one fresh helper
with a different view. If that does not change the next step, stop and hand
the state to a person. *Lives in:* an extension hook.

**Keep the model out of the data path.** A tool that gathers, joins and
filters should return a small decision, not raw output for the model to read
across ten turns. Compact noisy tool output before it reaches the model, and
redact secrets before compacting. *Lives in:* CLI verbs and an extension hook
on tool results.

**Sandbox by default.** The agent's shell and the checks run with no network
and no credentials. Anything that needs either goes through a narrow
extension tool with fixed arguments, so the model cannot compose an
arbitrary command with a token in it. *Lives in:* extensions.

**Report honestly.** Every rate carries its denominator, including the
attempts that failed. "Not proven" is a valid outcome and beats a guessed
pass. No report, dashboard or ledger is created unless something reads it
and acts on it. *Lives in:* the skills, and review.

**Keep the skill list short and named for the step.** In AgentOps' own
measurement, 5 of 28 skills never loaded on requests that did not name
them and 7 more loaded only sometimes, while skills asked for by name
loaded 22 of 24 times
([evaluation](../evals/2026-10-22-plugin-eval-opus-5-5.md)). Give people a
few skills they call by name for the step they are on, and measure loading
before trusting quiet routing. *Lives in:* how the catalog is organized.

**Turn repeated sequences into tools.** Mine session logs for tool sequences
that recur. Each one that becomes a verb shortens a skill and removes a place
the model can go wrong. *Lives in:* a CLI verb over session logs and a
regular review of its output.

## Practices that keep people learning

An agent that writes everything leaves a team owning code nobody has read.
These practices cost a little speed on purpose.

**The engineer owns the intent.** A person writes or approves the acceptance
example. The agent may draft it; it does not decide what "done" means.

**Read before you ask.** Before the agent explains a piece of code, the
engineer fills in a short card: the inputs, what state changes, one boundary
case, and one prediction that could turn out false. Then they run it and
check the prediction. The agent explains after, against the card.

**Explain back before merge.** Whoever merges the change says, in their own
words, what it does and why this approach. The agent may ask questions; it
does not answer them for the engineer. A change nobody on the team can
explain is a defect, even when it passes.

**A teaching mode that withholds the fix.** When someone is learning, the
agent helps in rungs: a question, then a pointer to the right place, then an
explanation, then a partial answer, and the full answer last. The engineer
picks the rung.

**Human-driven zones.** Choose the kinds of work where a person types and
the agent reviews: the core domain logic, the first test for a new behavior,
the fix for an incident. Rotate who does them.

**Review the diff, not the summary.** The agent's summary is a map of the
change. The review happens in the code.

**Pair or mob with the agent as one more participant.** The driver rotates.
The navigator's job is to ask why. A team that already pairs loses the least
when it adds an agent.

**Hand off in four lines.** What changed, what proves it, what is still
unknown, and what you need from the next person. Short enough that people
write it.

**Decisions with no way back are written by a person.** The agent can list
options and what would overturn each. The person records the choice.

**Start new engineers on reading.** Early tasks use the teaching mode and the
reading card, so the first weeks build a model of the system rather than a
habit of accepting output.

## What to measure

| Signal | What it tells you |
| --- | --- |
| Skill load rate on requests that do not name the skill | Whether routing works, or people must ask by name |
| Loop stops per hundred tool calls, and whether the next step changed | Whether the loop detector fires on real loops |
| Receipts passed, failed and not proven | Whether checks run where the work happens |
| Test changes approved versus tests added | Whether approval has become a rubber stamp |
| Changes where explain-back failed | Where the team's understanding is thin |
| Modules only one person, or no person, has read | Where a departure or an incident will hurt |
| Model tokens per merged change | Whether tools are replacing repeated model work |

Collect a few weeks of data before setting any threshold.

## Adopting it

1. Write the acceptance example and the write scope into every change. No
   tooling needed.
2. Run the checks in a sandbox and record a receipt.
3. Add the diff audit that protects tests.
4. Add the explain-back step and the reading card.
5. Mine sessions for repeated sequences and build the first verb.
6. Add the loop detector and output compaction once sessions are long enough
   to need them.

## What this page does not claim

The split and the practices are a design. AgentOps has measured one part,
skill loading, and has recorded why it moved to review only where a mistake
is costly, without claiming a measured quality effect. The learning practices
come from pairing and teaching practice and are not measured here. Treat the
whole as a starting point to test in your own team.

## How this maps to AgentOps

AgentOps supplies the skill part: [Plan](../../skills/plan/SKILL.md),
[Implement](../../skills/implement/SKILL.md),
[Test](../../skills/test/SKILL.md), [Review](../../skills/review/SKILL.md)
and [Validate](../../skills/validate/SKILL.md) carry the judgment practices
above. The deterministic CLI and the host extensions are yours to build for
your harness and your codebase.
