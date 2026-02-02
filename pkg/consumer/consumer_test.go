package consumer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumerGroup(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "with_id", id: "my-group"},
		{name: "empty_id_auto_generated", id: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := broker.NewInMemoryBroker()
			cg := NewConsumerGroup(tt.id, b)
			assert.NotEmpty(t, cg.ID())
			if tt.id != "" {
				assert.Equal(t, tt.id, cg.ID())
			}
			assert.False(t, cg.IsRunning())
		})
	}
}

func TestConsumerGroup_AddAndTopics(t *testing.T) {
	b := broker.NewInMemoryBroker()
	cg := NewConsumerGroup("g", b)

	cg.Add("topic1", func(_ context.Context, _ *broker.Message) error { return nil })
	cg.Add("topic2", func(_ context.Context, _ *broker.Message) error { return nil })

	topics := cg.Topics()
	assert.Len(t, topics, 2)
	assert.Contains(t, topics, "topic1")
	assert.Contains(t, topics, "topic2")
}

func TestConsumerGroup_StartStop(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	cg := NewConsumerGroup("g", b)
	cg.Add("topic1", func(_ context.Context, _ *broker.Message) error { return nil })

	require.NoError(t, cg.Start(ctx))
	assert.True(t, cg.IsRunning())

	// Start again should fail
	err := cg.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	require.NoError(t, cg.Stop())
	assert.False(t, cg.IsRunning())

	// Stop again should be safe
	require.NoError(t, cg.Stop())
}

func TestConsumerGroup_Start_BrokerNotConnected(t *testing.T) {
	b := broker.NewInMemoryBroker()
	cg := NewConsumerGroup("g", b)
	cg.Add("topic1", func(_ context.Context, _ *broker.Message) error { return nil })

	err := cg.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to subscribe")
}

func TestRetryPolicy_Default(t *testing.T) {
	rp := DefaultRetryPolicy()
	assert.Equal(t, 3, rp.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, rp.BackoffBase)
	assert.Equal(t, 30*time.Second, rp.BackoffMax)
	assert.Equal(t, 2.0, rp.BackoffMultiplier)
}

func TestRetryPolicy_Delay(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		expect  time.Duration
	}{
		{name: "first", attempt: 0, expect: 100 * time.Millisecond},
		{name: "second", attempt: 1, expect: 200 * time.Millisecond},
		{name: "third", attempt: 2, expect: 400 * time.Millisecond},
	}
	rp := DefaultRetryPolicy()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, rp.Delay(tt.attempt))
		})
	}
}

func TestRetryPolicy_Delay_Capped(t *testing.T) {
	rp := &RetryPolicy{
		BackoffBase:       1 * time.Second,
		BackoffMax:        5 * time.Second,
		BackoffMultiplier: 10.0,
		MaxRetries:        10,
	}
	// At attempt 5, 1s * 10^5 = 100000s, should be capped at 5s
	assert.Equal(t, 5*time.Second, rp.Delay(5))
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	rp := &RetryPolicy{MaxRetries: 3}
	assert.True(t, rp.ShouldRetry(0))
	assert.True(t, rp.ShouldRetry(2))
	assert.False(t, rp.ShouldRetry(3))
	assert.False(t, rp.ShouldRetry(4))
}

func TestWithRetry_Success(t *testing.T) {
	var callCount atomic.Int32
	handler := func(_ context.Context, _ *broker.Message) error {
		callCount.Add(1)
		return nil
	}

	retryHandler := WithRetry(handler, DefaultRetryPolicy())
	err := retryHandler(context.Background(), broker.NewMessage("t", nil))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())
}

func TestWithRetry_FailThenSuccess(t *testing.T) {
	var callCount atomic.Int32
	handler := func(_ context.Context, _ *broker.Message) error {
		if callCount.Add(1) <= 2 {
			return broker.NewBrokerError(broker.ErrCodeConnectionFailed, "fail", nil)
		}
		return nil
	}

	rp := &RetryPolicy{
		MaxRetries:        3,
		BackoffBase:       1 * time.Millisecond,
		BackoffMax:        10 * time.Millisecond,
		BackoffMultiplier: 1.0,
	}
	retryHandler := WithRetry(handler, rp)
	err := retryHandler(context.Background(), broker.NewMessage("t", nil))
	assert.NoError(t, err)
	assert.Equal(t, int32(3), callCount.Load())
}

func TestWithRetry_AllFail(t *testing.T) {
	errFail := errors.New("permanent failure")
	handler := func(_ context.Context, _ *broker.Message) error {
		return errFail
	}

	rp := &RetryPolicy{
		MaxRetries:        2,
		BackoffBase:       1 * time.Millisecond,
		BackoffMax:        10 * time.Millisecond,
		BackoffMultiplier: 1.0,
	}
	retryHandler := WithRetry(handler, rp)
	err := retryHandler(context.Background(), broker.NewMessage("t", nil))
	assert.ErrorIs(t, err, errFail)
}

