# User Guide

This guide covers practical usage of the `digital.vasic.messaging` module, including producer/consumer patterns for Kafka and RabbitMQ, the unified broker interface, dead letter queues, retry policies, and batch processing.

## Installation

```bash
go get digital.vasic.messaging
```

**Requirements**: Go 1.24.0 or later.

## Core Concepts

The module is built around a central `MessageBroker` interface defined in `pkg/broker`. All broker-specific adapters (Kafka, RabbitMQ) and all higher-level utilities (consumer groups, async producers) operate against this interface. This means you can write broker-agnostic application code and swap implementations without changing your business logic.

Key types:

- **`broker.MessageBroker`** -- the interface every broker implements
- **`broker.Message`** -- the universal message envelope (ID, topic, key, value, headers, timestamp)
- **`broker.Handler`** -- `func(ctx context.Context, msg *broker.Message) error`
- **`broker.Subscription`** -- returned by `Subscribe`, tracks active subscriptions

## Quick Start with InMemoryBroker

The `InMemoryBroker` requires no external infrastructure and is ideal for testing and prototyping.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.messaging/pkg/broker"
)

func main() {
    ctx := context.Background()
    b := broker.NewInMemoryBroker()
    if err := b.Connect(ctx); err != nil {
        panic(err)
    }
    defer b.Close(ctx)

    // Subscribe before publishing.
    sub, err := b.Subscribe(ctx, "orders", func(_ context.Context, msg *broker.Message) error {
        fmt.Printf("Received order: %s\n", string(msg.Value))
        return nil
    })
    if err != nil {
        panic(err)
    }
    defer sub.Unsubscribe()

    // Publish a message.
    msg := broker.NewMessage("orders", []byte(`{"item":"widget","qty":5}`))
    msg.SetHeader("source", "api")
    if err := b.Publish(ctx, "orders", msg); err != nil {
        panic(err)
    }

    // Give the async subscriber time to process.
    time.Sleep(50 * time.Millisecond)
}
```

## Kafka Producer and Consumer

The Kafka adapter lives in `pkg/kafka`. It provides `Producer` and `Consumer` types with full configuration for SASL, TLS, partitioning, batching, and consumer groups.

### Kafka Producer

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.messaging/pkg/broker"
    "digital.vasic.messaging/pkg/kafka"
)

func main() {
    ctx := context.Background()

    cfg := kafka.DefaultConfig()
    cfg.Brokers = []string{"kafka-1:9092", "kafka-2:9092"}
    cfg.ClientID = "order-service"
    cfg.RequiredAcks = -1       // Wait for all replicas.
    cfg.Idempotent = true
    cfg.Partitioning = kafka.Hash

    producer := kafka.NewProducer(cfg)

    // Inject your actual Kafka client's publish function.
    producer.SetPublishFunc(func(ctx context.Context, topic string, msg *broker.Message) error {
        // Replace with your kafka-go or confluent-kafka-go writer logic.
        fmt.Printf("Publishing to %s: %s\n", topic, string(msg.Value))
        return nil
    })

    if err := producer.Connect(ctx); err != nil {
        panic(err)
    }
    defer producer.Close(ctx)

    msg := broker.NewMessage("orders", []byte(`{"order_id":"123"}`))
    msg.WithStringKey("customer-456")
    if err := producer.Publish(ctx, "orders", msg); err != nil {
        panic(err)
    }
}
```

### Kafka Consumer

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.messaging/pkg/broker"
    "digital.vasic.messaging/pkg/kafka"
)

