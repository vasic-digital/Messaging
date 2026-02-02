// Package consumer provides utilities for message consumption including
// consumer groups, retry policies, dead letter handling, and batch processing.
package consumer

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/google/uuid"
)

// ConsumerGroup manages multiple consumers subscribing to different topics
// through a single broker.
type ConsumerGroup struct {
	id            string
	broker        broker.MessageBroker
	subscriptions map[string]broker.Subscription
	handlers      map[string]broker.Handler
	mu            sync.RWMutex
	running       atomic.Bool
}

// NewConsumerGroup creates a new consumer group.
func NewConsumerGroup(id string, b broker.MessageBroker) *ConsumerGroup {
	if id == "" {
		id = "cg-" + uuid.New().String()[:8]
	}
	return &ConsumerGroup{
		id:            id,
		broker:        b,
		subscriptions: make(map[string]broker.Subscription),
		handlers:      make(map[string]broker.Handler),
	}
}

// ID returns the consumer group identifier.
func (cg *ConsumerGroup) ID() string {
	return cg.id
}

// Add registers a handler for a topic.
func (cg *ConsumerGroup) Add(topic string, handler broker.Handler) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.handlers[topic] = handler
}

// Start subscribes to all registered topics.
func (cg *ConsumerGroup) Start(ctx context.Context) error {
	if cg.running.Load() {
		return fmt.Errorf("consumer group %s is already running", cg.id)
	}

	cg.mu.Lock()
	defer cg.mu.Unlock()

	for topic, handler := range cg.handlers {
		sub, err := cg.broker.Subscribe(ctx, topic, handler)
		if err != nil {
			// Clean up already-created subscriptions on failure.
			for _, s := range cg.subscriptions {
				_ = s.Unsubscribe()
			}
			cg.subscriptions = make(map[string]broker.Subscription)
			return fmt.Errorf("failed to subscribe to %s: %w", topic, err)
		}
		cg.subscriptions[topic] = sub
	}

	cg.running.Store(true)
	return nil
}

// Stop unsubscribes from all topics.
func (cg *ConsumerGroup) Stop() error {
	if !cg.running.Swap(false) {
		return nil
	}

	cg.mu.Lock()
	defer cg.mu.Unlock()

	errs := &broker.MultiError{}
	for topic, sub := range cg.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			errs.Add(fmt.Errorf("failed to unsubscribe from %s: %w", topic, err))
		}
	}
	cg.subscriptions = make(map[string]broker.Subscription)
	return errs.ErrorOrNil()
}

// IsRunning returns true if the consumer group is running.
func (cg *ConsumerGroup) IsRunning() bool {
	return cg.running.Load()
}

// Topics returns the list of subscribed topics.
func (cg *ConsumerGroup) Topics() []string {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	topics := make([]string, 0, len(cg.handlers))
	for t := range cg.handlers {
		topics = append(topics, t)
	}
	return topics
}

// RetryPolicy defines retry behavior for failed message processing.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// BackoffBase is the initial backoff duration.
	BackoffBase time.Duration
	// BackoffMax is the maximum backoff duration.
	BackoffMax time.Duration
	// BackoffMultiplier is the exponential backoff multiplier.
	BackoffMultiplier float64
}

// DefaultRetryPolicy returns a default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        3,
		BackoffBase:       100 * time.Millisecond,
		BackoffMax:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Delay calculates the delay for the given attempt number (0-based).
func (rp *RetryPolicy) Delay(attempt int) time.Duration {
	if attempt <= 0 {
		return rp.BackoffBase
	}
	delay := float64(rp.BackoffBase) * math.Pow(rp.BackoffMultiplier, float64(attempt))
	if delay > float64(rp.BackoffMax) {
		return rp.BackoffMax
	}
	return time.Duration(delay)
}

// ShouldRetry returns true if the given attempt is within the retry limit.
func (rp *RetryPolicy) ShouldRetry(attempt int) bool {
	return attempt < rp.MaxRetries
}

