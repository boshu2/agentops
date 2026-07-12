---
name: product
description: Create or refine PRODUCT.md.
---
# $product — Interactive PRODUCT.md Generation

> **Loop position:** move 1 (shape intent) of the [operating loop](../../docs/architecture/operating-loop.md) — defines the PRODUCT.md that anchors what counts as in-scope intent before discovery shapes a capability into testable behaviors (the S2 handoff into the [narrow-waist micro-cycle](../../docs/architecture/operating-loop.md#the-narrow-waist-micro-cycle-canonical--every-loop-skill-cites-this)).

> **Purpose:** Guide the user through creating a `PRODUCT.md` that unlocks product-aware reviews in `$pre-mortem` and `$validate`, including the default quick-mode inline paths.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

**CLI dependencies:** None required.

## Constraints

- **Preserve user authority over the product claim.** Never overwrite an existing `PRODUCT.md` without the user's explicit choice, because product intent cannot be inferred safely from repository text alone.
- **Separate evidence from aspiration.** Label unmeasured claims and pre-traction assumptions honestly, because this file governs downstream scope and review judgment.
- **Keep gaps and alternatives honest.** Record where competitors win and what remains broken, because marketing-only framing makes `$discovery`, `$pre-mortem`, and `$validate` optimize against fiction.
- **Consult the pawl before raising the andon.** A plain WARN, FAIL, or REFUTED result repairs and reruns automatically; only a breaker may enter HOLD or consume the one-helper lane.

## Breaker State Machine

- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the product artifact and rerun the pawl; plain rejection never enters HOLD and never consumes the helper lane.
- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** pause automation in HOLD and route exactly one bounded helper consultation.
- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, resume automatic repair, and re-earn an independent verdict before proceeding.
- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and surface the helper's escalation to the human operator.
- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** stop automation and route directly to the human operator with the helper skipped.

## Execution Steps

Given `$product [target-dir]`:

- `target-dir` defaults to the current working directory.

### Step 1: Pre-flight

Check if PRODUCT.md already exists:

```bash
ls PRODUCT.md 2>/dev/null
```

**If it exists:**

Use AskUserQuestion:
- **Question:** "PRODUCT.md already exists. What would you like to do?"
- **Options:**
  - "Overwrite — start fresh" → continue to Step 2
  - "Update — keep existing content as defaults" → read existing file, use its values as pre-populated suggestions in Step 3
  - "Cancel" → stop, report no changes

**If it does not exist:** continue to Step 2.

### Step 2: Gather Context

Read available project files to pre-populate suggestions:

1. **README.md** — extract project description, purpose, target audience
2. **package.json / pyproject.toml / go.mod / Cargo.toml** — extract project name
3. **Directory listing** — `ls` the project root for structural hints
4. **Existing product/release docs** — if present, read `PRODUCT.md`, `GOALS.md`, release notes, comparison docs, and recent `.agents/research/` or `.agents/plans/` artifacts for PMF, positioning, and evidence context

Use what you find to draft initial suggestions for each section. If no files exist, proceed with blank suggestions.

### Step 3: Interview

Ask the user about each section using AskUserQuestion. For each question, offer pre-populated suggestions from Step 2 where available.

#### 3a: Mission

Ask: "What is your product's mission? (One sentence: what does it do and for whom?)"

Options based on README analysis:
- Suggested mission derived from README (if available)
- A shorter/punchier variant
- "Let me type my own"

#### 3b: Target Personas

Ask: "Who are your primary users? Describe 2-3 personas."

For each persona, gather:
- **Role** (e.g., "Backend Developer", "DevOps Engineer")
- **Goal** — what they're trying to accomplish
- **Pain point** — what makes this hard today

Use AskUserQuestion for the first persona's role, then follow up conversationally for details and additional personas. Stop when the user says they're done or after 3 personas.

#### 3c: Core Value Propositions

Ask: "What makes your product worth using? List 2-4 key value propositions."

Options:
- Suggestions derived from README/project context
- "Let me type my own"

#### 3d: Competitive Positioning

Ask: "What alternatives exist, and how do you differentiate?"

Gather for each competitor:
- Alternative name
- Their strengths (where they win)
- Your differentiation (where you win)
- Feature-level comparison (specific capabilities, not just vibes)

Then ask: "What is the market trend you're betting on that competitors are ignoring?"

This produces the Strategic Bet section — the contrarian thesis that justifies your product's existence. Examples:
- "We bet that AI agents will need institutional memory, not just prompts"
- "We bet that local-first tools will win over cloud-dependent ones"

If the user says "none" or "skip" for competitors, write "No direct competitors identified" but still ask about the strategic bet.

#### 3e: Evidence (Traction + Impact)

Ask: "What evidence do you have that this product works?"

Gather what's available:
- **Usage data** — stars, downloads, clones, active users, installs
- **Measured impact** — bugs caught, time saved, regressions prevented, outcomes achieved
- **User feedback** — testimonials, retention signals, community activity

**Auto-gather if possible:**
- If the project has a GitHub remote, pull real metrics: `gh api repos/{owner}/{repo} --jq '{stars: .stargazers_count, forks: .forks_count, open_issues: .open_issues_count}'`
- If `.agents/` exists, count learnings, council verdicts, and retros as usage evidence
- If `GOALS.md` exists, pull fitness score as a quality metric

If the project is new with no evidence yet, write "Pre-traction — evidence to be gathered" and list what metrics to track.

#### 3f: Known Product Gaps

Ask: "What's broken, missing, or embarrassing about the product right now? Be honest."

This section is the most valuable one for internal product docs. It prevents the doc from being marketing copy. Gather:
- **Missing capabilities** — features users ask for that don't exist
- **Broken promises** — things the README claims that don't fully work
- **Onboarding friction** — where new users get stuck
- **Technical debt** — known limitations that affect product quality

If the user says "nothing", gently challenge: "Every product has gaps. What would a frustrated user complain about?" Push for at least 2 honest gaps.

#### 3g: Product Sense Pass

Ask: "What would give your target audience a 10-star experience?"

Use the nine lenses in [references/product-frameworks.md](references/product-frameworks.md#product-sense-lenses-framework-reference) as a mandatory judgment pass, but translate them into decisions rather than name-dropping frameworks. Capture the PMF wedge, anti-personas, first 30–60 minute 10-star experience, retention loop, moat, and adoption-killing friction.

#### 3h: Validated Principles (Auto-discovered)

**Do not ask the user.** Scan `.agents/planning-rules/`, `.agents/patterns/`, and `.agents/learnings/`. Include discovered principles with counts and source links; if none exist, omit the validated-principles block.

### Step 4: Generate PRODUCT.md

Write `PRODUCT.md` to the target directory with this structure:

Use the canonical [PRODUCT.md template](references/product-frameworks.md#canonical-productmd-template). Set `last_reviewed` to today's date (YYYY-MM-DD), preserve every required section, and omit only the explicitly optional validated-principles block.

**Checkpoint:** before writing, confirm the user-approved mission/personas, evidence-vs-aspiration labels, honest gaps/alternatives, PMF wedge, 10-star journey, and rollback choice for an existing file. After writing, validate the resolved `<target-dir>/PRODUCT.md` path from the Output Specification.

### Step 5: Report

Tell the user:

1. **What was created:** `PRODUCT.md` at `{path}`
2. **What it unlocks:**
   - `$pre-mortem` will now load product context by default, including in `--quick` mode; deeper modes add a dedicated product perspective
   - `$validate` will now load developer-experience context by default, including in `--quick` mode; deeper modes add a dedicated DX perspective
   - `$council --preset=product` and `$council --preset=developer-experience` are available on demand
3. **Next steps:** Suggest running `$pre-mortem` on their next plan to see product perspectives in action

## Output Specification

- **Path:** `<target-dir>/PRODUCT.md` in the user-selected target directory.
- **Filename convention:** `PRODUCT.md`.
- **Serialization/schema format:** Markdown with a closed leading YAML frontmatter block containing exactly one valid `last_reviewed`, followed by the canonical body section order in [references/product-frameworks.md](references/product-frameworks.md#canonical-productmd-template).
- **Validator command:** resolve the target once, then run `bash skills/product/scripts/validate.sh --artifact "<target-dir>/PRODUCT.md"`; never fall back to `./PRODUCT.md`.
- **Downstream handoff:** after validation, hand the same resolved `<target-dir>/PRODUCT.md` to `$discovery`, `$pre-mortem`, and `$validate`. Plain rejection returns here for AUTO-REDO; only the breaker state machine may enter HOLD or HUMAN.

## Quality Checklist

- Mission, personas, value propositions, PMF wedge, and 10-star journey agree rather than targeting different users.
- Evidence is dated and attributable; aspirations and pre-traction hypotheses are labeled instead of presented as measured fact.
- Competitive positioning says where alternatives win, and Known Product Gaps names at least two concrete adoption or product risks.
- The generated file preserves user-approved intent, passes the validator, and hands a testable scope boundary to `$discovery`.

## Examples

**User says:** `$product`

The agent confirms create/update intent, interviews for the required decisions, labels evidence honestly, runs the checkpoint, and writes the canonical template. Updating follows the same path with existing content as defaults; cancellation leaves the file unchanged.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| No context to pre-populate suggestions | Missing README or project metadata files | Continue with blank suggestions. Ask user to describe project in own words. Extract mission from conversation. |
| User unclear on personas vs users | Confusion about persona definition | Explain: "Personas are specific user archetypes with goals and pain points. Think of one real person who would use this." Provide example. |
| Competitive landscape feels forced | Genuinely novel product or niche tool | Accept "No direct competitors" as valid. Focus on alternative approaches (manual processes, scripts) rather than products. Still ask for strategic bet. |
| PRODUCT.md feels generic | Insufficient user input or rushed interview | Ask follow-up questions. Request specific examples. Challenge vague statements like "makes things easier" — easier how? Measured how? |
| 10-star experience is vague | User describes features instead of an experience | Walk through the first 30-60 minutes minute-by-minute. Ask what the user sees, trusts, shares, repeats, or would miss tomorrow. |
| PMF wedge is too broad | User lists every possible customer | Ask who would be very disappointed if the product disappeared and who should be ignored until that segment loves it. |
| User resists Known Gaps section | Discomfort admitting weaknesses | Explain: "This is an internal doc, not marketing. Honest gaps prevent the team from building on false assumptions. Every product has them." Push for at least 2. |
| No usage data available | Pre-launch or private project | Write "Pre-traction" with a list of metrics to track once launched. The section's presence reminds future updates to fill it in. |
| `gh api` fails or no GitHub remote | Private repo, no auth, or non-GitHub host | Skip auto-gather gracefully. Ask user to provide metrics manually. |
| No .agents/ directory for principles | Project doesn't use AgentOps | Skip the validated principles section entirely. Include user-stated design principles instead. |

## Reference Documents

- [references/product-frameworks.md](references/product-frameworks.md) — §3g product-sense lenses: the nine-framework name-drop table (Chesky, Rahul Vohra, April Dunford, Teresa Torres, Marty Cagan, Gibson Biddle, Elena Verna, Melissa Perri, Shreyas Doshi) with each lens's question and the section it shapes (moved out of SKILL.md in the generic-craft trim)
- [references/product.feature](references/product.feature) — Executable spec: context gather, interview-driven PRODUCT.md, product-aware council unlock, quick-mode inline (soc-qk4b)
