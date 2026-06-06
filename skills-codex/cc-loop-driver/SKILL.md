---
name: cc-loop-driver
description: |
  Run an assured control-plane tick loop on a subscription-billed agent harness — orchestrator pulls one ready bead, dispatches a fresh worker, dispatches a SEPARATE-context validator (author != judge), and only the orchestrator closes + commits + publishes on a verified PASS. Codex-native parity of the Claude-native cc-loop-driver.

  Triggers: "drive the loop", "cc-loop-driver", "assured tick loop", "run the control plane", "assured loop without NTM", "bd ready -> claim -> worker -> validate -> close", "single-writer close loop", "all-Codex factory loop".

  Use when:
  - You want the control-plane loop as a shippable skill in one session with no NTM/tmux dependency.
  - You have a `bd`/`br` bead ledger and want each bead worked + independently validated + closed + committed with evidence.
  - You need author != judge assurance via separate execution contexts.

  Perfect for: draining a ready queue overnight on the ChatGPT Pro subscription with no API burn; bootstrapping the factory before NTM/Agent-Mail turnout.
  Not ideal for: heavy parallel multi-repo swarms (use NTM + Agent Mail); work with no acceptance criteria (nothing to validate against).
---

# cc-loop-driver (Codex)

Codex-native parity wrapper. The full skill content — overview, the six critical
constraints (single writer, author != judge, no per-token API billing,
evidence-gated close, the-close-cannot-lie, scoped-stage), the six-phase
workflow, output spec, quality rubric, examples, and troubleshooting — lives in
the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
