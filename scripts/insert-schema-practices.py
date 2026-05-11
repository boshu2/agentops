#!/usr/bin/env python3
"""Pass-10: insert `"practices": [...]` immediately after the top-level
`"description"` line in each schema. Idempotent (skips if already present).
Preserves indentation and trailing comma style. Validates JSON afterward.
"""
import json
import pathlib
import re
import sys

MAPPING = {
    "agent-update.schema.json": ["gitops", "continuous-delivery"],
    "bead.v1.schema.json": ["dora-metrics", "lean-startup"],
    "briefing.v1.schema.json": ["wiki-knowledge-surface", "pragmatic-programmer"],
    "codex-marketplace.v1.schema.json": ["microservices", "team-topologies"],
    "codex-plugin-manifest.v1.schema.json": ["microservices", "design-by-contract"],
    "eval-run.v1.schema.json": ["llm-eval-harness", "dora-metrics"],
    "eval-suite.v1.schema.json": ["llm-eval-harness", "tdd"],
    "evidence-only-closure.v1.schema.json": ["adr", "cmm-process-maturity"],
    "execution-packet.schema.json": ["design-by-contract", "pragmatic-programmer"],
    "factory-admission.v1.schema.json": ["design-by-contract", "microservices"],
    "factory-work-order.v1.schema.json": ["microservices", "team-topologies"],
    "factory-yield.v1.schema.json": ["dora-metrics", "microservices"],
    "finding.json": ["dora-metrics", "wiki-knowledge-surface"],
    "handoff.v1.schema.json": ["event-sourcing-cqrs", "distributed-tracing"],
    "hooks-manifest.v1.schema.json": ["gitops", "infrastructure-as-code"],
    "learning.v1.schema.json": ["wiki-knowledge-surface", "lean-startup"],
    "memory-packet.v1.schema.json": ["wiki-knowledge-surface", "event-sourcing-cqrs"],
    "phase.v1.schema.json": ["continuous-delivery", "agile-manifesto"],
    "plugin-manifest.v1.schema.json": ["microservices", "hexagonal-architecture"],
    "quest.v1.schema.json": ["agile-manifesto", "lean-startup"],
    "release-readiness.v1.schema.json": ["continuous-delivery", "supply-chain-integrity"],
    "remote-compute-target.schema.json": ["microservices", "distributed-systems-design"],
    "remote-session-event.schema.json": ["event-sourcing-cqrs", "distributed-tracing"],
    "routing-policy.v1.schema.json": ["microservices", "service-mesh"],
    "rubric.v1.schema.json": ["llm-eval-harness", "design-by-contract"],
    "scenario.v1.schema.json": ["property-based-testing", "llm-eval-harness"],
    "schedule.schema.json": ["sre", "continuous-delivery"],
    "session-quality-signal.v1.schema.json": ["dora-metrics", "sre"],
    "skill-frontmatter.v1.schema.json": ["design-by-contract", "code-complete"],
    "swarm-evidence.schema.json": ["dora-metrics", "team-topologies"],
    "verdict.v1.schema.json": ["llm-eval-harness", "adr"],
    "watch-event.v1.schema.json": ["event-sourcing-cqrs", "distributed-tracing"],
    "worker-spec.v1.schema.json": ["microservices", "team-topologies"],
}

schemas_dir = pathlib.Path("schemas")
edited = 0
skipped = 0
errors = []

# Match a top-level "description": "..." line. We accept either a closing
# comma or a closing brace on the same line, but only insert when the line
# ends in a comma (i.e. another property follows).
DESC_LINE = re.compile(r'^(\s+)"description"\s*:\s*"(?:[^"\\]|\\.)*",\s*$')

for fname, slugs in MAPPING.items():
    path = schemas_dir / fname
    if not path.exists():
        errors.append(f"missing: {path}")
        continue
    text = path.read_text()
    if '"practices"' in text.split("\n", 50)[0:50].__str__():
        # extremely conservative idempotency guard
        pass
    lines = text.splitlines(keepends=True)
    if any('"practices"' in l for l in lines[:50]):
        skipped += 1
        continue
    new_lines = []
    inserted = False
    for line in lines:
        new_lines.append(line)
        if not inserted and DESC_LINE.match(line):
            indent = re.match(r'^(\s+)', line).group(1)
            slug_list = ", ".join(f'"{s}"' for s in slugs)
            new_lines.append(f'{indent}"practices": [{slug_list}],\n')
            inserted = True
    if not inserted:
        errors.append(f"no top-level description: {fname}")
        continue
    new_text = "".join(new_lines)
    # Validate JSON parses
    try:
        json.loads(new_text)
    except json.JSONDecodeError as e:
        errors.append(f"json invalid after edit: {fname}: {e}")
        continue
    path.write_text(new_text)
    edited += 1

print(f"edited={edited} skipped={skipped} errors={len(errors)}")
for e in errors:
    print(f"  ERR: {e}")
sys.exit(1 if errors else 0)
