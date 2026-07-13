# Domain Evolution Bootstrap

Use this reference when `/evolve` is asked to reshape AgentOps skills, CLI,
hooks, docs, tests, beads, or knowledge as one software-factory system.

## Product Frame

AgentOps is the SDLC control plane and context compiler for LLM agents. The
narrow waist is:

- BDD/Gherkin for observable intent;
- DDD for shared names and bounded contexts;
- Hexagonal architecture for ports and adapters;
- TDD for local proof of done;
- XP for small vertical slices;
- CI, SRE, ADRs, and provenance for repeated trust and memory.

Do not treat a skill, hook, or command as valuable by itself. It is valuable
only when it advances the factory loop: intent, boundary, proof, evidence, and
compounding context.

## Bootstrap Sequence

1. Read current direction from `PRODUCT.md`, `GOALS.md`,
   `docs/cdlc.md`, and `docs/architecture/operating-loop.md`.
2. Validate the durable control artifacts:

   ```bash
   bash scripts/check-agentops-domain-evolution-plan.sh
   ```

3. Select one domain and one vertical slice from:
   - `docs/reference/agentops-domain-evolution-bdd.md`
   - `docs/reference/agentops-skill-domain-map.md`
   - `docs/reference/agentops-hexagonal-architecture-map.md`
   - `docs/reference/agentops-domain-evolution-plan.md`
4. Use `skill-builder` and the heal-skill deep audit for skill changes. Use
   `skills/heal-skill/scripts/score_agentops_skill.py` to choose the
   smallest score-improving patch.
5. Keep CLI and hook changes behind typed ports or existing validation scripts.
6. Run focused validation before selecting the next slice.

## Hard Rules

- No broad rewrite before a Gherkin row, domain, and first proof are named.
- No third-party content copying; use clean-room structural observations only.
- No shipped skill deletion until replacement workflow and validation evidence
  exist.
- No shell-only read path when a typed port already exists.
- No unattended run from a dirty canonical root or stale installed `ao` binary.
