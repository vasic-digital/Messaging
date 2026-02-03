# Architecture

This document describes the design decisions, architectural patterns, and package relationships in the `digital.vasic.messaging` module.

## Design Goals

1. **Broker independence** -- Application code should not depend on any specific message broker. Swapping Kafka for RabbitMQ (or vice versa) should require only configuration changes, not code changes.
2. **Zero external client dependencies** -- The module itself does not import Kafka or AMQP client libraries. This keeps the dependency tree minimal and avoids version conflicts in consuming projects.
3. **Testability without infrastructure** -- All functionality is testable using the `InMemoryBroker` and injectable functions. No Docker, no network, no external services required.
4. **Composable utilities** -- Higher-level patterns (retry, dead letter, batching, async/sync production) compose on top of the core interface, not on specific implementations.

## Layered Architecture

The module is organized into three layers:

```
+-------------------------------------------------------+
|  Layer 3: Utilities (broker-agnostic patterns)         |
|  pkg/consumer: ConsumerGroup, RetryPolicy, DLQ, Batch |
|  pkg/producer: AsyncProducer, SyncProducer, Serializer |
+-------------------------------------------------------+
|  Layer 2: Adapters (broker-specific config & types)    |
|  pkg/kafka:    Producer, Consumer, Config, Partition   |
|  pkg/rabbitmq: Producer, Consumer, Config, Exchange    |
+-------------------------------------------------------+
|  Layer 1: Core (interfaces & types)                    |
|  pkg/broker: MessageBroker, Message, Handler,          |
|              Subscription, Config, BrokerError          |
+-------------------------------------------------------+
```

**Layer 1 (Core)** defines the contracts. Every other package depends on it, but it depends on nothing within the module.

**Layer 2 (Adapters)** provides broker-specific configuration, validation, and type-safe wrappers. Adapters import only Layer 1.

**Layer 3 (Utilities)** provides reusable consumption and production patterns. Utilities import only Layer 1 and operate through the `MessageBroker` interface.

There are no lateral dependencies between packages in the same layer (e.g., `kafka` never imports `rabbitmq`; `consumer` never imports `producer`).

## Design Patterns

### Adapter Pattern

The Kafka and RabbitMQ packages are adapters that bridge broker-specific concepts to the unified `MessageBroker` interface.

Each adapter provides:
- A `Config` struct with broker-specific settings (SASL, TLS, exchanges, partitions, etc.)
- A `Producer` that wraps publish operations with config validation and connection state
- A `Consumer` that wraps subscribe operations with subscription lifecycle management
- Type enums (`PartitionStrategy`, `ExchangeType`) for type-safe configuration

The adapters do not implement `MessageBroker` directly. Instead, they provide compatible method signatures and use injectable functions for actual I/O, keeping the module free of external client dependencies.

### Observer Pattern

The publish/subscribe mechanism follows the Observer pattern:

- **Subject**: The broker (via `Publish` method on a topic)
- **Observers**: Registered `Handler` functions (via `Subscribe`)
- **Notification**: When a message is published to a topic, all active subscribers for that topic receive a copy

In `InMemoryBroker`, this is implemented with a map of topic to subscriber list. Each subscriber's handler is invoked in a separate goroutine to avoid blocking the publisher.

The `ConsumerGroup` extends this by managing multiple topic subscriptions as a unit, with coordinated startup and shutdown.

### Factory Pattern

Factory functions are used throughout to create properly initialized instances with sensible defaults:

| Factory Function | Returns |
|---|---|
| `broker.NewInMemoryBroker()` | `*InMemoryBroker` with initialized maps and channels |
| `broker.NewMessage(topic, value)` | `*Message` with generated UUID, timestamp, empty headers |
| `broker.NewMessageWithKey(topic, key, value)` | `*Message` with a partition key |
| `broker.NewMessageWithID(id, topic, value)` | `*Message` with a specific ID |
| `broker.DefaultConfig()` | `*Config` with localhost:9092 defaults |
| `kafka.DefaultConfig()` | `*Config` with full Kafka defaults |
| `kafka.NewProducer(config)` | `*Producer` (nil config uses defaults) |
| `kafka.NewConsumer(config)` | `*Consumer` (nil config uses defaults) |
| `rabbitmq.DefaultConfig()` | `*Config` with full RabbitMQ defaults |
| `rabbitmq.NewProducer(config)` | `*Producer` (nil config uses defaults) |
| `rabbitmq.NewConsumer(config)` | `*Consumer` (nil config uses defaults) |
| `consumer.NewConsumerGroup(id, broker)` | `*ConsumerGroup` with auto-generated ID if empty |
| `consumer.DefaultRetryPolicy()` | `*RetryPolicy` with 3 retries, 100ms base, 2x backoff |
| `consumer.NewDeadLetterHandler(broker, topic)` | `*DeadLetterHandler` |
| `consumer.NewBatchConsumer(size, flush, handler)` | `*BatchConsumer` with clamped defaults |
| `producer.NewAsyncProducer(broker, bufferSize)` | `*AsyncProducer` with clamped buffer |
| `producer.NewSyncProducer(broker, timeout)` | `*SyncProducer` with default 30s timeout |

### Strategy Pattern

The `Serializer` and `Compressor` interfaces in the producer package implement the Strategy pattern. The `SyncProducer` and `AsyncProducer` accept interchangeable serialization and compression strategies:

- **Serializer strategies**: `JSONSerializer`, `ProtobufSerializer`, `AvroSerializer`
- **Compressor strategies**: `GzipCompressor`, `SnappyCompressor`, `LZ4Compressor`

