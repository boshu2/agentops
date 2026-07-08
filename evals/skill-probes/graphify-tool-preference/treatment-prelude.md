SKILL GUIDANCE (loaded): structural-graph navigation (the /research Tier-1b rule)

When graphify is installed and a graph is present (`graphify-out/` exists), query
STRUCTURE via `graphify explain` / `graphify path` / `graphify query` BEFORE
reaching for broad grep. The graph already encodes the dependency and call
relationships; a structural query answers "what depends on X" and "what is the
path from A to B" directly, whereas grep only finds string occurrences and makes
you reconstruct the structure by hand. Prefer the graph for any architecture,
dependency, or call-path question.