func main() {
    ctx := context.Background()

    cfg := kafka.DefaultConfig()
    cfg.Brokers = []string{"kafka-1:9092"}
    cfg.GroupID = "order-processors"
    cfg.AutoOffsetReset = "earliest"
    cfg.EnableAutoCommit = false

    consumer := kafka.NewConsumer(cfg)
    if err := consumer.Connect(ctx); err != nil {
        panic(err)
    }
    defer consumer.Close(ctx)

    sub, err := consumer.Subscribe(ctx, "orders", func(_ context.Context, msg *broker.Message) error {
        fmt.Printf("Processing order: %s\n", string(msg.Value))
        return nil
    })
    if err != nil {
        panic(err)
    }
    defer sub.Unsubscribe()

    fmt.Printf("Subscription active: %v, ID: %s\n", sub.IsActive(), sub.ID())
}
```

### Kafka with SASL/TLS

```go
cfg := kafka.DefaultConfig()
cfg.Brokers = []string{"kafka-secure:9093"}
cfg.SASLEnabled = true
cfg.SASLMechanism = "SCRAM-SHA-256"
cfg.SASLUsername = "my-user"
cfg.SASLPassword = "my-password"
cfg.TLSEnabled = true
cfg.TLSSkipVerify = false
```

## RabbitMQ Producer and Consumer

The RabbitMQ adapter lives in `pkg/rabbitmq`. It supports exchange types (direct, fanout, topic, headers), TLS, reconnection with exponential backoff, publisher confirms, and connection pooling.

### RabbitMQ Producer

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.messaging/pkg/broker"
    "digital.vasic.messaging/pkg/rabbitmq"
)

func main() {
    ctx := context.Background()

    cfg := rabbitmq.DefaultConfig()
    cfg.Host = "rabbitmq-server"
    cfg.Port = 5672
    cfg.Username = "app"
    cfg.Password = "secret"
    cfg.VHost = "/production"
    cfg.Exchange = "events"
    cfg.ExchType = rabbitmq.Topic
    cfg.PublishConfirm = true
    cfg.Durable = true

    producer := rabbitmq.NewProducer(cfg)

    // Inject your actual AMQP publish function.
    producer.SetPublishFunc(func(ctx context.Context, topic string, msg *broker.Message) error {
        fmt.Printf("Publishing to exchange/routing-key %s: %s\n", topic, string(msg.Value))
        return nil
    })

    if err := producer.Connect(ctx); err != nil {
        panic(err)
    }
    defer producer.Close(ctx)

    msg := broker.NewMessage("user.created", []byte(`{"user_id":"789"}`))
    if err := producer.Publish(ctx, "user.created", msg); err != nil {
        panic(err)
    }
}
```

### RabbitMQ Consumer

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.messaging/pkg/broker"
    "digital.vasic.messaging/pkg/rabbitmq"
)

func main() {
    ctx := context.Background()

    cfg := rabbitmq.DefaultConfig()
    cfg.Host = "rabbitmq-server"
    cfg.Queue = "user-events"
    cfg.Prefetch = 20
    cfg.AutoAck = false

    consumer := rabbitmq.NewConsumer(cfg)
    if err := consumer.Connect(ctx); err != nil {
        panic(err)
    }
    defer consumer.Close(ctx)

    sub, err := consumer.Subscribe(ctx, "user-events", func(_ context.Context, msg *broker.Message) error {
        fmt.Printf("Received: %s\n", string(msg.Value))
        return nil
    })
    if err != nil {
        panic(err)
    }
    defer sub.Unsubscribe()
}
```

### RabbitMQ with TLS and Reconnection

```go
cfg := rabbitmq.DefaultConfig()
cfg.TLSEnabled = true
cfg.TLSSkipVerify = false
cfg.ReconnectDelay = 2 * time.Second
cfg.MaxReconnectDelay = 120 * time.Second
cfg.ReconnectBackoff = 2.0
cfg.MaxReconnectCount = 10 // 0 = unlimited

connStr := cfg.ConnectionString() // "amqps://guest:guest@localhost:5672/"
```

## Unified Broker Interface

Both Kafka and RabbitMQ adapters implement separate Producer and Consumer types rather than a single `MessageBroker`. To use them with the higher-level `consumer` and `producer` packages, you can wrap them or use the `InMemoryBroker` as a unified broker.

The `InMemoryBroker` implements the full `MessageBroker` interface and is used in production by the consumer and producer utility packages.

```go
// Create a broker and pass it to both a SyncProducer and a ConsumerGroup.
b := broker.NewInMemoryBroker()
b.Connect(ctx)

sp := producer.NewSyncProducer(b, 10*time.Second)
cg := consumer.NewConsumerGroup("my-group", b)
```

Checking broker type at runtime:

```go
switch b.Type() {
case broker.BrokerTypeKafka:
    fmt.Println("Using Kafka")
case broker.BrokerTypeRabbitMQ:
    fmt.Println("Using RabbitMQ")
case broker.BrokerTypeInMemory:
    fmt.Println("Using InMemory")
}
```

## Retry Policies

The `consumer` package provides `RetryPolicy` with exponential backoff and a `WithRetry` handler wrapper.

### Default Retry Policy

```go
rp := consumer.DefaultRetryPolicy()
// MaxRetries: 3
// BackoffBase: 100ms
// BackoffMax: 30s
// BackoffMultiplier: 2.0
```

### Custom Retry Policy

```go
rp := &consumer.RetryPolicy{
    MaxRetries:        5,
    BackoffBase:       200 * time.Millisecond,
    BackoffMax:        60 * time.Second,
    BackoffMultiplier: 3.0,
}

// Calculate delay for attempt 2: 200ms * 3.0^2 = 1.8s
delay := rp.Delay(2)

