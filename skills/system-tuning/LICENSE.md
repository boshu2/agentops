# system-tuning — Attribution and Licensing

## Methodology Source

The triage method in this skill — diagnose → ordered kill hierarchy → confused-parent
detection → renice survivors → verify — was pattern-adopted from an external
`system-performance-remediation` skill corpus.

That external work is the conceptual reference. The shape of the kill hierarchy
(reap → stuck children → confused parents → renice) and the framing of the
"whack-a-mole" anti-pattern (kill the parent, not the child) are both due to
the original work.

## What Pattern-Adoption Means Here

This skill is a **clean-room reimplementation**:

- No prose, examples, tables, or shell snippets are copied verbatim from the source.
- Section structure, command choices, and per-runtime guidance were authored
  fresh for the AgentOps catalog, with different wording, different defaults,
  and AgentOps-specific operator surfaces (Bushido WSL, tmux, codex/claude
  agent topology).
- The intellectual contribution credited above is the methodology and the
  named anti-pattern. Implementation is original.

If you find a passage that overlaps the source by more than incidental
phrasing, treat it as a bug and rewrite it.

## License

Skill text (SKILL.md and references) is contributed under the AgentOps
repository license, the Apache License 2.0. See the repo-root LICENSE for
full terms.

## Contact

Issues, corrections, or attribution disputes: file a bd issue against the
`system-tuning` skill in this repo.
