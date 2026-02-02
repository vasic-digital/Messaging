package broker

import (
	"errors"
	"fmt"
)

// ErrorCode represents a messaging error code.
type ErrorCode string

const (
	// Connection errors.
	ErrCodeConnectionFailed  ErrorCode = "CONNECTION_FAILED"
	ErrCodeConnectionClosed  ErrorCode = "CONNECTION_CLOSED"
	ErrCodeConnectionTimeout ErrorCode = "CONNECTION_TIMEOUT"
	ErrCodeAuthFailed        ErrorCode = "AUTHENTICATION_FAILED"

	// Publish errors.
	ErrCodePublishFailed  ErrorCode = "PUBLISH_FAILED"
	ErrCodePublishTimeout ErrorCode = "PUBLISH_TIMEOUT"

	// Subscribe errors.
	ErrCodeSubscribeFailed  ErrorCode = "SUBSCRIBE_FAILED"
	ErrCodeConsumerCanceled ErrorCode = "CONSUMER_CANCELED"
	ErrCodeHandlerError     ErrorCode = "HANDLER_ERROR"

	// Message errors.
	ErrCodeMessageTooLarge ErrorCode = "MESSAGE_TOO_LARGE"
	ErrCodeMessageExpired  ErrorCode = "MESSAGE_EXPIRED"
	ErrCodeMessageInvalid  ErrorCode = "MESSAGE_INVALID"

	// Configuration errors.
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIG"

	// General errors.
	ErrCodeBrokerUnavailable ErrorCode = "BROKER_UNAVAILABLE"
	ErrCodeOperationCanceled ErrorCode = "OPERATION_CANCELED"
)

// Common sentinel errors for easy comparison.
var (
	ErrConnectionFailed  = errors.New("connection failed")
	ErrConnectionClosed  = errors.New("connection closed")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrNotConnected      = errors.New("not connected to broker")
	ErrPublishFailed     = errors.New("publish failed")
	ErrPublishTimeout    = errors.New("publish timeout")
	ErrSubscribeFailed   = errors.New("subscribe failed")
	ErrConsumerCanceled  = errors.New("consumer canceled")
	ErrHandlerError      = errors.New("message handler error")
	ErrMessageTooLarge   = errors.New("message too large")
	ErrMessageExpired    = errors.New("message expired")
	ErrMessageInvalid    = errors.New("invalid message")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrBrokerUnavailable = errors.New("broker unavailable")
	ErrOperationCanceled = errors.New("operation canceled")
	ErrTopicNotFound     = errors.New("topic not found")
	ErrQueueNotFound     = errors.New("queue not found")
)

// BrokerError represents a messaging broker error with detailed information.
type BrokerError struct {
	// Code is the error code.
	Code ErrorCode `json:"code"`
	// Message is the human-readable error message.
	Message string `json:"message"`
	// Cause is the underlying error.
	Cause error `json:"-"`
	// Retryable indicates if the operation can be retried.
	Retryable bool `json:"retryable"`
}

// NewBrokerError creates a new BrokerError.
func NewBrokerError(code ErrorCode, message string, cause error) *BrokerError {
	return &BrokerError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Retryable: isRetryable(code),
	}
}

// Error implements the error interface.
func (e *BrokerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *BrokerError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target.
func (e *BrokerError) Is(target error) bool {
	if t, ok := target.(*BrokerError); ok {
		return e.Code == t.Code
	}
	return errors.Is(e.Cause, target)
}

// isRetryable determines if an error code represents a retryable error.
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeConnectionFailed,
		ErrCodeConnectionTimeout,
		ErrCodePublishTimeout,
		ErrCodeBrokerUnavailable:
		return true
	default:
		return false
	}
}

// IsBrokerError checks if an error is a BrokerError.
func IsBrokerError(err error) bool {
	var brokerErr *BrokerError
	return errors.As(err, &brokerErr)
}

// GetBrokerError extracts a BrokerError from an error chain.
func GetBrokerError(err error) *BrokerError {
	var brokerErr *BrokerError
	if errors.As(err, &brokerErr) {
		return brokerErr
	}
	return nil
}

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if brokerErr := GetBrokerError(err); brokerErr != nil {
		return brokerErr.Retryable
	}
	return false
}

// IsConnectionError checks if an error is a connection error.
func IsConnectionError(err error) bool {
	if brokerErr := GetBrokerError(err); brokerErr != nil {
		switch brokerErr.Code {
		case ErrCodeConnectionFailed,
			ErrCodeConnectionClosed,
			ErrCodeConnectionTimeout,
			ErrCodeAuthFailed:
			return true
		}
	}
	return errors.Is(err, ErrConnectionFailed) ||
		errors.Is(err, ErrConnectionClosed) ||
		errors.Is(err, ErrConnectionTimeout)
}

// MultiError represents multiple errors.
type MultiError struct {
	Errors []error
}

// NewMultiError creates a new MultiError.
func NewMultiError(errs ...error) *MultiError {
	return &MultiError{Errors: errs}
}

// Error implements the error interface.
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("multiple errors (%d): %v", len(e.Errors), e.Errors[0])
}

// Add adds an error to the MultiError.
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if there are any errors.
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ErrorOrNil returns nil if there are no errors.
func (e *MultiError) ErrorOrNil() error {
	if e.HasErrors() {
		return e
	}
	return nil
}

// Unwrap returns the first error.
func (e *MultiError) Unwrap() error {
	if len(e.Errors) > 0 {
		return e.Errors[0]
	}
	return nil
}
