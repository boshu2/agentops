# Knowledge Activation DAG

## Goal

Turn a `.agents` corpus into operational knowledge surfaces that influence future work.

## DAG

```text
STEP 0  preflight
        verify .agents exists
        verify required builders exist
        verify retrieval substrate is healthy enough to refresh

STEP 1  consolidate evidence
        source manifests
        topic packets
        promoted packets
        historical chunk bundles

STEP 2  distill operator surfaces
        belief book
        playbook candidates
        thin-topic cautions

STEP 3  compile briefing
        given a goal, select relevant topics
        pull evidence chunks
        attach beliefs and warnings

STEP 4  reinforce flywheel
        write retro
        record thin topics
        suggest next work
```

## Gates

### Gate 1: Evidence Trust

Do not build operator surfaces if the packet layers failed to refresh.

### Gate 2: Thin Topic Boundary

Do not silently promote thin topics to canonical playbooks or beliefs.

### Gate 3: Consumer Check

Every surface must name a consumer:

- belief book -> future sessions
- playbooks -> operators and planning
- briefings -> bounded startup context
- gaps -> next mining or review work

### Gate 4: Determinism

Repeated unchanged runs should not drift in structure or file naming.
