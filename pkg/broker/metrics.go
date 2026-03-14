// Package broker provides core interfaces and types for the messaging system.
package broker

import (
	"sync/atomic"
	"time"
)

// BrokerMetrics holds metrics for a message broker.
type BrokerMetrics struct {
	// Connection metrics
	ConnectionAttempts  atomic.Int64 `json:"connection_attempts"`
	ConnectionSuccesses atomic.Int64 `json:"connection_successes"`
	ConnectionFailures  atomic.Int64 `json:"connection_failures"`
	CurrentConnections  atomic.Int64 `json:"current_connections"`

	// Publish metrics
	MessagesPublished atomic.Int64 `json:"messages_published"`
	PublishSuccesses  atomic.Int64 `json:"publish_successes"`
	PublishFailures   atomic.Int64 `json:"publish_failures"`
	BytesPublished    atomic.Int64 `json:"bytes_published"`

	// Subscribe/Consume metrics
	MessagesReceived    atomic.Int64 `json:"messages_received"`
	MessagesProcessed   atomic.Int64 `json:"messages_processed"`
	MessagesFailed      atomic.Int64 `json:"messages_failed"`
	BytesReceived       atomic.Int64 `json:"bytes_received"`
	ActiveSubscriptions atomic.Int64 `json:"active_subscriptions"`

	// Timestamps
	StartTime       time.Time `json:"start_time"`
	LastPublishTime time.Time `json:"last_publish_time"`
	LastReceiveTime time.Time `json:"last_receive_time"`
	LastErrorTime   time.Time `json:"last_error_time"`
}

// NewBrokerMetrics creates a new BrokerMetrics instance.
func NewBrokerMetrics() *BrokerMetrics {
	return &BrokerMetrics{
		StartTime: time.Now().UTC(),
	}
}
