# Messaging

Generic, reusable Go module for message broker abstractions and adapters.
The `pkg/broker` core defines the `MessageBroker` interface and ships
`InMemoryBroker` for tests, dev, and Challenge runs. Adapters under
`pkg/kafka` and `pkg/rabbitmq` plug real brokers in behind the same
contract; `pkg/producer` and `pkg/consumer` layer common patterns
(async/sync producers, serializers, compressors, consumer groups,
retry policies, dead-letter handlers, batch consumers) on top.

**Module**: `digital.vasic.messaging`

Per CONST-051 this submodule is fully decoupled from any consuming
project. The Messaging i18n catalogue lives under `pkg/i18n/bundles/`;
consumers inject a real `i18n.Translator` (the noop default is
production-safe per CONST-046 because it returns the key verbatim,
keeping logs actionable).

## Packages

| Package | Description |
|---------|-------------|
| `pkg/broker` | Core interfaces: `MessageBroker`, `Message`, `Handler`, `Subscription`, `Config`. Includes `InMemoryBroker` for testing and structured error types (`BrokerError`, `MultiError`, retryable / connection / config error helpers). |
| `pkg/kafka` | Kafka adapter with `Producer`, `Consumer`, `Config` validation, `PartitionStrategy` (RoundRobin, Hash, Manual), SASL/TLS support. Real broker IO is injected via `SetPublishFunc` so the adapter is unit-testable without a Kafka cluster. |
| `pkg/rabbitmq` | RabbitMQ adapter with `Producer`, `Consumer`, `Config` validation, `ExchangeType` (Direct, Fanout, Topic, Headers), TLS/reconnection support. Same injection seam as Kafka. |
| `pkg/consumer` | Consumer utilities: `ConsumerGroup` for multi-topic subscription, `RetryPolicy` with exponential backoff, `DeadLetterHandler`, `BatchConsumer` with auto-flush. |
| `pkg/producer` | Producer utilities: `AsyncProducer` (buffered, non-blocking), `SyncProducer` (guaranteed delivery), `Serializer` (JSON, Protobuf, Avro), `Compressor` (Gzip, Snappy, LZ4). |
| `pkg/i18n` | Locale-aware message translation seam (`Translator` interface, `NoopTranslator` default). CONST-046 compliance for user-facing strings emitted by the broker / consumer layers. |

## Installation

```bash
go get digital.vasic.messaging
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.messaging/pkg/broker"
    "digital.vasic.messaging/pkg/consumer"
    "digital.vasic.messaging/pkg/producer"
)

func main() {
    ctx := context.Background()
    b := broker.NewInMemoryBroker()
    _ = b.Connect(ctx)

    // Subscribe to a topic.
    _, _ = b.Subscribe(ctx, "events", func(_ context.Context, msg *broker.Message) error {
        fmt.Printf("Received: %s\n", string(msg.Value))
        return nil
    })

    // Publish with SyncProducer (JSON serializer).
    sp := producer.NewSyncProducer(b, 0)
    sp.SetSerializer(&producer.JSONSerializer{})
    _ = sp.SendValue(ctx, "events", map[string]string{"action": "created"})

    // Consumer group with retry + dead-letter handling.
    cg := consumer.NewConsumerGroup("my-group", b)
    rp := consumer.DefaultRetryPolicy()
    dlq := consumer.NewDeadLetterHandler(b, "events-dlq")
    cg.Add("events", consumer.WithRetry(
        func(ctx context.Context, msg *broker.Message) error {
            // ... real downstream work ...
            return nil
        }, rp,
    ))
    _ = cg.Start(ctx)
    defer func() { _ = cg.Stop() }()
    _ = dlq // wire dlq.Handle in the WithRetry wrapper on exhaustion
}
```

## Testing

```bash
go test ./... -count=1 -race
```

All in-tree tests run without external infrastructure using
`InMemoryBroker`. Kafka and RabbitMQ adapter tests use injectable
functions (`SetPublishFunc`) for broker IO so no cluster is required.

## Anti-bluff guarantees (round-250 / Article XI §11.9)

> Verbatim 2026-05-19 operator mandate: "all existing tests and Challenges
> do work in anti-bluff manner — they MUST confirm that all tested codebase
> really works as expected! We had been in position that all tests do
> execute with success and all Challenges as well, but in reality the most
> of the features does not work and can't be used! This MUST NOT be the
> case and execution of tests and Challenges MUST guarantee the quality,
> the completition and full usability by end users of the product!"

The round-250 sweep added four interlocking pieces of evidence:

1. **`docs/test-coverage.md`** — a CONST-050(B) ledger mapping every
   exported symbol in `pkg/{broker,consumer,producer,kafka,rabbitmq,i18n}`
   to the test or Challenge that exercises it. Machine-verified by the
   describe challenge below.
2. **`tests/fixtures/i18n/payloads.json`** — 5-locale bilingual fixture
   (`en`, `sr-Latn`, `sr-Cyrl`, `ja`, `ar`) with non-ASCII topics, keys,
   values, and headers so encoding-path regressions cannot hide behind
   an ASCII-only test corpus.
3. **`challenges/runner/main.go`** — real broker exerciser. Drives the
   real `InMemoryBroker` through Connect → Subscribe → Publish (per
   locale, byte-asserted) → JSON Marshal/Unmarshal (per locale) →
   PublishBatch → ConsumerGroup multi-topic dispatch (cross-talk check)
   → RetryPolicy + DLQ payload-integrity → SyncProducer + Serializer →
   compile-time interface contract → graceful Close. Every PASS line
   includes the real locale, the real topic, and the real byte length —
   no metadata-only PASS.
4. **`challenges/messaging_describe_challenge.sh`** — paired-mutation
   gate. Clean run exits 0; `--anti-bluff-mutate` plants a
   symbol-rename in a tmp copy of the ledger, asserts the gate FAILS
   (exit 99) — proving the ledger walker actually catches drift instead
   of rubber-stamping it.

Run the gate:

```bash
bash challenges/messaging_describe_challenge.sh                       # exit 0
bash challenges/messaging_describe_challenge.sh --anti-bluff-mutate   # exit 99
```

## Dependencies

- `github.com/stretchr/testify` — test assertions
- `github.com/google/uuid` — ID generation
- `gopkg.in/yaml.v3` — i18n bundle loading (consumer-side translators)

No broker client libraries are required. Kafka and RabbitMQ adapters use
injectable functions for actual broker communication; real clients are a
wiring concern of the consuming project (per CONST-051(B)).

## License

See repository for license details.
