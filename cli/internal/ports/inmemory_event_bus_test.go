// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// Sibling pattern: inmemory_operator_test.go (cycle 104).

func TestInMemoryEventBus_PublishDispatchesToSubscriber(t *testing.T) {
	b := NewInMemoryEventBus()
	var received int32
	cancel, err := b.Subscribe(context.Background(), "cycle.completed", func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	out, err := b.Publish(context.Background(), Event{Topic: "cycle.completed", Payload: []byte("hello")})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if out.ID == "" {
		t.Fatal("Publish returned empty event ID")
	}
	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("received = %d, want 1", received)
	}
}

func TestInMemoryEventBus_PublishEmptyTopicRejected(t *testing.T) {
	b := NewInMemoryEventBus()
	_, err := b.Publish(context.Background(), Event{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error on empty Topic, got nil")
	}
}

func TestInMemoryEventBus_PublishWithoutSubscribersNoOp(t *testing.T) {
	b := NewInMemoryEventBus()
	out, err := b.Publish(context.Background(), Event{Topic: "no.one.listens"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if out.ID == "" {
		t.Fatal("Publish should assign ID even with no subscribers")
	}
}

func TestInMemoryEventBus_TopicFilteringIsExact(t *testing.T) {
	b := NewInMemoryEventBus()
	var aReceived, bReceived int32
	_, _ = b.Subscribe(context.Background(), "topic.a", func(_ context.Context, _ Event) error {
		atomic.AddInt32(&aReceived, 1)
		return nil
	})
	_, _ = b.Subscribe(context.Background(), "topic.b", func(_ context.Context, _ Event) error {
		atomic.AddInt32(&bReceived, 1)
		return nil
	})
	_, _ = b.Publish(context.Background(), Event{Topic: "topic.a"})
	if atomic.LoadInt32(&aReceived) != 1 || atomic.LoadInt32(&bReceived) != 0 {
		t.Fatalf("aReceived=%d (want 1), bReceived=%d (want 0)", aReceived, bReceived)
	}
}

func TestInMemoryEventBus_CancelStopsHandler(t *testing.T) {
	b := NewInMemoryEventBus()
	var received int32
	cancel, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	})
	_, _ = b.Publish(context.Background(), Event{Topic: "t"})
	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("pre-cancel received = %d, want 1", received)
	}
	cancel()
	_, _ = b.Publish(context.Background(), Event{Topic: "t"})
	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("post-cancel received = %d, want 1 (no new dispatch)", received)
	}
}

func TestInMemoryEventBus_HandlerErrorDoesNotStopOtherSubscribers(t *testing.T) {
	b := NewInMemoryEventBus()
	var subjectReceived int32
	_, _ = b.Subscribe(context.Background(), "t", func(_ context.Context, _ Event) error {
		return errors.New("subscriber failed")
	})
	_, _ = b.Subscribe(context.Background(), "t", func(_ context.Context, _ Event) error {
		atomic.AddInt32(&subjectReceived, 1)
		return nil
	})
	_, _ = b.Publish(context.Background(), Event{Topic: "t"})
	if atomic.LoadInt32(&subjectReceived) != 1 {
		t.Fatalf("subjectReceived = %d, want 1 (handler error must not block siblings)", subjectReceived)
	}
}

func TestInMemoryEventBus_SubscribeRejectsEmptyTopicOrNilHandler(t *testing.T) {
	b := NewInMemoryEventBus()
	if _, err := b.Subscribe(context.Background(), "", func(_ context.Context, _ Event) error { return nil }); err == nil {
		t.Fatal("empty topic: expected error, got nil")
	}
	if _, err := b.Subscribe(context.Background(), "t", nil); err == nil {
		t.Fatal("nil handler: expected error, got nil")
	}
}

func TestInMemoryEventBus_HonorsContextCancellation(t *testing.T) {
	b := NewInMemoryEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Publish(ctx, Event{Topic: "t"}); err == nil {
		t.Fatal("Publish: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish error = %v, want context.Canceled", err)
	}
	if _, err := b.Subscribe(ctx, "t", func(_ context.Context, _ Event) error { return nil }); err == nil {
		t.Fatal("Subscribe: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Subscribe error = %v, want context.Canceled", err)
	}
}

func TestInMemoryEventBus_EventIDsAreSequentialAndPrefixed(t *testing.T) {
	b := NewInMemoryEventBus()
	out1, _ := b.Publish(context.Background(), Event{Topic: "t"})
	out2, _ := b.Publish(context.Background(), Event{Topic: "t"})
	if !strings.HasPrefix(out1.ID, "evt-") || !strings.HasPrefix(out2.ID, "evt-") {
		t.Fatalf("IDs not prefixed 'evt-': %q %q", out1.ID, out2.ID)
	}
	if out1.ID == out2.ID {
		t.Fatalf("two Publishes produced identical IDs: %q", out1.ID)
	}
}
