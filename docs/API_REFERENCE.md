# API Reference

Complete reference for every exported type, function, and method in the `digital.vasic.messaging` module.

---

## Package `broker`

Import: `digital.vasic.messaging/pkg/broker`

Core interfaces, types, error system, and the in-memory broker implementation.

### Types

#### `BrokerType`

```go
type BrokerType string
```

Represents the type of message broker.

**Constants:**

| Constant | Value | Description |
|---|---|---|
| `BrokerTypeKafka` | `"kafka"` | Apache Kafka |
| `BrokerTypeRabbitMQ` | `"rabbitmq"` | RabbitMQ |
| `BrokerTypeInMemory` | `"inmemory"` | In-memory (testing) |

**Methods:**

- `func (bt BrokerType) String() string` -- Returns the string representation.
- `func (bt BrokerType) IsValid() bool` -- Returns true if the value is one of the defined constants.

---

#### `MessageBroker` (interface)

```go
type MessageBroker interface {
    Connect(ctx context.Context) error
    Close(ctx context.Context) error
    HealthCheck(ctx context.Context) error
    IsConnected() bool
    Publish(ctx context.Context, topic string, msg *Message) error
    Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error)
    Unsubscribe(topic string) error
    Type() BrokerType
}
```

The core interface for all message brokers.

| Method | Description |
|---|---|
| `Connect` | Establishes a connection to the broker. |
| `Close` | Closes the connection and cleans up resources. |
| `HealthCheck` | Checks if the broker is healthy and reachable. |
| `IsConnected` | Returns true if currently connected. |
| `Publish` | Sends a message to a topic or queue. |
| `Subscribe` | Creates a subscription to a topic with a handler. Returns a `Subscription`. |
| `Unsubscribe` | Cancels all subscriptions for a topic. |
| `Type` | Returns the `BrokerType`. |

---

#### `Handler`

```go
type Handler func(ctx context.Context, msg *Message) error
```

A function that processes messages. Used as the callback for subscriptions.

---

#### `Subscription` (interface)

```go
type Subscription interface {
    Unsubscribe() error
    IsActive() bool
    Topic() string
    ID() string
}
```

Represents an active subscription.

| Method | Description |
|---|---|
| `Unsubscribe` | Cancels the subscription. |
| `IsActive` | Returns true if the subscription is still active. |
| `Topic` | Returns the subscribed topic or queue name. |
| `ID` | Returns the unique subscription identifier. |

---

#### `Message`

```go
type Message struct {
    ID        string            `json:"id"`
    Topic     string            `json:"topic"`
    Key       []byte            `json:"key,omitempty"`
    Value     []byte            `json:"value"`
    Headers   map[string]string `json:"headers,omitempty"`
    Timestamp time.Time         `json:"timestamp"`
}
```

Represents a message in the messaging system.

**Constructor functions:**

