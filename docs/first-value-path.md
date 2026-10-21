# First value path

1. Give your native coding agent a concrete behavior and acceptance examples
   using your project's domain terms. For example: given a Job has completed,
   when it is received again, return its result without repeating the side effect.
2. Let it inspect the relevant code, implement the change and run the repository's
   checks. Planning and specialist guidance are available when they help.
3. For an ordinary change those checks and CI are the gate. For the Job example,
   the checks show both the returned result and the absence of a repeated side
   effect. Add one fresh author-distinct read only when you ask for it, a mistake
   cannot be cheaply undone after it lands, or no deterministic check covers the
   changed behavior.
4. Repair concrete findings and confirm each repair with a check; a repair does
   not start another review. Finish when the accepted behavior is shown; report
   missing evidence honestly. The [agent workflow
   reference](agent-workflow-reference.md) states these steps.

This is behavior-driven development (BDD): agree on examples before coding,
then validate the result against them. [How it works](how-it-works.md) explains
BDD and shared domain language in more detail.

No AgentOps skill installation or RPI invocation is required. `ao quick-start`
explains this path and `ao demo` prints a sample task. `ao` supplies deterministic
checks and evidence utilities on demand; the coding agent supplies judgment.
Use a native goal for continuity and your existing tracker for work and handoffs.

For a small task this should be one reviewable change gated by its checks and
CI. Persist `verdict.v2` only when a caller or declared consumer needs
machine-readable evidence, using protected external non-Git storage.

Success is the accepted behavior, shown by checks bound to acceptance and
content identities, with one fresh judgment where a mistake would be costly. Pushing or releasing that content follows repository policy.
The fix and regression check remain available for future changes; preserve
useful decisions in your existing work record so the next session can continue
from what was established.
