# Uncaptured Lesson Patterns

> 30 lessons identified from post-mortem and retro analysis across 14,753 sessions.
> These represent recurring failure modes that were experienced but never captured as learnings.
> Forge should look for these patterns when mining transcripts.

## How to Use

During transcript mining, match session content against these 26 patterns. When a match is found, extract it as a learning with the corresponding category and confidence score.

---

## Infrastructure & Tooling (8 lessons)

### 1. Dead Infrastructure Activation
**Pattern:** Enforcement tooling (git hooks, pre-commit, linters) exists but isn't activated by default. Manual config required = never runs.
**Signal in transcript:** Discussion of hooks that should have caught something, mentions of "we have X but it wasn't enabled"
**Category:** tooling | **Confidence:** 0.85

### 2. Worktree Agents Must Commit Before Exit
**Pattern:** Agents working in git worktrees exit without committing, losing all work.
**Signal:** Worktree cleanup with uncommitted changes, "work was lost" discussions
**Category:** operations | **Confidence:** 0.80

### 3. Use /bin/cp in Automation
**Pattern:** Shell scripts use `cp` which may be aliased (to `cp -i` etc.), breaking non-interactive automation.
**Signal:** Interactive prompts blocking automation, "script hung" on copy operations
**Category:** tooling | **Confidence:** 0.90

### 4. Namespace Daemon State Per Project
**Pattern:** Global daemon state causes cross-project interference. State must be scoped per-project.
**Signal:** "Wrong project" errors, state from one project leaking into another
**Category:** tooling | **Confidence:** 0.80

### 5. Never Use Hostname as Identity
**Pattern:** Hostnames change. Use UUID per invocation for identity.
**Signal:** Identity collisions, "duplicate agent" errors after hostname change
**Category:** tooling | **Confidence:** 0.85

### 6. Never Hardcode Tokens
**Pattern:** Even in prototypes, hardcoded tokens leak to git history and are expensive to rotate.
**Signal:** Secrets in code, token rotation discussions, `.env` not in `.gitignore`
**Category:** security | **Confidence:** 0.95

### 7. Run Formatter Proactively
**Pattern:** Running formatters before commits prevents CI lint failures and noisy diffs.
**Signal:** CI failures on formatting, "just whitespace changes" commits
**Category:** tooling | **Confidence:** 0.85

---

## Planning & Execution (8 lessons)

### 8. Plan Oscillation Doubles Cost
**Pattern:** Reversing direction mid-execution (create → flatten, add → remove) doubles the mechanical propagation cost.
**Signal:** "Actually let's do the opposite", direction changes after files already modified
**Category:** process | **Confidence:** 0.90

### 9. Commit Per Wave, Not Per Session
**Pattern:** Batching commits to session end means one bad file contaminates the entire batch. Wave boundaries are natural commit points.
**Signal:** 50+ uncommitted files, "which changes go together?" confusion
**Category:** process | **Confidence:** 0.85

### 10. Merge to Main Frequently During Multi-Day Sprints
**Pattern:** Long-lived branches diverge. Merge to main at least daily during multi-day work.
**Signal:** Painful merge conflicts, "branch is way behind main", rebase nightmares
**Category:** process | **Confidence:** 0.85

### 11. Validate Direction Before Propagation
**Pattern:** Council-validate architectural direction BEFORE starting propagation work across files.
**Signal:** Large refactors that get reversed, "we should have checked first"
**Category:** process | **Confidence:** 0.90

### 12. Full Propagation Surface Enumeration
**Pattern:** Before CLI/namespace restructuring, enumerate ALL surfaces: Go source, tests, embedded hooks, external hooks, SKILL.md, docs, scripts.
**Signal:** Missed files after refactoring, "forgot to update X"
**Category:** process | **Confidence:** 0.85

### 13. Mark Dependency Drops Explicitly
**Pattern:** When a plan drops a dependency, mark it explicitly in the plan document so reviewers know it was intentional.
**Signal:** "Was this supposed to be removed?", confusion about missing dependencies
**Category:** process | **Confidence:** 0.80

### 14. Shared Types Package First
**Pattern:** Before isolating packages into `internal/`, create a shared types package to prevent circular imports.
**Signal:** Circular import errors, "can't import X from Y"
**Category:** architecture | **Confidence:** 0.85

### 15. Recognize Diminishing Returns
**Pattern:** After structural changes land, further refinement has diminishing returns. Stop and move on.
**Signal:** Repeated small fixes to the same area, "just one more tweak"
**Category:** process | **Confidence:** 0.80

---

## Knowledge & Validation (9 lessons)

### 16. Context Budget Rule: 40% Critical
**Pattern:** Session context above 40% causes quality degradation. Above 60% = 99% information loss. Fresh sessions per phase.
**Signal:** Hallucinations, forgotten earlier context, repeated questions
**Category:** operations | **Confidence:** 0.90

