// Round-123 §11.4 — i18n seam wiring for the broker package.
//
// Per CONST-046 every user-visible string surface MUST be reachable through
// the i18n.Translator seam; the NoopTranslator default preserves legacy
// behaviour (key-verbatim) so existing callers stay actionable until a
// consuming project wires a real translator.
//
// Per CONST-051(B) this file injects no project-specific context — the
// package-level translator is a package-private variable swapped via
// SetTranslator, never reached for via a hardcoded parent-tree path.
//
// CONST-050(A) compliance: this is production code (no `_test.go` suffix);
// it imports the i18n seam, never any unit-test mock. The Translator
// default is the production-safe NoopTranslator declared inside the i18n
// package itself.
package broker

import (
	"sync/atomic"

	"digital.vasic.messaging/pkg/i18n"
)

// pkgTranslator stores the active broker-wide translator behind an
// atomic.Value so consumers may swap it at any time without explicit
// locking on the read path. The zero value is i18n.NoopTranslator{} per
// resetTranslator below.
var pkgTranslator atomic.Value

func init() {
	resetTranslator()
}

// resetTranslator restores the production-safe default. Exposed only
// for tests via the package-internal pkgResetTranslatorForTest alias in
// the test file.
func resetTranslator() {
	pkgTranslator.Store(i18n.Translator(i18n.NoopTranslator{}))
}

// SetTranslator wires the i18n translator used by the broker package's
// user-visible string surfaces (error messages, log lines emitted via
// the i18n seam). Passing nil restores the NoopTranslator default —
// callers can use this to revert after a temporary override.
func SetTranslator(t i18n.Translator) {
	if t == nil {
		resetTranslator()
		return
	}
	pkgTranslator.Store(t)
}

// tr returns the active translator. Always non-nil; falls back to
// NoopTranslator if the atomic.Value is somehow empty.
func tr() i18n.Translator {
	v := pkgTranslator.Load()
	if v == nil {
		return i18n.NoopTranslator{}
	}
	t, ok := v.(i18n.Translator)
	if !ok || t == nil {
		return i18n.NoopTranslator{}
	}
	return t
}
