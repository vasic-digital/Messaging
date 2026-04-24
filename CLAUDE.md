# CLAUDE.md


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# In-memory broker: publish → subscribe with retry + dead-letter
cd Messaging && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./tests/integration/...
```
Expect: PASS; `broker.NewInMemoryBroker`, `producer.NewSyncProducer`, `consumer.NewConsumerGroup` per `Messaging/README.md`. Kafka/RabbitMQ backends use the same interface via injected publish funcs.


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

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | ConversationContext, HelixLLM |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
