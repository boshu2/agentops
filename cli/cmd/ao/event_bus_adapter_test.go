// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 117 ci_status_adapter_test.go.

func TestProductionEventBus_PublishDispatchesToSubscriber(t *testing.T) {
	b := newProductionEventBus()
	var got ports.Event
	var wg sync.WaitGroup
	wg.Add(1)
	cancel, err := b.Subscribe(context.Background(), "test.topic", func(_ context.Context, e ports.Event) error {
		got = e
		wg.Done()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, err = b.Publish(context.Background(), ports.Event{Topic: "test.topic", Payload: []byte("hi")})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if string(got.Payload) != "hi" {
		t.Fatalf("Payload = %q", got.Payload)
	}
	if got.ID == "" {
		t.Fatal("ID should be auto-assigned")
	}
}

func TestProductionEventBus_PublishAutoAssignsIDs(t *testing.T) {
	b := newProductionEventBus()
	e1, _ := b.Publish(context.Background(), ports.Event{Topic: "x"})
	e2, _ := b.Publish(context.Background(), ports.Event{Topic: "x"})
	if e1.ID == "" || e2.ID == "" {
		t.Fatal("IDs not assigned")
	}
	if e1.ID == e2.ID {
		t.Fatalf("IDs should differ: %q vs %q", e1.ID, e2.ID)
	}
}

func TestProductionEventBus_PublishHonorsCallerProvidedID(t *testing.T) {
	b := newProductionEventBus()
	got, _ := b.Publish(context.Background(), ports.Event{ID: "my-id", Topic: "x"})
	if got.ID != "my-id" {
		t.Fatalf("ID overwritten: got %q", got.ID)
	}
}

func TestProductionEventBus_EmptyTopicErrors(t *testing.T) {
	b := newProductionEventBus()
	_, err := b.Publish(context.Background(), ports.Event{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error on empty topic, got nil")
	}
}

func TestProductionEventBus_DispatchIsTopicSpecific(t *testing.T) {
	b := newProductionEventBus()
	var aHits, bHits atomic.Int32
	cancelA, _ := b.Subscribe(context.Background(), "topicA", func(_ context.Context, _ ports.Event) error {
		aHits.Add(1)
		return nil
	})
	defer cancelA()
	cancelB, _ := b.Subscribe(context.Background(), "topicB", func(_ context.Context, _ ports.Event) error {
		bHits.Add(1)
		return nil
	})
	defer cancelB()
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "topicA"})
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "topicA"})
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "topicB"})
	// Sync dispatch — no wait needed.
	if aHits.Load() != 2 || bHits.Load() != 1 {
		t.Fatalf("topic-specific dispatch wrong: A=%d B=%d", aHits.Load(), bHits.Load())
	}
}

func TestProductionEventBus_MultipleSubscribersToSameTopic(t *testing.T) {
	b := newProductionEventBus()
	var hits atomic.Int32
	c1, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		hits.Add(1)
		return nil
	})
	defer c1()
	c2, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		hits.Add(1)
		return nil
	})
	defer c2()
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "t"})
	if hits.Load() != 2 {
		t.Fatalf("fanout wrong: hits = %d, want 2", hits.Load())
	}
}

func TestProductionEventBus_CancelUnregisters(t *testing.T) {
	b := newProductionEventBus()
	var hits atomic.Int32
	cancel, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		hits.Add(1)
		return nil
	})
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "t"})
	cancel()
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "t"})
	if hits.Load() != 1 {
		t.Fatalf("cancel did not unregister: hits = %d, want 1", hits.Load())
	}
	if b.activeSubscribers("t") != 0 {
		t.Fatalf("subscriber not removed: count = %d", b.activeSubscribers("t"))
	}
}

func TestProductionEventBus_CancelIsIdempotent(t *testing.T) {
	b := newProductionEventBus()
	cancel, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		return nil
	})
	cancel()
	cancel() // must not panic
	cancel()
}

func TestProductionEventBus_CancelBlocksInFlightCallback(t *testing.T) {
	b := newProductionEventBus()
	released := make(chan struct{})
	started := make(chan struct{})
	cancel, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		close(started)
		<-released
		return nil
	})
	go func() { _, _ = b.Publish(context.Background(), ports.Event{Topic: "t"}) }()
	<-started // handler is mid-flight

	cancelDone := make(chan struct{})
	go func() {
		cancel()
		close(cancelDone)
	}()
	select {
	case <-cancelDone:
		t.Fatal("cancel returned before in-flight callback finished")
	case <-time.After(50 * time.Millisecond):
		// good — cancel is waiting
	}
	close(released) // let the handler finish
	<-cancelDone    // now cancel should return
}

func TestProductionEventBus_SubscribeEmptyTopicErrors(t *testing.T) {
	b := newProductionEventBus()
	_, err := b.Subscribe(context.Background(), "", func(_ context.Context, _ ports.Event) error { return nil })
	if err == nil {
		t.Fatal("expected error on empty topic, got nil")
	}
}

func TestProductionEventBus_SubscribeNilHandlerErrors(t *testing.T) {
	b := newProductionEventBus()
	_, err := b.Subscribe(context.Background(), "t", nil)
	if err == nil {
		t.Fatal("expected error on nil handler, got nil")
	}
}

func TestProductionEventBus_HandlerErrorsDoNotStopFanout(t *testing.T) {
	b := newProductionEventBus()
	var hits atomic.Int32
	c1, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		hits.Add(1)
		return errors.New("boom")
	})
	defer c1()
	c2, _ := b.Subscribe(context.Background(), "t", func(_ context.Context, _ ports.Event) error {
		hits.Add(1)
		return nil
	})
	defer c2()
	_, _ = b.Publish(context.Background(), ports.Event{Topic: "t"})
	if hits.Load() != 2 {
		t.Fatalf("second handler should still run despite first's error: hits = %d", hits.Load())
	}
}

func TestProductionEventBus_PublishHonorsContextCancellation(t *testing.T) {
	b := newProductionEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Publish(ctx, ports.Event{Topic: "t"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
