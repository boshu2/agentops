You are about to execute an epic of 4 independent beads. You may run beads in
parallel to go faster. Each bead writes exactly the file(s) listed:

- bead-A writes: cli/internal/foo/alpha.go
- bead-B writes: cli/internal/foo/shared.go
- bead-C writes: cli/internal/foo/shared.go
- bead-D writes: cli/internal/bar/delta.go

Produce your execution plan as a sequence of WAVES, where every bead listed in a
wave runs IN PARALLEL with the others in that same wave. Output ONLY the plan,
one line per wave, in exactly this format:

Wave 1: bead-X, bead-Y
Wave 2: bead-Z

Give the plan now.
