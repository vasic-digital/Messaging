package stress

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"digital.vasic.messaging/pkg/consumer"
	"digital.vasic.messaging/pkg/producer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryBroker_ConcurrentPublish_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	var receivedCount int64
	var mu sync.Mutex

	_, err := b.Subscribe(ctx, "stress-topic",
		func(_ context.Context, _ *broker.Message) error {
			mu.Lock()
			receivedCount++
			mu.Unlock()
			return nil
		})
	require.NoError(t, err)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			msg := broker.NewMessage("stress-topic",
				[]byte(fmt.Sprintf("msg-%d", idx)))
			err := b.Publish(ctx, "stress-topic", msg)
			assert.NoError(t, err, "publish should succeed in goroutine %d", idx)
		}(i)
	}
	wg.Wait()

	// Allow async delivery
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, int64(goroutines), receivedCount,
		"should receive all messages")
	mu.Unlock()
}

func TestInMemoryBroker_ConcurrentSubscribeUnsubscribe_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		topic := fmt.Sprintf("topic-%d", i)
		go func(t string) {
			defer wg.Done()
			_, _ = b.Subscribe(ctx, t,
				func(_ context.Context, _ *broker.Message) error { return nil })
		}(topic)
		go func(t string) {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			_ = b.Unsubscribe(t)
		}(topic)
	}
	wg.Wait()
}

func TestAsyncProducer_HighThroughput_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	ap := producer.NewAsyncProducer(b, 5000)
	ap.Start(ctx)

	const goroutines = 50
	const messagesPerGoroutine = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := broker.NewMessage("high-throughput",
					[]byte(fmt.Sprintf("msg-%d-%d", idx, j)))
				_ = ap.Send("high-throughput", msg)
			}
		}(i)
	}
	wg.Wait()

	ap.Stop()

	total := ap.SentCount() + ap.FailedCount()
	assert.Greater(t, total, int64(0), "should have processed messages")
}

func TestConsumerGroup_ConcurrentStartStop_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			cg := consumer.NewConsumerGroup(
				fmt.Sprintf("cg-%d", idx), b,
			)
			cg.Add(fmt.Sprintf("topic-%d", idx),
				func(_ context.Context, _ *broker.Message) error { return nil })

			err := cg.Start(ctx)
			if err == nil {
				_ = cg.Stop()
			}
		}(i)
	}
	wg.Wait()
}

func TestMessageClone_ConcurrentAccess_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	original := broker.NewMessageWithKey("topic", []byte("key"), []byte("value"))
	original.SetHeader("x-trace", "abc123")

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	clones := make([]*broker.Message, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			clones[idx] = original.Clone()
			// Modify clone
			clones[idx].Value = []byte(fmt.Sprintf("modified-%d", idx))
			clones[idx].SetHeader("x-trace", fmt.Sprintf("trace-%d", idx))
		}(i)
	}
	wg.Wait()

	// Original should be unmodified
	assert.Equal(t, []byte("value"), original.Value)
	assert.Equal(t, "abc123", original.GetHeader("x-trace"))

	// Each clone should have unique modification
	for i, c := range clones {
		assert.Equal(t, fmt.Sprintf("modified-%d", i), string(c.Value))
		assert.Equal(t, fmt.Sprintf("trace-%d", i), c.GetHeader("x-trace"))
	}
}

func TestSyncProducer_ConcurrentSend_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))
	defer func() { _ = b.Close(ctx) }()

	sp := producer.NewSyncProducer(b, 10*time.Second)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			msg := broker.NewMessage("sync-stress",
				[]byte(fmt.Sprintf("sync-msg-%d", idx)))
			err := sp.Send(ctx, "sync-stress", msg)
			assert.NoError(t, err,
				"sync send should succeed in goroutine %d", idx)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(goroutines), sp.SentCount())
	assert.Equal(t, int64(0), sp.FailedCount())
}

func TestGzipCompressor_ConcurrentCompress_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	comp := &producer.GzipCompressor{}

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("Data payload %d with some content", idx))
			compressed, err := comp.Compress(data)
			assert.NoError(t, err)
			decompressed, err := comp.Decompress(compressed)
			assert.NoError(t, err)
			assert.Equal(t, data, decompressed)
		}(i)
	}
	wg.Wait()
}
