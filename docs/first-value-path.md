# First value path

1. Give your native coding agent a concrete behavior and acceptance examples.
2. Let it inspect the relevant code, implement the change and run the repository's
   checks. Planning and specialist guidance are available when they help.
3. Have a fresh, author-distinct context review the exact change against the
   unchanged acceptance and evidence.
4. Repair concrete findings and review the changed content. Finish when the
   accepted behavior is proven; report missing evidence honestly.

No AgentOps skill installation or RPI invocation is required. `ao quick-start`
explains this path and `ao demo` prints a sample task. `ao` supplies deterministic
checks and evidence utilities on demand; the coding agent supplies judgment.
Use a native goal for continuity and your existing tracker for work and handoffs.

For a small task this should be one reviewable change and one independent
judgment. Persist `verdict.v2` only when a caller or declared consumer needs
machine-readable evidence, using protected external non-Git storage.

Success is fresh independent judgment bound to acceptance and content
identities. Pushing or releasing that content follows repository policy.
