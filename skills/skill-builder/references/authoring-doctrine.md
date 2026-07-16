# Skill Authoring Doctrine

Prose-quality principles for writing `SKILL.md` bodies that produce the same
agent *process* on every run. Structure checks (frontmatter, sections, output
contracts) live in [skill-template.md](skill-template.md) and
[audit-checks.md](audit-checks.md); this reference governs the sentences
inside that structure. The deep audit's advisory `authoring` block
(see [audit-checks.md](audit-checks.md)) mechanically flags the three
detectable failure modes below; the rest is author judgment.

Idea provenance: these principles were distilled, clean-room, from the
skill-authoring doctrine published at
<https://github.com/mattpocock/skills> (MIT). Concepts only; no upstream
prose, names, prompts, scripts, or examples are reproduced here.

## 1. The no-op test

A line earns its tokens only if it changes agent behavior versus what the
model would do anyway. "Be thorough" fails the test: the model is already
approximately thorough, so the line spends context to change nothing. The
fix is never to restate the wish louder in more words — it is a sharper,
behavior-changing instruction ("read every `references/*.md` before editing")
or a stronger single word the model has strong priors for.

The test is model-relative. Two authors disagreeing about whether a line is
a no-op disagree about the model's default, and settle it by running the
skill, not by argument. Because it is model-relative, the audit's
`noop-phrase` finding is advisory: it names suspects from a fixed phrase
list; the author decides.

## 2. Negation

Steering by prohibition drags the banned behavior into context and makes it
more available, not less: a "never write verbose comments" line makes
verbosity the freshest pattern in the window. State the positive target
instead ("write one-line comments"), so the banned behavior is never named.

A prohibition is still correct as a hard guardrail that cannot be phrased
positively (this repository's "never run `claude -p`" is one). Even then,
pair it with the positive alternative in the same paragraph — guardrail plus
recovery path — which is exactly the shape this repo's existing
"Anti-pattern: X. Corrective: Y" convention enforces. The audit's
`negation-without-positive` finding flags a paragraph whose every sentence
prohibits and none instructs.

## 3. Completion criteria

Every workflow step ends on a condition the agent can check, and the
condition should demand the full work. Two properties matter:

- **Checkable** — the agent can tell done from not-done. "Understanding
  reached" is not checkable; "every changed path appears in the manifest"
  is.
- **Exhaustive** — the bound covers the whole obligation. "Produce a change
  list" permits a partial list; "every modified file accounted for" does
  not.

A vague bound invites the named failure mode *premature completion*: the
agent's attention slips from the work to being done, and the step ends
early. Sharpen the bound before restructuring the skill — it is the cheap,
local fix. The audit's `step-missing-done-condition` finding flags workflow
subphases that carry no done-condition phrasing at all ("Done when",
"Checkpoint:", "Stop after", "until … exit 0").

## 4. Leading words

A leading word is a single pretrained concept token that anchors a region of
behavior: *tight* (loop), *red* (failing check), *fresh* (context),
*frontier*, *quarantine*. Repeated as a token — never re-explained — it
accumulates a distributed definition across the skill and recruits priors
the model already holds, buying behavior for almost no context cost.

Hunt for triads and restatements begging to collapse: "fast, deterministic,
low-overhead" is one quality said three ways, and *tight* says it once. A
coined word works only if defined once, clearly; a pretrained word is free.
A leading word too weak to beat the model's default is itself a no-op — the
fix is a stronger word, not more words.

## 5. Description discipline

The frontmatter description is the one part of a skill that is loaded every
turn, so it earns the hardest pruning. It does two jobs: state what the
skill is, and list the genuinely distinct trigger branches that should fire
it. One trigger per branch — synonyms that rename the same branch are
duplication that spends context and dilutes match sharpness. Word triggers
with the leading words callers actually use, because a description that
shares vocabulary with the caller's prompt fires more reliably. This
sharpens, not replaces, the structural `description-has-triggers` and
`trigger-clarity` checks.

## 6. Context load vs cognitive load

Every skill charges one of two accounts. A model-discoverable skill's
description sits in the window every turn — a permanent **context load**
paid whether or not the skill fires. A human-only skill costs nothing per
turn but spends **cognitive load**: the human is the index that must
remember it exists and reach for it at the right moment.

Choosing the account is a required authoring decision, and this repository
already carries the levers: `user-invocable` frontmatter, the `tier` and
`disposition` metadata, and the generated router (`docs/SKILL-ROUTER.md`)
that serves as the cure for cognitive load once human-reached skills
multiply. Prefer model discovery only when the agent must reach the skill on
its own or another skill must depend on it; otherwise keep the window clean.
Splitting one skill into several is spending one of the two loads — split
only when a distinct trigger vocabulary or an independently reachable
behavior pays for the new entry.

## Applying the doctrine

When creating or healing a skill, pass the body once per principle: strike
no-op lines, convert prohibitions to positive targets (keeping paired
guardrails), give each workflow step a checkable exhaustive bound, collapse
restatements into leading words, prune the description to one trigger per
branch, and confirm the load account is the one intended. The advisory
`authoring` audit block names the mechanical suspects; the author owns the
judgment calls.