### 17. Forensic Multi-Agent Retros
**Pattern:** Single-agent retros miss ~23% of issues (hallucination baseline). Use multiple agents for retrospectives.
**Signal:** Post-mortem findings that contradict reality, "this learning is wrong"
**Category:** process | **Confidence:** 0.85

### 18. Vibe Check Is Essential
**Pattern:** Vibe checks (code quality reviews) catch bugs that tests miss. Don't skip them.
**Signal:** Bugs found in review that tests didn't catch, "good thing we reviewed this"
**Category:** process | **Confidence:** 0.85

### 19. Pre-Mortem Iterations Proportional to Blast Radius
**Pattern:** High-blast-radius changes need more pre-mortem iterations (10+). Low-blast changes need fewer (1-3).
**Signal:** Pre-mortem that missed a critical failure mode, "we should have checked more"
**Category:** process | **Confidence:** 0.80

### 20. Anchor Reverse Engineering to Registries
**Pattern:** Use registries, sitemaps, and structured indexes as anchors for reverse engineering — not string heuristics.
**Signal:** False positives in code analysis, "it picked up the wrong thing"
**Category:** tooling | **Confidence:** 0.80

### 21. Test Business Model Alignment Early
**Pattern:** Validate that the technical approach aligns with the business model before deep investment.
**Signal:** "This doesn't actually solve the problem", pivot after significant work
**Category:** process | **Confidence:** 0.85

### 27. Knowledge Normalization Defects
**Pattern:** Learning files with stacked frontmatter, bundled multiple learnings per file, placeholder patterns with no content, or duplicated headings that break extraction and citation tracking.
**Signal:** Frontmatter parse errors, multiple `## Learning` headings in one file, empty content after `---`, citation mismatches
**Category:** knowledge | **Confidence:** 0.85

### 28. Promotion Pipeline Bottleneck
**Pattern:** High learning production (>100 learnings) with low pattern extraction (<5 patterns). The 1.2% promotion rate indicates a pipeline gap, not a production problem.
**Signal:** Growing learning pool with stagnant pattern count, "we have lots of learnings but no patterns"
**Category:** knowledge | **Confidence:** 0.80

### 29. Three-Tier Taxonomy Violation
**Pattern:** Treating learnings as universal principles or principles as local learnings. The taxonomy is: learning (local, one rig) → pattern (domain, transferable) → principle (universal, 3+ domains).
**Signal:** Learnings applied cross-rig without validation, principles with single-rig evidence, "this learning applies everywhere"
**Category:** knowledge | **Confidence:** 0.75

---

## Git & Operations (5 lessons)

### 22. Beads Prefix Routing
**Pattern:** Any beads prefix needs a corresponding rig registered, or issues become orphaned.
**Signal:** "Unknown prefix" errors, issues that nobody picks up
**Category:** tooling | **Confidence:** 0.80

### 23. Formula Format Validation
**Pattern:** TOML template drift needs automated validation. Templates diverge from schemas silently.
**Signal:** Config parse errors, "the template doesn't match the expected format"
**Category:** tooling | **Confidence:** 0.75

### 24. Directory Naming Verification
**Pattern:** Validate directory names BEFORE creation, not after. Renaming directories is expensive.
**Signal:** "Wrong name, need to rename", propagation cost of directory renames
**Category:** process | **Confidence:** 0.85

### 25. Relative Link Adjustment on Extraction
**Pattern:** When extracting content to a subdirectory, all relative links need +1 `../` level.
**Signal:** Broken links after file moves, "link was working before the move"
**Category:** tooling | **Confidence:** 0.85

### 26. Build Visibility During Development
**Pattern:** Make work visible as it progresses (commits, PRs, status updates), not retrospectively.
**Signal:** "What have you been working on?", surprise at end-of-sprint
**Category:** process | **Confidence:** 0.80

### 30. Stale Contradiction Reports
**Pattern:** Contradiction findings that reference evidence from before the latest extraction pass. Later extractions may have resolved the contradiction, making the report misleading.
**Signal:** Contradiction report cites old timestamps, "this was already fixed", evidence predates last defrag
**Category:** knowledge | **Confidence:** 0.80

---

## Pattern Matching Guide

When mining a transcript, score matches as:

| Match Quality | Confidence Boost | Action |
|--------------|-----------------|--------|
| Exact match (same failure mode described) | +0.1 | Extract as learning immediately |
| Partial match (related failure, different context) | +0.05 | Extract with lower confidence |
| Thematic match (same category, different specifics) | +0.0 | Note for future pattern clustering |

## Integration with Forge Workflow

1. During transcript scan, check each significant event against all 30 patterns
2. When a match is found, pre-fill the learning template with the pattern's category and base confidence
3. Boost confidence by the match quality modifier
4. Add the pattern number as a tag (e.g., `uncaptured-lesson-8` for plan oscillation)
5. Track which patterns have been captured — goal is to capture all 30 within the knowledge base
