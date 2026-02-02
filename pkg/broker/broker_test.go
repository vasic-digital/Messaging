package broker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrokerType_String(t *testing.T) {
	tests := []struct {
		name     string
		bt       BrokerType
		expected string
	}{
		{name: "kafka", bt: BrokerTypeKafka, expected: "kafka"},
		{name: "rabbitmq", bt: BrokerTypeRabbitMQ, expected: "rabbitmq"},
		{name: "inmemory", bt: BrokerTypeInMemory, expected: "inmemory"},
		{name: "custom", bt: BrokerType("custom"), expected: "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.bt.String())
		})
	}
}

func TestBrokerType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		bt       BrokerType
		expected bool
	}{
		{name: "kafka_valid", bt: BrokerTypeKafka, expected: true},
		{name: "rabbitmq_valid", bt: BrokerTypeRabbitMQ, expected: true},
		{name: "inmemory_valid", bt: BrokerTypeInMemory, expected: true},
		{name: "custom_invalid", bt: BrokerType("custom"), expected: false},
		{name: "empty_invalid", bt: BrokerType(""), expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.bt.IsValid())
		})
	}
}

func TestNewMessage(t *testing.T) {
	tests := []struct {
		name  string
		topic string
		value []byte
	}{
		{name: "basic", topic: "test-topic", value: []byte("hello")},
		{name: "empty_value", topic: "t", value: []byte{}},
		{name: "nil_value", topic: "t", value: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessage(tt.topic, tt.value)
			assert.NotEmpty(t, msg.ID)
			assert.Equal(t, tt.topic, msg.Topic)
			assert.Equal(t, tt.value, msg.Value)
			assert.NotNil(t, msg.Headers)
			assert.False(t, msg.Timestamp.IsZero())
		})
	}
}

func TestNewMessageWithKey(t *testing.T) {
	msg := NewMessageWithKey("topic", []byte("key"), []byte("value"))
	assert.Equal(t, []byte("key"), msg.Key)
	assert.Equal(t, []byte("value"), msg.Value)
	assert.Equal(t, "topic", msg.Topic)
}

func TestNewMessageWithID(t *testing.T) {
	msg := NewMessageWithID("custom-id", "topic", []byte("value"))
	assert.Equal(t, "custom-id", msg.ID)
	assert.Equal(t, "topic", msg.Topic)
}

func TestMessage_SetHeader_GetHeader(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "simple", key: "content-type", value: "application/json"},
		{name: "empty_value", key: "key", value: ""},
		{name: "special_chars", key: "x-custom", value: "a=b&c=d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessage("topic", nil)
			msg.SetHeader(tt.key, tt.value)
			assert.Equal(t, tt.value, msg.GetHeader(tt.key))
		})
	}
}

func TestMessage_GetHeader_NilHeaders(t *testing.T) {
	msg := &Message{}
	assert.Equal(t, "", msg.GetHeader("any"))
}

func TestMessage_SetHeader_NilHeaders(t *testing.T) {
	msg := &Message{}
	msg.SetHeader("key", "value")
	assert.Equal(t, "value", msg.GetHeader("key"))
}

func TestMessage_WithKey(t *testing.T) {
	msg := NewMessage("t", nil)
	result := msg.WithKey([]byte("key"))
	assert.Equal(t, []byte("key"), result.Key)
	assert.Same(t, msg, result)
}

func TestMessage_WithStringKey(t *testing.T) {
	msg := NewMessage("t", nil)
	result := msg.WithStringKey("key")
	assert.Equal(t, []byte("key"), result.Key)
}

func TestMessage_Clone(t *testing.T) {
	msg := NewMessage("topic", []byte("value"))
	msg.Key = []byte("key")
	msg.SetHeader("h1", "v1")

	clone := msg.Clone()

	assert.Equal(t, msg.ID, clone.ID)
	assert.Equal(t, msg.Topic, clone.Topic)
	assert.Equal(t, msg.Key, clone.Key)
	assert.Equal(t, msg.Value, clone.Value)
	assert.Equal(t, msg.Headers, clone.Headers)
	assert.Equal(t, msg.Timestamp, clone.Timestamp)

	// Verify deep copy
	clone.Value[0] = 'X'
	assert.NotEqual(t, msg.Value[0], clone.Value[0])

	clone.Key[0] = 'X'
	assert.NotEqual(t, msg.Key[0], clone.Key[0])

	clone.Headers["h1"] = "changed"
	assert.NotEqual(t, msg.Headers["h1"], clone.Headers["h1"])
}