- `func NewMessage(topic string, value []byte) *Message` -- Creates a message with a generated UUID, UTC timestamp, and empty headers.
- `func NewMessageWithKey(topic string, key, value []byte) *Message` -- Creates a message with a specific partition key.
- `func NewMessageWithID(id, topic string, value []byte) *Message` -- Creates a message with a specific ID.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetHeader` | `(key, value string) *Message` | Sets a header. Returns self for chaining. |
| `GetHeader` | `(key string) string` | Gets a header value. Returns empty string if not found. |
| `WithKey` | `(key []byte) *Message` | Sets the message key. Returns self for chaining. |
| `WithStringKey` | `(key string) *Message` | Sets the message key from a string. Returns self for chaining. |
| `Clone` | `() *Message` | Creates a deep copy of the message. |
| `MarshalJSON` | `() ([]byte, error)` | Implements `json.Marshaler`. |
| `UnmarshalJSON` | `(data []byte) error` | Implements `json.Unmarshaler`. |

---

#### `Config`

```go
type Config struct {
    Brokers  []string `json:"brokers" yaml:"brokers"`
    ClientID string   `json:"client_id" yaml:"client_id"`
}
```

Common configuration for message brokers.

**Methods:**

- `func (c *Config) Validate() error` -- Validates that at least one non-empty broker address and a non-empty `ClientID` are set. Returns `ErrInvalidConfig` on failure.

**Constructor functions:**

- `func DefaultConfig() *Config` -- Returns a config with `Brokers: ["localhost:9092"]` and `ClientID: "messaging-client"`.

---

#### `ErrorCode`

```go
type ErrorCode string
```

**Constants:**

| Constant | Value | Category |
|---|---|---|
| `ErrCodeConnectionFailed` | `"CONNECTION_FAILED"` | Connection |
| `ErrCodeConnectionClosed` | `"CONNECTION_CLOSED"` | Connection |
| `ErrCodeConnectionTimeout` | `"CONNECTION_TIMEOUT"` | Connection |
| `ErrCodeAuthFailed` | `"AUTHENTICATION_FAILED"` | Connection |
| `ErrCodePublishFailed` | `"PUBLISH_FAILED"` | Publish |
| `ErrCodePublishTimeout` | `"PUBLISH_TIMEOUT"` | Publish |
| `ErrCodeSubscribeFailed` | `"SUBSCRIBE_FAILED"` | Subscribe |
| `ErrCodeConsumerCanceled` | `"CONSUMER_CANCELED"` | Subscribe |
| `ErrCodeHandlerError` | `"HANDLER_ERROR"` | Subscribe |
| `ErrCodeMessageTooLarge` | `"MESSAGE_TOO_LARGE"` | Message |
| `ErrCodeMessageExpired` | `"MESSAGE_EXPIRED"` | Message |
| `ErrCodeMessageInvalid` | `"MESSAGE_INVALID"` | Message |
| `ErrCodeInvalidConfig` | `"INVALID_CONFIG"` | Configuration |
| `ErrCodeBrokerUnavailable` | `"BROKER_UNAVAILABLE"` | General |
| `ErrCodeOperationCanceled` | `"OPERATION_CANCELED"` | General |

---

#### Sentinel Errors

```go
var (
    ErrConnectionFailed  = errors.New("connection failed")
    ErrConnectionClosed  = errors.New("connection closed")
    ErrConnectionTimeout = errors.New("connection timeout")
    ErrAuthFailed        = errors.New("authentication failed")
    ErrNotConnected      = errors.New("not connected to broker")
    ErrPublishFailed     = errors.New("publish failed")
    ErrPublishTimeout    = errors.New("publish timeout")
    ErrSubscribeFailed   = errors.New("subscribe failed")
    ErrConsumerCanceled  = errors.New("consumer canceled")
    ErrHandlerError      = errors.New("message handler error")
    ErrMessageTooLarge   = errors.New("message too large")
    ErrMessageExpired    = errors.New("message expired")
    ErrMessageInvalid    = errors.New("invalid message")
    ErrInvalidConfig     = errors.New("invalid configuration")
    ErrBrokerUnavailable = errors.New("broker unavailable")
    ErrOperationCanceled = errors.New("operation canceled")
    ErrTopicNotFound     = errors.New("topic not found")
    ErrQueueNotFound     = errors.New("queue not found")
)
```

---

#### `BrokerError`

```go
type BrokerError struct {
    Code      ErrorCode `json:"code"`
    Message   string    `json:"message"`
    Cause     error     `json:"-"`
    Retryable bool      `json:"retryable"`
}
```

Structured error with code, message, cause, and retryability flag.

**Constructor:**

- `func NewBrokerError(code ErrorCode, message string, cause error) *BrokerError` -- Creates a `BrokerError`. `Retryable` is automatically set based on the error code.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Error` | `() string` | Format: `[CODE] message: cause` or `[CODE] message`. |
| `Unwrap` | `() error` | Returns the underlying `Cause`. |
| `Is` | `(target error) bool` | Matches by `Code` for `*BrokerError` targets, or delegates to `errors.Is` on `Cause`. |

**Helper functions:**

- `func IsBrokerError(err error) bool` -- Checks if the error chain contains a `BrokerError`.
- `func GetBrokerError(err error) *BrokerError` -- Extracts a `BrokerError` from the error chain. Returns nil if not found.
- `func IsRetryableError(err error) bool` -- Returns true if the error is a retryable `BrokerError`.
- `func IsConnectionError(err error) bool` -- Returns true if the error is a connection-related error.

---

#### `MultiError`

```go
type MultiError struct {
    Errors []error
}
```

Aggregates multiple errors.

**Constructor:**

