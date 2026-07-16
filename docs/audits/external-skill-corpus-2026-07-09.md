# External skill corpus audit — 2026-07-09

## Decision

AgentOps should own a small set of behavior contracts, improve existing owners,
and leave product-specific utilities external. The selected additions are
`idea-genie`, `dueling-idea-genies`, `pattern-mining`, and `pawl-review`.
The existing `codebase-recon`, `ntm`, `agent-mail`, `using-gc`, `validate`,
`research`, `operationalize`, `agent-native`, and entry-point skills are improved
in place. `using-atm` is merged into `agent-native` with its transport/review/
coordination behavior split to the proper owners; no `repo-recon` alias is
introduced.

This is a clean-room behavior audit. Package names and structural metadata are
inventory facts. No protected prose, scripts, examples, prompts, fixed funnels,
or provider-specific mechanics are copied into AgentOps.

## Corpus boundary and reproducibility

The current JSM query returned two relevant sets:

| Set | Count | Reproduction | Sorted-name SHA-256 |
|---|---:|---|---|
| Official Jeffrey packages | 100 | `jsm list --remote --jeffreys --json` → `.skills[].name` | `998d60db8d18a807cee49c4773bdf394a4835a3f80b39139a3b11e67d3c2f7f9` |
| Local registry | 125 | `jsm list --json` → `.skills[].name` | `290f7006917342788d7291b56d096bad6b5d8a5f2e577d5eec88d71d126d14dc` |

All 100 official packages are installed. The other 25 are local/community
companions. The prior 2026-05-16 snapshot contained 118 locally registered
packages and 3,377 files; the current official packages contain 3,727 files,
while the local 125-package superset contains 4,198. Package structure was
inventoried for every package. The requested/high-usage packages and additional
high-value candidates were read deeply; reading every protected reference and
script body would be unnecessary for disposition and would weaken the clean-room
boundary.

Completeness check: parsing the two disposition tables and comparing them to the
captured sorted manifests reports `official rows=100 unique=100 missing=[]
extra=[]` and `local rows=25 unique=25 missing=[] extra=[]`.

## Prior AgentOps work

- A 45-package pattern-only absorption matrix existed before commit
  `c1a46cee2`; it routed codebase methods into research/review/doc, testing
  methods into test, and orchestration methods into coordination owners.
- `idea-option-forge` was tried, received zero measured use, and was retired.
- The four codebase packages were previously imported, then consolidated and
  removed. Their recurring output survived as the `docs/audits/codebase-recon-*`
  capability; four later recon runs demonstrate continuing value.
- `codebase-audit` moved through `review` into `validate`; `review` is now
  retired. Any new recon surface must compose `validate`, not revive review.
- `vibing-with-ntm` is already dispositioned into `ntm`, but the live catalog
  still splits tending and dispatch across contradictory ATM-era docs.
- `brainstorm` was merged into discovery, yet discovery still delegates to the
  deleted skill and retains a fixed-count ideation block. This is a live broken
  ownership seam, not a reason to restore the old package.
- `ao skills graph`, `skills/catalog.json`, and the generated context map already
  exist. The gap is completeness: dependency metadata and mesh health are not
  included in the maintained graph.

## Usage and value evidence

The 2026-06-10 usage audit recorded `operationalizing-expertise` 15,
`idea-wizard` 9, `vibing-with-ntm` 8, `codebase-audit` 7,
`codebase-archaeology` 4, `codebase-pattern-extraction` 3,
`dueling-idea-wizards` 2, and `codebase-report` 2. The four recorded recon packs
also found new live defects on later runs, so recon has demonstrated repeat
value rather than one-off novelty.

The useful behavior kernels are:

- ideas: ground in current reality, diverge before selection, reconcile
  overlap, and hand survivors to BDD;
- idea challenge: seal independent proposals, cross-review them, preserve
  disagreement, and use stronger ceremony only for one-way doors;
- recon: follow entry points and representative data flows, persist evidence,
  distinguish observation from inference, and prefer deltas on repeat runs;
- pattern mining: require multiple exemplars, separate invariant from variation,
  test a holdout, and back-apply the proposed abstraction;
- NTM factory operation: observe → decide → act → verify → stop, with explicit
  pane roles, liveness proof, bounded recovery, and Agent Mail coordination;
- operationalization: route proven knowledge into the weakest sufficient shape.

