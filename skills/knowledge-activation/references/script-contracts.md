# Script Contracts

## Builder Boundary

`ao knowledge activate` uses workspace packet builders when they are present and a full source-manifest/topic/promotion/chunk rebuild is needed. When builders are absent, native activation can continue if either topic/packet substrate already exists or `WORKSPACE/.agents/harvest/latest.json` has promoted artifacts that can seed healthy topics, promoted packets, and chunk bundles.

### Packet Builders

- `source_manifest_build.py`
- `topic_packet_build.py`
- `corpus_packet_promote.py`
- `knowledge_chunk_build.py`

### Native Activation Surfaces

These product surfaces are implemented inside the `ao` binary and no longer require workspace-local Python builders:

- `ao knowledge beliefs`
- `ao knowledge playbooks`
- `ao knowledge brief --goal "<goal>"`
- `ao knowledge gaps`
- `ao knowledge activate` harvest-catalog substrate materialization

## Command Ownership

### `ao knowledge activate`

Runs the full outer loop:

1. source manifests via workspace packet builders when present
2. topic packets or native harvest-catalog topic materialization
3. promoted packets or native harvest-catalog promotion materialization
4. chunk bundles or native harvest-catalog chunk materialization
5. native belief book build
6. native playbook build
7. optional native briefing build for `--goal`

### `ao knowledge beliefs`

Refreshes only the belief book.

### `ao knowledge playbooks`

Refreshes candidate playbooks from healthy topics.

### `ao knowledge brief --goal "<goal>"`

Compiles a goal-time briefing.

### `ao knowledge gaps`

Reads generated artifacts and reports thin topics, promotion gaps, weak claims, and next recommended work.

## Roadmap Boundary

This slice now splits responsibility:

- full packet refresh remains workspace-local while the corpus contracts keep moving
- harvest-catalog-to-topic substrate materialization is durable `ao`-native fallback behavior
- belief/playbook/brief/gap surfaces are durable `ao`-native product surfaces

The skill contract stays stable across that boundary.
