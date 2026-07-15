---
name: idea-genie
description: Generate an evidence-grounded opportunity
---
# Idea Genie

Generate a small portfolio of evidenced options. This skill explores; it does
not select, schedule, track, implement, or validate work.

1. State the question, constraints, non-goals, and sources.
2. Separate cited observations from assumptions.
3. Give each candidate its supporting evidence, overlap with existing
   capabilities, and one normal or edge scenario.
4. Run a novelty pass, merge equivalents, and discard unsupported ideas.
5. Stop when no materially new evidenced candidate appears.
6. Write and validate `idea-portfolio.v1`, then return it to the caller or Plan.

An empty `no-new-work` portfolio is valid. Plan alone may turn an option into a
PlanPacket.
