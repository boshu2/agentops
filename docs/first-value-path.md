# First value path

1. Give RPI one concrete behavior.
2. Plan writes normal and edge Given/When/Then examples, non-goals, evidence,
   and an explicit write scope.
3. Implement runs one bounded experiment and records actual changed paths and
   factual checks.
4. A fresh context runs Validate against the exact subject manifest.
5. Validate returns the result; RPI reports it and stops. Persist `verdict.v2`
   only when machine-readable evidence was requested.

For a small task this should be one reviewable change and one independent
judgment—not a project-management ceremony. Optional premortems, councils,
specialists, trackers, and factory adapters are chosen by the caller only when
they add value.

Success is fresh independent judgment bound to acceptance and content
identities. Pushing or releasing that content is a separate repository decision.
