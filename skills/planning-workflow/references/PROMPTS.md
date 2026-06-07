# Planning Prompts — Complete Reference

## Table of Contents
- [Plan Review (GPT Pro)](#plan-review-gpt-pro)
- [Integrate Revisions (Claude Code)](#integrate-revisions-claude-code)
- [Multi-Model Blend (GPT Pro)](#multi-model-blend-gpt-pro)
- [Initial Plan Creation](#initial-plan-creation)

---

## Plan Review (GPT Pro)

Use with GPT Pro with Extended Reasoning enabled. Paste your entire markdown plan:

```
Carefully review this entire plan for me and come up with your best revisions in terms of better architecture, new features, changed features, etc. to make it better, more robust/reliable, more performant, more compelling/useful, etc. For each proposed change, give me your detailed analysis and rationale/justification for why it would make the project better along with the git-diff style change versus the original plan shown below:

<PASTE YOUR EXISTING COMPLETE PLAN HERE>
```

---

## Integrate Revisions (Claude Code)

After GPT Pro finishes (may take 20-30 minutes for complex plans), paste the output into Claude Code:

```
OK, now integrate these revisions to the markdown plan in-place; use ultrathink and be meticulous. At the end, you can tell me which changes you wholeheartedly agree with, which you somewhat agree with, and which you disagree with:

```[Pasted text from GPT Pro]```
```

### Repeat Until Steady-State

- Start fresh ChatGPT conversations for each round
- After 4-5 rounds, suggestions become very incremental
- You'll see massive improvements from v2 to v3, continuing to the end
- This phase can take 2-3 hours for complex features — this is normal

---

## Multi-Model Blend (GPT Pro)

Get competing plans from Gemini 3.1 Pro (Deep Think), Grok4 Heavy, and Opus 4.7, then use GPT Pro as final arbiter:

```
I asked 3 competing LLMs to do the exact same thing and they came up with pretty different plans which you can read below. I want you to REALLY carefully analyze their plans with an open mind and be intellectually honest about what they did that's better than your plan. Then I want you to come up with the best possible revisions to your plan (you should simply update your existing document for your original plan with the revisions) that artfully and skillfully blends the "best of all worlds" to create a true, ultimate, superior hybrid version of the plan that best achieves our stated goals and will work the best in real-world practice to solve the problems we are facing and our overarching goals while ensuring the extreme success of the enterprise as best as possible; you should provide me with a complete series of git-diff style changes to your original plan to turn it into the new, enhanced, much longer and detailed plan that integrates the best of all the plans with every good idea included (you don't need to mention which ideas came from which models in the final revised enhanced plan):

[Paste competing plans here]
```

---

## Initial Plan Creation

When starting a new project, include:

1. **Goals and Intent** — What you're really trying to accomplish
2. **Workflows** — How the final software should work from the user's perspective
3. **Tech Stack** — Be specific (e.g., "TypeScript, Next.js 16, React 19, Tailwind, Supabase")
4. **Architecture Decisions** — High-level structure and patterns
5. **The "Why"** — The more the model understands your end goal, the better it performs

You don't even need to write the initial markdown plan yourself. You can write that with GPT Pro, just explaining what it is you want to make.
