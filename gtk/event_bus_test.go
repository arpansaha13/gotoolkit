package gtk

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBusTopicPublishSubscribe(t *testing.T) {
	var topic EventBusTopic[int]
	got := make(chan int, 1)
	unsubscribe := topic.Subscribe(func(v int) { got <- v }, 1)
	defer unsubscribe()

	topic.Publish(7)

	select {
	case v := <-got:
		if v != 7 {
			t.Fatalf("got %d, want 7", v)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBusTopicUnsubscribeStopsDelivery(t *testing.T) {
	var topic EventBusTopic[int]
	var n atomic.Int32
	unsubscribe := topic.Subscribe(func(int) { n.Add(1) }, 1)
	unsubscribe()

	topic.Publish(1)
	if n.Load() != 0 {
		t.Fatalf("handler called after unsubscribe: %d", n.Load())
	}
}

func TestEventBusTopicUnsubscribeWaitsForHandler(t *testing.T) {
	var topic EventBusTopic[int]
	inFn := make(chan struct{})
	release := make(chan struct{})
	unsubscribe := topic.Subscribe(func(int) {
		close(inFn)
		<-release
	}, 1)

	topic.Publish(1)
	select {
	case <-inFn:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	done := make(chan struct{})
	go func() {
		unsubscribe()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("unsubscribe returned while handler running")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unsubscribe did not return after handler finished")
	}
}

func TestEventBusTopicPublishWithNoSubscribers(t *testing.T) {
	var topic EventBusTopic[int]
	topic.Publish(1)
}

func TestNewEventBusTopicWithBufferSize(t *testing.T) {
	topic := NewEventBusTopic[int](context.Background(), WithBufferSize(1), nil)
	if topic.buffer != 1 {
		t.Fatalf("buffer = %d, want 1", topic.buffer)
	}

	var running atomic.Int32
	block := make(chan struct{})
	unsubscribe := topic.Subscribe(func(int) {
		running.Add(1)
		<-block
	}, 1)
	defer func() {
		close(block)
		unsubscribe()
	}()

	topic.Publish(1)
	waitUntil(t, func() bool { return running.Load() == 1 })

	topic.Publish(2)

	done := make(chan struct{})
	go func() {
		topic.Publish(3)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("third publish should block when buffer is full")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNewEventBusTopicIgnoresNonPositiveBuffer(t *testing.T) {
	topic := NewEventBusTopic[int](context.Background(), WithBufferSize(0))
	if topic.buffer != defaultEventBusTopicBuffer {
		t.Fatalf("buffer = %d, want %d", topic.buffer, defaultEventBusTopicBuffer)
	}
}

func TestSubscribeWorkerPoolIsPerSubscriber(t *testing.T) {
	var topic EventBusTopic[int]
	started := make(chan struct{}, 2)
	block := make(chan struct{})
	unsubscribe := topic.Subscribe(func(int) {
		started <- struct{}{}
		<-block
	}, 2)
	defer func() {
		close(block)
		unsubscribe()
	}()

	topic.Publish(1)
	topic.Publish(2)
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start")
		}
	}
}

func TestEventBusTopicMultipleSubscribers(t *testing.T) {
	var topic EventBusTopic[string]
	gotA := make(chan string, 1)
	gotB := make(chan string, 1)
	unsubscribeA := topic.Subscribe(func(v string) { gotA <- v }, 1)
	unsubscribeB := topic.Subscribe(func(v string) { gotB <- v }, 1)
	defer unsubscribeA()
	defer unsubscribeB()

	topic.Publish("x")

	for i, ch := range []<-chan string{gotA, gotB} {
		select {
		case got := <-ch:
			if got != "x" {
				t.Fatalf("subscriber %d got %q, want x", i, got)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
}

func waitUntil(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestEventBusTopicContextCancelUnsubscribes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	topic := NewEventBusTopic[int](ctx)
	var n atomic.Int32
	topic.Subscribe(func(int) { n.Add(1) }, 1)

	cancel()
	waitUntil(t, func() bool {
		topic.mu.RLock()
		defer topic.mu.RUnlock()
		return len(topic.subs) == 0
	})

	topic.Publish(1)
	if n.Load() != 0 {
		t.Fatalf("handler called after context cancel: %d", n.Load())
	}
}

func TestEventBusTopicSubscribeNilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	var topic EventBusTopic[int]
	topic.Subscribe(nil, 1)
}

func TestEventBusTopicSubscribeInvalidWorkersPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	var topic EventBusTopic[int]
	topic.Subscribe(func(int) {}, 0)
}
