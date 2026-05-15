// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// Event is one factory event flowing through the bus. Topic names the
// event category (e.g. "operator.intent", "cycle.completed",
// "ci.failed"); Payload is the typed body (opaque to the bus, parsed
// by subscribers); ID is an adapter-assigned event identifier (empty
// at publish time, populated by the adapter).
type Event struct {
	ID      string
	Topic   string
	Payload []byte
}

// EventHandler is the subscriber callback shape. Returning a non-nil
// error from a handler does NOT cause the bus to retry; the adapter
// MAY log the error for observability. Subscribers must implement
// their own retry/backoff.
type EventHandler func(ctx context.Context, event Event) error

// EventBusPort is the BC4 Factory async dispatch surface. Callers —
// /post-mortem aggregators, dream's compounding loop, CI-result
// listeners, and any future factory-side subscriber — depend on this
// port so they can publish and subscribe without coupling to a
// specific transport (in-memory channel, NATS, Kafka, etc).
//
// Contract:
//
//   - Publish MUST return after the event is enqueued (synchronous
//     publish-acknowledge, async dispatch). The returned event has ID
//     populated.
//   - Subscribe MUST return a cancellation function. Calling it
//     unregisters the handler and blocks until any in-flight callback
//     for that handler completes.
//   - Subscribers see events for matching Topic only (exact match;
//     no wildcards in this port; future adapters MAY add globbing).
//   - Empty Topic on Publish is a structural-rejection error.
//   - Context cancellation MUST be honored on Publish best-effort.
//
// See docs/contracts/ubiquitous-language.md (BC4 row). soc-2klg epic.
// Sibling: OperatorPort (cycle 104) records intents that are typically
// then Published onto an EventBus topic.
type EventBusPort interface {
	Publish(ctx context.Context, event Event) (Event, error)
	Subscribe(ctx context.Context, topic string, handler EventHandler) (cancel func(), err error)
}