- `func NewMultiError(errs ...error) *MultiError`

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Error` | `() string` | Returns "no errors", the single error, or "multiple errors (N): first". |
| `Add` | `(err error)` | Appends a non-nil error. |
| `HasErrors` | `() bool` | Returns true if there are any errors. |
| `ErrorOrNil` | `() error` | Returns nil if empty, self otherwise. |
| `Unwrap` | `() error` | Returns the first error. |

---

#### `InMemoryBroker`

```go
type InMemoryBroker struct { /* unexported fields */ }
```

In-memory message broker for testing and development. Implements `MessageBroker`.

**Constructor:**

- `func NewInMemoryBroker() *InMemoryBroker`

**Methods:** All `MessageBroker` interface methods. `Type()` returns `BrokerTypeInMemory`.

---

## Package `kafka`

Import: `digital.vasic.messaging/pkg/kafka`

Kafka adapter with Producer, Consumer, and configuration.

### Types

#### `PartitionStrategy`

```go
type PartitionStrategy int
```

**Constants:**

| Constant | Value | Description |
|---|---|---|
| `RoundRobin` | `0` | Distributes messages evenly across partitions. |
| `Hash` | `1` | Uses key hashing to assign partitions. |
| `Manual` | `2` | Allows specifying the partition explicitly. |

**Methods:**

- `func (ps PartitionStrategy) String() string` -- Returns `"round_robin"`, `"hash"`, `"manual"`, or `"unknown"`.

---

#### `Config`

```go
type Config struct {
    // Broker connection.
    Brokers  []string
    ClientID string

    // Consumer group.
    GroupID string
    Topics  []string

    // SASL authentication.
    SASLEnabled   bool
    SASLMechanism string
    SASLUsername   string
    SASLPassword   string

    // TLS.
    TLSEnabled    bool
    TLSConfig     *tls.Config
    TLSSkipVerify bool

    // Producer settings.
    RequiredAcks     int
    MaxRetries       int
    RetryBackoff     time.Duration
    BatchSize        int
    BatchTimeout     time.Duration
    MaxMessageBytes  int
    CompressionCodec string
    Idempotent       bool
    Partitioning     PartitionStrategy

    // Consumer settings.
    AutoOffsetReset    string
    EnableAutoCommit   bool
    AutoCommitInterval time.Duration
    SessionTimeout     time.Duration
    HeartbeatInterval  time.Duration
    MaxPollRecords     int
    FetchMinBytes      int
    FetchMaxBytes      int
    FetchMaxWait       time.Duration

    // Connection settings.
    DialTimeout     time.Duration
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    MetadataRefresh time.Duration
}
```

All fields support JSON and YAML struct tags.

**Constructor:**

- `func DefaultConfig() *Config` -- Returns a config with sensible defaults (localhost:9092, RoundRobin, LZ4, idempotent, etc.).

**Methods:**

- `func (c *Config) Validate() error` -- Validates brokers, client ID, batch size, and SASL requirements.

---

#### `Producer`

```go
type Producer struct { /* unexported fields */ }
```

**Constructor:**

- `func NewProducer(config *Config) *Producer` -- Nil config uses `DefaultConfig()`.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetPublishFunc` | `(fn func(ctx context.Context, topic string, msg *broker.Message) error)` | Injects the actual Kafka write function. |
| `Connect` | `(ctx context.Context) error` | Validates config and marks connected. |
| `Close` | `(ctx context.Context) error` | Marks closed. Idempotent. |
| `IsConnected` | `() bool` | True if connected and not closed. |
| `Publish` | `(ctx context.Context, topic string, msg *broker.Message) error` | Publishes via the injected function. Returns `ErrNotConnected` or `ErrMessageInvalid` on error. |
| `Config` | `() *Config` | Returns the configuration. |

---

#### `Consumer`

```go
type Consumer struct { /* unexported fields */ }
```

**Constructor:**

