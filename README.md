# Messaging

Generic, reusable Go module for message broker abstractions and adapters.

**Module**: `digital.vasic.messaging`

## Packages

| Package | Description |
|---------|-------------|
| `pkg/broker` | Core interfaces: MessageBroker, Message, Handler, Subscription, Config. Includes InMemoryBroker for testing and structured error types. |
| `pkg/kafka` | Kafka adapter with Producer, Consumer, Config validation, PartitionStrategy (RoundRobin, Hash, Manual), SASL/TLS support. |
| `pkg/rabbitmq` | RabbitMQ adapter with Producer, Consumer, Config validation, ExchangeType (Direct, Fanout, Topic, Headers), TLS/reconnection support. |
| `pkg/consumer` | Consumer utilities: ConsumerGroup for multi-topic subscription, RetryPolicy with exponential backoff, DeadLetterHandler, BatchConsumer with auto-flush. |
| `pkg/producer` | Producer utilities: AsyncProducer (buffered, non-blocking), SyncProducer (guaranteed delivery), Serializer (JSON, Protobuf, Avro), Compressor (Gzip, Snappy, LZ4). |

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

    // Publish with SyncProducer.
    sp := producer.NewSyncProducer(b, 0)
    _ = sp.SendValue(ctx, "events", map[string]string{"action": "created"})

    // Consumer group with retry and dead letter handling.
    cg := consumer.NewConsumerGroup("my-group", b)
    rp := consumer.DefaultRetryPolicy()
    cg.Add("events", consumer.WithRetry(
        func(_ context.Context, msg *broker.Message) error {
            return nil
        }, rp,
    ))
}
```

## Testing

```bash
go test ./... -count=1 -race
```

All tests run without external infrastructure using the InMemoryBroker.

## Dependencies

- `github.com/stretchr/testify` - Test assertions
- `github.com/google/uuid` - ID generation

No broker client libraries are required. Kafka and RabbitMQ adapters use injectable functions for actual broker communication.

## License

See repository for license details.