// WithRetry wraps a handler with retry logic based on the given policy.
func WithRetry(handler broker.Handler, policy *RetryPolicy) broker.Handler {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	return func(ctx context.Context, msg *broker.Message) error {
		var lastErr error
		for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
			if attempt > 0 {
				delay := policy.Delay(attempt - 1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
			err := handler(ctx, msg)
			if err == nil {
				return nil
			}
			lastErr = err
			if !broker.IsRetryableError(err) && !policy.ShouldRetry(attempt+1) {
				return err
			}
		}
		return lastErr
	}
}

// DeadLetterHandler handles messages that have exhausted all retries.
type DeadLetterHandler struct {
	broker    broker.MessageBroker
	dlqTopic  string
	onFailure func(ctx context.Context, msg *broker.Message, err error)
	mu        sync.RWMutex
	count     atomic.Int64
}

// NewDeadLetterHandler creates a new dead letter handler.
func NewDeadLetterHandler(b broker.MessageBroker, dlqTopic string) *DeadLetterHandler {
	return &DeadLetterHandler{
		broker:   b,
		dlqTopic: dlqTopic,
	}
}

// SetOnFailure sets a callback for when a message is sent to the DLQ.
func (dlh *DeadLetterHandler) SetOnFailure(
	fn func(ctx context.Context, msg *broker.Message, err error),
) {
	dlh.mu.Lock()
	defer dlh.mu.Unlock()
	dlh.onFailure = fn
}

// Handle sends a failed message to the dead letter queue.
func (dlh *DeadLetterHandler) Handle(
	ctx context.Context, msg *broker.Message, originalErr error,
) error {
	// Set failure metadata in headers.
	dlqMsg := msg.Clone()
	dlqMsg.SetHeader("x-dlq-reason", originalErr.Error())
	dlqMsg.SetHeader("x-dlq-original-topic", msg.Topic)
	dlqMsg.SetHeader("x-dlq-timestamp", time.Now().UTC().Format(time.RFC3339))

	if err := dlh.broker.Publish(ctx, dlh.dlqTopic, dlqMsg); err != nil {
		return fmt.Errorf("failed to publish to DLQ %s: %w", dlh.dlqTopic, err)
	}

	dlh.count.Add(1)

	dlh.mu.RLock()
	fn := dlh.onFailure
	dlh.mu.RUnlock()
	if fn != nil {
		fn(ctx, msg, originalErr)
	}

	return nil
}

// Count returns the number of messages sent to the DLQ.
func (dlh *DeadLetterHandler) Count() int64 {
	return dlh.count.Load()
}

// DLQTopic returns the dead letter queue topic.
func (dlh *DeadLetterHandler) DLQTopic() string {
	return dlh.dlqTopic
}

// BatchConsumer processes messages in batches.
type BatchConsumer struct {
	handler    func(ctx context.Context, msgs []*broker.Message) error
	batchSize  int
	flushAfter time.Duration
	buffer     []*broker.Message
	mu         sync.Mutex
	flushCh    chan struct{}
	stopCh     chan struct{}
	running    atomic.Bool
}

// NewBatchConsumer creates a new batch consumer.
func NewBatchConsumer(
	batchSize int,
	flushAfter time.Duration,
	handler func(ctx context.Context, msgs []*broker.Message) error,
) *BatchConsumer {
	if batchSize < 1 {
		batchSize = 100
	}
	if flushAfter <= 0 {
		flushAfter = 5 * time.Second
	}
	return &BatchConsumer{
		handler:    handler,
		batchSize:  batchSize,
		flushAfter: flushAfter,
		buffer:     make([]*broker.Message, 0, batchSize),
		flushCh:    make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}
}

// Add adds a message to the batch buffer. If the buffer reaches
// batchSize, the batch is flushed.
func (bc *BatchConsumer) Add(msg *broker.Message) {
	bc.mu.Lock()
	bc.buffer = append(bc.buffer, msg)
	shouldFlush := len(bc.buffer) >= bc.batchSize
	bc.mu.Unlock()

	if shouldFlush {
		select {
		case bc.flushCh <- struct{}{}:
		default:
		}
	}
}

// Flush processes all buffered messages immediately.
func (bc *BatchConsumer) Flush(ctx context.Context) error {
	bc.mu.Lock()
	if len(bc.buffer) == 0 {
		bc.mu.Unlock()
		return nil
	}
	batch := bc.buffer
	bc.buffer = make([]*broker.Message, 0, bc.batchSize)
	bc.mu.Unlock()

	return bc.handler(ctx, batch)
}

// Start starts the background flush loop.
func (bc *BatchConsumer) Start(ctx context.Context) {
	if bc.running.Swap(true) {
		return
	}
	go func() {
		ticker := time.NewTicker(bc.flushAfter)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = bc.Flush(ctx)
			case <-bc.flushCh:
				_ = bc.Flush(ctx)
			case <-bc.stopCh:
				_ = bc.Flush(ctx)
				return
			case <-ctx.Done():
				_ = bc.Flush(context.Background())
				return
			}
		}
	}()
}

// Stop stops the background flush loop and flushes remaining messages.
func (bc *BatchConsumer) Stop(ctx context.Context) error {
	if !bc.running.Swap(false) {
		return nil
	}
	close(bc.stopCh)
	return bc.Flush(ctx)
}

// BufferLen returns the current number of buffered messages.
func (bc *BatchConsumer) BufferLen() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return len(bc.buffer)
}

// BatchSize returns the configured batch size.
func (bc *BatchConsumer) BatchSize() int {
	return bc.batchSize
}

// AsHandler returns a broker.Handler that adds messages to the batch.
func (bc *BatchConsumer) AsHandler() broker.Handler {
	return func(_ context.Context, msg *broker.Message) error {
		bc.Add(msg)
		return nil
	}
}
