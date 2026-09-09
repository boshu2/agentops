---
type: Reference
title: Synthetic request behavior reference
description: The synthetic example service returns a fixed success response.
status: stable
knowledge_use: maintained-reference
sources:
  - id: example
    resource: https://example.invalid/declared-source
    title: Synthetic source for the test fixture
generated: {by: example/1, at: 2026-09-08T12:00:00Z}
verified: {by: human:example, at: 2026-09-08T13:00:00Z}
---

## Applicability

The caller's synthetic example service only.

## Claim

The synthetic example response is fixed.[^example]

## Limitations

This fixture demonstrates document structure. It establishes no actual service
behavior, independent review, disclosure permission or demonstrated usefulness.

## Consumer

The profile checker's deterministic fixture tests.

## Retirement condition

Review when the selected profile changes; retire when the fixture has no test
consumer.

[^example]: Synthetic source; the checker must not fetch this URL.
