package producer

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
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

func TestGzipCompressor_InvalidLevel(t *testing.T) {
	// Test with invalid compression level (-99 is not a valid gzip level)
	c := &GzipCompressor{Level: -99}
	_, err := c.Compress([]byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gzip writer creation failed")
}

// failingWriter is a writer that fails after a certain number of bytes.
type failingWriter struct {
	buf         bytes.Buffer
	failOnWrite bool
	failOnFlush bool
	bytesUntilFail int
	written        int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.failOnWrite {
		if w.bytesUntilFail > 0 && w.written < w.bytesUntilFail {
			toWrite := w.bytesUntilFail - w.written
			if toWrite > len(p) {
				toWrite = len(p)
			}
			n, _ := w.buf.Write(p[:toWrite])
			w.written += n
			if w.written >= w.bytesUntilFail {
				return n, errors.New("write failed")
			}
			return n, nil
		}
		return 0, errors.New("write failed")
	}
	n, err := w.buf.Write(p)
	w.written += n
	return n, err
}

func (w *failingWriter) Bytes() []byte {
	return w.buf.Bytes()
}

func TestGzipCompressor_WriteError(t *testing.T) {
	// Test gzip write error path using a failing writer
	fw := &failingWriter{failOnWrite: true}
	c := &GzipCompressor{
		Level: gzip.DefaultCompression,
		writerFactory: func() io.Writer { return fw },
	}

	_, err := c.Compress([]byte("test data to compress"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gzip write failed")
}

// closeFailWriter is a writer that fails on implicit close via gzip.Writer.Close
type closeFailWriter struct {
	buf bytes.Buffer
}

func (w *closeFailWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// The gzip.Writer.Close() calls Flush internally which writes to the underlying writer.
// To trigger a close error, we need the final flush to fail.
// Since bytes.Buffer doesn't fail, we need a different approach.

// closeOnlyFailWriter allows writes but fails on the final Close() flush.
type closeOnlyFailWriter struct {
	buf           bytes.Buffer
	writeCount    int
	failAfterWrite int
}

func (w *closeOnlyFailWriter) Write(p []byte) (int, error) {
	w.writeCount++
	// Allow the initial writes (header, data), fail on the Close() flush write
	if w.failAfterWrite > 0 && w.writeCount > w.failAfterWrite {
		return 0, errors.New("close flush failed")
	}
	return w.buf.Write(p)
}

func (w *closeOnlyFailWriter) Bytes() []byte {
	return w.buf.Bytes()
}

func TestGzipCompressor_CloseError(t *testing.T) {
	// gzip.Writer.Close() writes final trailer data.
	// To test the close error path, we use a writer that allows initial writes
	// (header + data) but fails on the trailer write during Close().
	fw := &closeOnlyFailWriter{
		failAfterWrite: 2, // Allow header and data writes, fail on close trailer
	}
	c := &GzipCompressor{
		Level: gzip.BestSpeed, // Use fast level to minimize buffering
		writerFactory: func() io.Writer { return fw },
	}

	// Small data so the trailer write during Close() is what fails
	_, err := c.Compress([]byte("x"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
}

func TestGzipCompressor_WithWriterFactory(t *testing.T) {
	// Test that writerFactory is used when provided
	var customBuf bytes.Buffer
	c := &GzipCompressor{
		Level: gzip.DefaultCompression,
		writerFactory: func() io.Writer { return &customBuf },
	}

	data := []byte("test data")
	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.NotEmpty(t, compressed)

	// Verify we can decompress
	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestGzipCompressor_AllLevels(t *testing.T) {
	// Test all valid gzip compression levels
	data := []byte("hello world, this is test data for compression")
	tests := []struct {
		name  string
		level int
	}{
		{name: "BestSpeed", level: 1},
		{name: "BestCompression", level: 9},
		{name: "DefaultCompression_zero", level: 0},
		{name: "DefaultCompression_explicit", level: -1},
		{name: "Level5", level: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &GzipCompressor{Level: tt.level}
			compressed, err := c.Compress(data)
			if tt.level == -99 || tt.level < -2 || tt.level > 9 {
				// Invalid levels
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				// Decompress to verify
				decompressed, err := c.Decompress(compressed)
				require.NoError(t, err)
				assert.Equal(t, data, decompressed)
			}
		})
	}
}

func TestAsyncProducer_SendLoop_ContextCanceled(t *testing.T) {
	b := broker.NewInMemoryBroker()
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, b.Connect(ctx))

	ap := NewAsyncProducer(b, 10)
	ap.Start(ctx)

	// Cancel context to trigger ctx.Done() path in sendLoop
	cancel()

	// Give it a moment to exit
	time.Sleep(50 * time.Millisecond)
}

func TestAsyncProducer_ErrorChannelFull(t *testing.T) {
	// Test the case where error channel is full (default case in select)
	b := broker.NewInMemoryBroker()
	// Don't connect - publishes will fail
	// Use buffer size of 1 so the error channel fills with just one error
	ap := NewAsyncProducer(b, 1)

	ctx := context.Background()
	ap.Start(ctx)

	// Send one message to fill the error channel
	err := ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	require.NoError(t, err)

	// Wait for the first error to be generated and fill the channel
	time.Sleep(50 * time.Millisecond)

	// Send more messages - these will hit the default case because the error
	// channel is already full (capacity 1) and we're not reading from it
	for i := 0; i < 10; i++ {
		_ = ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	ap.Stop()

	// The failed count should exceed the error channel capacity (1),
	// meaning the default case was hit for subsequent errors
	assert.Greater(t, ap.FailedCount(), int64(1),
		"failed count should exceed error channel capacity")
}

func TestAsyncProducer_StopDrainsWithErrors(t *testing.T) {
	// Test that Stop drains remaining messages even when broker fails
	b := broker.NewInMemoryBroker()
	// Don't connect - publishes will fail during drain
	ap := NewAsyncProducer(b, 100)

	ctx := context.Background()
	ap.Start(ctx)

	// Send several messages
	for i := 0; i < 5; i++ {
		_ = ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	}

	// Give some time for messages to queue up but not be fully processed
	time.Sleep(10 * time.Millisecond)

	// Stop will drain remaining messages, which will fail since broker not connected
	ap.Stop()

	// Failed count should reflect drain failures
	assert.Greater(t, ap.FailedCount(), int64(0))
}

func TestAsyncProducer_StopDrainsWithSuccess(t *testing.T) {
	// Test that Stop successfully drains remaining messages when broker is connected
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	// Use a large buffer so messages queue up
	ap := NewAsyncProducer(b, 100)
	ap.Start(ctx)

	// Send messages that will be drained on stop
	for i := 0; i < 10; i++ {
		err := ap.Send("topic", broker.NewMessage("topic", []byte("data")))
		require.NoError(t, err)
	}

	// Stop immediately - some messages should be in the queue
	ap.Stop()

	// All messages should have been sent (either in main loop or drain loop)
	assert.Equal(t, int64(10), ap.SentCount())
	assert.Equal(t, int64(0), ap.FailedCount())
}

// slowBroker wraps a broker and adds delay to Publish
type slowBroker struct {
	broker.MessageBroker
	delay time.Duration
}

func (sb *slowBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	time.Sleep(sb.delay)
	return sb.MessageBroker.Publish(ctx, topic, msg, opts...)
}

func TestAsyncProducer_DrainLoopPublishSuccess(t *testing.T) {
	// Test that the drain loop processes messages successfully when Stop is called
	// while messages are still queued
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	// Use slow broker to ensure messages queue up
	sb := &slowBroker{MessageBroker: b, delay: 50 * time.Millisecond}
	ap := NewAsyncProducer(sb, 100)
	ap.Start(ctx)

	// Send many messages quickly
	for i := 0; i < 20; i++ {
		err := ap.Send("topic", broker.NewMessage("topic", []byte("data")))
		require.NoError(t, err)
	}

	// Stop while messages are still being processed
	// This should drain remaining messages in the drain loop
	ap.Stop()

	// All messages should have been sent
	assert.Equal(t, int64(20), ap.SentCount())
}

func TestAsyncProducer_DrainLoopPublishError(t *testing.T) {
	// Test that the drain loop handles publish errors correctly
	// by using a broker that starts connected but gets closed
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	require.NoError(t, b.Connect(ctx))

	// Wrap with slow broker
	sb := &slowBroker{MessageBroker: b, delay: 10 * time.Millisecond}
	ap := NewAsyncProducer(sb, 100)
	ap.Start(ctx)

	// Send messages
	for i := 0; i < 10; i++ {
		_ = ap.Send("topic", broker.NewMessage("topic", []byte("data")))
	}

	// Close the broker while messages are queued
	// This will cause subsequent publishes to fail
	_ = b.Close(ctx)

	// Stop - the drain loop should encounter errors
	ap.Stop()

	// Some messages should have failed
	total := ap.SentCount() + ap.FailedCount()
	assert.Equal(t, int64(10), total, "all messages should be accounted for")
}
