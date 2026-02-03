# Changelog

All notable changes to the `digital.vasic.messaging` module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-03

Initial release of the `digital.vasic.messaging` module.

### Added

- **pkg/broker**: Core interfaces and types.
  - `MessageBroker` interface with `Connect`, `Close`, `HealthCheck`, `IsConnected`, `Publish`, `Subscribe`, `Unsubscribe`, `Type` methods.
  - `Subscription` interface with `Unsubscribe`, `IsActive`, `Topic`, `ID` methods.
  - `Handler` function type for message processing callbacks.
  - `Message` struct with ID, Topic, Key, Value, Headers, Timestamp fields.
  - `NewMessage`, `NewMessageWithKey`, `NewMessageWithID` factory functions.
  - Message builder methods: `SetHeader`, `GetHeader`, `WithKey`, `WithStringKey`, `Clone`.
  - JSON marshaling/unmarshaling support for `Message`.
  - `BrokerType` enum with `BrokerTypeKafka`, `BrokerTypeRabbitMQ`, `BrokerTypeInMemory`.
  - `Config` struct with `Validate` and `DefaultConfig`.
  - `InMemoryBroker` implementing `MessageBroker` for testing and development.
  - Structured error system: `BrokerError` with `ErrorCode`, retryability detection.
  - 15 error code constants covering connection, publish, subscribe, message, config, and general errors.
  - 18 sentinel errors for `errors.Is` matching.
  - Helper functions: `IsBrokerError`, `GetBrokerError`, `IsRetryableError`, `IsConnectionError`.
  - `MultiError` for aggregating multiple errors.

- **pkg/kafka**: Kafka adapter.
  - `Config` with 30+ configuration fields covering brokers, SASL, TLS, producer, consumer, and connection settings.
  - `DefaultConfig` with production-ready defaults.
  - `PartitionStrategy` enum: `RoundRobin`, `Hash`, `Manual`.
  - `Producer` with injectable publish function (`SetPublishFunc`), connection lifecycle, and config access.
  - `Consumer` with subscription management, consumer group support, and connection lifecycle.

- **pkg/rabbitmq**: RabbitMQ adapter.
  - `Config` with 25+ configuration fields covering connection, exchange, consumer, TLS, connection pool, reconnection, publisher, and queue settings.
  - `DefaultConfig` with production-ready defaults.
  - `ExchangeType` enum: `Direct`, `Fanout`, `Topic`, `Headers`.
  - `ConnectionString` method for AMQP URI generation with TLS support.
  - `Producer` with injectable publish function (`SetPublishFunc`), connection lifecycle, and config access.
  - `Consumer` with subscription management and connection lifecycle.

- **pkg/consumer**: Consumer utilities.
  - `ConsumerGroup` for multi-topic subscription management with coordinated start/stop.
  - `RetryPolicy` with exponential backoff (`Delay`, `ShouldRetry`).
  - `DefaultRetryPolicy` factory (3 retries, 100ms base, 2x multiplier, 30s max).
  - `WithRetry` handler decorator with context-aware backoff.
  - `DeadLetterHandler` for routing failed messages to a DLQ topic with metadata headers.
  - `BatchConsumer` with configurable batch size, auto-flush timer, background flush loop, and `AsHandler` adapter.

- **pkg/producer**: Producer utilities.
  - `Serializer` interface with `Serialize`, `Deserialize`, `ContentType`.
  - `JSONSerializer` (built-in), `ProtobufSerializer` (injectable), `AvroSerializer` (injectable).
  - `Compressor` interface with `Compress`, `Decompress`, `Algorithm`.
  - `GzipCompressor` (built-in), `SnappyCompressor` (injectable), `LZ4Compressor` (injectable).
  - `AsyncProducer` with buffered channel, background send loop, error channel, drain on stop.
  - `SyncProducer` with timeout, serialization, compression, and `SendValue` convenience method.
