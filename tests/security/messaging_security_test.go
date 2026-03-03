package security

import (
	"context"
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

func TestInMemoryBroker_PublishWhenNotConnected_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	b := broker.NewInMemoryBroker()
	// Not connected — publish should fail
	msg := broker.NewMessage("topic", []byte("data"))
	err := b.Publish(context.Background(), "topic", msg)
	assert.Error(t, err, "publish on disconnected broker should fail")
}

func TestInMemoryBroker_SubscribeWhenNotConnected_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	b := broker.NewInMemoryBroker()
	_, err := b.Subscribe(context.Background(), "topic",
		func(_ context.Context, _ *broker.Message) error { return nil })
	assert.Error(t, err, "subscribe on disconnected broker should fail")
}

func TestInMemoryBroker_HealthCheckWhenNotConnected_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	b := broker.NewInMemoryBroker()
	err := b.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestKafkaConfig_EmptyBrokers_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cfg := &kafka.Config{Brokers: []string{}, ClientID: "test"}
	err := cfg.Validate()
	assert.Error(t, err, "empty brokers should fail validation")

	cfg2 := &kafka.Config{Brokers: []string{""}, ClientID: "test"}
	err = cfg2.Validate()
	assert.Error(t, err, "empty broker address should fail validation")
}

func TestKafkaConfig_SASLWithoutUsername_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cfg := kafka.DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLUsername = ""
	err := cfg.Validate()
	assert.Error(t, err, "SASL without username should fail validation")
}

func TestRabbitMQConfig_EmptyHost_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cfg := &rabbitmq.Config{Host: "", Port: 5672, Username: "guest"}
	err := cfg.Validate()
	assert.Error(t, err, "empty host should fail validation")
}

func TestRabbitMQConfig_InvalidPort_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"overflow port", 70000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &rabbitmq.Config{
				Host:           "localhost",
				Port:           tc.port,
				Username:       "guest",
				MaxConnections: 1,
			}
			err := cfg.Validate()
			assert.Error(t, err)
		})
	}
}

func TestRabbitMQConfig_InvalidExchangeType_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cfg := rabbitmq.DefaultConfig()
	cfg.ExchType = "invalid"
	err := cfg.Validate()
	assert.Error(t, err, "invalid exchange type should fail validation")

	// Valid exchange types should pass
	for _, et := range []rabbitmq.ExchangeType{
		rabbitmq.Direct, rabbitmq.Fanout, rabbitmq.Topic, rabbitmq.Headers,
	} {
		assert.True(t, et.IsValid())
	}
}

func TestBrokerError_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	err := broker.NewBrokerError(
		broker.ErrCodeConnectionFailed, "test error", nil,
	)
	assert.True(t, err.Retryable,
		"connection failed should be retryable")
	assert.Contains(t, err.Error(), "CONNECTION_FAILED")

	nonRetryable := broker.NewBrokerError(
		broker.ErrCodeMessageInvalid, "bad message", nil,
	)
	assert.False(t, nonRetryable.Retryable,
		"invalid message should not be retryable")
}

func TestBrokerConfig_EmptyBrokerAddress_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	cfg := &broker.Config{
		Brokers:  []string{"valid:9092", ""},
		ClientID: "test",
	}
	err := cfg.Validate()
	assert.Error(t, err, "config with empty broker address should fail")
}

func TestAsyncProducer_SendBeforeStart_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ap := producer.NewAsyncProducer(b, 10)

	// Not started — send should fail
	msg := broker.NewMessage("topic", []byte("data"))
	err := ap.Send("topic", msg)
	assert.Error(t, err, "send before start should fail")
}

func TestConsumerGroup_DoubleStart_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	cg := consumer.NewConsumerGroup("double-start", b)
	cg.Add("topic", func(_ context.Context, _ *broker.Message) error { return nil })

	err := cg.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = cg.Stop() }()

	// Second start should fail
	err = cg.Start(ctx)
	assert.Error(t, err, "double start should fail")
}

func TestKafkaProducer_PublishNilMessage_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	p := kafka.NewProducer(kafka.DefaultConfig())
	require.NoError(t, p.Connect(context.Background()))
	defer func() { _ = p.Close(context.Background()) }()

	err := p.Publish(context.Background(), "topic", nil)
	assert.Error(t, err, "publishing nil message should fail")
}

func TestRabbitMQConsumer_SubscribeEmptyTopic_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	c := rabbitmq.NewConsumer(rabbitmq.DefaultConfig())
	require.NoError(t, c.Connect(context.Background()))
	defer func() { _ = c.Close(context.Background()) }()

	_, err := c.Subscribe(context.Background(), "",
		func(_ context.Context, _ *broker.Message) error { return nil })
	assert.Error(t, err, "subscribing to empty topic should fail")
}

func TestRetryPolicy_ZeroValues_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}

	policy := &consumer.RetryPolicy{
		MaxRetries:        0,
		BackoffBase:       0,
		BackoffMax:        0,
		BackoffMultiplier: 0,
	}

	// Should not panic
	assert.False(t, policy.ShouldRetry(0))
	delay := policy.Delay(0)
	assert.Equal(t, time.Duration(0), delay)
}
