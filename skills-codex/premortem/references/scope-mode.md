# Scope Mode Selection

> Extracted from premortem/SKILL.md on 2026-04-11.

Before running council, determine the review posture. Three modes:

| Mode | When to Use | Posture |
|------|-------------|---------|
| **SCOPE EXPANSION** | Greenfield features, user says "go big" | Dream big. What's the 10-star version? Push scope UP. |
| **HOLD SCOPE** | Bug fixes, refactors, most plans | Maximum rigor within accepted scope. Make it bulletproof. |
| **SCOPE REDUCTION** | Plan touches >15 files, overbuilt | Strip to essentials. What's the minimum that ships value? |

## Auto-Detection (when user doesn't specify)

- Greenfield feature → default EXPANSION
- Bug fix or hotfix → default HOLD SCOPE
- Refactor → default HOLD SCOPE
- Plan touching >15 files → suggest REDUCTION
- User says "go big" / "ambitious" → EXPANSION

## Critical Rule

Once mode is selected, COMMIT to it in the council packet. Do not silently drift. Include `scope_mode: <expansion|hold|reduction>` in the council packet context.

## Mode-Specific Council Instructions

- **EXPANSION:** Add to judge prompt: "What would make this 10x more ambitious for 2x the effort? What's the platonic ideal? List 3 delight opportunities."
- **HOLD SCOPE:** Add to judge prompt: "The plan's scope is accepted. Your job: find every failure mode, test every edge case, ensure observability. Do not argue for less work."
- **REDUCTION:** Add to judge prompt: "Find the minimum viable version. Everything else is deferred. What can be a follow-up? Separate must-ship from nice-to-ship."