- `func NewConsumer(config *Config) *Consumer` -- Nil config uses `DefaultConfig()`.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Connect` | `(ctx context.Context) error` | Validates config and marks connected. |
| `Close` | `(ctx context.Context) error` | Unsubscribes all and marks closed. Idempotent. |
| `IsConnected` | `() bool` | True if connected and not closed. |
| `Subscribe` | `(ctx context.Context, topic string, handler broker.Handler) (broker.Subscription, error)` | Creates a subscription. Returns error if not connected, topic is empty, or handler is nil. |
| `Unsubscribe` | `(topic string) error` | Cancels subscription for a topic. |
| `Config` | `() *Config` | Returns the configuration. |

---

## Package `rabbitmq`

Import: `digital.vasic.messaging/pkg/rabbitmq`

RabbitMQ adapter with Producer, Consumer, and configuration.

### Types

#### `ExchangeType`

```go
type ExchangeType string
```

**Constants:**

| Constant | Value | Description |
|---|---|---|
| `Direct` | `"direct"` | Routes by exact routing key match. |
| `Fanout` | `"fanout"` | Broadcasts to all bound queues. |
| `Topic` | `"topic"` | Routes by routing key pattern. |
| `Headers` | `"headers"` | Routes by message headers. |

**Methods:**

- `func (et ExchangeType) String() string` -- Returns the string value.
- `func (et ExchangeType) IsValid() bool` -- Returns true if the value is one of the defined constants.

---

#### `Config`

```go
type Config struct {
    // Connection.
    URL      string
    Host     string
    Port     int
    Username string
    Password string
    VHost    string

    // Exchange.
    Exchange    string
    ExchType    ExchangeType
    Queue       string
    RoutingKey  string
    BindingKeys []string

    // Consumer.
    Prefetch    int
    ConsumerTag string
    AutoAck     bool
    Exclusive   bool

    // TLS.
    TLSEnabled    bool
    TLSConfig     *tls.Config
    TLSSkipVerify bool

    // Connection pool.
    MaxConnections    int
    MaxChannels       int
    ConnectionTimeout time.Duration
    HeartbeatInterval time.Duration

    // Reconnection.
    ReconnectDelay    time.Duration
    MaxReconnectDelay time.Duration
    ReconnectBackoff  float64
    MaxReconnectCount int

    // Publisher.
    PublishConfirm bool
    PublishTimeout time.Duration
    Mandatory      bool
    Immediate      bool

    // Queue.
    Durable    bool
    AutoDelete bool
}
```

All fields support JSON and YAML struct tags.

**Constructor:**

- `func DefaultConfig() *Config` -- Returns defaults (localhost:5672, guest/guest, topic exchange, prefetch 10, durable, publisher confirms).

**Methods:**

- `func (c *Config) Validate() error` -- Validates host, port, username, connections, prefetch, and exchange type. If `URL` is set, validation is skipped.
- `func (c *Config) ConnectionString() string` -- Returns AMQP URI. Uses `URL` if set, otherwise constructs from fields. Uses `amqps://` scheme when `TLSEnabled` is true.

---

#### `Producer`

```go
type Producer struct { /* unexported fields */ }
```

**Constructor:**

- `func NewProducer(config *Config) *Producer` -- Nil config uses `DefaultConfig()`.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetPublishFunc` | `(fn func(ctx context.Context, topic string, msg *broker.Message) error)` | Injects the actual AMQP publish function. |
| `Connect` | `(ctx context.Context) error` | Validates config and marks connected. |
| `Close` | `(ctx context.Context) error` | Marks closed. Idempotent. |
| `IsConnected` | `() bool` | True if connected and not closed. |
| `Publish` | `(ctx context.Context, topic string, msg *broker.Message) error` | Publishes via the injected function. |
| `Config` | `() *Config` | Returns the configuration. |

---

#### `Consumer`

```go
type Consumer struct { /* unexported fields */ }
```

**Constructor:**

- `func NewConsumer(config *Config) *Consumer` -- Nil config uses `DefaultConfig()`.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Connect` | `(ctx context.Context) error` | Validates config and marks connected. |
| `Close` | `(ctx context.Context) error` | Unsubscribes all and marks closed. Idempotent. |
| `IsConnected` | `() bool` | True if connected and not closed. |
| `Subscribe` | `(_ context.Context, topic string, handler broker.Handler) (broker.Subscription, error)` | Creates a subscription. Returns error if not connected, topic is empty, or handler is nil. |
| `Unsubscribe` | `(topic string) error` | Cancels subscription for a topic/queue. |
| `Config` | `() *Config` | Returns the configuration. |

---

## Package `consumer`

Import: `digital.vasic.messaging/pkg/consumer`

Higher-level consumption patterns: consumer groups, retry policies, dead letter handling, batch processing.

### Types

#### `ConsumerGroup`

```go
type ConsumerGroup struct { /* unexported fields */ }
```

Manages multiple consumers subscribing to different topics through a single broker.

**Constructor:**

