package kafka

import (
	"context"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		ps       PartitionStrategy
		expected string
	}{
		{name: "round_robin", ps: RoundRobin, expected: "round_robin"},
		{name: "hash", ps: Hash, expected: "hash"},
		{name: "manual", ps: Manual, expected: "manual"},
		{name: "unknown", ps: PartitionStrategy(99), expected: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ps.String())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotEmpty(t, cfg.Brokers)
	assert.NotEmpty(t, cfg.ClientID)
	assert.NotEmpty(t, cfg.GroupID)
	assert.Equal(t, -1, cfg.RequiredAcks)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.True(t, cfg.Idempotent)
	assert.Equal(t, "lz4", cfg.CompressionCodec)
	assert.Equal(t, "PLAIN", cfg.SASLMechanism)
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid_default",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "no_brokers",
			modify:  func(c *Config) { c.Brokers = nil },
			wantErr: true,
			errMsg:  "at least one broker",
		},
		{
			name:    "empty_broker_addr",
			modify:  func(c *Config) { c.Brokers = []string{""} },
			wantErr: true,
			errMsg:  "broker address cannot be empty",
		},
		{
			name:    "no_client_id",
			modify:  func(c *Config) { c.ClientID = "" },
			wantErr: true,
			errMsg:  "client_id is required",
		},
		{
			name:    "zero_batch_size",
			modify:  func(c *Config) { c.BatchSize = 0 },
			wantErr: true,
			errMsg:  "batch_size must be at least 1",
		},
		{
			name: "sasl_no_username",
			modify: func(c *Config) {
				c.SASLEnabled = true
				c.SASLUsername = ""
			},
			wantErr: true,
			errMsg:  "sasl_username is required",
		},
		{
			name: "sasl_with_username",
			modify: func(c *Config) {
				c.SASLEnabled = true
				c.SASLUsername = "user"
				c.SASLPassword = "pass"
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProducer_ConnectClose(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()

	assert.False(t, p.IsConnected())
	require.NoError(t, p.Connect(ctx))
	assert.True(t, p.IsConnected())
	require.NoError(t, p.Close(ctx))
	assert.False(t, p.IsConnected())
}

func TestProducer_Connect_InvalidConfig(t *testing.T) {
	cfg := &Config{Brokers: nil, ClientID: ""}
	p := NewProducer(cfg)
	err := p.Connect(context.Background())
	assert.Error(t, err)
	assert.True(t, broker.IsBrokerError(err))
}

func TestProducer_Connect_AlreadyConnected(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))
	require.NoError(t, p.Connect(ctx)) // Should not error
}

func TestProducer_Close_DoubleClose(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))
	require.NoError(t, p.Close(ctx))
	require.NoError(t, p.Close(ctx)) // Should not error
}

func TestProducer_Publish_NotConnected(t *testing.T) {
	p := NewProducer(nil)
	err := p.Publish(context.Background(), "topic",
		broker.NewMessage("topic", []byte("v")))
	assert.ErrorIs(t, err, broker.ErrNotConnected)
}

func TestProducer_Publish_NilMessage(t *testing.T) {
	p := NewProducer(nil)
	require.NoError(t, p.Connect(context.Background()))
	err := p.Publish(context.Background(), "topic", nil)
	assert.ErrorIs(t, err, broker.ErrMessageInvalid)
}

func TestProducer_Publish_WithPublishFunc(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))

	var receivedTopic string
	var receivedMsg *broker.Message
	p.SetPublishFunc(func(_ context.Context, topic string, msg *broker.Message) error {
		receivedTopic = topic
		receivedMsg = msg
		return nil
	})

	msg := broker.NewMessage("my-topic", []byte("data"))
	require.NoError(t, p.Publish(ctx, "my-topic", msg))
	assert.Equal(t, "my-topic", receivedTopic)
	assert.Equal(t, msg.ID, receivedMsg.ID)
}

func TestProducer_Publish_NoPublishFunc(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))

	// Without a publish function, Publish should succeed (config layer only)
	msg := broker.NewMessage("topic", []byte("data"))
	require.NoError(t, p.Publish(ctx, "topic", msg))
}

func TestProducer_Config(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ClientID = "custom"
	p := NewProducer(cfg)
	assert.Equal(t, "custom", p.Config().ClientID)
}

func TestConsumer_ConnectClose(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()

	assert.False(t, c.IsConnected())
	require.NoError(t, c.Connect(ctx))
	assert.True(t, c.IsConnected())
	require.NoError(t, c.Close(ctx))
	assert.False(t, c.IsConnected())
}

func TestConsumer_Connect_InvalidConfig(t *testing.T) {
	cfg := &Config{Brokers: nil, ClientID: ""}
	c := NewConsumer(cfg)
	err := c.Connect(context.Background())
	assert.Error(t, err)
}

func TestConsumer_Connect_AlreadyConnected(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	require.NoError(t, c.Connect(ctx))
}

func TestConsumer_Close_DoubleClose(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	require.NoError(t, c.Close(ctx))
	require.NoError(t, c.Close(ctx))
}

func TestConsumer_Subscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "test-topic",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())
	assert.Equal(t, "test-topic", sub.Topic())
	assert.NotEmpty(t, sub.ID())
}

func TestConsumer_Subscribe_NotConnected(t *testing.T) {
	c := NewConsumer(nil)
	_, err := c.Subscribe(context.Background(), "topic",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	assert.ErrorIs(t, err, broker.ErrNotConnected)
}

func TestConsumer_Subscribe_EmptyTopic(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	_, err := c.Subscribe(ctx, "",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	assert.Error(t, err)
}

func TestConsumer_Subscribe_NilHandler(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	_, err := c.Subscribe(ctx, "topic", nil)
	assert.Error(t, err)
}

func TestConsumer_Unsubscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "topic",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())

	require.NoError(t, c.Unsubscribe("topic"))
	assert.False(t, sub.IsActive())
}

func TestConsumer_Unsubscribe_NonExistentTopic(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	// Should not error for non-existent topic
	require.NoError(t, c.Unsubscribe("nonexistent"))
}

func TestConsumer_Close_UnsubscribesAll(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	sub1, err := c.Subscribe(ctx, "topic1",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)
	sub2, err := c.Subscribe(ctx, "topic2",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)

	require.NoError(t, c.Close(ctx))
	assert.False(t, sub1.IsActive())
	assert.False(t, sub2.IsActive())
}

func TestConsumer_Config(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GroupID = "custom-group"
	c := NewConsumer(cfg)
	assert.Equal(t, "custom-group", c.Config().GroupID)
}

func TestKafkaSubscription_DoubleUnsubscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "topic",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)

	require.NoError(t, sub.Unsubscribe())
	require.NoError(t, sub.Unsubscribe()) // Should not error
}

func TestConfig_TLSSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TLSEnabled = true
	cfg.TLSSkipVerify = true
	assert.True(t, cfg.TLSEnabled)
	assert.True(t, cfg.TLSSkipVerify)
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Timeouts(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 30*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 5*time.Minute, cfg.MetadataRefresh)
}
