# FAQ

## Is AgentOps CI or a merge queue?

No. It produces engineering intent and independent semantic evidence. Your
repository owns Git, CI, merging, release, and deployment.

## What happens after FAIL or NOT_PROVEN?

The invocation reports the result and stops. The caller decides whether to
revise, re-plan, abandon, or begin a new invocation.

## Must I use multiple models?

No. A fresh subagent with a distinct declared context identity is the default
independent validator. Councils and mixed-model strategies are optional.

## Must I use the ao CLI?

No. The semantic loop and content manifest are designed to work without Git or
`ao`. The CLI provides repository-specific deterministic tools.

## Where does learning happen?

Learn is an optional later consumer of verdict collections. It is not required
for a verdict and cannot change the completed invocation.