// Check if attempt 4 should retry (true if < MaxRetries)
shouldRetry := rp.ShouldRetry(4) // true (4 < 5)
```

### Wrapping a Handler with Retry

```go
handler := func(ctx context.Context, msg *broker.Message) error {
    // Process message -- may return retryable errors.
    return processOrder(ctx, msg)
}

retryHandler := consumer.WithRetry(handler, consumer.DefaultRetryPolicy())

// Use with a consumer group.
cg := consumer.NewConsumerGroup("order-group", b)
cg.Add("orders", retryHandler)
cg.Start(ctx)
```

The `WithRetry` wrapper respects `broker.IsRetryableError`. If the error is a `BrokerError` with a retryable code (e.g., `CONNECTION_FAILED`, `CONNECTION_TIMEOUT`, `PUBLISH_TIMEOUT`, `BROKER_UNAVAILABLE`), retries proceed up to `MaxRetries`.

## Dead Letter Queues

When retries are exhausted, the `DeadLetterHandler` routes failed messages to a dedicated dead letter queue (DLQ) topic.

```go
b := broker.NewInMemoryBroker()
b.Connect(ctx)

dlh := consumer.NewDeadLetterHandler(b, "orders.dlq")

// Optional: set a callback for DLQ events.
dlh.SetOnFailure(func(ctx context.Context, msg *broker.Message, err error) {
    log.Printf("Message %s sent to DLQ: %v", msg.ID, err)
})

// When a message fails all retries, send it to the DLQ.
originalErr := fmt.Errorf("processing failed after retries")
msg := broker.NewMessage("orders", []byte(`{"order_id":"broken"}`))
if err := dlh.Handle(ctx, msg, originalErr); err != nil {
    log.Printf("Failed to send to DLQ: %v", err)
}

// The DLQ message includes metadata headers:
//   x-dlq-reason: "processing failed after retries"
//   x-dlq-original-topic: "orders"
//   x-dlq-timestamp: "2026-01-15T10:30:00Z"

fmt.Printf("DLQ message count: %d\n", dlh.Count())
fmt.Printf("DLQ topic: %s\n", dlh.DLQTopic())
```

### Combining Retry + Dead Letter

```go
b := broker.NewInMemoryBroker()
b.Connect(ctx)

dlh := consumer.NewDeadLetterHandler(b, "events.dlq")
rp := consumer.DefaultRetryPolicy()

handler := func(ctx context.Context, msg *broker.Message) error {
    err := processEvent(ctx, msg)
    if err != nil {
        return err
    }
    return nil
}

retryHandler := consumer.WithRetry(handler, rp)

// Wrap with DLQ fallback.
dlqHandler := func(ctx context.Context, msg *broker.Message) error {
    err := retryHandler(ctx, msg)
    if err != nil {
        // All retries exhausted, send to DLQ.
        return dlh.Handle(ctx, msg, err)
    }
    return nil
}

cg := consumer.NewConsumerGroup("event-processors", b)
cg.Add("events", dlqHandler)
cg.Start(ctx)
```

## Batch Processing

The `BatchConsumer` accumulates messages and processes them in batches, flushing either when the buffer reaches `batchSize` or after `flushAfter` elapses.

```go
batchHandler := func(ctx context.Context, msgs []*broker.Message) error {
    fmt.Printf("Processing batch of %d messages\n", len(msgs))
    for _, msg := range msgs {
        // Bulk insert, batch API call, etc.
        _ = msg
    }
    return nil
}

bc := consumer.NewBatchConsumer(50, 2*time.Second, batchHandler)
bc.Start(ctx)
defer bc.Stop(ctx)

// Use AsHandler() to plug into a subscription or consumer group.
sub, _ := b.Subscribe(ctx, "metrics", bc.AsHandler())

// Or add messages directly.
bc.Add(broker.NewMessage("metrics", []byte(`{"cpu":42}`)))

// Check buffer state.
fmt.Printf("Buffered: %d / %d\n", bc.BufferLen(), bc.BatchSize())

// Force an immediate flush.
bc.Flush(ctx)
```

## Async vs. Sync Producers

### SyncProducer -- Guaranteed Delivery

The `SyncProducer` blocks until the broker acknowledges the message. Supports serialization and compression.

```go
sp := producer.NewSyncProducer(b, 10*time.Second)
sp.SetSerializer(&producer.JSONSerializer{})
sp.SetCompressor(&producer.GzipCompressor{Level: gzip.BestSpeed})

// Send raw message.
msg := broker.NewMessage("events", []byte(`{"type":"click"}`))
if err := sp.Send(ctx, "events", msg); err != nil {
    log.Fatal(err)
}

// Send a Go value (auto-serialized to JSON).
if err := sp.SendValue(ctx, "events", map[string]string{"type": "purchase"}); err != nil {
    log.Fatal(err)
}

