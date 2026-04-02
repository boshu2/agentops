# Script Contracts

## Workspace-Local Builders

The current product slice expects builders under `WORKSPACE/.agents/scripts/`.

### Packet Builders

- `source_manifest_build.py`
- `topic_packet_build.py`
- `corpus_packet_promote.py`
- `knowledge_chunk_build.py`

### Activation Builders

- `book_of_beliefs_build.py`
- `playbook_build.py`
- `briefing_build.py`

## Command Ownership

### `ao knowledge activate`

Runs the full outer loop:

1. source manifests
2. topic packets
3. promoted packets
4. chunk bundles
5. belief book
6. playbooks
7. optional briefing build for `--goal`

### `ao knowledge beliefs`

Refreshes only the belief book.

### `ao knowledge playbooks`

Refreshes candidate playbooks from healthy topics.

### `ao knowledge brief --goal "<goal>"`

Compiles a goal-time briefing.

### `ao knowledge gaps`

Reads generated artifacts and reports thin topics, promotion gaps, weak claims, and next recommended work.

## Roadmap Boundary

This slice intentionally keeps the builders outside the `ao` binary for now.
Once the contracts stabilize, the builders can migrate into durable product or CLI surfaces without changing the skill contract.
