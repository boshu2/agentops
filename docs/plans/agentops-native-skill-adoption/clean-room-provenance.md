# Clean-room provenance and implementation isolation

## Permitted implementation inputs

New AgentOps skill bodies may use only:

- `behaviors.md`, `spec.md`, `beads-manifest.md`, and the executable acceptance tests;
- `docs/audits/external-skill-corpus-2026-07-09.md` (the behavior-level audit);
- the two captured name-only manifests under `docs/audits/manifests/`;
- AgentOps-owned code, docs, prior recon packs, schemas, tests, and skill contracts;
- live self-describing NTM/Agent Mail/GC command and schema output where adapter
  conformance requires it.

External package bodies, references, scripts, examples, prompts, and assets are
not permitted implementation inputs. Names are inventory facts; the chosen
AgentOps names were explicitly directed by the operator.

## Isolation procedure

1. A fresh-context implementation worker receives only the permitted tracked
   paths above and the target write scope. Its prompt explicitly forbids reading
   user-local external skill roots.
2. The integrating agent may review the draft only against AgentOps contracts and
   acceptance tests. It must not resolve wording questions by opening an external
   package.
3. A mechanical clean-room check scans new source skills for forbidden external
   package names, provider mechanics, copied examples, and suspicious shared
   word sequences when a local external root is available.
4. The local similarity run writes a receipt with compared paths and findings;
   CI, where the private corpus is absent, still runs the name/provider/example
   denylist and validates this provenance file plus captured manifest hashes.
   Tests tamper a copied manifest and require rejection, so the check cannot
   merely count lines.
5. A fresh-context reviewer records PASS/WARN/FAIL before the new skills can land.

## Known exposure disclosure

The research lane read the named/high-value package bodies to produce the audit.
The integrating agent saw research summaries and a small number of cited source
excerpts while diagnosing prior-art drift. Therefore this is not a legal-style
double-blind clean room. The control is behavioral re-specification plus a fresh
drafting context and mechanical similarity review; no claim stronger than that
is made.

## Required receipt

The final proof packet must record:

- fresh implementation context id and allowed-path prompt;
- compared AgentOps source paths;
- external root used locally, or `unavailable`;
- longest suspicious shared sequence and disposition;
- forbidden name/provider/example scan result;
- independent reviewer verdict.

The tracked machine receipt is
`docs/audits/clean-room-adoption-receipt-2026-07-09.json`. The gate requires a
nonempty fresh implementation context id, the exact allowed-path prompt digest,
both captured manifest digests, compared source paths, local-external-root or
`unavailable`, longest shared sequence and disposition, CI denylist outcome,
and the independent reviewer verdict. Missing or placeholder fields fail.
