// Package broker provides core interfaces and types for the messaging system.
// It defines the MessageBroker contract, Message struct, and common types
// used across all broker implementations.
package broker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BrokerType represents the type of message broker.
type BrokerType string

const (
	// BrokerTypeKafka represents Apache Kafka message broker.
	BrokerTypeKafka BrokerType = "kafka"
	// BrokerTypeRabbitMQ represents RabbitMQ message broker.
	BrokerTypeRabbitMQ BrokerType = "rabbitmq"
	// BrokerTypeInMemory represents in-memory message broker for testing.
	BrokerTypeInMemory BrokerType = "inmemory"
)

// String returns the string representation of BrokerType.
func (bt BrokerType) String() string {
	return string(bt)
}

// IsValid checks if the BrokerType is valid.
func (bt BrokerType) IsValid() bool {
	switch bt {
	case BrokerTypeKafka, BrokerTypeRabbitMQ, BrokerTypeInMemory:
		return true
	default:
		return false
	}
}

// MessageBroker defines the core interface for message brokers.
type MessageBroker interface {
	// Connect establishes a connection to the broker.
	Connect(ctx context.Context) error
	// Close closes the connection to the broker.
	Close(ctx context.Context) error
	// HealthCheck checks if the broker is healthy.
	HealthCheck(ctx context.Context) error
	// IsConnected returns true if connected to the broker.
	IsConnected() bool
	// Publish sends a message to a topic or queue.
	Publish(ctx context.Context, topic string, msg *Message) error
	// Subscribe creates a subscription to a topic or queue.
	Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error)
	// Unsubscribe cancels a subscription for a topic.
	Unsubscribe(topic string) error
	// Type returns the type of this broker.
	Type() BrokerType
}

// Handler is a function that processes messages.
type Handler func(ctx context.Context, msg *Message) error

// Subscription represents an active subscription to a topic or queue.
type Subscription interface {
	// Unsubscribe cancels the subscription.
	Unsubscribe() error
	// IsActive returns true if the subscription is still active.
	IsActive() bool
	// Topic returns the subscribed topic or queue name.
	Topic() string
	// ID returns the subscription identifier.
	ID() string
}

// Message represents a message in the messaging system.
type Message struct {
	// ID is the unique identifier for the message.
	ID string `json:"id"`
	// Topic is the topic or queue the message belongs to.
	Topic string `json:"topic"`
	// Key is an optional key for partitioning.
	Key []byte `json:"key,omitempty"`
	// Value is the message content as raw bytes.
	Value []byte `json:"value"`
	// Headers contains message metadata.
	Headers map[string]string `json:"headers,omitempty"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewMessage creates a new message with default values.
func NewMessage(topic string, value []byte) *Message {
	return &Message{
		ID:        uuid.New().String(),
		Topic:     topic,
		Value:     value,
		Headers:   make(map[string]string),
		Timestamp: time.Now().UTC(),
	}
}

// NewMessageWithKey creates a new message with a specific key.
func NewMessageWithKey(topic string, key, value []byte) *Message {
	msg := NewMessage(topic, value)
	msg.Key = key
	return msg
}

// NewMessageWithID creates a new message with a specific ID.
func NewMessageWithID(id, topic string, value []byte) *Message {
	msg := NewMessage(topic, value)
	msg.ID = id
	return msg
}

// SetHeader sets a header value.
func (m *Message) SetHeader(key, value string) *Message {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
	return m
}

// GetHeader gets a header value.
func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// WithKey sets the message key.
func (m *Message) WithKey(key []byte) *Message {
	m.Key = key
	return m
}

// WithStringKey sets the message key from a string.
func (m *Message) WithStringKey(key string) *Message {
	m.Key = []byte(key)
	return m
}

// Clone creates a deep copy of the message.
func (m *Message) Clone() *Message {
	clone := &Message{
		ID:        m.ID,
		Topic:     m.Topic,
		Key:       make([]byte, len(m.Key)),
		Value:     make([]byte, len(m.Value)),
		Timestamp: m.Timestamp,
	}
	copy(clone.Key, m.Key)
	copy(clone.Value, m.Value)
	if m.Headers != nil {
		clone.Headers = make(map[string]string, len(m.Headers))
		for k, v := range m.Headers {
			clone.Headers[k] = v
		}
	}
	return clone
}

// MarshalJSON implements json.Marshaler.
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal((*Alias)(m))
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	return json.Unmarshal(data, (*Alias)(m))
}

// Config holds common configuration for message brokers.
type Config struct {
	// Brokers is the list of broker addresses.
	Brokers []string `json:"brokers" yaml:"brokers"`
	// ClientID is the client identifier.
	ClientID string `json:"client_id" yaml:"client_id"`
}

// Validate validates the broker configuration.
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrInvalidConfig
	}
	for _, b := range c.Brokers {
		if b == "" {
			return ErrInvalidConfig
		}
	}
	if c.ClientID == "" {
		return ErrInvalidConfig
	}
	return nil
}

// DefaultConfig returns a default broker configuration.
func DefaultConfig() *Config {
	return &Config{
		Brokers:  []string{"localhost:9092"},
		ClientID: "messaging-client",
	}
}