The rejected traits are fixed idea quotas, mandatory backlog inflation,
false-precision scores, mandatory NTM for reversible thought work, unbounded
search, auto-hook report generation, unrefuted audit findings, duplicated tool
manuals, and ATM alias/fork mechanics as product doctrine.

## Official 100 — one disposition each

| # | External package | AgentOps disposition |
|---:|---|---|
| 1 | `agent-fungibility-philosophy` | absorb into `agent-native` + `swarm` role/replaceability rules |
| 2 | `asupersync-mega-skill` | keep external; unrelated runtime specialization |
| 3 | `beads-compliance-and-completion-verification` | absorb into `validate` + `ao done` evidence checks |
| 4 | `brennerbot-with-ntm` | keep external/on-demand; investigation specialization |
| 5 | `cass` | existing owner `cass` |
| 6 | `code-review-gemini-swarm-with-ntm` | absorb into `council` (review-swarm fresh sessions, model-downgrade detection) + `validate` completion verification |
| 7 | `codebase-archaeology` | improve existing `codebase-recon` |
| 8 | `codebase-audit` | improve `codebase-recon` bounded audit + `validate` verdict |
| 9 | `codebase-pattern-extraction` | create `pattern-mining`, consumed by `operationalize` |
| 10 | `codebase-report` | improve existing `codebase-recon` durable report/delta |
| 11 | `csctf` | keep external; domain utility |
| 12 | `cursor` | keep external; editor adapter |
| 13 | `dcg` | existing owner `dcg` |
| 14 | `de-monolithize-your-codebase-isomorphically` | absorb into `refactor` |
| 15 | `de-slopify` | absorb into `doc` + `refactor` |
| 16 | `documentation-website-for-software-project` | keep external; product-specific generator |
| 17 | `dsr` | keep external; release/delivery is caller-owned (4.0 boundary) |
| 18 | `dueling-idea-wizards` | create original `dueling-idea-genies` |
| 19 | `e2e-testing-for-webapps` | absorb into `test` + `validate` |
| 20 | `extreme-software-optimization` | absorb into `refactor` + `test` proof loop |
| 21 | `focr` | keep external; specialized tool |
| 22 | `frankensearch-integration-for-rust-projects` | keep external; project-specific integration |
| 23 | `frankensuite-website-development` | keep external; project-specific development |
| 24 | `frankentui` | keep external; specialized TUI toolkit |
| 25 | `fsnow` | keep external; specialized tool |
| 26 | `ga4` | keep external; external product adapter |
| 27 | `gcloud` | keep external; external product adapter |
| 28 | `gdb-for-debugging` | keep external/on-demand; debugger specialization |
| 29 | `gh-actions` | keep external; CI/delivery is caller-owned (4.0 boundary) |
| 30 | `gh-cli` | keep external; publishing is caller-owned (4.0 boundary) |
| 31 | `gh-og-share-images` | keep external; content utility |
| 32 | `gh-triage-ru` | keep external; domain-specific triage |
| 33 | `ghostty` | keep external; terminal adapter |
| 34 | `giil` | keep external; specialized utility |
| 35 | `git-repo-janitor` | keep external/on-demand; repository maintenance utility |
| 36 | `git-stash-janitor` | keep external/on-demand; repository maintenance utility |
| 37 | `git-worktree-branch-rationalization` | keep external/on-demand; repository maintenance utility |
| 38 | `idea-wizard` | create original `idea-genie`; routed from `plan` (discovery retired, 4.0) |
| 39 | `installer-workmanship` | absorb into `scaffold` installer quality |
| 40 | `interactive-visualization-creator` | keep external; output specialization |
| 41 | `lean-formal-feedback-loop` | keep external; language/tool specialization |
| 42 | `library-updater` | keep external; dependency/release ratchet is caller-owned (4.0 boundary) |
| 43 | `mcp-server-design` | absorb into `standards` + `scaffold` |
| 44 | `mock-code-finder` | absorb into `validate` bounded audit |
| 45 | `modes-of-reasoning-project-analysis` | absorb into `council` perspectives |
| 46 | `multi-model-triangulation` | absorb into `council` and membrane diversity rules |
| 47 | `multi-pass-bug-hunting` | absorb into `validate` convergence passes |
| 48 | `ntm` | improve existing AgentOps `ntm`; live contract stays external |
| 49 | `og-share-images` | keep external; content utility |
| 50 | `operationalizing-expertise` | improve existing `operationalize`; remove retired routes |
| 51 | `path-rationalization` | keep external/on-demand; filesystem specialization |
| 52 | `pi-agent-rust` | keep external; project specialization |
| 53 | `profiling-software-performance` | absorb into `refactor` + `test` evidence loop |
| 54 | `rch` | existing owner `rch` |
| 55 | `readme-writing` | absorb into `doc --mode=readme` |
| 56 | `reality-check-for-project` | existing owner `reality-check` |
| 57 | `release-preparations` | keep external; release is caller-owned (4.0 boundary); changelog craft absorbed into `doc` |
| 58 | `repeatedly-apply-skill` | absorb into bounded `rpi` convergence (evolve retired, 4.0) |
| 59 | `research-software` | existing owner `research` |
| 60 | `ru-multi-repo-workflow` | absorb into `rch` validation; release side caller-owned (4.0 boundary) |
| 61 | `running-the-gauntlet-on-your-rust-port` | absorb into `test` + `validate` |
| 62 | `rust-cli-with-sqlite` | absorb into `scaffold` reference patterns |
| 63 | `rust-crates-publishing` | keep external; publishing is caller-owned (4.0 boundary) |
| 64 | `rust-undefined-behavior-exorcist` | absorb into `security` + `test` |
| 65 | `rust-unsafe-code-exorcist` | absorb into `security` + `test` |
| 66 | `saas-billing-patterns-for-stripe-and-paypal` | keep external; SaaS domain |
| 67 | `saas-cli-auth-flow` | keep external; SaaS domain |
| 68 | `saas-customer-analytics` | keep external; SaaS domain |
| 69 | `security-audit-for-saas` | absorb generic proof shape into `security`; keep SaaS specifics external |
| 70 | `seo-for-saas-businesses` | keep external; SaaS/GTM domain |
| 71 | `simplify-and-refactor-code-isomorphically` | absorb into `refactor` |
| 72 | `slack-migration-to-mattermost-phase-1-extraction` | keep external; migration domain |
| 73 | `slack-migration-to-mattermost-phase-2-setup-and-import` | keep external; migration domain |
| 74 | `slack-migration-to-mattermost-phase-3-ongoing-maintenance` | keep external; migration domain |
| 75 | `slb` | absorb into `dcg` + `scope` approval policy |
| 76 | `ssh` | absorb only remote execution boundaries into `rch` + `agent-native`; keep command manual external |
| 77 | `stripe-checkout` | keep external; product adapter |
| 78 | `supabase` | keep external; product adapter |
| 79 | `system-performance-remediation` | absorb into `sbh`, `status`, and `rch` |
| 80 | `tanstack` | keep external; framework specialization |
| 81 | `tax-return-preparation-and-advice-generic` | keep external; regulated domain |
| 82 | `testing-conformance-harnesses` | absorb into `test` |
| 83 | `testing-fuzzing` | absorb into `test` |
| 84 | `testing-golden-artifacts` | absorb into `test` |
| 85 | `testing-real-service-e2e-no-mocks` | absorb into `test` + `validate` |
| 86 | `tui-glamorous` | keep external; UI specialization |
| 87 | `tui-inspector` | keep external; UI specialization |
| 88 | `ubs` | absorb into `validate` + `security` |
| 89 | `ui-polish` | keep external; UI specialization |
| 90 | `user-support-ticketing-system-for-saas` | keep external; SaaS domain |
| 91 | `user-support-triage-for-saas-and-open-source-projects` | keep external; support domain |
| 92 | `ux-audit` | absorb generic evidence lens into `codebase-recon` + `validate` |
| 93 | `vercel` | keep external; publishing is caller-owned (4.0 boundary), platform manual external |
| 94 | `vibing-with-ntm` | keep retired; split behavior between `ntm` adapter mechanics and `agent-native` lifecycle |
| 95 | `video-obs-youtube-music` | keep external; media utility |
| 96 | `wezterm` | absorb pane/terminal boundary only into `ntm`; keep terminal manual external |
| 97 | `wills-and-estate-planning-skill` | keep external; regulated domain |
| 98 | `world-class-doctor-mode-for-cli-tools` | absorb into `standards` + live CLI doctor/gates |
| 99 | `wrangler` | keep external; platform adapter |
| 100 | `xf` | absorb search routing into `research` + `cass`; keep network tool external |

