// Package kafka provides a Kafka adapter implementing the broker interfaces.
// It includes a Producer for publishing messages and a Consumer for
// subscribing to topics with consumer group support.
package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.messaging/pkg/broker"
	"github.com/google/uuid"
)

// PartitionStrategy defines how messages are assigned to partitions.
type PartitionStrategy int

const (
	// RoundRobin distributes messages evenly across partitions.
	RoundRobin PartitionStrategy = iota
	// Hash uses key hashing to assign partitions.
	Hash
	// Manual allows specifying the partition explicitly.
	Manual
)

// String returns the string representation of PartitionStrategy.
func (ps PartitionStrategy) String() string {
	switch ps {
	case RoundRobin:
		return "round_robin"
	case Hash:
		return "hash"
	case Manual:
		return "manual"
	default:
		return "unknown"
	}
}

// Config holds Kafka connection and behavior configuration.
type Config struct {
	// Broker connection settings.
	Brokers  []string `json:"brokers" yaml:"brokers"`
	ClientID string   `json:"client_id" yaml:"client_id"`

	// Consumer group settings.
	GroupID string   `json:"group_id" yaml:"group_id"`
	Topics  []string `json:"topics" yaml:"topics"`

	// SASL authentication.
	SASLEnabled   bool   `json:"sasl_enabled" yaml:"sasl_enabled"`
	SASLMechanism string `json:"sasl_mechanism" yaml:"sasl_mechanism"`
	SASLUsername  string `json:"sasl_username" yaml:"sasl_username"`
	SASLPassword  string `json:"sasl_password" yaml:"sasl_password"`

	// TLS configuration.
	TLSEnabled    bool        `json:"tls_enabled" yaml:"tls_enabled"`
	TLSConfig     *tls.Config `json:"-" yaml:"-"`
	TLSSkipVerify bool        `json:"tls_skip_verify" yaml:"tls_skip_verify"`

	// Producer settings.
	RequiredAcks     int               `json:"required_acks" yaml:"required_acks"`
	MaxRetries       int               `json:"max_retries" yaml:"max_retries"`
	RetryBackoff     time.Duration     `json:"retry_backoff" yaml:"retry_backoff"`
	BatchSize        int               `json:"batch_size" yaml:"batch_size"`
	BatchTimeout     time.Duration     `json:"batch_timeout" yaml:"batch_timeout"`
	MaxMessageBytes  int               `json:"max_message_bytes" yaml:"max_message_bytes"`
	CompressionCodec string            `json:"compression_codec" yaml:"compression_codec"`
	Idempotent       bool              `json:"idempotent" yaml:"idempotent"`
	Partitioning     PartitionStrategy `json:"partitioning" yaml:"partitioning"`

	// Consumer settings.
	AutoOffsetReset    string        `json:"auto_offset_reset" yaml:"auto_offset_reset"`
	EnableAutoCommit   bool          `json:"enable_auto_commit" yaml:"enable_auto_commit"`
	AutoCommitInterval time.Duration `json:"auto_commit_interval" yaml:"auto_commit_interval"`
	SessionTimeout     time.Duration `json:"session_timeout" yaml:"session_timeout"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	MaxPollRecords     int           `json:"max_poll_records" yaml:"max_poll_records"`
	FetchMinBytes      int           `json:"fetch_min_bytes" yaml:"fetch_min_bytes"`
	FetchMaxBytes      int           `json:"fetch_max_bytes" yaml:"fetch_max_bytes"`
	FetchMaxWait       time.Duration `json:"fetch_max_wait" yaml:"fetch_max_wait"`

	// Connection settings.
	DialTimeout     time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`
	MetadataRefresh time.Duration `json:"metadata_refresh" yaml:"metadata_refresh"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Brokers:            []string{"localhost:9092"},
		ClientID:           "messaging-client",
		GroupID:            "messaging-group",
		SASLMechanism:      "PLAIN",
		RequiredAcks:       -1,
		MaxRetries:         3,
		RetryBackoff:       100 * time.Millisecond,
		BatchSize:          16384,
		BatchTimeout:       10 * time.Millisecond,
		MaxMessageBytes:    1048576,
		CompressionCodec:   "lz4",
		Idempotent:         true,
		Partitioning:       RoundRobin,
		AutoOffsetReset:    "earliest",
		EnableAutoCommit:   false,
		AutoCommitInterval: 5 * time.Second,
		SessionTimeout:     30 * time.Second,
		HeartbeatInterval:  3 * time.Second,
		MaxPollRecords:     500,
		FetchMinBytes:      1,
		FetchMaxBytes:      52428800,
		FetchMaxWait:       500 * time.Millisecond,
		DialTimeout:        30 * time.Second,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		MetadataRefresh:    5 * time.Minute,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("at least one broker is required")
	}
	for _, b := range c.Brokers {
		if b == "" {
			return fmt.Errorf("broker address cannot be empty")
		}
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("batch_size must be at least 1")
	}
	if c.SASLEnabled && c.SASLUsername == "" {
		return fmt.Errorf("sasl_username is required when SASL is enabled")
	}
	return nil
}

// Producer implements the publish side of the Kafka adapter.
type Producer struct {
	config    *Config
	connected atomic.Bool
	closed    atomic.Bool
	mu        sync.RWMutex
	// In a real implementation, this would hold kafka.Writer instances.
	// This module provides the interface and config layer; actual Kafka
	// I/O is delegated to the consumer of this library.
	publishFn func(ctx context.Context, topic string, msg *broker.Message) error
}

// NewProducer creates a new Kafka producer.
func NewProducer(config *Config) *Producer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Producer{
		config: config,
	}
}

// SetPublishFunc sets the function used to publish messages.
// This allows injection of the actual Kafka client.
func (p *Producer) SetPublishFunc(
	fn func(ctx context.Context, topic string, msg *broker.Message) error,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishFn = fn
}

// Connect establishes a connection to Kafka.
func (p *Producer) Connect(ctx context.Context) error {
	if p.connected.Load() {
		return nil
	}
	if err := p.config.Validate(); err != nil {
		return broker.NewBrokerError(
			broker.ErrCodeInvalidConfig,
			"invalid kafka producer config",
			err,
		)
	}
	p.connected.Store(true)
	return nil
}

// Close closes the producer.
func (p *Producer) Close(_ context.Context) error {
	if p.closed.Swap(true) {
		return nil
	}
	p.connected.Store(false)
	return nil
}

// IsConnected returns true if the producer is connected.
func (p *Producer) IsConnected() bool {
	return p.connected.Load() && !p.closed.Load()
}

// Publish sends a message to a Kafka topic.
func (p *Producer) Publish(
	ctx context.Context, topic string, msg *broker.Message,
) error {
	if !p.IsConnected() {
		return broker.ErrNotConnected
	}
	if msg == nil {
		return broker.ErrMessageInvalid
	}
	p.mu.RLock()
	fn := p.publishFn
	p.mu.RUnlock()

	if fn != nil {
		return fn(ctx, topic, msg)
	}
	// No publish function set; this is a configuration layer only.
	return nil
}

// Config returns the producer configuration.
func (p *Producer) Config() *Config {
	return p.config
}

// Consumer implements the subscribe side of the Kafka adapter.
type Consumer struct {
	config        *Config
	connected     atomic.Bool
	closed        atomic.Bool
	mu            sync.RWMutex
	subscriptions map[string]*kafkaSubscription
	subCounter    atomic.Int64
}

// kafkaSubscription holds subscription state.
type kafkaSubscription struct {
	id       string
	topic    string
	groupID  string
	handler  broker.Handler
	cancelFn context.CancelFunc
	active   atomic.Bool
}

func (s *kafkaSubscription) Unsubscribe() error {
	if !s.active.Swap(false) {
		return nil
	}
	if s.cancelFn != nil {
		s.cancelFn()
	}
	return nil
}

func (s *kafkaSubscription) IsActive() bool {
	return s.active.Load()
}

func (s *kafkaSubscription) Topic() string {
	return s.topic
}

func (s *kafkaSubscription) ID() string {
	return s.id
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(config *Config) *Consumer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Consumer{
		config:        config,
		subscriptions: make(map[string]*kafkaSubscription),
	}
}

// Connect establishes a connection for the consumer.
func (c *Consumer) Connect(ctx context.Context) error {
	if c.connected.Load() {
		return nil
	}
	if err := c.config.Validate(); err != nil {
		return broker.NewBrokerError(
			broker.ErrCodeInvalidConfig,
			"invalid kafka consumer config",
			err,
		)
	}
	c.connected.Store(true)
	return nil
}

// Close closes the consumer and all subscriptions.
func (c *Consumer) Close(_ context.Context) error {
	if c.closed.Swap(true) {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range c.subscriptions {
		_ = sub.Unsubscribe()
	}
	c.connected.Store(false)
	return nil
}

// IsConnected returns true if the consumer is connected.
func (c *Consumer) IsConnected() bool {
	return c.connected.Load() && !c.closed.Load()
}

// Subscribe creates a subscription to a Kafka topic with consumer group.
func (c *Consumer) Subscribe(
	ctx context.Context, topic string, handler broker.Handler,
) (broker.Subscription, error) {
	if !c.IsConnected() {
		return nil, broker.ErrNotConnected
	}
	if topic == "" {
		return nil, broker.NewBrokerError(
			broker.ErrCodeSubscribeFailed,
			"topic is required",
			nil,
		)
	}
	if handler == nil {
		return nil, broker.NewBrokerError(
			broker.ErrCodeSubscribeFailed,
			"handler is required",
			nil,
		)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	subCtx, cancelFn := context.WithCancel(ctx)
	subID := fmt.Sprintf("kafka-sub-%d-%s",
		c.subCounter.Add(1), uuid.New().String()[:8])

	sub := &kafkaSubscription{
		id:       subID,
		topic:    topic,
		groupID:  c.config.GroupID,
		handler:  handler,
		cancelFn: cancelFn,
	}
	sub.active.Store(true)
	c.subscriptions[topic] = sub

	// In a real implementation, this would start a Kafka reader goroutine
	// consuming messages from the topic with the specified consumer group.
	_ = subCtx // Used by actual consumer goroutine.

	return sub, nil
}

// Unsubscribe cancels a subscription for a topic.
func (c *Consumer) Unsubscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sub, ok := c.subscriptions[topic]
	if !ok {
		return nil
	}
	err := sub.Unsubscribe()
	delete(c.subscriptions, topic)
	return err
}

// Config returns the consumer configuration.
func (c *Consumer) Config() *Config {
	return c.config
}
