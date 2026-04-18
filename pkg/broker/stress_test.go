// SPDX-License-Identifier: Apache-2.0
//
// Stress tests for digital.vasic.messaging/pkg/broker, modelled on the
// canonical P3 stress-test template in
// digital.vasic.buildcheck/pkg/buildcheck/stress_test.go.
package broker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	stressGoroutines   = 8
	stressIterations   = 200
	stressMaxWallClock = 15 * time.Second
)

// TestStress_InMemoryBroker_ConcurrentPubSub stresses the in-memory
// broker with many concurrent publishers and subscribers.
func TestStress_InMemoryBroker_ConcurrentPubSub(t *testing.T) {
	b := NewInMemoryBroker()
	require.NoError(t, b.Connect(context.Background()))
	defer func() { _ = b.Close(context.Background()) }()

	var received atomic.Int64
	sub, err := b.Subscribe(context.Background(), "stress.topic",
		func(_ context.Context, _ *Message) error {
			received.Add(1)
			return nil
		})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }()

	startGoroutines := runtime.NumGoroutine()
	var wg sync.WaitGroup
	var pubErrors atomic.Int64
	deadline := time.Now().Add(stressMaxWallClock)

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				m := NewMessage("stress.topic", []byte(fmt.Sprintf("p%d-%d", id, j)))
				if err := b.Publish(context.Background(), "stress.topic", m); err != nil {
					pubErrors.Add(1)
				}
			}
		}(g)
	}
	wg.Wait()

	// Give the broker a moment to drain pending deliveries.
	time.Sleep(200 * time.Millisecond)
	runtime.Gosched()

	assert.Equal(t, int64(0), pubErrors.Load(), "Publish should not error under load")
	assert.Greater(t, received.Load(), int64(0), "subscriber must receive some messages")

	endGoroutines := runtime.NumGoroutine()
	assert.LessOrEqual(t, endGoroutines-startGoroutines, 4,
		"goroutine leak: worker count grew by %d", endGoroutines-startGoroutines)
}

// BenchmarkStress_InMemoryBroker_Publish establishes a publish-path
// throughput baseline.
func BenchmarkStress_InMemoryBroker_Publish(b *testing.B) {
	br := NewInMemoryBroker()
	_ = br.Connect(context.Background())
	defer func() { _ = br.Close(context.Background()) }()
	m := NewMessage("bench", []byte("payload"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = br.Publish(context.Background(), "bench", m)
	}
}