fmt.Printf("Sent: %d, Failed: %d\n", sp.SentCount(), sp.FailedCount())
```

### AsyncProducer -- Buffered, Non-blocking

The `AsyncProducer` queues messages in a buffered channel and sends them in a background goroutine.

```go
ap := producer.NewAsyncProducer(b, 5000) // 5000 message buffer
ap.SetSerializer(&producer.JSONSerializer{})
ap.Start(ctx)
defer ap.Stop()

// Non-blocking send.
msg := broker.NewMessage("logs", []byte(`{"level":"info","msg":"started"}`))
if err := ap.Send("logs", msg); err != nil {
    // Buffer is full.
    log.Printf("Send failed: %v", err)
}

// Monitor errors in a separate goroutine.
go func() {
    for err := range ap.Errors() {
        log.Printf("Async send error: %v", err)
    }
}()

fmt.Printf("Sent: %d, Failed: %d\n", ap.SentCount(), ap.FailedCount())
```

## Serialization

Three serializer implementations are provided.

### JSON (built-in)

```go
s := &producer.JSONSerializer{}
data, _ := s.Serialize(map[string]int{"count": 42})
// data = []byte(`{"count":42}`)

var result map[string]int
s.Deserialize(data, &result)
// result["count"] == 42

s.ContentType() // "application/json"
```

### Protobuf (injectable)

```go
s := &producer.ProtobufSerializer{
    MarshalFunc: func(v interface{}) ([]byte, error) {
        return proto.Marshal(v.(proto.Message))
    },
    UnmarshalFunc: func(data []byte, v interface{}) error {
        return proto.Unmarshal(data, v.(proto.Message))
    },
}
s.ContentType() // "application/x-protobuf"
```

### Avro (injectable)

```go
s := &producer.AvroSerializer{
    MarshalFunc:   myAvroMarshal,
    UnmarshalFunc: myAvroUnmarshal,
}
s.ContentType() // "application/avro"
```

## Compression

Three compressor implementations are provided.

### Gzip (built-in)

```go
c := &producer.GzipCompressor{Level: gzip.BestCompression}
compressed, _ := c.Compress([]byte("hello world"))
original, _ := c.Decompress(compressed)
c.Algorithm() // "gzip"
```

### Snappy and LZ4 (injectable)

```go
c := &producer.SnappyCompressor{
    CompressFunc:   snappy.Encode,
    DecompressFunc: snappy.Decode,
}
c.Algorithm() // "snappy"

c2 := &producer.LZ4Compressor{
    CompressFunc:   myLZ4Compress,
    DecompressFunc: myLZ4Decompress,
}
c2.Algorithm() // "lz4"
```

## Error Handling

The `pkg/broker` package provides a structured error system.

### Checking Error Types

```go
err := b.Publish(ctx, "topic", msg)

if broker.IsBrokerError(err) {
    be := broker.GetBrokerError(err)
    fmt.Printf("Code: %s, Retryable: %v\n", be.Code, be.Retryable)
}

if broker.IsRetryableError(err) {
    // Safe to retry.
}

if broker.IsConnectionError(err) {
    // Reconnect.
}

if errors.Is(err, broker.ErrNotConnected) {
    // Broker not connected.
}
```

### Creating Structured Errors

```go
err := broker.NewBrokerError(
    broker.ErrCodePublishFailed,
    "failed to publish order event",
    originalErr,
)
// err.Retryable is automatically set based on the error code.
```

## Consumer Groups

The `ConsumerGroup` manages multiple topic subscriptions through a single broker.

```go
cg := consumer.NewConsumerGroup("analytics-group", b)

cg.Add("page-views", handlePageView)
cg.Add("clicks", handleClick)
cg.Add("purchases", handlePurchase)

if err := cg.Start(ctx); err != nil {
    log.Fatal(err)
}
defer cg.Stop()

fmt.Printf("Group %s running: %v\n", cg.ID(), cg.IsRunning())
fmt.Printf("Topics: %v\n", cg.Topics())
```

## Health Checks

All `MessageBroker` implementations expose `HealthCheck` and `IsConnected`.

```go
if err := b.HealthCheck(ctx); err != nil {
    log.Printf("Broker unhealthy: %v", err)
}

if !b.IsConnected() {
    log.Println("Broker disconnected")
}
```

## Testing

All tests run without external infrastructure:

```bash
go test ./... -count=1 -race
```

Use `InMemoryBroker` in your own tests:

```go
func TestOrderProcessor(t *testing.T) {
    b := broker.NewInMemoryBroker()
    require.NoError(t, b.Connect(context.Background()))
    defer b.Close(context.Background())

    // Set up subscriptions, publish test messages, assert results.
}
```
