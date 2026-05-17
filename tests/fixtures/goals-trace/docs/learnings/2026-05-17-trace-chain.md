---
title: Trace chain wiring for the fitness gate
directive_id: d-fitness-gate-bdd
scenario_id: s-2026-05-17-001
source: .agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md
date: 2026-05-17
---

The fitness gate now consumes scenario-results artifacts. This learning is
linked to directive d-fitness-gate-bdd via explicit frontmatter, so the walker
must emit a high-confidence directive_has_learning edge. Its `source` field
names the RPI verdict artifact at
.agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md, so the walker also emits a
high-confidence artifact_cited_by_learning edge.
