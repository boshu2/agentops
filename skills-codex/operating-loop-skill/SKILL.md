---
name: operating-loop-skill
description: "Drive one bead end-to-end through the assured operating loop: claim → work → independent-validate → close → persist. The on-disk, harness-portable form of the operating-loop + bead-crank plugin commands — loadable by Codex and Gemini, which cannot invoke Claude slash-commands. Operator-side; runs the flywheel binaries (br/bd + git) directly. Codex execution profile in `prompt.md`; full methodology in the sibling `../SKILL.md`."
---

# Operating Loop Skill (Codex)

Codex-portable form of the AgentOps operating loop + bead-crank. It drives **one tracker bead** through the assured trajectory **claim → work → independent-validate → close → persist** without depending on Claude plugin slash-commands.

- **Full methodology, constraints, output spec, rubric, examples, troubleshooting:** see the sibling `../SKILL.md` (the canonical, harness-neutral skill body).
- **Codex execution profile (numbered steps + guardrails):** see `prompt.md` in this directory.

The invariant is the loop; Codex realizes worker/validator fan-out via fresh `codex exec` invocations (or NTM panes). The **separate context** — not a separate vendor — is what makes "author ≠ judge" hold.
