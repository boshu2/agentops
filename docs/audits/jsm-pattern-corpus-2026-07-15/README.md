# JSM pattern corpus — 2026-07-15

Structural pattern extraction over the full locally readable JSM reference
corpus: 117 of the 125 packages dispositioned in
[`external-skill-corpus-2026-07-09.md`](../external-skill-corpus-2026-07-09.md)
(5 excluded as byte-identical AgentOps-owned installs: cass, dcg, ntm,
agent-mail, cc-hooks; 3 not on disk: beads-br, beads-bv, beads-workflow).

Seven parallel analysts each read their batch's SKILL.md in full and scored it
against the 12 structural elements recorded in each row (`elements.e1..e12`).
All notes are paraphrase-only (clean-room:
no run of more than 8 consecutive source words).

## Files

- `skill-list.json` — the 117 analyzed packages with paths and dispositions
- `batch-{1..7}.jsonl` — raw per-analyst rows
- `corpus.jsonl` — merged, sorted by elite count (117 rows)

Row schema: name, disposition, category, sizes, `elements.e1..e12`
(present/partial/absent + note), frozen_prompt_count, loop, convergence,
quantified_rules, provenance_style, signature_craft, kernel_value, elite_count.

## Corpus statistics

Elite distribution: **24 elite (10–12)** · 37 strong (7–9) · 40 mid (4–6) ·
16 thin (0–3).

Element frequency (present, of 117): runnable commands 100 · router+refs 90 ·
anti-patterns w/ correctives 76 · trigger descriptions 74 · outcome/done 72 ·
quantified rules 71 · causal insight 61 · frozen prompts 60 (194 prompts
total) · named failure 49 · loop+convergence 49 (64 have both loop and stop
condition) · negative space 47 · **real provenance 33** (weakest element even
in this corpus).

Category quality (mean elite): security 10.3 · validation 9.0 ·
refactor-perf 8.2 · workflow 8.0 · ideation/judgment 8.0 · testing 7.8 ·
orchestration 7.8 · docs-content 7.9 · recon 5.8 · meta 5.0 ·
tooling-adapter 4.7 (27 skills — cheat-sheet adapters by design) ·
release-ci 4.7.

Perfect scores: `beads-compliance-and-completion-verification`,
`git-repo-janitor`, `git-stash-janitor`.

## Findings

1. **Ceremony scales with blast radius.** The 12/12 and 11/12 tier is
   dominated by skills gating destructive or costly operations (git janitors,
   stash/worktree rationalization, unsafe-Rust exorcists, billing): axiom
   kernels, byte-verified recovery bundles, verbatim authorization phrases,
   adversarial defeat attempts. Craft depth is proportional to
   irreversibility — the same proportionality doctrine AgentOps already
   states, executed with far more mechanism.
2. **The elite tier validates the 12-element template.** Elements cluster:
   skills with a causal insight almost always also carry quantified rules and
   a convergence condition; thin adapters lack all three. The template's red
   rows (frozen prompts, quantified rules, causal insight, provenance) are
   exactly what separates elite from mid.
3. **Provenance is the open flank.** Only 33/117 cite real repos/files where a
   rule was earned — the weakest element even in the reference corpus. Our
   verdict store and session archives can beat the reference standard here.
4. **Low scores are not always defects.** Pure routers
   (`operationalizing-expertise`, 2/12 surface score) keep depth in
   references; tool adapters are deliberately cheat-sheets. Scores measure
   SKILL.md surface structure, not value. Read `disposition` alongside
   `elite_count`.
5. **Kernel index for the enrichment waves.** `kernel_value` per row names the
   strongest transplantable method. High-value kernels for Waves 3–5 include:
   guilty-until-artifact-proven completion checks (validate), fresh-eyes
   multi-pass convergence (validate), scratch-branch seam experiments +
   isomorphism proof cards (refactor), oracle-strength hierarchies and
   mutation-kill proof (test), detection-stack + caller tracing (validate),
   ambition-escalation rounds with frozen templates (plan/reality-check).

## Caveats

- Scores are single-reader, SKILL.md-surface-only; reference bodies were sized
  but not deeply read. They identify candidates for closer review, not a work
  plan or authorization to enrich every skill.
- Category assignment and element judgments are one analyst's call per skill;
  treat ±1 element as noise.
- This corpus is evidence for authoring priorities. It is not a verdict and
  authorizes no wave.