func TestWithRetry_NilPolicy(t *testing.T) {
	handler := func(_ context.Context, _ *broker.Message) error {
		return nil
	}
	retryHandler := WithRetry(handler, nil)
	assert.NoError(t, retryHandler(context.Background(), broker.NewMessage("t", nil)))
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	handler := func(_ context.Context, _ *broker.Message) error {
		return broker.NewBrokerError(broker.ErrCodeConnectionFailed, "fail", nil)
	}

	rp := &RetryPolicy{
		MaxRetries:        10,
		BackoffBase:       1 * time.Second,
		BackoffMax:        10 * time.Second,
		BackoffMultiplier: 1.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	retryHandler := WithRetry(handler, rp)
	err := retryHandler(ctx, broker.NewMessage("t", nil))
	assert.Error(t, err)
}

func TestDeadLetterHandler(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	dlh := NewDeadLetterHandler(b, "dlq-topic")
	assert.Equal(t, "dlq-topic", dlh.DLQTopic())
	assert.Equal(t, int64(0), dlh.Count())

	msg := broker.NewMessage("original-topic", []byte("data"))
	originalErr := errors.New("processing failed")

	err := dlh.Handle(ctx, msg, originalErr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlh.Count())
}

func TestDeadLetterHandler_WithCallback(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	var callbackCalled atomic.Bool
	dlh := NewDeadLetterHandler(b, "dlq")
	dlh.SetOnFailure(func(_ context.Context, _ *broker.Message, _ error) {
		callbackCalled.Store(true)
	})

	msg := broker.NewMessage("t", []byte("d"))
	require.NoError(t, dlh.Handle(ctx, msg, errors.New("err")))
	assert.True(t, callbackCalled.Load())
}

func TestDeadLetterHandler_BrokerNotConnected(t *testing.T) {
	b := broker.NewInMemoryBroker()
	dlh := NewDeadLetterHandler(b, "dlq")

	msg := broker.NewMessage("t", []byte("d"))
	err := dlh.Handle(context.Background(), msg, errors.New("err"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to publish to DLQ")
}

func TestBatchConsumer_AddAndFlush(t *testing.T) {
	var received []*broker.Message
	var mu sync.Mutex
	handler := func(_ context.Context, msgs []*broker.Message) error {
		mu.Lock()
		received = append(received, msgs...)
		mu.Unlock()
		return nil
	}

	bc := NewBatchConsumer(5, time.Hour, handler) // Large flush interval
	assert.Equal(t, 5, bc.BatchSize())

	for i := 0; i < 3; i++ {
		bc.Add(broker.NewMessage("t", []byte("msg")))
	}
	assert.Equal(t, 3, bc.BufferLen())

	require.NoError(t, bc.Flush(context.Background()))
	assert.Equal(t, 0, bc.BufferLen())

	mu.Lock()
	assert.Len(t, received, 3)
	mu.Unlock()
}

func TestBatchConsumer_FlushEmpty(t *testing.T) {
	handler := func(_ context.Context, msgs []*broker.Message) error {
		t.Fatal("should not be called")
		return nil
	}
	bc := NewBatchConsumer(10, time.Hour, handler)
	require.NoError(t, bc.Flush(context.Background()))
}

func TestBatchConsumer_AutoFlushOnBatchSize(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	handler := func(_ context.Context, msgs []*broker.Message) error {
		if len(msgs) >= 3 {
			wg.Done()
		}
		return nil
	}

	bc := NewBatchConsumer(3, time.Hour, handler)
	ctx := context.Background()
	bc.Start(ctx)
	defer func() { _ = bc.Stop(ctx) }()

	for i := 0; i < 3; i++ {
		bc.Add(broker.NewMessage("t", []byte("msg")))
	}

	wg.Wait() // Should complete because batch size reached
}

func TestBatchConsumer_StartStop(t *testing.T) {
	handler := func(_ context.Context, _ []*broker.Message) error {
		return nil
	}
	bc := NewBatchConsumer(10, 10*time.Millisecond, handler)
	ctx := context.Background()

	bc.Start(ctx)
	bc.Start(ctx) // Double start should be safe

	bc.Add(broker.NewMessage("t", nil))
	time.Sleep(50 * time.Millisecond) // Wait for timer-based flush

	require.NoError(t, bc.Stop(ctx))
	require.NoError(t, bc.Stop(ctx)) // Double stop should be safe
}

func TestBatchConsumer_DefaultValues(t *testing.T) {
	handler := func(_ context.Context, _ []*broker.Message) error {
		return nil
	}
	bc := NewBatchConsumer(0, 0, handler) // Invalid values get defaults
	assert.Equal(t, 100, bc.BatchSize())
}

func TestBatchConsumer_AsHandler(t *testing.T) {
	handler := func(_ context.Context, _ []*broker.Message) error {
		return nil
	}
	bc := NewBatchConsumer(10, time.Hour, handler)
	h := bc.AsHandler()

	msg := broker.NewMessage("t", []byte("data"))
	require.NoError(t, h(context.Background(), msg))
	assert.Equal(t, 1, bc.BufferLen())
}
