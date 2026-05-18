// Round-123 §11.4 — i18n message helpers for the consumer package.
//
// The consumer package reuses the broker package's translator (set via
// broker.SetTranslator) to keep callers from juggling two translator
// registries. Per CONST-051(B) no project-specific context is baked in;
// per CONST-046 every user-visible string surface is routed through the
// i18n seam; per CONST-050(A) this is production code that imports no
// unit-test mocks.
package consumer

import (
	"fmt"

	"digital.vasic.messaging/pkg/i18n"
)

// translatorProvider is the indirection used by the consumer package
// to resolve the active translator. Defaults to NoopTranslator; tests
// may swap it via setTranslatorProviderForTest in the _test.go file.
//
// The provider deliberately returns a brand-new NoopTranslator each
// call when no override is wired — this matches the broker package's
// fallback behaviour and keeps the helpers race-free.
var translatorProvider = func() i18n.Translator {
	return i18n.NoopTranslator{}
}

// renderAlreadyRunning produces the user-visible "already running" message
// for ConsumerGroup.Start. NoopTranslator returns the key verbatim, in
// which case the helper falls back to the legacy formatted message so
// existing wire-evidence parses unchanged.
func renderAlreadyRunning(id string) string {
	rendered := translatorProvider().T(
		"messaging_broker_consumer_group_already_running",
		map[string]any{"id": id},
	)
	if rendered == "messaging_broker_consumer_group_already_running" {
		return fmt.Sprintf("consumer group %s is already running", id)
	}
	return rendered
}

// renderSubscribeFailure produces the user-visible prefix for a
// per-topic subscribe failure inside ConsumerGroup.Start. The caller
// concatenates the wrapped error with %w to preserve the chain.
// NoopTranslator returns the key verbatim, falling back to the legacy
// "failed to subscribe to <topic>" phrasing.
func renderSubscribeFailure(topic string) string {
	rendered := translatorProvider().T(
		"messaging_broker_subscribe_failed",
		map[string]any{"topic": topic},
	)
	if rendered == "messaging_broker_subscribe_failed" {
		return fmt.Sprintf("failed to subscribe to %s", topic)
	}
	return rendered
}
