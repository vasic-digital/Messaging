// Package producer provides utilities for message production including
// async and sync producers, serializers, and compressors.
package producer

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.messaging/pkg/broker"
)

// Serializer defines the interface for message serialization.
type Serializer interface {
	// Serialize converts a value to bytes.
	Serialize(v interface{}) ([]byte, error)
	// Deserialize converts bytes back to a value.
	Deserialize(data []byte, v interface{}) error
	// ContentType returns the MIME content type.
	ContentType() string
}

// JSONSerializer implements Serializer using JSON encoding.
type JSONSerializer struct{}

// Serialize converts a value to JSON bytes.
func (s *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Deserialize converts JSON bytes back to a value.
func (s *JSONSerializer) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// ContentType returns the JSON content type.
func (s *JSONSerializer) ContentType() string {
	return "application/json"
}

// ProtobufSerializer implements Serializer for protocol buffer messages.
// The actual marshal/unmarshal functions must be provided since this module
// does not depend on protobuf.
type ProtobufSerializer struct {
	MarshalFunc   func(v interface{}) ([]byte, error)
	UnmarshalFunc func(data []byte, v interface{}) error
}

// Serialize converts a protobuf message to bytes.
func (s *ProtobufSerializer) Serialize(v interface{}) ([]byte, error) {
	if s.MarshalFunc == nil {
		return nil, fmt.Errorf("protobuf marshal function not set")
	}
	return s.MarshalFunc(v)
}

// Deserialize converts bytes back to a protobuf message.
func (s *ProtobufSerializer) Deserialize(data []byte, v interface{}) error {
	if s.UnmarshalFunc == nil {
		return fmt.Errorf("protobuf unmarshal function not set")
	}
	return s.UnmarshalFunc(data, v)
}

// ContentType returns the protobuf content type.
func (s *ProtobufSerializer) ContentType() string {
	return "application/x-protobuf"
}

// AvroSerializer implements Serializer for Avro-encoded messages.
// The actual marshal/unmarshal functions must be provided since this module
// does not depend on Avro libraries.
type AvroSerializer struct {
	MarshalFunc   func(v interface{}) ([]byte, error)
	UnmarshalFunc func(data []byte, v interface{}) error
}

// Serialize converts an Avro message to bytes.
func (s *AvroSerializer) Serialize(v interface{}) ([]byte, error) {
	if s.MarshalFunc == nil {
		return nil, fmt.Errorf("avro marshal function not set")
	}
	return s.MarshalFunc(v)
}

// Deserialize converts bytes back to an Avro message.
func (s *AvroSerializer) Deserialize(data []byte, v interface{}) error {
	if s.UnmarshalFunc == nil {
		return fmt.Errorf("avro unmarshal function not set")
	}
	return s.UnmarshalFunc(data, v)
}

// ContentType returns the Avro content type.
func (s *AvroSerializer) ContentType() string {
	return "application/avro"
}

// Compressor defines the interface for message compression.
type Compressor interface {
	// Compress compresses data.
	Compress(data []byte) ([]byte, error)
	// Decompress decompresses data.
	Decompress(data []byte) ([]byte, error)
	// Algorithm returns the compression algorithm name.
	Algorithm() string
}

// GzipCompressor implements Compressor using gzip.
type GzipCompressor struct {
	Level int // gzip.DefaultCompression if zero
}

// Compress compresses data using gzip.
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	level := c.Level
	if level == 0 {
		level = gzip.DefaultCompression
	}
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, fmt.Errorf("gzip writer creation failed: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip close failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress decompresses gzip data.
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader creation failed: %w", err)
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}

// Algorithm returns "gzip".
func (c *GzipCompressor) Algorithm() string {
	return "gzip"
}

// SnappyCompressor implements Compressor using Snappy-compatible encoding.
// Since this module avoids external compression deps, it provides a simple
// pass-through that can be replaced with actual Snappy when needed.
type SnappyCompressor struct {
	CompressFunc   func(data []byte) ([]byte, error)
	DecompressFunc func(data []byte) ([]byte, error)
}

// Compress compresses data using Snappy.
func (c *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	if c.CompressFunc == nil {
		return data, nil // Pass-through if no function set.
	}
	return c.CompressFunc(data)
}

// Decompress decompresses Snappy data.
func (c *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	if c.DecompressFunc == nil {
		return data, nil // Pass-through if no function set.
	}
	return c.DecompressFunc(data)
}

// Algorithm returns "snappy".
func (c *SnappyCompressor) Algorithm() string {
	return "snappy"
}

// LZ4Compressor implements Compressor using LZ4-compatible encoding.
// Since this module avoids external compression deps, it provides a simple
// pass-through that can be replaced with actual LZ4 when needed.
type LZ4Compressor struct {
	CompressFunc   func(data []byte) ([]byte, error)
	DecompressFunc func(data []byte) ([]byte, error)
}

// Compress compresses data using LZ4.
func (c *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	if c.CompressFunc == nil {
		return data, nil
	}
	return c.CompressFunc(data)
}

// Decompress decompresses LZ4 data.
func (c *LZ4Compressor) Decompress(data []byte) ([]byte, error) {
	if c.DecompressFunc == nil {
		return data, nil
	}
	return c.DecompressFunc(data)
}

// Algorithm returns "lz4".
func (c *LZ4Compressor) Algorithm() string {
	return "lz4"
}

// AsyncProducer buffers messages and sends them asynchronously.
type AsyncProducer struct {
	broker     broker.MessageBroker
	serializer Serializer
	compressor Compressor
	bufferSize int
	msgCh      chan pendingMessage
	errCh      chan error
	stopCh     chan struct{}
	running    atomic.Bool
	sent       atomic.Int64
	failed     atomic.Int64
	wg         sync.WaitGroup
}

type pendingMessage struct {
	topic string
	msg   *broker.Message
}

// NewAsyncProducer creates a new asynchronous producer.
func NewAsyncProducer(
	b broker.MessageBroker, bufferSize int,
) *AsyncProducer {
	if bufferSize < 1 {
		bufferSize = 1000
	}
	return &AsyncProducer{
		broker:     b,
		bufferSize: bufferSize,
		msgCh:      make(chan pendingMessage, bufferSize),
		errCh:      make(chan error, bufferSize),
		stopCh:     make(chan struct{}),
	}
}

// SetSerializer sets the serializer for the producer.
func (ap *AsyncProducer) SetSerializer(s Serializer) {
	ap.serializer = s
}

// SetCompressor sets the compressor for the producer.
func (ap *AsyncProducer) SetCompressor(c Compressor) {
	ap.compressor = c
}

// Start starts the background send loop.
func (ap *AsyncProducer) Start(ctx context.Context) {
	if ap.running.Swap(true) {
		return
	}
	ap.wg.Add(1)
	go ap.sendLoop(ctx)
}

// sendLoop processes pending messages.
func (ap *AsyncProducer) sendLoop(ctx context.Context) {
	defer ap.wg.Done()
	for {
		select {
		case pm := <-ap.msgCh:
			if err := ap.broker.Publish(ctx, pm.topic, pm.msg); err != nil {
				ap.failed.Add(1)
				select {
				case ap.errCh <- err:
				default:
				}
			} else {
				ap.sent.Add(1)
			}
		case <-ap.stopCh:
			// Drain remaining messages.
			for {
				select {
				case pm := <-ap.msgCh:
					if err := ap.broker.Publish(ctx, pm.topic, pm.msg); err != nil {
						ap.failed.Add(1)
					} else {
						ap.sent.Add(1)
					}
				default:
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// Send queues a message for asynchronous sending.
func (ap *AsyncProducer) Send(topic string, msg *broker.Message) error {
	if !ap.running.Load() {
		return fmt.Errorf("async producer not started")
	}
	select {
	case ap.msgCh <- pendingMessage{topic: topic, msg: msg}:
		return nil
	default:
		return fmt.Errorf("send buffer full (capacity: %d)", ap.bufferSize)
	}
}

// Errors returns the error channel for receiving send errors.
func (ap *AsyncProducer) Errors() <-chan error {
	return ap.errCh
}

// Stop stops the producer and waits for pending messages to drain.
func (ap *AsyncProducer) Stop() {
	if !ap.running.Swap(false) {
		return
	}
	close(ap.stopCh)
	ap.wg.Wait()
}

// SentCount returns the number of successfully sent messages.
func (ap *AsyncProducer) SentCount() int64 {
	return ap.sent.Load()
}

// FailedCount returns the number of failed messages.
func (ap *AsyncProducer) FailedCount() int64 {
	return ap.failed.Load()
}

// SyncProducer provides guaranteed delivery by waiting for publish
// acknowledgment before returning.
type SyncProducer struct {
	broker     broker.MessageBroker
	serializer Serializer
	compressor Compressor
	timeout    time.Duration
	sent       atomic.Int64
	failed     atomic.Int64
}

// NewSyncProducer creates a new synchronous producer.
func NewSyncProducer(b broker.MessageBroker, timeout time.Duration) *SyncProducer {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SyncProducer{
		broker:  b,
		timeout: timeout,
	}
}

// SetSerializer sets the serializer for the producer.
func (sp *SyncProducer) SetSerializer(s Serializer) {
	sp.serializer = s
}

// SetCompressor sets the compressor for the producer.
func (sp *SyncProducer) SetCompressor(c Compressor) {
	sp.compressor = c
}

// Send publishes a message and waits for acknowledgment.
func (sp *SyncProducer) Send(
	ctx context.Context, topic string, msg *broker.Message,
) error {
	ctx, cancel := context.WithTimeout(ctx, sp.timeout)
	defer cancel()

	// Apply serialization and compression to the message value if configured.
	if sp.serializer != nil && msg.Value != nil {
		msg.SetHeader("content-type", sp.serializer.ContentType())
	}
	if sp.compressor != nil && msg.Value != nil {
		compressed, err := sp.compressor.Compress(msg.Value)
		if err != nil {
			sp.failed.Add(1)
			return fmt.Errorf("compression failed: %w", err)
		}
		msg.Value = compressed
		msg.SetHeader("content-encoding", sp.compressor.Algorithm())
	}

	if err := sp.broker.Publish(ctx, topic, msg); err != nil {
		sp.failed.Add(1)
		return err
	}
	sp.sent.Add(1)
	return nil
}

// SendValue serializes a value and publishes it.
func (sp *SyncProducer) SendValue(
	ctx context.Context, topic string, value interface{},
) error {
	s := sp.serializer
	if s == nil {
		s = &JSONSerializer{}
	}
	data, err := s.Serialize(value)
	if err != nil {
		sp.failed.Add(1)
		return fmt.Errorf("serialization failed: %w", err)
	}
	msg := broker.NewMessage(topic, data)
	msg.SetHeader("content-type", s.ContentType())
	return sp.Send(ctx, topic, msg)
}

// SentCount returns the number of successfully sent messages.
func (sp *SyncProducer) SentCount() int64 {
	return sp.sent.Load()
}

// FailedCount returns the number of failed messages.
func (sp *SyncProducer) FailedCount() int64 {
	return sp.failed.Load()
}
