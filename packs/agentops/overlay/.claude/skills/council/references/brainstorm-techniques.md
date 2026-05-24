# Brainstorming Techniques

Structured techniques for `/council brainstorm` mode. Use `--technique=<name>` to activate.

When no technique is specified, brainstorm mode uses unstructured exploration (current behavior).

## Technique Names

Canonical allowlist for `--technique=<name>` (case-insensitive):

| Name | Section |
|------|---------|
| `scamper` | SCAMPER |
| `six-hats` | Six Thinking Hats |
| `reverse` | Reverse Brainstorming |

## SCAMPER

Systematic derivative generation. Each judge applies the SCAMPER framework to the brainstorm topic:

- **S**ubstitute — What can be replaced?
- **C**ombine — What can be merged?
- **A**dapt — What can be borrowed from elsewhere?
- **M**odify — What can be enlarged, minimized, or altered?
- **P**ut to other uses — What else could this be used for?
- **E**liminate — What can be removed?
- **R**everse — What if we did the opposite?

**When to use:** Feature ideation, product improvement, exploring variations of existing solutions.

**Judge prompt injection:**
```
Apply the SCAMPER framework to this brainstorm topic. For each of the 7 SCAMPER lenses (Substitute, Combine, Adapt, Modify, Put to other uses, Eliminate, Reverse), generate at least one concrete idea. Prioritize the top 3 ideas by feasibility and impact.
```

## Six Thinking Hats

Parallel perspectives technique. Each judge is assigned a "hat" that determines their analysis angle:

| Hat | Color | Focus |
|-----|-------|-------|
| White | Facts | What data do we have? What's missing? |
| Red | Emotions | Gut reactions, intuitions, feelings about the ideas |
| Black | Caution | Risks, dangers, what could go wrong |
| Yellow | Benefits | Value, advantages, why it might work |
| Green | Creativity | New ideas, alternatives, provocations |
| Blue | Process | Meta-view: what should we explore next? |

**When to use:** Complex decisions requiring multiple analytical angles, team alignment on priorities.

**Judge prompt injection:**
```
Apply the Six Thinking Hats framework. Analyze this topic from ALL six perspectives (White=facts, Red=emotions, Black=caution, Yellow=benefits, Green=creativity, Blue=process). Structure your response with a section per hat. Highlight which hat reveals the most critical insight.
```

## Reverse Brainstorming

"How could we make this worse?" then invert. Judges deliberately brainstorm ways to fail, then flip each failure into a solution.

**When to use:** Problem-solving when direct approaches feel stuck, identifying hidden risks, stress-testing plans.

**Judge prompt injection:**
```
Use Reverse Brainstorming. First, brainstorm at least 5 ways to make this problem WORSE or guarantee failure. Then, for each failure mode, invert it into a specific, actionable solution. The inversions are your recommendations.
```
