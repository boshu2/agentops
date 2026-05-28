// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionEventBus satisfies ports.EventBusPort with a sync,
// in-memory dispatch loop. Publish-acknowledge is synchronous;
// dispatch to subscribers is also synchronous in the publishing
// goroutine, matching the InMemoryEventBus contract used by tests.
//
// TRANSPORT NOTE (read before swapping): this is intentionally a
// process-local adapter. It is suitable for the current
// single-process /evolve loop, where Publish→Subscribe round-trips
// happen inside the same `ao` invocation. When the factory moves to
// a multi-process or distributed shape (NATS, Kafka, Redis Streams,
// etc.), a sibling adapter at e.g. cli/cmd/ao/event_bus_nats.go
// should be wired in front of this default. The port surface
// (Publish + Subscribe-with-cancel) is unchanged, so swap-in is a
// constructor change.
//
// Semantics enforced:
//   - Publish auto-assigns a monotonically-increasing ID when
//     event.ID is empty. Topic empty → error (port contract).
//   - Subscribe registers a handler under topic and returns a cancel
//     func. Cancel unregisters AND blocks until any in-flight
//     callback for THAT handler completes (per port contract).
//   - Dispatch is exact-topic-match only (no globbing).
//   - Errors returned from handlers are NOT retried; the bus simply
//     advances to the next handler. Adapters MAY log them; this
//     adapter intentionally does not.
type productionEventBus struct {
	mu     sync.RWMutex
	nextID atomic.Uint64
	subs   map[string][]*eventBusSubscription
}

// eventBusSubscription tracks one Subscribe call. inFlight is the
// per-handler waitgroup that Cancel uses to block until any in-flight
// dispatch finishes before unregistering.
type eventBusSubscription struct {
	id       uint64
	handler  ports.EventHandler
	inFlight sync.WaitGroup
	canceled atomic.Bool
}

func newProductionEventBus() *productionEventBus {
	return &productionEventBus{
		subs: make(map[string][]*eventBusSubscription),
	}
}

// Publish enqueues + dispatches the event synchronously to all
// subscribers of event.Topic. Returns the event with ID populated.
func (b *productionEventBus) Publish(ctx context.Context, event ports.Event) (ports.Event, error) {
	if err := ctx.Err(); err != nil {
		return ports.Event{}, err
	}
	if event.Topic == "" {
		return ports.Event{}, errors.New("productionEventBus: Topic required")
	}
	if event.ID == "" {
		event.ID = strconv.FormatUint(b.nextID.Add(1), 10)
	}

	b.mu.RLock()
	subs := append([]*eventBusSubscription{}, b.subs[event.Topic]...)
	b.mu.RUnlock()

	for _, sub := range subs {
		if sub.canceled.Load() {
			continue
		}
		sub.inFlight.Add(1)
		func(s *eventBusSubscription) {
			defer s.inFlight.Done()
			if s.canceled.Load() {
				return
			}
			_ = s.handler(ctx, event)
		}(sub)
	}
	return event, nil
}

// Subscribe registers handler for topic. Returns a cancel function.
func (b *productionEventBus) Subscribe(ctx context.Context, topic string, handler ports.EventHandler) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topic == "" {
		return nil, errors.New("productionEventBus: topic required")
	}
	if handler == nil {
		return nil, errors.New("productionEventBus: handler required")
	}
	sub := &eventBusSubscription{
		id:      b.nextID.Add(1),
		handler: handler,
	}
	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], sub)
	b.mu.Unlock()

	cancel := func() {
		if !sub.canceled.CompareAndSwap(false, true) {
			return // already cancelled
		}
		b.mu.Lock()
		bucket := b.subs[topic]
		for i, s := range bucket {
			if s.id == sub.id {
				b.subs[topic] = append(bucket[:i], bucket[i+1:]...)
				break
			}
		}
		if len(b.subs[topic]) == 0 {
			delete(b.subs, topic)
		}
		b.mu.Unlock()
		sub.inFlight.Wait() // block until in-flight callbacks finish
	}
	return cancel, nil
}

// activeSubscribers returns the number of registered handlers for a
// topic. Used by tests as a structural probe.
func (b *productionEventBus) activeSubscribers(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[topic])
}

// String is used by debug callsites that want to inspect the
// adapter's identity.
func (b *productionEventBus) String() string {
	return fmt.Sprintf("productionEventBus(inProcess; topics=%d)", b.topicCount())
}

func (b *productionEventBus) topicCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Compile-time assertion: productionEventBus satisfies the port.
var _ ports.EventBusPort = (*productionEventBus)(nil)
