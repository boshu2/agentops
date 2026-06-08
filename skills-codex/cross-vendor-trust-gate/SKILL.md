---
name: cross-vendor-trust-gate
description: "Run the skill-factory final trust gate to grade a skill's cross-vendor trust before it lands — operate trust-gate.sh, read the skill.trust.json verdict, and enforce --require-cross. Never hand-wave parity."
---

# cross-vendor-trust-gate (Codex)

This is the Codex-runtime entry point for the **cross-vendor-trust-gate** skill.
The full operating guide — the three-level trust model, the flag interface, what
the gate validates on each side, the step-by-step grading procedure, the exit-code
table, and the guardrails — lives in the sibling base skill:

- **Canonical skill:** [`../../skills/cross-vendor-trust-gate/SKILL.md`](../../skills/cross-vendor-trust-gate/SKILL.md)

Read that file first. Then follow the Codex Execution Profile in [`prompt.md`](./prompt.md) for the runtime-specific tool mapping and guardrails.
