package broker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryBroker is an in-memory message broker for testing and development.
type InMemoryBroker struct {
	queues      map[string][]*Message
	subscribers map[string][]subscriberInfo
	mu          sync.RWMutex
	connected   bool
	stopCh      chan struct{}
}

type subscriberInfo struct {
	id      string
	handler Handler
	active  bool
}

// NewInMemoryBroker creates a new in-memory broker.
func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{
		queues:      make(map[string][]*Message),
		subscribers: make(map[string][]subscriberInfo),
		stopCh:      make(chan struct{}),
	}
}

// Connect establishes a connection (no-op for in-memory).
func (b *InMemoryBroker) Connect(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = true
	return nil
}

// Close closes the broker.
func (b *InMemoryBroker) Close(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.connected {
		return nil
	}
	close(b.stopCh)
	b.connected = false
	b.queues = make(map[string][]*Message)
	b.subscribers = make(map[string][]subscriberInfo)
	return nil
}

// HealthCheck checks if the broker is healthy.
func (b *InMemoryBroker) HealthCheck(_ context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !b.connected {
		return ErrNotConnected
	}
	return nil
}

// IsConnected returns true if connected.
func (b *InMemoryBroker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Publish publishes a message to a topic.
func (b *InMemoryBroker) Publish(
	ctx context.Context, topic string, msg *Message,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.connected {
		return ErrNotConnected
	}
	clone := msg.Clone()
	clone.Topic = topic
	clone.Timestamp = time.Now().UTC()

	b.queues[topic] = append(b.queues[topic], clone)

	// Deliver to subscribers
	for i, sub := range b.subscribers[topic] {
		if sub.active {
			go func(h Handler, m *Message) {
				_ = h(ctx, m)
			}(b.subscribers[topic][i].handler, clone.Clone())
		}
	}
	return nil
}

// Subscribe creates a subscription to a topic.
func (b *InMemoryBroker) Subscribe(
	_ context.Context, topic string, handler Handler,
) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.connected {
		return nil, ErrNotConnected
	}
	subID := uuid.New().String()
	b.subscribers[topic] = append(b.subscribers[topic], subscriberInfo{
		id:      subID,
		handler: handler,
		active:  true,
	})
	return &inMemorySub{
		id:     subID,
		topic:  topic,
		broker: b,
		active: true,
	}, nil
}

// Unsubscribe cancels all subscriptions for a topic.
func (b *InMemoryBroker) Unsubscribe(topic string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.subscribers[topic] {
		b.subscribers[topic][i].active = false
	}
	delete(b.subscribers, topic)
	return nil
}

// Type returns the broker type.
func (b *InMemoryBroker) Type() BrokerType {
	return BrokerTypeInMemory
}

// inMemorySub is an in-memory subscription.
type inMemorySub struct {
	id     string
	topic  string
	broker *InMemoryBroker
	active bool
	mu     sync.RWMutex
}

func (s *inMemorySub) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return nil
	}
	s.active = false

	s.broker.mu.Lock()
	defer s.broker.mu.Unlock()
	subs := s.broker.subscribers[s.topic]
	for i, sub := range subs {
		if sub.id == s.id {
			subs[i].active = false
			break
		}
	}
	return nil
}

func (s *inMemorySub) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *inMemorySub) Topic() string {
	return s.topic
}

func (s *inMemorySub) ID() string {
	return s.id
}
