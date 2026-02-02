package rabbitmq

import (
	"context"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeType_String(t *testing.T) {
	tests := []struct {
		name     string
		et       ExchangeType
		expected string
	}{
		{name: "direct", et: Direct, expected: "direct"},
		{name: "fanout", et: Fanout, expected: "fanout"},
		{name: "topic", et: Topic, expected: "topic"},
		{name: "headers", et: Headers, expected: "headers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.et.String())
		})
	}
}

func TestExchangeType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		et       ExchangeType
		expected bool
	}{
		{name: "direct_valid", et: Direct, expected: true},
		{name: "fanout_valid", et: Fanout, expected: true},
		{name: "topic_valid", et: Topic, expected: true},
		{name: "headers_valid", et: Headers, expected: true},
		{name: "invalid", et: ExchangeType("invalid"), expected: false},
		{name: "empty", et: ExchangeType(""), expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.et.IsValid())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5672, cfg.Port)
	assert.Equal(t, "guest", cfg.Username)
	assert.Equal(t, "guest", cfg.Password)
	assert.Equal(t, "/", cfg.VHost)
	assert.Equal(t, Topic, cfg.ExchType)
	assert.Equal(t, 10, cfg.Prefetch)
	assert.True(t, cfg.PublishConfirm)
	assert.True(t, cfg.Durable)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
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
			name:    "valid_with_url",
			modify:  func(c *Config) { c.URL = "amqp://user:pass@host:5672/" },
			wantErr: false,
		},
		{
			name:    "no_host",
			modify:  func(c *Config) { c.Host = "" },
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name:    "invalid_port_zero",
			modify:  func(c *Config) { c.Port = 0 },
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name:    "invalid_port_too_high",
			modify:  func(c *Config) { c.Port = 70000 },
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name:    "no_username",
			modify:  func(c *Config) { c.Username = "" },
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name:    "invalid_max_connections",
			modify:  func(c *Config) { c.MaxConnections = 0 },
			wantErr: true,
			errMsg:  "max_connections must be at least 1",
		},
		{
			name:    "negative_prefetch",
			modify:  func(c *Config) { c.Prefetch = -1 },
			wantErr: true,
			errMsg:  "prefetch cannot be negative",
		},
		{
			name: "invalid_exchange_type",
			modify: func(c *Config) {
				c.ExchType = ExchangeType("invalid")
			},
			wantErr: true,
			errMsg:  "invalid exchange type",
		},
		{
			name:    "empty_exchange_type_valid",
			modify:  func(c *Config) { c.ExchType = "" },
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

func TestConfig_ConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		modify   func(c *Config)
		expected string
	}{
		{
			name:     "default",
			modify:   func(c *Config) {},
			expected: "amqp://guest:guest@localhost:5672//",
		},
		{
			name:     "with_url",
			modify:   func(c *Config) { c.URL = "amqp://custom" },
			expected: "amqp://custom",
		},
		{
			name:     "tls",
			modify:   func(c *Config) { c.TLSEnabled = true },
			expected: "amqps://guest:guest@localhost:5672//",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			assert.Equal(t, tt.expected, cfg.ConnectionString())
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
	cfg := &Config{Host: "", Port: 0, Username: ""}
	p := NewProducer(cfg)
	err := p.Connect(context.Background())
	assert.Error(t, err)
	assert.True(t, broker.IsBrokerError(err))
}

func TestProducer_Connect_AlreadyConnected(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))
	require.NoError(t, p.Connect(ctx))
}

func TestProducer_Close_DoubleClose(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))
	require.NoError(t, p.Close(ctx))
	require.NoError(t, p.Close(ctx))
}

func TestProducer_Publish_NotConnected(t *testing.T) {
	p := NewProducer(nil)
	err := p.Publish(context.Background(), "q",
		broker.NewMessage("q", []byte("v")))
	assert.ErrorIs(t, err, broker.ErrNotConnected)
}

func TestProducer_Publish_NilMessage(t *testing.T) {
	p := NewProducer(nil)
	require.NoError(t, p.Connect(context.Background()))
	err := p.Publish(context.Background(), "q", nil)
	assert.ErrorIs(t, err, broker.ErrMessageInvalid)
}

func TestProducer_Publish_WithPublishFunc(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))

	var receivedTopic string
	p.SetPublishFunc(func(_ context.Context, topic string, _ *broker.Message) error {
		receivedTopic = topic
		return nil
	})

	msg := broker.NewMessage("q", []byte("data"))
	require.NoError(t, p.Publish(ctx, "my-queue", msg))
	assert.Equal(t, "my-queue", receivedTopic)
}

func TestProducer_Publish_NoFunc(t *testing.T) {
	p := NewProducer(nil)
	ctx := context.Background()
	require.NoError(t, p.Connect(ctx))

	msg := broker.NewMessage("q", []byte("data"))
	require.NoError(t, p.Publish(ctx, "q", msg))
}

func TestProducer_Config(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exchange = "my-exchange"
	p := NewProducer(cfg)
	assert.Equal(t, "my-exchange", p.Config().Exchange)
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
	cfg := &Config{Host: "", Port: 0, Username: ""}
	c := NewConsumer(cfg)
	err := c.Connect(context.Background())
	assert.Error(t, err)
}

func TestConsumer_Subscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "test-queue",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())
	assert.Equal(t, "test-queue", sub.Topic())
	assert.NotEmpty(t, sub.ID())
}

func TestConsumer_Subscribe_NotConnected(t *testing.T) {
	c := NewConsumer(nil)
	_, err := c.Subscribe(context.Background(), "q",
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

	_, err := c.Subscribe(ctx, "q", nil)
	assert.Error(t, err)
}

func TestConsumer_Unsubscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "q",
		func(_ context.Context, _ *broker.Message) error {
			return nil
		})
	require.NoError(t, err)

	require.NoError(t, c.Unsubscribe("q"))
	assert.False(t, sub.IsActive())
}

func TestConsumer_Unsubscribe_NonExistent(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	require.NoError(t, c.Unsubscribe("nonexistent"))
}

func TestConsumer_Close_UnsubscribesAll(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	sub1, _ := c.Subscribe(ctx, "q1",
		func(_ context.Context, _ *broker.Message) error { return nil })
	sub2, _ := c.Subscribe(ctx, "q2",
		func(_ context.Context, _ *broker.Message) error { return nil })

	require.NoError(t, c.Close(ctx))
	assert.False(t, sub1.IsActive())
	assert.False(t, sub2.IsActive())
}

func TestConsumer_Close_DoubleClose(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	require.NoError(t, c.Close(ctx))
	require.NoError(t, c.Close(ctx))
}

func TestConsumer_Config(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Queue = "my-queue"
	c := NewConsumer(cfg)
	assert.Equal(t, "my-queue", c.Config().Queue)
}

func TestRabbitSubscription_DoubleUnsubscribe(t *testing.T) {
	c := NewConsumer(nil)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer func() { _ = c.Close(ctx) }()

	sub, err := c.Subscribe(ctx, "q",
		func(_ context.Context, _ *broker.Message) error { return nil })
	require.NoError(t, err)

	require.NoError(t, sub.Unsubscribe())
	require.NoError(t, sub.Unsubscribe())
}
