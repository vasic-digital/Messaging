# AGENTS.md

Multi-agent coordination guide for the `digital.vasic.messaging` module.

## Module Identity

- **Module path**: `digital.vasic.messaging`
- **Language**: Go 1.24.0
- **Packages**: `pkg/broker`, `pkg/kafka`, `pkg/rabbitmq`, `pkg/consumer`, `pkg/producer`
- **Role**: Generic, reusable message broker abstraction layer with adapter pattern

## Agent Responsibilities

### Broker Agent

- **Scope**: `pkg/broker/` -- core interfaces, message types, error system, in-memory broker
- **Owns**: `MessageBroker` interface, `Message` struct, `Subscription` interface, `Handler` type, `Config`, `BrokerType`, all error types (`BrokerError`, `MultiError`, sentinel errors)
- **Constraints**: This package has zero internal imports. It is the foundation of the module. All other packages depend on it. Changes here ripple everywhere.
- **Testing**: `InMemoryBroker` is the canonical test double. All cross-package tests use it. Never introduce external infrastructure dependencies in broker tests.

### Kafka Agent

- **Scope**: `pkg/kafka/` -- Kafka adapter with Producer, Consumer, and Config
- **Owns**: `kafka.Config`, `kafka.Producer`, `kafka.Consumer`, `PartitionStrategy` enum
- **Constraints**: Must not import any Kafka client library (kafka-go, confluent-kafka-go, etc.). All actual I/O is injectable via `SetPublishFunc`. Config validation is self-contained.
- **Dependencies**: `pkg/broker` only.

### RabbitMQ Agent

- **Scope**: `pkg/rabbitmq/` -- RabbitMQ adapter with Producer, Consumer, and Config
- **Owns**: `rabbitmq.Config`, `rabbitmq.Producer`, `rabbitmq.Consumer`, `ExchangeType` enum
- **Constraints**: Must not import any AMQP client library (amqp091-go, etc.). All actual I/O is injectable via `SetPublishFunc`. Config validation is self-contained. `ConnectionString()` generates AMQP URIs.
- **Dependencies**: `pkg/broker` only.

### Consumer Agent

- **Scope**: `pkg/consumer/` -- higher-level consumption patterns
- **Owns**: `ConsumerGroup`, `RetryPolicy`, `DeadLetterHandler`, `BatchConsumer`, `WithRetry` wrapper
- **Constraints**: Operates on the `broker.MessageBroker` interface. Never references Kafka or RabbitMQ directly. Must remain broker-agnostic.
- **Dependencies**: `pkg/broker` only.

### Producer Agent

- **Scope**: `pkg/producer/` -- higher-level production patterns
- **Owns**: `AsyncProducer`, `SyncProducer`, `Serializer` interface, `Compressor` interface, `JSONSerializer`, `ProtobufSerializer`, `AvroSerializer`, `GzipCompressor`, `SnappyCompressor`, `LZ4Compressor`
- **Constraints**: Operates on the `broker.MessageBroker` interface. Never references Kafka or RabbitMQ directly. Must remain broker-agnostic.
- **Dependencies**: `pkg/broker` only.

## Coordination Rules

### Dependency Direction

All dependencies flow inward toward `pkg/broker`. No lateral dependencies exist between sibling packages.

```
producer --> broker <-- consumer
              ^  ^
              |  |
          kafka  rabbitmq
```

### Interface Contracts

1. **MessageBroker** is the single integration point. Any new broker adapter must implement it fully: `Connect`, `Close`, `HealthCheck`, `IsConnected`, `Publish`, `Subscribe`, `Unsubscribe`, `Type`.
2. **Subscription** returned by `Subscribe` must support `Unsubscribe`, `IsActive`, `Topic`, `ID`.
3. **Handler** is `func(ctx context.Context, msg *broker.Message) error`. All consumer utilities wrap this type.
4. **Serializer** and **Compressor** are open extension points in the producer package. New formats plug in without modifying existing code.

### Adding a New Broker Adapter

1. Create `pkg/<brokername>/` with `Config`, `Producer`, `Consumer` types.
2. Add a new `BrokerType` constant in `pkg/broker/broker.go`.
3. Both `Producer` and `Consumer` must expose `Connect`, `Close`, `IsConnected`, `Publish` or `Subscribe`, `Unsubscribe`, and `Config()` methods.
4. Use injectable function pattern (`SetPublishFunc`) to avoid importing client libraries.
5. Write table-driven tests using `InMemoryBroker` or injectable functions.

### Error Handling Protocol

- All agents must use `broker.BrokerError` for structured errors and `broker.NewBrokerError` to construct them.
- Sentinel errors (`broker.ErrNotConnected`, `broker.ErrMessageInvalid`, etc.) are for quick comparison.
- `broker.IsRetryableError` determines retry eligibility. The consumer agent's `RetryPolicy` and `WithRetry` depend on this.
- `broker.MultiError` aggregates errors during batch teardown (e.g., `ConsumerGroup.Stop`).

### Testing Protocol

- All tests run with `go test ./... -count=1 -race`.
- No external infrastructure is required. `InMemoryBroker` and injectable functions cover all paths.
- Use `github.com/stretchr/testify` for assertions.
- Table-driven test style: `Test<Type>_<Method>_<Scenario>`.

### Concurrency Model

- `InMemoryBroker` uses `sync.RWMutex` for thread-safe queue and subscriber access. Message delivery to subscribers runs in separate goroutines.
- Kafka and RabbitMQ adapters use `sync/atomic` for connection state (`atomic.Bool`) and counters (`atomic.Int64`). Subscription maps are guarded by `sync.RWMutex`.
- `AsyncProducer` uses a buffered channel as a send queue with a dedicated goroutine for the send loop. Drains on stop.
- `BatchConsumer` uses a mutex-guarded buffer with a ticker-based flush loop. Context cancellation triggers a final flush.
- `ConsumerGroup` tracks running state with `atomic.Bool` and guards subscription maps with `sync.RWMutex`.

## Communication Between Agents

Agents do not communicate directly. All coordination happens through the `broker.MessageBroker` interface. A consumer agent and a producer agent can operate on the same broker instance concurrently. The broker implementation is responsible for thread safety.

## Release Checklist

1. All tests pass: `go test ./... -count=1 -race`
2. No vet warnings: `go vet ./...`
3. Formatting applied: `gofmt -w .`
4. Dependencies tidy: `go mod tidy`
5. CHANGELOG.md updated
6. Tag follows semver: `v0.1.0`, `v0.2.0`, etc.
