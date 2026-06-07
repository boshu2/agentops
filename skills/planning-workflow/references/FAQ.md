# Planning FAQ — Reference

## Table of Contents
- [Skeleton-First vs Planning](#skeleton-first-vs-planning)
- [Handling Unanticipated Problems](#handling-unanticipated-problems)
- [Task Division for Agents](#task-division-for-agents)
- [Agent Specialization](#agent-specialization)
- [Design Decisions Location](#design-decisions-location)

---

## Skeleton-First vs Planning

**Q: Shouldn't I code a skeleton first?**

A: You get a better result faster by creating one big comprehensive, detailed, granular plan. That's the only way to get models to understand the entire system at once. Once you start turning it into code, it gets too big to understand.

---

## Handling Unanticipated Problems

**Q: What about problems I didn't anticipate?**

A: Finding the flaws and fixing them is the whole point of all the iterations and blending in feedback from all the frontier models. If you follow the procedure using those specific models and prompts, after enough rounds, you will have an extremely good plan that will "just work."

After implementing v1, you create another plan for v2. Nothing says you can only do one plan.

---

## Task Division for Agents

**Q: How do I divide tasks for agents?**

A: Each agent uses bv to find the next optimal bead and marks it in-progress. Distributed, robust, fungible agents.

---

## Agent Specialization

**Q: Do agents need specialization?**

A: No. Every agent is fungible and a generalist. They all use the same base model and read the same AGENTS.md. Simply telling one it's a "frontend agent" doesn't make it better at frontend.

---

## Design Decisions Location

**Q: Should design decisions be in markdown or beads?**

A: The beads themselves can and should contain this markdown. You can have long descriptions/comments inside the beads—they don't need to be short bullet point type entries.
