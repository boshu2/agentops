SKILL GUIDANCE (loaded): crank — execute epics through waves.

Core wave rule (the one invariant that makes parallelism safe): GROUP INTO A
WAVE ONLY WHEN WRITE SCOPES DO NOT COLLIDE. Parallelism is explicit file
ownership. Two beads that write the SAME file MUST NOT run in the same wave —
they collide and one clobbers the other (file collisions are the #1 swarm
failure). Default to sequential; parallelize only beads whose write-sets are
disjoint. Put write-scope-colliding beads in DIFFERENT waves so they serialize.