These are set at runtime via `SetSerializer` and `SetCompressor`, allowing the same producer to use different encoding schemes without modification.

### Decorator Pattern

The `WithRetry` function decorates a `broker.Handler` with retry logic:

```
Original Handler --> WithRetry wrapper --> Handler with retry behavior
```

This pattern allows composing behaviors: a handler can be wrapped with retry, and the resulting handler can be passed to a `ConsumerGroup` or used with a `DeadLetterHandler` for further composition.

### Dependency Injection

The injectable function pattern (`SetPublishFunc`) is a form of constructor/setter injection. Instead of importing a Kafka client library, the adapter accepts a function that performs the actual I/O:

```go
producer.SetPublishFunc(func(ctx context.Context, topic string, msg *broker.Message) error {
    // Real Kafka write happens here, injected by the caller.
    return kafkaWriter.WriteMessages(ctx, ...)
})
```

This inverts the dependency: the adapter defines what it needs (a function signature), and the consumer of the library provides the implementation.

## Error Architecture

The error system uses three complementary approaches:

1. **Sentinel errors** (`ErrNotConnected`, `ErrMessageInvalid`, etc.) for quick `errors.Is` comparisons
2. **Structured errors** (`BrokerError` with `Code`, `Message`, `Cause`, `Retryable`) for rich error information
3. **Aggregate errors** (`MultiError`) for operations that can produce multiple errors (e.g., closing multiple subscriptions)

Error codes are grouped by category:
- Connection: `CONNECTION_FAILED`, `CONNECTION_CLOSED`, `CONNECTION_TIMEOUT`, `AUTHENTICATION_FAILED`
- Publish: `PUBLISH_FAILED`, `PUBLISH_TIMEOUT`
- Subscribe: `SUBSCRIBE_FAILED`, `CONSUMER_CANCELED`, `HANDLER_ERROR`
- Message: `MESSAGE_TOO_LARGE`, `MESSAGE_EXPIRED`, `MESSAGE_INVALID`
- Config: `INVALID_CONFIG`
- General: `BROKER_UNAVAILABLE`, `OPERATION_CANCELED`

Retryability is determined by error code. Only transient errors (`CONNECTION_FAILED`, `CONNECTION_TIMEOUT`, `PUBLISH_TIMEOUT`, `BROKER_UNAVAILABLE`) are marked retryable. This feeds into the `RetryPolicy` and `WithRetry` mechanisms in the consumer package.

## Concurrency Model

### InMemoryBroker

Uses `sync.RWMutex` to protect the queue and subscriber maps. Publishing acquires a write lock, reads subscriber list, and dispatches to each handler in a separate goroutine. This ensures the publisher is never blocked by slow handlers.

### Kafka/RabbitMQ Adapters

Use `sync/atomic` for boolean state flags (`connected`, `closed`) to avoid lock contention on hot paths like `IsConnected()`. Subscription maps are protected by `sync.RWMutex` since they are modified infrequently (subscribe/unsubscribe) but read often (publish routing).

### AsyncProducer

Uses a buffered channel (`msgCh`) as a work queue. A single goroutine runs the send loop, pulling messages from the channel and publishing them via the broker. On stop, it drains the remaining messages before returning. A `sync.WaitGroup` ensures graceful shutdown.

### BatchConsumer

Uses a mutex-guarded slice as a buffer. A background goroutine runs a ticker loop that flushes on timeout or when signaled via a flush channel. The flush channel is non-blocking (`select` with `default`) to avoid deadlock when the flush loop is busy.

### ConsumerGroup

Uses `atomic.Bool` for the `running` state and `sync.RWMutex` for the subscription and handler maps. Start and Stop are idempotent. On Start failure, already-created subscriptions are cleaned up before returning the error.

## Message Lifecycle

1. **Creation**: `broker.NewMessage(topic, value)` generates a UUID, sets the timestamp, and initializes empty headers.
2. **Enrichment**: Optional key (`WithKey`, `WithStringKey`), headers (`SetHeader`), and cloning (`Clone`).
3. **Serialization** (optional): `SyncProducer` applies `Serializer.Serialize` and sets `content-type` header.
4. **Compression** (optional): `SyncProducer` applies `Compressor.Compress` and sets `content-encoding` header.
5. **Publishing**: Broker's `Publish` method delivers to topic. `InMemoryBroker` clones the message and appends to the queue.
6. **Delivery**: Active subscribers receive a clone of the message via their `Handler` function.
7. **Processing**: Handler processes the message. On failure, `WithRetry` may retry with exponential backoff.
8. **Dead Letter**: If all retries fail, `DeadLetterHandler` routes to a DLQ topic with failure metadata headers.

## Configuration Validation

Both Kafka and RabbitMQ configs validate eagerly on `Connect`. Validation rules:

**Kafka**: At least one non-empty broker address, non-empty `ClientID`, `BatchSize >= 1`, SASL username required when SASL is enabled.

**RabbitMQ**: If `URL` is set, it takes precedence and skips field validation. Otherwise: non-empty `Host`, valid port (1-65535), non-empty `Username`, `MaxConnections >= 1`, non-negative `Prefetch`, valid `ExchangeType` if set.

**Base Config**: At least one non-empty broker address, non-empty `ClientID`.

## Dependencies

The module has exactly two external dependencies:

- `github.com/google/uuid` -- UUID generation for message IDs and subscription IDs
- `github.com/stretchr/testify` -- Test assertions (test-only)

No message broker client libraries are imported. This is a deliberate architectural choice to keep the module lightweight and avoid version conflicts when the consuming project already uses a specific client library version.
