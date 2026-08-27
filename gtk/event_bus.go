package gtk

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

const defaultEventBusTopicBuffer = 16

// EventBusTopicOption configures NewEventBusTopic.
type EventBusTopicOption interface {
	applyEventBusTopic(*eventBusTopicConfig)
}

type eventBusTopicConfig struct {
	buffer int
}

type eventBusTopicOptionFunc func(*eventBusTopicConfig)

func (f eventBusTopicOptionFunc) applyEventBusTopic(c *eventBusTopicConfig) { f(c) }

// WithBufferSize sets the per-subscriber channel buffer.
// Zero or omitted uses 16.
func WithBufferSize(n int) EventBusTopicOption {
	return eventBusTopicOptionFunc(func(c *eventBusTopicConfig) {
		if n > 0 {
			c.buffer = n
		}
	})
}

func applyEventBusTopicOptions(opts []EventBusTopicOption) eventBusTopicConfig {
	var cfg eventBusTopicConfig
	for _, opt := range opts {
		if opt != nil {
			opt.applyEventBusTopic(&cfg)
		}
	}
	if cfg.buffer <= 0 {
		cfg.buffer = defaultEventBusTopicBuffer
	}
	return cfg
}

// EventBusTopic is a typed in-process fan-out for one payload type.
// The zero value is ready to use and uses the default buffer.
type EventBusTopic[V any] struct {
	mu     sync.RWMutex
	next   uint64
	subs   map[uint64]chan V
	unsubs []func()
	buffer int
	ctx    context.Context
}

// NewEventBusTopic creates a topic. Subscribe after construction.
// When ctx is cancelled, every subscriber is unsubscribed. Nil ctx means
// no auto-unsubscribe.
func NewEventBusTopic[V any](ctx context.Context, opts ...EventBusTopicOption) *EventBusTopic[V] {
	cfg := applyEventBusTopicOptions(opts)
	t := &EventBusTopic[V]{ctx: ctx, buffer: cfg.buffer}
	if ctx != nil {
		go t.watch()
	}
	return t
}

func (t *EventBusTopic[V]) watch() {
	<-t.ctx.Done()
	t.unsubscribeAll()
}

func (t *EventBusTopic[V]) unsubscribeAll() {
	t.mu.Lock()
	// Swap out the slice so later Subscribe calls start a new list.
	// append(nil, unsub) is valid and does not panic.
	unsubs := t.unsubs
	t.unsubs = nil
	t.mu.Unlock()

	var g errgroup.Group
	for _, unsub := range unsubs {
		g.Go(func() error {
			unsub()
			return nil
		})
	}
	_ = g.Wait()
}

func (t *EventBusTopic[V]) bufferSize() int {
	if t == nil || t.buffer <= 0 {
		return defaultEventBusTopicBuffer
	}
	return t.buffer
}

// Subscribe starts a per-subscriber worker pool that calls fn for each
// published value. workers is that pool size and must be > 0.
// The returned unsubscribe removes the subscriber, closes the internal
// channel, and waits for in-flight fn calls to return. Do not call it from fn.
// Unsubscribe is safe to call more than once. fn must not be nil.
func (t *EventBusTopic[V]) Subscribe(fn func(V), workers int) func() {
	if fn == nil {
		panic("gtk: EventBusTopic.Subscribe: nil handler")
	}
	if workers <= 0 {
		panic("gtk: EventBusTopic.Subscribe: workers must be > 0")
	}

	ch := make(chan V, t.bufferSize())
	t.mu.Lock()
	if t.subs == nil {
		t.subs = make(map[uint64]chan V)
	}
	t.next++
	id := t.next
	t.subs[id] = ch
	unsub := t.newUnsubscribe(id, t.runSubscriber(ch, fn, workers))
	// After unsubscribeAll sets t.unsubs to nil, this allocates a new slice.
	// That handle is not in the slice unsubscribeAll is already walking.
	t.unsubs = append(t.unsubs, unsub)
	cancelled := t.ctx != nil && t.ctx.Err() != nil
	t.mu.Unlock()
	// ctx.Err() is set as soon as ctx is done. If watch already finished,
	// this subscriber would stay registered unless we unsub here. sync.Once
	// makes this safe if watch also got the handle.
	if cancelled {
		unsub()
	}
	return unsub
}

func (t *EventBusTopic[V]) runSubscriber(ch <-chan V, fn func(V), workers int) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		var g errgroup.Group
		for i := 0; i < workers; i++ {
			g.Go(func() error {
				for v := range ch {
					func() {
						defer func() { _ = recover() }()
						fn(v)
					}()
				}
				return nil
			})
		}
		_ = g.Wait()
	}()
	return done
}

func (t *EventBusTopic[V]) newUnsubscribe(id uint64, done <-chan struct{}) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			if sub, ok := t.subs[id]; ok {
				delete(t.subs, id)
				close(sub)
			}
			t.mu.Unlock()
			<-done
		})
	}
}

// Publish delivers v to every current subscriber. It holds the lock for the
// send so unsubscribe cannot close a channel mid-send.
func (t *EventBusTopic[V]) Publish(v V) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, ch := range t.subs {
		ch <- v
	}
}