## Local/community 25 — one disposition each

| # | Companion | AgentOps disposition |
|---:|---|---|
| 1 | `ab-testing` | absorb into `goals`/evaluation evidence; no standalone skill |
| 2 | `admin-page-for-nextjs-sites` | keep external; application-domain template |
| 3 | `agent-ergonomics-and-intuitiveness-maximization-for-cli-tools` | absorb into `standards` + CLI gates |
| 4 | `agent-mail` | improve existing AgentOps `agent-mail` as BC6 adapter |
| 5 | `automating-your-automations` | existing owner `toil-mining` |
| 6 | `bd-to-br-migration` | absorb into `beads-br` migration references |
| 7 | `beads-br` | existing owner `beads-br` |
| 8 | `beads-bv` | existing owner `beads-bv` |
| 9 | `beads-workflow` | absorb into `plan` bead-shaping (discovery retired, 4.0) |
| 10 | `brenner` | keep external/on-demand; investigation specialization |
| 11 | `browser-extension-automation` | keep external; browser specialization |
| 12 | `browser-testing-with-ntm` | absorb worker lifecycle into `agent-native`; test method into `test` |
| 13 | `caam` | existing owner `account-rotation` |
| 14 | `cass-memory` | existing owner `cass` |
| 15 | `cc-hooks` | existing optional owner `cc-hooks` |
| 16 | `changelog-md-workmanship` | absorb into `doc` changelog craft; release mechanics caller-owned (4.0 boundary) |
| 17 | `deadlock-finder-and-fixer` | absorb into `validate` bounded audit + `status` recovery |
| 18 | `detect-forgotten-sessions-post-crash` | absorb into `ntm` liveness + `status` recovery |
| 19 | `document-to-latex` | keep external; format specialization |
| 20 | `papers` | absorb research routing into `research`; keep network/package external |
| 21 | `planning-workflow` | existing owner `plan` (discovery retired, 4.0) |
| 22 | `redacting-sensitive-parts-of-screencast-videos` | keep external; media utility |
| 23 | `rg-optimized` | absorb bounded retrieval hints into `research`; keep utility external |
| 24 | `testing-metamorphic` | absorb into `test` |
| 25 | `transcribing-audio-from-calls-and-meetings` | keep external; media utility |

