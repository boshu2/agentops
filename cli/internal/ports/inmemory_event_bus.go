// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// InMemoryEventBus is an EventBusPort backed by per-topic handler
// slices. Dispatch is SYNCHRONOUS in this adapter — Publish calls
// each handler inline and returns after they all complete. This
// keeps tests deterministic and avoids goroutine-leak risk. Future
// production adapters (NATS, Kafka, etc.) handle the async-dispatch
// semantics the port contract permits.
//
// Thread-safe via mutex on the subscribers map. Each call to
// Publish acquires the mutex briefly to snapshot the handler slice,
// then releases it before calling handlers (so handlers can
// Subscribe/cancel during dispatch without deadlock).
type InMemoryEventBus struct {
	mu          sync.Mutex
	subscribers map[string][]*eventSubscription
	nextID      int
}

type eventSubscription struct {
	id      int
	handler EventHandler
	mu      sync.Mutex // serializes in-flight calls + cancel
	dead    bool
}

// NewInMemoryEventBus returns an empty bus.
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		subscribers: map[string][]*eventSubscription{},
	}
}

// Publish dispatches the event to subscribers of event.Topic. See
// package-level contract for the synchronous dispatch caveat.
func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if event.Topic == "" {
		return Event{}, errors.New("ports: Event.Topic required")
	}
	b.mu.Lock()
	b.nextID++
	event.ID = fmt.Sprintf("evt-%d", b.nextID)
	subs := append([]*eventSubscription{}, b.subscribers[event.Topic]...)
	b.mu.Unlock()
	for _, sub := range subs {
		sub.mu.Lock()
		if sub.dead {
			sub.mu.Unlock()
			continue
		}
		// Call handler holding sub.mu so cancel can wait us out.
		_ = sub.handler(ctx, event)
		sub.mu.Unlock()
	}
	return event, nil
}

// Subscribe registers handler for events with matching topic. Returns
// a cancel function that blocks until in-flight dispatch completes.
func (b *InMemoryEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topic == "" {
		return nil, errors.New("ports: Subscribe topic required")
	}
	if handler == nil {
		return nil, errors.New("ports: Subscribe handler required")
	}
	b.mu.Lock()
	b.nextID++
	sub := &eventSubscription{id: b.nextID, handler: handler}
	b.subscribers[topic] = append(b.subscribers[topic], sub)
	b.mu.Unlock()
	cancel := func() {
		// Wait for in-flight dispatch to complete (via sub.mu) then
		// mark dead and remove from the subscribers slice.
		sub.mu.Lock()
		sub.dead = true
		sub.mu.Unlock()
		b.mu.Lock()
		defer b.mu.Unlock()
		s := b.subscribers[topic]
		for i, x := range s {
			if x == sub {
				b.subscribers[topic] = append(s[:i], s[i+1:]...)
				break
			}
		}
	}
	return cancel, nil
}

// Compile-time assertion: InMemoryEventBus satisfies the port.
var _ EventBusPort = (*InMemoryEventBus)(nil)
