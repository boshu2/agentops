# Deletion proposal — repo operating contract (DRAFT, Bo ratifies)

> Evidence base: 80 scored runs across 5 executors (luna, terra, sol-config,
> Opus 5, Fable) — sweeps 1/2/4 + tier-2 pilot + probe waves. Every verdict
> cites its measurement. This PR edits `AGENTS.md` per those verdicts and
> merges only on Bo's explicit ratification. The study supported a ~55% cut
> against its 2026-08-05 subject, but current `main` has since added mandatory
> anti-ceremony, federated-authority, and canonical-source contracts outside
> that experiment. This rebased proposal keeps those additions, yielding a
> narrower ~21% current-main reduction. The separate historical Codex sweep
> reported +8-12% tokens and no execution delta within its bounded design; it
> is not a claim of current skill-probe coverage.

| Section | Verdict | Evidence | Action |
|---|---|---|---|
| Intro + loop diagram | KEEP | orientation; 3 lines | trim |
| Authority and trust | **SURVIVOR** | Opus/Fable t03/t04 applied its rules mid-execution (sweep 4); the one contract surface observed changing behavior | keep, light trim |
| Honest work and anti-ceremony | POST-STUDY CONTRACT | added on current main after the measured subject | keep current-main text |
| Runtime floor | ENFORCED | no-claude-p hook carries it; prose never observed load-bearing | shrink to pointer |
| Federated source authority | POST-STUDY CONTRACT | current federated ownership boundary | keep current-main text |
| Source precedence | KEEP (cheap) | orchestrator-facing, 4 lines | keep |
| Constraint floor | ENFORCED | python-ratchet gate + ADR carry it | shrink to pointer |
| Core loop | PRODUCT | dead for executors (sweeps 1-2: zero effect on GPT-line); alive as orchestrator spec (this session ran it all day) | compress ~50% |
| Product boundary | PRODUCT | boundary behavior; sweep-4 scope discipline consistent with it | keep current-main federated/factory rules |
| Concurrency | MOSTLY-NATIVE | rt-01: correct collision handling unaided; current rule also constrains delegation authority | keep current-main rule |
| Triggered-sources table | REQUIRED INDEX | `validate-agents-split` requires live links to CI and Codex contracts | keep current-main table |
| Closeout | PRODUCT | validate flow carries mechanics | compress |

Rebased net: ~10.5KB → ~8.3KB (-21%). Compressions reword older measured
sections; post-study doctrine and live contract links remain intact.
Follow-ups (separate PRs, not this one): (1) codex adapter scopes skill
injection by task class (sweep-1 receipt); (2) closure-artifact formats that
carry caveats (sweep-4 disclosure-lives-in-chat finding); (3) S3 rules-file
lever unmeasured — rules stay untouched (wave-1 measured their INLINE form
BEHAVIORAL; do not delete unmeasured surface).
