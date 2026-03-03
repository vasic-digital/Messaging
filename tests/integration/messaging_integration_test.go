package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"digital.vasic.messaging/pkg/consumer"
	"digital.vasic.messaging/pkg/kafka"
	"digital.vasic.messaging/pkg/producer"
	"digital.vasic.messaging/pkg/rabbitmq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryBroker_PublishSubscribe_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	assert.True(t, b.IsConnected())
	assert.Equal(t, broker.BrokerTypeInMemory, b.Type())

	var received *broker.Message
	var mu sync.Mutex

	sub, err := b.Subscribe(ctx, "test-topic", func(_ context.Context, msg *broker.Message) error {
		mu.Lock()
		received = msg
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())
	assert.Equal(t, "test-topic", sub.Topic())

	msg := broker.NewMessage("test-topic", []byte("hello world"))
	err = b.Publish(ctx, "test-topic", msg)
	require.NoError(t, err)

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.NotNil(t, received)
	assert.Equal(t, []byte("hello world"), received.Value)
	mu.Unlock()
}

func TestInMemoryBroker_Unsubscribe_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	callCount := 0
	var mu sync.Mutex

	_, err := b.Subscribe(ctx, "topic-a", func(_ context.Context, _ *broker.Message) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)

	err = b.Unsubscribe("topic-a")
	require.NoError(t, err)

	// Publish after unsubscribe — handler should not be called
	msg := broker.NewMessage("topic-a", []byte("data"))
	err = b.Publish(ctx, "topic-a", msg)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 0, callCount, "handler should not be called after unsubscribe")
	mu.Unlock()
}

func TestConsumerGroup_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	cg := consumer.NewConsumerGroup("test-cg", b)
	assert.Equal(t, "test-cg", cg.ID())

	var receivedA, receivedB int
	var mu sync.Mutex

	cg.Add("topic-a", func(_ context.Context, _ *broker.Message) error {
		mu.Lock()
		receivedA++
		mu.Unlock()
		return nil
	})
	cg.Add("topic-b", func(_ context.Context, _ *broker.Message) error {
		mu.Lock()
		receivedB++
		mu.Unlock()
		return nil
	})

	err := cg.Start(ctx)
	require.NoError(t, err)
	assert.True(t, cg.IsRunning())

	topics := cg.Topics()
	assert.Equal(t, 2, len(topics))

	// Publish messages
	_ = b.Publish(ctx, "topic-a", broker.NewMessage("topic-a", []byte("a1")))
	_ = b.Publish(ctx, "topic-b", broker.NewMessage("topic-b", []byte("b1")))
	_ = b.Publish(ctx, "topic-a", broker.NewMessage("topic-a", []byte("a2")))

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 2, receivedA)
	assert.Equal(t, 1, receivedB)
	mu.Unlock()

	err = cg.Stop()
	assert.NoError(t, err)
	assert.False(t, cg.IsRunning())
}

func TestKafkaProducer_ConfigValidation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := kafka.DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)

	// Invalid config
	invalidCfg := &kafka.Config{Brokers: []string{}, ClientID: "test"}
	err = invalidCfg.Validate()
	assert.Error(t, err)
}

func TestRabbitMQConfig_ConnectionString_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := rabbitmq.DefaultConfig()
	connStr := cfg.ConnectionString()
	assert.Contains(t, connStr, "amqp://guest:guest@localhost:5672")

	cfg.TLSEnabled = true
	connStr = cfg.ConnectionString()
	assert.Contains(t, connStr, "amqps://")

	cfg.URL = "amqp://custom-url"
	connStr = cfg.ConnectionString()
	assert.Equal(t, "amqp://custom-url", connStr)
}

func TestSyncProducer_WithBroker_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	sp := producer.NewSyncProducer(b, 5*time.Second)
	msg := broker.NewMessage("test-topic", []byte("sync message"))
	err := sp.Send(ctx, "test-topic", msg)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sp.SentCount())
	assert.Equal(t, int64(0), sp.FailedCount())
}

func TestSyncProducer_SendValue_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	sp := producer.NewSyncProducer(b, 5*time.Second)
	sp.SetSerializer(&producer.JSONSerializer{})

	data := map[string]string{"key": "value"}
	err := sp.SendValue(ctx, "json-topic", data)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sp.SentCount())
}
