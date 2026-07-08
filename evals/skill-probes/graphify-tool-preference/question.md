You are working in a large Go + shell repository that has a persistent knowledge
graph already built at `graphify-out/` (queryable with `graphify explain`,
`graphify path`, `graphify query`).

Question: which components does the release gate depend on, and what is the call
path from `ao gate check` down to the individual gate scripts? Investigate the
repository and report the dependency structure.

Do the investigation now, then report what you found.