## Selected implementation portfolio

1. `idea-genie`: adaptive evidence-grounded ideation with a valid no-new-work
   result; discovery owns tracker creation.
2. `dueling-idea-genies`: independent challenge for contested one-way doors;
   no mandatory NTM and no false-precision aggregate.
3. `codebase-recon`: formalize and improve the existing recurring recon suite;
   delta-first, evidence-bounded, reusable across entry points.
4. `pattern-mining`: the strongest unowned method; promotion is earned by
   exemplars and holdout proof, then routed through `operationalize`.
5. `pawl-review`: transport-neutral reviewer execution, generalized from
   `pre-land-refuters`, with warm NTM panes as one adapter and deterministic
   `ao pawl` verdict binding kept separate.
6. Improve `agent-native` as the substrate-neutral worker lifecycle owner; split
   `using-atm` into `agent-native` lifecycle, `ntm` mechanics, `pawl-review`
   reviewer execution, and `agent-mail` coordination.
7. Improve `ntm`, `agent-mail`, `using-gc`, `operationalize`, discovery/plan,
   research/validate/doc/refactor, swarm/crank/evolve, and the automation router
   so the skill mesh has real inbound routes.
8. Extend the existing generated catalog/context graph into a complete,
   drift-gated skill graph; Graphify is an optional reader only.

## Risks and controls

- **Catalog regrowth:** five new skills are offset by retiring `using-atm` and
  `pre-land-refuters`; every
  addition has repeated use evidence, a product binding, executable acceptance,
  and inbound consumers.
- **Substrate coupling:** NTM, Agent Mail, and GC remain adapters behind BC6
  ports. GC delegation is explicit and optional; no mutual fallback is added.
- **Duplicated orchestration:** `agent-native` dispatches whole loop skills and
  never reimplements discovery/RPI/crank/validate phases; `pawl-review` owns only
  reviewer execution, not the deterministic verdict writer.
- **Graph graveyard:** graph artifacts are generated from frontmatter and gated
  by the existing regen path; no node or edge is manually curated.
- **Clean-room leakage:** implementation is based on this behavior summary and
  AgentOps-owned evidence, with an explicit name/prose/script/example review.

## Disclosure

This audit gives every official package and every local companion one
disposition. Structural inventory covered the full corpus. Deep semantic review
focused on the requested/high-usage packages and the additional methods selected
for AgentOps; it did not ingest all protected reference/script bodies.
