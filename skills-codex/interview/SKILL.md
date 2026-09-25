---
name: interview
description: 'Interview the caller one question at a time to settle a big outcome before agents work alone. Use when: shaping a goal or large RPI. Not for one question on one slice; use Plan.'
---
# Interview

Shape a big outcome with the caller, one question per turn, before agents run
alone. You look up facts; the caller makes choices. **Why:** answering shapes
the caller's thinking, and control is highest before launch. Interview creates
no goal, bead or file and changes no status, claim or closure.

## Each turn

1. **Look it up** in the tracker, docs and code first. Ask only for choices:
   outcome, proof, non-goals, authority, budgets, priorities, risk tolerance.
2. **Pick the branch.** Start with the outcome, then the open branch that most
   changes acceptance, scope or authority. Defer any that change no decision.
3. **Ask one question** in this shape, so the caller can accept, amend or
   reject in one line. Wait for the answer.
4. **Record** only what the caller decides. A skip stays open; "your call"
   accepts the recommendation shown. A revision reopens dependent answers.

```text
Q<n>: <the decision>. <the question, with 2 to 4 options when they exist>
My recommendation: <answer>, because <source or reason>.
Tradeoff: <the one cost that matters>
```

## Answerer

The caller answers by default. On request ("let a council answer my
interview"), a council answers through [Council](../council/SKILL.md)'s
interview-panel mode. The caller still accepts or amends those answers in one
pass before anything is recorded; authority, budgets and acceptance changes
stay the caller's.

## BDD: acceptance as examples

Drive each criterion to a Given/When/Then with an observable result and its
proving evidence. Draft the example yourself as the recommendation; the caller
accepts or edits it. "Works reliably" is not acceptance; ask what would be seen.

```gherkin
Given a Job has already completed
When the worker receives that Job again
Then it returns the completed result without a second side effect
# Proof: a redelivery test asserts one side effect and the completed result
```

## DDD: one term per concept

When a word is vague, overloaded or has synonyms, ask which term the domain uses.
Record one term with a one-line definition and use only it in examples, notes,
code and tests. [Domain](../domain/SKILL.md) owns deeper modeling.

## Show the state

Open each turn with one line: what just settled and how many choices stay
open. Show these lists on request, after a revision and at stop:

- **Decided:** each choice; acceptance carries its Given/When/Then and proof.
- **Terms:** each settled term with its one-line definition.
- **Open:** unanswered choices, most consequential first.
- **Deferred:** choices that change no next decision, and what revives them.

Within authority, append settled decisions and terms to the intent source
(root epic, issue or conversation), and Open and Deferred at stop. If a goal
already runs on that epic, do not append a changed criterion or term; list it
under Open as an acceptance change so the caller can hold and re-craft first. In BD:

```bash
bd context --json    # confirm the destination before any write
bd show <epic-id>    # on resume: reuse settled notes, start from Open
bd update <epic-id> --append-notes "decided: <choice> | example: <Given/When/Then>"
```

## Stop and hand off

Stop when the caller stops, the next question would only restate a settled
answer, the work proves to be one slice, or Craft Goal admission is decided:

1. outcome and non-goals;
2. terminal acceptance, each criterion with its proving evidence;
3. authority: reads, writes, external effects, Git, and when agents must ask;
4. numeric wave and hard budgets, and the no-ratchet count that triggers HOLD;
5. the first falsifiable question.

Hand over the lists; the caller starts the next step: Craft Goal for several
related experiments, RPI or [Plan](../plan/SKILL.md) for one outcome with
items 1 to 3 and real bounds. Open items stay open; never fill one to finish.
