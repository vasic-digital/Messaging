// Package broker provides core interfaces and types for the messaging system.
package broker

// PublishOptions holds options for publishing messages.
type PublishOptions struct {
	// Placeholder for future options
}

// PublishOption is a function that modifies PublishOptions.
type PublishOption func(*PublishOptions)

// SubscribeOptions holds options for subscribing to messages.
type SubscribeOptions struct {
	// Placeholder for future options
}

// SubscribeOption is a function that modifies SubscribeOptions.
type SubscribeOption func(*SubscribeOptions)
