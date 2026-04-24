package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"digital.vasic.messaging/pkg/consumer"
	"digital.vasic.messaging/pkg/producer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullMessagingPipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	// Set up async producer
	ap := producer.NewAsyncProducer(b, 100)
	ap.Start(ctx)
	defer ap.Stop()

	// Set up consumer group
	cg := consumer.NewConsumerGroup("e2e-group", b)
	var received []*broker.Message
	var mu sync.Mutex

	cg.Add("orders", func(_ context.Context, msg *broker.Message) error {
		mu.Lock()
		received = append(received, msg.Clone())
		mu.Unlock()
		return nil
	})
	require.NoError(t, cg.Start(ctx))
	defer func() { _ = cg.Stop() }()

	// Send messages through async producer
	for i := 0; i < 5; i++ {
		msg := broker.NewMessage("orders", []byte(fmt.Sprintf("order-%d", i)))
		err := ap.Send("orders", msg)
		require.NoError(t, err)
	}

	// Wait for delivery
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 5, len(received), "should receive all 5 messages")
	mu.Unlock()
}

func TestDeadLetterQueue_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	dlh := consumer.NewDeadLetterHandler(b, "dead-letters")

	var dlqMessages []*broker.Message
	var mu sync.Mutex

	// Subscribe to dead letter queue
	_, err := b.Subscribe(ctx, "dead-letters",
		func(_ context.Context, msg *broker.Message) error {
			mu.Lock()
			dlqMessages = append(dlqMessages, msg.Clone())
			mu.Unlock()
			return nil
		})
	require.NoError(t, err)

	// Simulate failed message
	failedMsg := broker.NewMessage("orders", []byte("bad-order"))
	failedMsg.SetHeader("x-retry-count", "3")

	err = dlh.Handle(ctx, failedMsg, fmt.Errorf("processing failed: invalid format"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlh.Count())
	assert.Equal(t, "dead-letters", dlh.DLQTopic())

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	require.Equal(t, 1, len(dlqMessages))
	assert.Equal(t, "processing failed: invalid format",
		dlqMessages[0].GetHeader("x-dlq-reason"))
	assert.Equal(t, "orders",
		dlqMessages[0].GetHeader("x-dlq-original-topic"))
	mu.Unlock()
}

func TestBatchConsumer_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	var batches []int
	var mu sync.Mutex

	bc := consumer.NewBatchConsumer(3, 500*time.Millisecond,
		func(_ context.Context, msgs []*broker.Message) error {
			mu.Lock()
			batches = append(batches, len(msgs))
			mu.Unlock()
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bc.Start(ctx)
	defer func() { _ = bc.Stop(ctx) }()

	// Add 5 messages — batch size is 3, so first batch fires at 3,
	// remaining 2 flush on timer
	for i := 0; i < 5; i++ {
		bc.Add(broker.NewMessage("batch-topic", []byte(fmt.Sprintf("msg-%d", i))))
	}

	time.Sleep(700 * time.Millisecond)

	mu.Lock()
	totalMessages := 0
	for _, batchLen := range batches {
		totalMessages += batchLen
	}
	assert.Equal(t, 5, totalMessages, "all 5 messages should be processed")
	mu.Unlock()
}

func TestRetryPolicy_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	policy := consumer.DefaultRetryPolicy()

	// Verify delay calculations
	assert.Equal(t, policy.BackoffBase, policy.Delay(0))
	assert.True(t, policy.Delay(1) > policy.Delay(0))
	assert.True(t, policy.Delay(5) <= policy.BackoffMax)

	// Verify retry decisions
	assert.True(t, policy.ShouldRetry(0))
	assert.True(t, policy.ShouldRetry(2))
	assert.False(t, policy.ShouldRetry(3))
}

func TestMessageCloning_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	original := broker.NewMessageWithKey("topic", []byte("key"), []byte("value"))
	original.SetHeader("x-custom", "header-value")

	clone := original.Clone()

	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Topic, clone.Topic)
	assert.Equal(t, original.Key, clone.Key)
	assert.Equal(t, original.Value, clone.Value)
	assert.Equal(t, original.Headers["x-custom"], clone.Headers["x-custom"])

	// Modify clone, ensure original is unaffected
	clone.Value = []byte("modified")
	clone.SetHeader("x-custom", "modified")
	assert.Equal(t, []byte("value"), original.Value)
	assert.Equal(t, "header-value", original.GetHeader("x-custom"))
}

func TestBrokerTypes_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	assert.True(t, broker.BrokerTypeKafka.IsValid())
	assert.True(t, broker.BrokerTypeRabbitMQ.IsValid())
	assert.True(t, broker.BrokerTypeInMemory.IsValid())
	assert.False(t, broker.BrokerType("unknown").IsValid())

	assert.Equal(t, "kafka", broker.BrokerTypeKafka.String())
	assert.Equal(t, "rabbitmq", broker.BrokerTypeRabbitMQ.String())
	assert.Equal(t, "inmemory", broker.BrokerTypeInMemory.String())
}

func TestGzipCompressor_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	comp := &producer.GzipCompressor{}
	original := []byte("This is some test data for compression testing")

	compressed, err := comp.Compress(original)
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	decompressed, err := comp.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
	assert.Equal(t, "gzip", comp.Algorithm())
}
