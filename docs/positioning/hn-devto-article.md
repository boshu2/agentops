# HN / dev.to Article Draft

## Status

Publish-ready repo draft for `na-gtm.4`.

External posting still requires an authenticated dev.to/Hacker News account. Use
the copy below as the source of truth when posting.

## Working Title

Coding agents do not do their own bookkeeping

## Alternate Titles

- AgentOps: the operational layer for coding agents
- The missing operational layer around coding agents
- Better prompts will not fix agent amnesia
- Coding agents need bookkeeping, validation, and repeatable flows

## dev.to Front Matter

```yaml
---
title: "Coding agents do not do their own bookkeeping"
published: false
description: "AgentOps is an open-source operational layer for coding agents: bookkeeping, validation, primitives, and flows so work compounds instead of resetting every session."
tags: ai, opensource, devtools, agents
canonical_url:
---
```

## Draft

Most discussion about coding agents still focuses on the wrong layer.

People ask:

- which model is best
- how many agents to run
- whether the workflow should be one-shot or multi-agent
- which prompt pattern gets the best output

Those questions matter, but they are not the core problem.

The core problem is that coding agents do not do their own bookkeeping.

They do not naturally preserve what mattered from prior work. They do not
reliably challenge their own plans before implementation. They do not turn
completed work into better next work unless someone builds a system around them
that does exactly that.

That is the idea behind AgentOps.

The simplest way to describe it now is:

**AgentOps is the operational layer for coding agents.**

It gives coding agents four things they do not have by default:

1. **Bookkeeping**
   Sessions should not leave behind only chat history. They should leave behind
   reusable learnings, findings, and context that can be surfaced back later.
2. **Validation**
   Plans and code should be challenged before shipping, not only after breakage
   shows up in CI or production.
3. **Primitives**
   Operators need reusable building blocks, not one giant all-or-nothing
   workflow.
4. **Flows**
   Those primitives need to connect into repeatable paths for discovery,
   implementation, validation, and knowledge extraction.

The reason this matters is compounding.

Without an operational layer, every session starts too close to zero. You
re-brief the agent. You re-explain the repo. You rediscover old mistakes. You
lose the result of yesterday's debugging the moment the context window moves on.

With an operational layer, the environment changes.

Research is written down. Findings are promoted. Learnings get retrieved.
Validation gates get sharper. The next session starts inside a stronger
operating context than the previous one had.

That is the product thesis.

Technically, I think the best internal description is that AgentOps acts like a
**context compiler**.

A normal compiler takes raw source code and transforms it into an executable
artifact with stronger guarantees than the original text had on its own.

AgentOps does something similar for agent work:

- raw transcripts, retros, and failures become learnings
- learnings become curated patterns and findings
- findings become rules, checks, and better planning context
- the next task starts with compiled context instead of vague memory

That framing is useful internally because it explains why the product is not
just "memory" and not just "workflow tooling."

Publicly, though, I think the more useful sentence is still:

**coding agents do not do their own bookkeeping**

That line has become more convincing to me because I keep seeing the same
convergence from different directions.

Karpathy has talked about the bookkeeping burden around an LLM wiki.

People at Block have described the moat as the feedback loop around the markdown
file and the signal flowing through it.

And building AgentOps in production kept teaching the same lesson:

the model is not the product.

The system around the model is the product.

That system needs:

- bookkeeping so knowledge does not disappear
- validation so errors get caught before shipping
- reusable primitives so operators can control the work
- connected flows so the whole thing compounds

Here is the concrete version.

AgentOps has a full-lifecycle flow:

```text
> /rpi "add retry backoff to rate limiter"

[research]      Find relevant prior learnings
[plan]          Decompose the work into tracked issues
[pre-mortem]    Challenge the plan before implementation
[crank]         Execute the implementation
[vibe]          Validate the resulting code
[post-mortem]   Extract learnings and next work
```

It also has an autonomous improvement loop:

```text
> /evolve

[evolve]        Measure repo goals
[cycle-1]       Pick the worst failing goal
                Run /rpi against that slice
                Validate that the score improved
                Revert or narrow if it regressed
[cycle-2...]    Repeat until the stop condition fires
```

That is the day loop: it can touch source code, but it is supposed to stay
bounded by goals, issue tracking, validation commands, and regression gates.

There is also a night loop:

```text
> /dream start

[overnight]     Harvest session artifacts
[reduce]        Deduplicate and defrag knowledge
[close-loop]    Promote findings into next work
[measure]       Capture corpus-quality deltas
[halt]          Stop on plateau or rollback on regression
```

That one does not mutate source. It compounds the knowledge corpus so the next
day loop starts against a better environment.

In v2.36.0, the CLI exposes this split directly:

```bash
ao evolve --max-cycles 1
ao rpi loop --supervisor --max-cycles 1
ao search "prior rate limit fixes"
ao lookup --query "repo release lessons"
```

The new `PROGRAM.md` contract defines the bounds for autonomous work in the repo:
mutable scope, immutable scope, validation commands, decision policy, escalation
rules, and stop conditions. That gives the autonomous loop something more
concrete than a prompt to obey.

Install paths are deliberately boring:

```bash
# Codex CLI
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# OpenCode
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash

# Selected skills for other agents
bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
```

The important part is not that a command exists.

The important part is that the repo starts accumulating operating memory:

- issues that know what is blocked
- plans that can be checked before execution
- validation gates that fail before commit
- learnings that are retrievable in later work
- postmortems that produce better next work

If you already use coding agents every day, that is probably the pain you feel
too.

Not "the model is dumb."

More like:

- "why did it forget the decision we made two days ago?"
- "why did it repeat the exact failed fix?"
- "why did it say this was done without actually validating it?"
- "why does every new session feel like I am onboarding a contractor with
  amnesia?"

That is the problem I think the category needs to solve.

Not more demos of autonomous generation.

Not bigger swarms for their own sake.

An operational layer.

That is what I am building toward with AgentOps.

## HN Submission

### Title

Coding agents do not do their own bookkeeping

### URL

Use the dev.to canonical URL after publishing there, or submit the GitHub repo if
posting as a Show HN.

### Text Blurb

AgentOps is an open-source operational layer for coding agents. The core thesis
is that agents do not do their own bookkeeping, so work resets every session
unless you build bookkeeping, validation, primitives, and repeatable flows around
them.

The post argues that the real category is not "memory" or "workflow tooling" in
isolation, but an operational layer that turns raw session signal into reusable
learnings, findings, validation, and better next work. It also includes the
current v2.36 flow split: `/rpi` for one lifecycle, `/evolve` for the autonomous
day loop, `/dream` for the overnight knowledge loop, and `PROGRAM.md` as the
repo-local contract that bounds autonomous work.

## Publish Checklist

- Post the `Draft` section to dev.to using the front matter above.
- Add a screenshot or terminal capture from README's `/evolve` or `/dream`
  transcript if the platform supports an image.
- Submit to Hacker News using the HN title and text blurb above.
- After the external URLs exist, add them to `na-gtm.4` as `external_ref` or
  notes, then close the bead.
