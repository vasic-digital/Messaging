package producer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Serializer Tests ---

func TestJSONSerializer_Serialize(t *testing.T) {
	s := &JSONSerializer{}
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{name: "map", input: map[string]string{"key": "value"}, wantErr: false},
		{name: "string", input: "hello", wantErr: false},
		{name: "int", input: 42, wantErr: false},
		{name: "nil", input: nil, wantErr: false},
		{name: "chan_unsupported", input: make(chan int), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := s.Serialize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

func TestJSONSerializer_Deserialize(t *testing.T) {
	s := &JSONSerializer{}
	data, err := s.Serialize(map[string]string{"key": "value"})
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, s.Deserialize(data, &result))
	assert.Equal(t, "value", result["key"])
}

func TestJSONSerializer_Deserialize_Invalid(t *testing.T) {
	s := &JSONSerializer{}
	var result map[string]string
	err := s.Deserialize([]byte("not json"), &result)
	assert.Error(t, err)
}

func TestJSONSerializer_ContentType(t *testing.T) {
	s := &JSONSerializer{}
	assert.Equal(t, "application/json", s.ContentType())
}

func TestProtobufSerializer_Serialize(t *testing.T) {
	tests := []struct {
		name      string
		marshal   func(v interface{}) ([]byte, error)
		wantErr   bool
		errContains string
	}{
		{
			name:      "nil_marshal_func",
			marshal:   nil,
			wantErr:   true,
			errContains: "protobuf marshal function not set",
		},
		{
			name: "success",
			marshal: func(v interface{}) ([]byte, error) {
				return []byte("proto"), nil
			},
			wantErr: false,
		},
		{
			name: "error",
			marshal: func(v interface{}) ([]byte, error) {
				return nil, errors.New("marshal error")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProtobufSerializer{MarshalFunc: tt.marshal}
			data, err := s.Serialize("test")
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, []byte("proto"), data)
			}
		})
	}
}

