package benchmark

import (
	"context"
	"fmt"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"digital.vasic.messaging/pkg/consumer"
	"digital.vasic.messaging/pkg/producer"
)

func BenchmarkNewMessage(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	value := []byte("benchmark message payload")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = broker.NewMessage("bench-topic", value)
	}
}

func BenchmarkMessage_Clone(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	msg := broker.NewMessageWithKey("topic", []byte("key"), []byte("value"))
	msg.SetHeader("x-header-1", "value1")
	msg.SetHeader("x-header-2", "value2")
	msg.SetHeader("x-header-3", "value3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.Clone()
	}
}

func BenchmarkMessage_SetGetHeader(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	msg := broker.NewMessage("topic", []byte("data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.SetHeader("key", "value")
		_ = msg.GetHeader("key")
	}
}

func BenchmarkInMemoryBroker_PublishSubscribe(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	br := broker.NewInMemoryBroker()
	ctx := context.Background()
	_ = br.Connect(ctx)
	defer func() { _ = br.Close(ctx) }()

	_, _ = br.Subscribe(ctx, "bench-topic",
		func(_ context.Context, _ *broker.Message) error { return nil })

	msg := broker.NewMessage("bench-topic", []byte("benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = br.Publish(ctx, "bench-topic", msg)
	}
}

func BenchmarkSyncProducer_Send(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	br := broker.NewInMemoryBroker()
	ctx := context.Background()
	_ = br.Connect(ctx)
	defer func() { _ = br.Close(ctx) }()

	sp := producer.NewSyncProducer(br, 30*time.Second)
	msg := broker.NewMessage("bench-topic", []byte("sync benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sp.Send(ctx, "bench-topic", msg)
	}
}

func BenchmarkRetryPolicy_Delay(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	policy := consumer.DefaultRetryPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Delay(i % 5)
	}
}

func BenchmarkGzipCompressor_Compress(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	comp := &producer.GzipCompressor{}
	data := []byte("This is benchmark data for compression testing. " +
		"It contains enough text to make compression meaningful.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = comp.Compress(data)
	}
}

func BenchmarkGzipCompressor_Decompress(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	comp := &producer.GzipCompressor{}
	data := []byte("This is benchmark data for decompression testing. " +
		"It needs to be compressed first.")
	compressed, _ := comp.Compress(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = comp.Decompress(compressed)
	}
}

func BenchmarkBrokerConfig_Validate(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	cfg := &broker.Config{
		Brokers:  []string{"localhost:9092", "localhost:9093"},
		ClientID: "bench-client",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

func BenchmarkJSONSerializer_SerializeDeserialize(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	serializer := &producer.JSONSerializer{}
	data := map[string]interface{}{
		"id":      "msg-123",
		"payload": "benchmark data",
		"count":   42,
		"tags":    []string{"bench", "test"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := serializer.Serialize(data)
		var decoded map[string]interface{}
		_ = serializer.Deserialize(bytes, &decoded)
	}
}

func BenchmarkBatchConsumer_Add(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	bc := consumer.NewBatchConsumer(1000, time.Minute,
		func(_ context.Context, _ []*broker.Message) error { return nil })

	msgs := make([]*broker.Message, b.N)
	for i := range msgs {
		msgs[i] = broker.NewMessage("bench", []byte(fmt.Sprintf("msg-%d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bc.Add(msgs[i])
	}
}
