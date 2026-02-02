# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Messaging is a standalone Go module (`digital.vasic.messaging`) providing generic, reusable message broker abstractions and adapters. It defines core interfaces for message publishing, subscribing, and consumption patterns without depending on any specific broker client library.

**Module**: `digital.vasic.messaging` (Go 1.24.0)

## Packages

- **pkg/broker** - Core interfaces (MessageBroker, Message, Handler, Subscription, Config), error types, and InMemoryBroker for testing
- **pkg/kafka** - Kafka adapter (Producer, Consumer, Config, PartitionStrategy) with injectable publish functions
- **pkg/rabbitmq** - RabbitMQ adapter (Producer, Consumer, Config, ExchangeType) with injectable publish functions
- **pkg/consumer** - Consumer utilities (ConsumerGroup, RetryPolicy with exponential backoff, DeadLetterHandler, BatchConsumer)
- **pkg/producer** - Producer utilities (AsyncProducer, SyncProducer, Serializer interface, Compressor interface)

## Build & Test

```bash
go mod tidy
go test ./... -count=1 -race
go vet ./...
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Table-driven tests with `github.com/stretchr/testify`
- Naming: `camelCase` private, `PascalCase` exported
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Interfaces: small/focused, accept interfaces return structs

## Dependencies

- `github.com/stretchr/testify` - Test assertions
- `github.com/google/uuid` - ID generation

No broker client libraries (kafka-go, amqp091-go) are required. Adapters use injectable function patterns for actual broker communication.

## Architecture

The module follows a layered design:

1. **broker** package defines the core `MessageBroker` interface and `Message` type
2. **kafka** and **rabbitmq** packages provide adapter implementations with config validation
3. **consumer** and **producer** packages provide higher-level patterns built on the broker interface

All adapters use injectable functions (`SetPublishFunc`) to avoid hard dependencies on broker client libraries. The `InMemoryBroker` in the broker package enables full testing without external infrastructure.