- `func NewConsumerGroup(id string, b broker.MessageBroker) *ConsumerGroup` -- Empty `id` generates a UUID-based ID.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `ID` | `() string` | Returns the group identifier. |
| `Add` | `(topic string, handler broker.Handler)` | Registers a handler for a topic. |
| `Start` | `(ctx context.Context) error` | Subscribes to all registered topics. Cleans up on partial failure. |
| `Stop` | `() error` | Unsubscribes from all topics. Returns `MultiError` if any fail. |
| `IsRunning` | `() bool` | Returns true if started and not stopped. |
| `Topics` | `() []string` | Returns the list of registered topics. |

---

#### `RetryPolicy`

```go
type RetryPolicy struct {
    MaxRetries        int
    BackoffBase       time.Duration
    BackoffMax        time.Duration
    BackoffMultiplier float64
}
```

Defines retry behavior with exponential backoff.

**Constructor:**

- `func DefaultRetryPolicy() *RetryPolicy` -- Returns `MaxRetries: 3`, `BackoffBase: 100ms`, `BackoffMax: 30s`, `BackoffMultiplier: 2.0`.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Delay` | `(attempt int) time.Duration` | Calculates delay for the given attempt (0-based). Formula: `BackoffBase * BackoffMultiplier^attempt`, capped at `BackoffMax`. |
| `ShouldRetry` | `(attempt int) bool` | Returns true if `attempt < MaxRetries`. |

---

#### `WithRetry`

```go
func WithRetry(handler broker.Handler, policy *RetryPolicy) broker.Handler
```

Wraps a handler with retry logic. Nil policy uses `DefaultRetryPolicy()`. Retries on error up to `MaxRetries` with exponential backoff. Respects context cancellation. Checks `broker.IsRetryableError` and `policy.ShouldRetry` to determine whether to continue retrying.

---

#### `DeadLetterHandler`

```go
type DeadLetterHandler struct { /* unexported fields */ }
```

Handles messages that have exhausted all retries by routing them to a dead letter queue topic.

**Constructor:**

- `func NewDeadLetterHandler(b broker.MessageBroker, dlqTopic string) *DeadLetterHandler`

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetOnFailure` | `(fn func(ctx context.Context, msg *broker.Message, err error))` | Sets an optional callback invoked when a message is sent to the DLQ. |
| `Handle` | `(ctx context.Context, msg *broker.Message, originalErr error) error` | Clones the message, adds DLQ metadata headers (`x-dlq-reason`, `x-dlq-original-topic`, `x-dlq-timestamp`), and publishes to the DLQ topic. |
| `Count` | `() int64` | Returns the number of messages sent to the DLQ. |
| `DLQTopic` | `() string` | Returns the DLQ topic name. |

---

#### `BatchConsumer`

```go
type BatchConsumer struct { /* unexported fields */ }
```

Processes messages in batches. Flushes when buffer reaches `batchSize` or after `flushAfter` elapses.

**Constructor:**

- `func NewBatchConsumer(batchSize int, flushAfter time.Duration, handler func(ctx context.Context, msgs []*broker.Message) error) *BatchConsumer` -- `batchSize` is clamped to minimum 1 (default 100). `flushAfter` defaults to 5 seconds if not positive.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `Add` | `(msg *broker.Message)` | Adds a message to the buffer. Triggers flush if buffer is full. |
| `Flush` | `(ctx context.Context) error` | Processes all buffered messages immediately. |
| `Start` | `(ctx context.Context)` | Starts the background flush loop. Idempotent. |
| `Stop` | `(ctx context.Context) error` | Stops the flush loop and flushes remaining messages. |
| `BufferLen` | `() int` | Returns the current number of buffered messages. |
| `BatchSize` | `() int` | Returns the configured batch size. |
| `AsHandler` | `() broker.Handler` | Returns a `broker.Handler` that adds messages to the batch. |

---

## Package `producer`

Import: `digital.vasic.messaging/pkg/producer`

Higher-level production patterns: async/sync producers, serialization, compression.

### Interfaces

#### `Serializer`

```go
type Serializer interface {
    Serialize(v interface{}) ([]byte, error)
    Deserialize(data []byte, v interface{}) error
    ContentType() string
}
```

#### `Compressor`

```go
type Compressor interface {
    Compress(data []byte) ([]byte, error)
    Decompress(data []byte) ([]byte, error)
    Algorithm() string
}
```

### Serializer Implementations

#### `JSONSerializer`

```go
type JSONSerializer struct{}
```

- `Serialize(v interface{}) ([]byte, error)` -- JSON marshals the value.
- `Deserialize(data []byte, v interface{}) error` -- JSON unmarshals into the value.
- `ContentType() string` -- Returns `"application/json"`.