func TestMessage_Clone_NilHeaders(t *testing.T) {
	msg := &Message{ID: "id", Value: []byte("v")}
	clone := msg.Clone()
	assert.Nil(t, clone.Headers)
}

func TestMessage_MarshalJSON(t *testing.T) {
	msg := NewMessage("topic", []byte("hello"))
	data, err := msg.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"topic":"topic"`)
}

func TestMessage_UnmarshalJSON(t *testing.T) {
	original := NewMessage("topic", []byte("hello"))
	original.SetHeader("k", "v")
	data, err := original.MarshalJSON()
	require.NoError(t, err)

	var decoded Message
	err = decoded.UnmarshalJSON(data)
	require.NoError(t, err)
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Topic, decoded.Topic)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid",
			config:  &Config{Brokers: []string{"localhost:9092"}, ClientID: "test"},
			wantErr: false,
		},
		{
			name:    "no_brokers",
			config:  &Config{Brokers: []string{}, ClientID: "test"},
			wantErr: true,
		},
		{
			name:    "empty_broker",
			config:  &Config{Brokers: []string{""}, ClientID: "test"},
			wantErr: true,
		},
		{
			name:    "no_client_id",
			config:  &Config{Brokers: []string{"localhost"}, ClientID: ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotEmpty(t, cfg.Brokers)
	assert.NotEmpty(t, cfg.ClientID)
	assert.NoError(t, cfg.Validate())
}

func TestInMemoryBroker_ConnectClose(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()

	assert.False(t, b.IsConnected())
	require.NoError(t, b.Connect(ctx))
	assert.True(t, b.IsConnected())
	require.NoError(t, b.HealthCheck(ctx))
	require.NoError(t, b.Close(ctx))
	assert.False(t, b.IsConnected())
}

func TestInMemoryBroker_HealthCheck_NotConnected(t *testing.T) {
	b := NewInMemoryBroker()
	err := b.HealthCheck(context.Background())
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestInMemoryBroker_Type(t *testing.T) {
	b := NewInMemoryBroker()
	assert.Equal(t, BrokerTypeInMemory, b.Type())
}

func TestInMemoryBroker_PublishSubscribe(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	var received *Message
	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := b.Subscribe(ctx, "test-topic", func(_ context.Context, msg *Message) error {
		received = msg
		wg.Done()
		return nil
	})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())
	assert.Equal(t, "test-topic", sub.Topic())
	assert.NotEmpty(t, sub.ID())

	msg := NewMessage("test-topic", []byte("hello"))
	require.NoError(t, b.Publish(ctx, "test-topic", msg))

	wg.Wait()
	assert.NotNil(t, received)
	assert.Equal(t, []byte("hello"), received.Value)
}

func TestInMemoryBroker_Publish_NotConnected(t *testing.T) {
	b := NewInMemoryBroker()
	err := b.Publish(context.Background(), "t", NewMessage("t", nil))
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestInMemoryBroker_Subscribe_NotConnected(t *testing.T) {
	b := NewInMemoryBroker()
	_, err := b.Subscribe(context.Background(), "t", func(_ context.Context, _ *Message) error {
		return nil
	})
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestInMemoryBroker_Unsubscribe(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	_, err := b.Subscribe(ctx, "topic", func(_ context.Context, _ *Message) error {
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, b.Unsubscribe("topic"))
}

func TestInMemoryBroker_Subscription_Unsubscribe(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	sub, err := b.Subscribe(ctx, "topic", func(_ context.Context, _ *Message) error {
		return nil
	})
	require.NoError(t, err)
	assert.True(t, sub.IsActive())

	require.NoError(t, sub.Unsubscribe())
	assert.False(t, sub.IsActive())

	// Double unsubscribe should be safe
	require.NoError(t, sub.Unsubscribe())
}

func TestInMemoryBroker_Close_DoubleClose(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	require.NoError(t, b.Close(ctx))
	// Double close should not panic
	require.NoError(t, b.Close(ctx))
}

func TestInMemoryBroker_MultipleSubscribers(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	var mu sync.Mutex
	count := 0
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		_, err := b.Subscribe(ctx, "topic", func(_ context.Context, _ *Message) error {
			mu.Lock()
			count++
			mu.Unlock()
			wg.Done()
			return nil
		})
		require.NoError(t, err)
	}

	require.NoError(t, b.Publish(ctx, "topic", NewMessage("topic", []byte("msg"))))

	wg.Wait()
	assert.Equal(t, 3, count)
}

func TestBrokerError(t *testing.T) {
	tests := []struct {
		name      string
		code      ErrorCode
		message   string
		cause     error
		retryable bool
	}{
		{
			name:      "connection_failed_retryable",
			code:      ErrCodeConnectionFailed,
			message:   "dial tcp failed",
			cause:     nil,
			retryable: true,
		},
		{
			name:      "publish_failed_not_retryable",
			code:      ErrCodePublishFailed,
			message:   "queue full",
			cause:     ErrPublishFailed,
			retryable: false,
		},
		{
			name:      "timeout_retryable",
			code:      ErrCodeConnectionTimeout,
			message:   "timeout",
			cause:     nil,
			retryable: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBrokerError(tt.code, tt.message, tt.cause)
			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, tt.cause, err.Cause)
			assert.Equal(t, tt.retryable, err.Retryable)
			assert.Contains(t, err.Error(), tt.message)
			assert.Equal(t, tt.retryable, IsRetryableError(err))
		})
	}
}

func TestBrokerError_Unwrap(t *testing.T) {
	cause := ErrPublishFailed
	err := NewBrokerError(ErrCodePublishFailed, "msg", cause)
	assert.ErrorIs(t, err, cause)
}

func TestBrokerError_Is(t *testing.T) {
	err1 := NewBrokerError(ErrCodePublishFailed, "a", nil)
	err2 := NewBrokerError(ErrCodePublishFailed, "b", nil)
	err3 := NewBrokerError(ErrCodeConnectionFailed, "c", nil)

	assert.True(t, err1.Is(err2))
	assert.False(t, err1.Is(err3))
}

func TestIsBrokerError(t *testing.T) {
	assert.True(t, IsBrokerError(NewBrokerError(ErrCodePublishFailed, "m", nil)))
	assert.False(t, IsBrokerError(ErrPublishFailed))
}

func TestGetBrokerError(t *testing.T) {
	be := NewBrokerError(ErrCodePublishFailed, "m", nil)
	assert.Equal(t, be, GetBrokerError(be))
	assert.Nil(t, GetBrokerError(ErrPublishFailed))
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "broker_error_connection",
			err:      NewBrokerError(ErrCodeConnectionFailed, "m", nil),
			expected: true,
		},
		{
			name:     "broker_error_publish",
			err:      NewBrokerError(ErrCodePublishFailed, "m", nil),
			expected: false,
		},
		{
			name:     "sentinel_connection",
			err:      ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "sentinel_timeout",
			err:      ErrConnectionTimeout,
			expected: true,
		},
		{
			name:     "unrelated",
			err:      ErrMessageInvalid,
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsConnectionError(tt.err))
		})
	}
}

func TestMultiError(t *testing.T) {
	me := NewMultiError()
	assert.False(t, me.HasErrors())
	assert.Nil(t, me.ErrorOrNil())
	assert.Equal(t, "no errors", me.Error())

	me.Add(nil) // should be ignored
	assert.False(t, me.HasErrors())

	me.Add(ErrPublishFailed)
	assert.True(t, me.HasErrors())
	assert.Equal(t, ErrPublishFailed.Error(), me.Error())

	me.Add(ErrConnectionFailed)
	assert.Contains(t, me.Error(), "multiple errors (2)")
	assert.ErrorIs(t, me, ErrPublishFailed)
}

func TestIsRetryableError_NonBrokerError(t *testing.T) {
	assert.False(t, IsRetryableError(ErrPublishFailed))
}

func TestInMemoryBroker_PublishCreatesTopicOnDemand(t *testing.T) {
	b := NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	// Publish to a topic that has no subscribers yet
	msg := NewMessage("new-topic", []byte("data"))
	require.NoError(t, b.Publish(ctx, "new-topic", msg))
}

func TestMessage_Timestamp(t *testing.T) {
	before := time.Now().UTC()
	msg := NewMessage("t", nil)
	after := time.Now().UTC()

	assert.False(t, msg.Timestamp.Before(before))
	assert.False(t, msg.Timestamp.After(after))
}
