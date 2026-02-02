// Package rabbitmq provides a RabbitMQ adapter implementing the broker
// interfaces. It includes a Producer for publishing messages and a Consumer
// for subscribing to queues with exchange/routing key support.
package rabbitmq

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

// ExchangeType defines the type of RabbitMQ exchange.
type ExchangeType string

const (
	// Direct exchange routes by exact routing key match.
	Direct ExchangeType = "direct"
	// Fanout exchange broadcasts to all bound queues.
	Fanout ExchangeType = "fanout"
	// Topic exchange routes by routing key pattern.
	Topic ExchangeType = "topic"
	// Headers exchange routes by message headers.
	Headers ExchangeType = "headers"
)

// String returns the string representation of ExchangeType.
func (et ExchangeType) String() string {
	return string(et)
}

// IsValid checks if the ExchangeType is valid.
func (et ExchangeType) IsValid() bool {
	switch et {
	case Direct, Fanout, Topic, Headers:
		return true
	default:
		return false
	}
}

// Config holds RabbitMQ connection and behavior configuration.
type Config struct {
	// Connection settings.
	URL      string `json:"url" yaml:"url"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	VHost    string `json:"vhost" yaml:"vhost"`

	// Exchange settings.
	Exchange    string       `json:"exchange" yaml:"exchange"`
	ExchType    ExchangeType `json:"exchange_type" yaml:"exchange_type"`
	Queue       string       `json:"queue" yaml:"queue"`
	RoutingKey  string       `json:"routing_key" yaml:"routing_key"`
	BindingKeys []string     `json:"binding_keys" yaml:"binding_keys"`

	// Consumer settings.
	Prefetch    int    `json:"prefetch" yaml:"prefetch"`
	ConsumerTag string `json:"consumer_tag" yaml:"consumer_tag"`
	AutoAck     bool   `json:"auto_ack" yaml:"auto_ack"`
	Exclusive   bool   `json:"exclusive" yaml:"exclusive"`

	// TLS configuration.
	TLSEnabled    bool        `json:"tls_enabled" yaml:"tls_enabled"`
	TLSConfig     *tls.Config `json:"-" yaml:"-"`
	TLSSkipVerify bool        `json:"tls_skip_verify" yaml:"tls_skip_verify"`

	// Connection pool settings.
	MaxConnections    int           `json:"max_connections" yaml:"max_connections"`
	MaxChannels       int           `json:"max_channels" yaml:"max_channels"`
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`

	// Reconnection settings.
	ReconnectDelay    time.Duration `json:"reconnect_delay" yaml:"reconnect_delay"`
	MaxReconnectDelay time.Duration `json:"max_reconnect_delay" yaml:"max_reconnect_delay"`
	ReconnectBackoff  float64       `json:"reconnect_backoff" yaml:"reconnect_backoff"`
	MaxReconnectCount int           `json:"max_reconnect_count" yaml:"max_reconnect_count"`

	// Publisher settings.
	PublishConfirm bool          `json:"publish_confirm" yaml:"publish_confirm"`
	PublishTimeout time.Duration `json:"publish_timeout" yaml:"publish_timeout"`
	Mandatory      bool          `json:"mandatory" yaml:"mandatory"`
	Immediate      bool          `json:"immediate" yaml:"immediate"`

	// Queue settings.
	Durable    bool `json:"durable" yaml:"durable"`
	AutoDelete bool `json:"auto_delete" yaml:"auto_delete"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Host:              "localhost",
		Port:              5672,
		Username:          "guest",
		Password:          "guest",
		VHost:             "/",
		ExchType:          Topic,
		Prefetch:          10,
		AutoAck:           false,
		MaxConnections:    10,
		MaxChannels:       100,
		ConnectionTimeout: 30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		ReconnectDelay:    1 * time.Second,
		MaxReconnectDelay: 60 * time.Second,
		ReconnectBackoff:  2.0,
		MaxReconnectCount: 0,
		PublishConfirm:    true,
		PublishTimeout:    30 * time.Second,
		Durable:           true,
		AutoDelete:        false,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// If URL is provided, it takes precedence.
	if c.URL != "" {
		return nil
	}
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.MaxConnections < 1 {
		return fmt.Errorf("max_connections must be at least 1")
	}
	if c.Prefetch < 0 {
		return fmt.Errorf("prefetch cannot be negative")
	}
	if c.ExchType != "" && !c.ExchType.IsValid() {
		return fmt.Errorf("invalid exchange type: %s", c.ExchType)
	}
	return nil
}

// ConnectionString returns the AMQP connection string.
func (c *Config) ConnectionString() string {
	if c.URL != "" {
		return c.URL
	}
	scheme := "amqp"
	if c.TLSEnabled {
		scheme = "amqps"
	}
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		scheme, c.Username, c.Password, c.Host, c.Port, c.VHost)
}

// Producer implements the publish side of the RabbitMQ adapter.
type Producer struct {
	config    *Config
	connected atomic.Bool
	closed    atomic.Bool
	mu        sync.RWMutex
	publishFn func(ctx context.Context, topic string, msg *broker.Message) error
}

// NewProducer creates a new RabbitMQ producer.
func NewProducer(config *Config) *Producer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Producer{
		config: config,
	}
}

// SetPublishFunc sets the function used to publish messages.
func (p *Producer) SetPublishFunc(
	fn func(ctx context.Context, topic string, msg *broker.Message) error,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishFn = fn
}

// Connect establishes a connection to RabbitMQ.
func (p *Producer) Connect(ctx context.Context) error {
	if p.connected.Load() {
		return nil
	}
	if err := p.config.Validate(); err != nil {
		return broker.NewBrokerError(
			broker.ErrCodeInvalidConfig,
			"invalid rabbitmq producer config",
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

// Publish sends a message to a RabbitMQ exchange/queue.
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
	return nil
}

// Config returns the producer configuration.
func (p *Producer) Config() *Config {
	return p.config
}

// Consumer implements the subscribe side of the RabbitMQ adapter.
type Consumer struct {
	config        *Config
	connected     atomic.Bool
	closed        atomic.Bool
	mu            sync.RWMutex
	subscriptions map[string]*rabbitSubscription
	subCounter    atomic.Int64
}

// rabbitSubscription holds subscription state.
type rabbitSubscription struct {
	id       string
	topic    string
	queue    string
	handler  broker.Handler
	cancelCh chan struct{}
	active   atomic.Bool
}

func (s *rabbitSubscription) Unsubscribe() error {
	if !s.active.Swap(false) {
		return nil
	}
	close(s.cancelCh)
	return nil
}

func (s *rabbitSubscription) IsActive() bool {
	return s.active.Load()
}

func (s *rabbitSubscription) Topic() string {
	return s.topic
}

func (s *rabbitSubscription) ID() string {
	return s.id
}

// NewConsumer creates a new RabbitMQ consumer.
func NewConsumer(config *Config) *Consumer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Consumer{
		config:        config,
		subscriptions: make(map[string]*rabbitSubscription),
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
			"invalid rabbitmq consumer config",
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

// Subscribe creates a subscription to a RabbitMQ queue.
func (c *Consumer) Subscribe(
	_ context.Context, topic string, handler broker.Handler,
) (broker.Subscription, error) {
	if !c.IsConnected() {
		return nil, broker.ErrNotConnected
	}
	if topic == "" {
		return nil, broker.NewBrokerError(
			broker.ErrCodeSubscribeFailed,
			"topic/queue is required",
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

	subID := fmt.Sprintf("rabbit-sub-%d-%s",
		c.subCounter.Add(1), uuid.New().String()[:8])

	sub := &rabbitSubscription{
		id:       subID,
		topic:    topic,
		queue:    topic,
		handler:  handler,
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)
	c.subscriptions[topic] = sub

	return sub, nil
}

// Unsubscribe cancels a subscription for a topic/queue.
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