#### `ProtobufSerializer`

```go
type ProtobufSerializer struct {
    MarshalFunc   func(v interface{}) ([]byte, error)
    UnmarshalFunc func(data []byte, v interface{}) error
}
```

- `Serialize(v interface{}) ([]byte, error)` -- Calls `MarshalFunc`. Returns error if nil.
- `Deserialize(data []byte, v interface{}) error` -- Calls `UnmarshalFunc`. Returns error if nil.
- `ContentType() string` -- Returns `"application/x-protobuf"`.

#### `AvroSerializer`

```go
type AvroSerializer struct {
    MarshalFunc   func(v interface{}) ([]byte, error)
    UnmarshalFunc func(data []byte, v interface{}) error
}
```

- `Serialize(v interface{}) ([]byte, error)` -- Calls `MarshalFunc`. Returns error if nil.
- `Deserialize(data []byte, v interface{}) error` -- Calls `UnmarshalFunc`. Returns error if nil.
- `ContentType() string` -- Returns `"application/avro"`.

### Compressor Implementations

#### `GzipCompressor`

```go
type GzipCompressor struct {
    Level int // gzip.DefaultCompression if zero
}
```

- `Compress(data []byte) ([]byte, error)` -- Gzip compresses the data.
- `Decompress(data []byte) ([]byte, error)` -- Gzip decompresses the data.
- `Algorithm() string` -- Returns `"gzip"`.

#### `SnappyCompressor`

```go
type SnappyCompressor struct {
    CompressFunc   func(data []byte) ([]byte, error)
    DecompressFunc func(data []byte) ([]byte, error)
}
```

- `Compress(data []byte) ([]byte, error)` -- Calls `CompressFunc`. Pass-through if nil.
- `Decompress(data []byte) ([]byte, error)` -- Calls `DecompressFunc`. Pass-through if nil.
- `Algorithm() string` -- Returns `"snappy"`.

#### `LZ4Compressor`

```go
type LZ4Compressor struct {
    CompressFunc   func(data []byte) ([]byte, error)
    DecompressFunc func(data []byte) ([]byte, error)
}
```

- `Compress(data []byte) ([]byte, error)` -- Calls `CompressFunc`. Pass-through if nil.
- `Decompress(data []byte) ([]byte, error)` -- Calls `DecompressFunc`. Pass-through if nil.
- `Algorithm() string` -- Returns `"lz4"`.

### Types

#### `AsyncProducer`

```go
type AsyncProducer struct { /* unexported fields */ }
```

Buffers messages and sends them asynchronously in a background goroutine.

**Constructor:**

- `func NewAsyncProducer(b broker.MessageBroker, bufferSize int) *AsyncProducer` -- `bufferSize` defaults to 1000 if less than 1.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetSerializer` | `(s Serializer)` | Sets the serializer. |
| `SetCompressor` | `(c Compressor)` | Sets the compressor. |
| `Start` | `(ctx context.Context)` | Starts the background send loop. Idempotent. |
| `Send` | `(topic string, msg *broker.Message) error` | Queues a message. Returns error if not started or buffer is full. |
| `Errors` | `() <-chan error` | Returns a channel for receiving async send errors. |
| `Stop` | `()` | Stops the producer and drains pending messages. |
| `SentCount` | `() int64` | Returns the number of successfully sent messages. |
| `FailedCount` | `() int64` | Returns the number of failed messages. |

---

#### `SyncProducer`

```go
type SyncProducer struct { /* unexported fields */ }
```

Provides guaranteed delivery by waiting for publish acknowledgment.

**Constructor:**

- `func NewSyncProducer(b broker.MessageBroker, timeout time.Duration) *SyncProducer` -- `timeout` defaults to 30 seconds if not positive.

**Methods:**

| Method | Signature | Description |
|---|---|---|
| `SetSerializer` | `(s Serializer)` | Sets the serializer. |
| `SetCompressor` | `(c Compressor)` | Sets the compressor. |
| `Send` | `(ctx context.Context, topic string, msg *broker.Message) error` | Publishes a message with timeout. Applies compression if configured. Sets `content-type` and `content-encoding` headers. |
| `SendValue` | `(ctx context.Context, topic string, value interface{}) error` | Serializes a value (defaults to JSON if no serializer set) and publishes. |
| `SentCount` | `() int64` | Returns the number of successfully sent messages. |
| `FailedCount` | `() int64` | Returns the number of failed messages. |