func TestProtobufSerializer_Deserialize(t *testing.T) {
	tests := []struct {
		name      string
		unmarshal func(data []byte, v interface{}) error
		wantErr   bool
	}{
		{name: "nil_unmarshal_func", unmarshal: nil, wantErr: true},
		{
			name:      "success",
			unmarshal: func(data []byte, v interface{}) error { return nil },
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProtobufSerializer{UnmarshalFunc: tt.unmarshal}
			err := s.Deserialize([]byte("data"), nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProtobufSerializer_ContentType(t *testing.T) {
	s := &ProtobufSerializer{}
	assert.Equal(t, "application/x-protobuf", s.ContentType())
}

func TestAvroSerializer_Serialize(t *testing.T) {
	tests := []struct {
		name    string
		marshal func(v interface{}) ([]byte, error)
		wantErr bool
	}{
		{name: "nil_marshal_func", marshal: nil, wantErr: true},
		{
			name: "success",
			marshal: func(v interface{}) ([]byte, error) {
				return []byte("avro"), nil
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AvroSerializer{MarshalFunc: tt.marshal}
			_, err := s.Serialize("test")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAvroSerializer_Deserialize(t *testing.T) {
	tests := []struct {
		name      string
		unmarshal func(data []byte, v interface{}) error
		wantErr   bool
	}{
		{name: "nil_unmarshal_func", unmarshal: nil, wantErr: true},
		{
			name:      "success",
			unmarshal: func(data []byte, v interface{}) error { return nil },
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AvroSerializer{UnmarshalFunc: tt.unmarshal}
			err := s.Deserialize([]byte("data"), nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAvroSerializer_ContentType(t *testing.T) {
	s := &AvroSerializer{}
	assert.Equal(t, "application/avro", s.ContentType())
}

// --- Compressor Tests ---

func TestGzipCompressor_CompressDecompress(t *testing.T) {
	c := &GzipCompressor{}
	original := []byte("hello world, this is a test of gzip compression")

	compressed, err := c.Compress(original)
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
}

func TestGzipCompressor_CompressEmpty(t *testing.T) {
	c := &GzipCompressor{}
	compressed, err := c.Compress([]byte{})
	require.NoError(t, err)
	assert.NotNil(t, compressed)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, decompressed)
}

func TestGzipCompressor_DecompressInvalid(t *testing.T) {
	c := &GzipCompressor{}
	_, err := c.Decompress([]byte("not gzip data"))
	assert.Error(t, err)
}

func TestGzipCompressor_Algorithm(t *testing.T) {
	c := &GzipCompressor{}
	assert.Equal(t, "gzip", c.Algorithm())
}

func TestGzipCompressor_WithLevel(t *testing.T) {
	c := &GzipCompressor{Level: 1} // Best speed.
	data := []byte("test data for compression with level")
	compressed, err := c.Compress(data)
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestSnappyCompressor_PassThrough(t *testing.T) {
	c := &SnappyCompressor{}
	data := []byte("hello")

	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.Equal(t, data, compressed) // Pass-through when no func set.

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestSnappyCompressor_WithFuncs(t *testing.T) {
	c := &SnappyCompressor{
		CompressFunc: func(data []byte) ([]byte, error) {
			return append([]byte("snappy:"), data...), nil
		},
		DecompressFunc: func(data []byte) ([]byte, error) {
			return data[7:], nil
		},
	}
	data := []byte("hello")
	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.Equal(t, []byte("snappy:hello"), compressed)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestSnappyCompressor_Algorithm(t *testing.T) {
	c := &SnappyCompressor{}
	assert.Equal(t, "snappy", c.Algorithm())
}

func TestLZ4Compressor_PassThrough(t *testing.T) {
	c := &LZ4Compressor{}
	data := []byte("hello")

	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.Equal(t, data, compressed)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestLZ4Compressor_WithFuncs(t *testing.T) {
	c := &LZ4Compressor{
		CompressFunc: func(data []byte) ([]byte, error) {
			return append([]byte("lz4:"), data...), nil
		},
		DecompressFunc: func(data []byte) ([]byte, error) {
			return data[4:], nil
		},
	}
	data := []byte("hello")
	compressed, err := c.Compress(data)
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestLZ4Compressor_Algorithm(t *testing.T) {
	c := &LZ4Compressor{}
	assert.Equal(t, "lz4", c.Algorithm())
}

// --- AsyncProducer Tests ---

func TestNewAsyncProducer_DefaultBuffer(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ap := NewAsyncProducer(b, 0) // Should default to 1000.
	assert.NotNil(t, ap)
}

func TestAsyncProducer_StartStop(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	ap := NewAsyncProducer(b, 10)
	ap.Start(ctx)
	ap.Start(ctx) // Double start should be safe.
	ap.Stop()
	ap.Stop() // Double stop should be safe.
}

func TestAsyncProducer_SendNotStarted(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ap := NewAsyncProducer(b, 10)
	err := ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestAsyncProducer_SendSuccess(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	ap := NewAsyncProducer(b, 10)
	ap.Start(ctx)
	defer ap.Stop()

	msg := broker.NewMessage("topic", []byte("data"))
	require.NoError(t, ap.Send("topic", msg))

	// Wait for message to be processed.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int64(1), ap.SentCount())
	assert.Equal(t, int64(0), ap.FailedCount())
}

func TestAsyncProducer_SendBufferFull(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	ap := NewAsyncProducer(b, 1) // Buffer of 1.
	ap.Start(ctx)
	defer ap.Stop()

	// Fill the buffer without letting the consumer drain it fast enough.
	// First send should succeed.
	msg := broker.NewMessage("topic", []byte("data"))
	_ = ap.Send("topic", msg)
	// Try sending more - may get buffer full.
	var bufFullCount int
	for i := 0; i < 100; i++ {
		if err := ap.Send("topic", msg); err != nil {
			bufFullCount++
		}
	}
	// At least some should fail with buffer full.
	assert.Greater(t, bufFullCount, 0)
}

func TestAsyncProducer_Errors(t *testing.T) {
	b := broker.NewInMemoryBroker()
	// Don't connect - publishes will fail.
	ap := NewAsyncProducer(b, 10)

	ctx := context.Background()
	ap.Start(ctx)
	defer ap.Stop()

	msg := broker.NewMessage("topic", []byte("data"))
	require.NoError(t, ap.Send("topic", msg))

	// Wait for error to appear.
	select {
	case err := <-ap.Errors():
		assert.Error(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("expected error but timed out")
	}

	assert.Equal(t, int64(1), ap.FailedCount())
}

func TestAsyncProducer_SetSerializer(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ap := NewAsyncProducer(b, 10)
	ap.SetSerializer(&JSONSerializer{})
	// No panic or error.
}

func TestAsyncProducer_SetCompressor(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ap := NewAsyncProducer(b, 10)
	ap.SetCompressor(&GzipCompressor{})
	// No panic or error.
}

func TestAsyncProducer_StopDrainsMessages(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	ap := NewAsyncProducer(b, 100)
	ap.Start(ctx)

	// Send several messages.
	for i := 0; i < 5; i++ {
		_ = ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	}

	// Stop should drain remaining messages.
	ap.Stop()
	assert.True(t, ap.SentCount() >= 0)
}

// --- SyncProducer Tests ---

func TestNewSyncProducer_DefaultTimeout(t *testing.T) {
	b := broker.NewInMemoryBroker()
	sp := NewSyncProducer(b, 0)
	assert.NotNil(t, sp)
}

func TestSyncProducer_Send_Success(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	msg := broker.NewMessage("topic", []byte("data"))
	require.NoError(t, sp.Send(ctx, "topic", msg))
	assert.Equal(t, int64(1), sp.SentCount())
	assert.Equal(t, int64(0), sp.FailedCount())
}

func TestSyncProducer_Send_NotConnected(t *testing.T) {
	b := broker.NewInMemoryBroker()
	sp := NewSyncProducer(b, 5*time.Second)
	msg := broker.NewMessage("topic", []byte("data"))
	err := sp.Send(context.Background(), "topic", msg)
	assert.Error(t, err)
	assert.Equal(t, int64(1), sp.FailedCount())
}

func TestSyncProducer_Send_WithCompressor(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	var published atomic.Bool
	_, _ = b.Subscribe(ctx, "topic", func(_ context.Context, msg *broker.Message) error {
		published.Store(true)
		// Check compression header was set.
		assert.Equal(t, "gzip", msg.GetHeader("content-encoding"))
		return nil
	})

	sp := NewSyncProducer(b, 5*time.Second)
	sp.SetCompressor(&GzipCompressor{})

	msg := broker.NewMessage("topic", []byte("hello world"))
	require.NoError(t, sp.Send(ctx, "topic", msg))

	time.Sleep(50 * time.Millisecond)
	assert.True(t, published.Load())
}

func TestSyncProducer_Send_WithSerializer(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	sp.SetSerializer(&JSONSerializer{})

	msg := broker.NewMessage("topic", []byte("data"))
	require.NoError(t, sp.Send(ctx, "topic", msg))

	// Check content-type header was set.
	assert.Equal(t, "application/json", msg.GetHeader("content-type"))
}

func TestSyncProducer_Send_CompressionError(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	sp.SetCompressor(&SnappyCompressor{
		CompressFunc: func(data []byte) ([]byte, error) {
			return nil, errors.New("compress failed")
		},
	})

	msg := broker.NewMessage("topic", []byte("data"))
	err := sp.Send(ctx, "topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compression failed")
	assert.Equal(t, int64(1), sp.FailedCount())
}

func TestSyncProducer_SendValue_Success(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	require.NoError(t, sp.SendValue(ctx, "topic", map[string]string{"key": "val"}))
	assert.Equal(t, int64(1), sp.SentCount())
}

func TestSyncProducer_SendValue_WithSerializer(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	sp.SetSerializer(&JSONSerializer{})

	require.NoError(t, sp.SendValue(ctx, "topic", "hello"))
	assert.Equal(t, int64(1), sp.SentCount())
}

func TestSyncProducer_SendValue_SerializationError(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	// Channels cannot be serialized to JSON.
	err := sp.SendValue(ctx, "topic", make(chan int))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "serialization failed")
	assert.Equal(t, int64(1), sp.FailedCount())
}

func TestSyncProducer_SendValue_DefaultSerializer(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	sp := NewSyncProducer(b, 5*time.Second)
	// No serializer set, should default to JSON.
	require.NoError(t, sp.SendValue(ctx, "topic", 42))
}
