# Advance one delivery transition

This optional pack command invokes exactly one crash-only reducer transition.
It requires explicit deployment-pinned `AGENTOPS_GC_DELIVERY_BIN` and `GC_BIN`
paths plus the rig-scoped identity environment. It does not start a city, route
semantic work, or run a delivery loop.
