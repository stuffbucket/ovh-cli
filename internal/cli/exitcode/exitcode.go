// Package exitcode is the canonical exit-code and error-code registry.
//
// Authoritative tables live in PRD-01 (docs/specs/01-cli-ux.md) §Canonical
// exit-code registry and §Canonical error-code registry. Constants here MUST
// match those tables row-for-row; phase 2 adds a generator that asserts so.
package exitcode

import "errors"

// Process exit codes. PRD-01 §Canonical exit-code registry.
const (
	OK           = 0
	Generic      = 1
	Usage        = 2
	AuthFailure  = 3
	MissingInput = 4
	NotFound     = 5
	Conflict     = 6
	RateLimited  = 7
	Network      = 8
)

// JSON error.code strings emitted in machine-readable error output.
// PRD-01 §Canonical error-code registry.
const (
	CodeAuthRequired       = "AUTH_REQUIRED"
	CodeAuthInvalid        = "AUTH_INVALID"
	CodeMissingInput       = "MISSING_INPUT"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodePreconditionFailed = "PRECONDITION_FAILED"
	CodeRateLimited        = "RATE_LIMITED"
	CodeNetwork            = "NETWORK"
	CodeUsage              = "USAGE"
	CodeInternal           = "INTERNAL"
)

// Coded is implemented by error wrappers that carry an explicit exit code.
type Coded interface {
	error
	ExitCode() int
}

// From extracts the exit code from err. Unwrapped errors map to Generic.
// Returns OK on nil.
func From(err error) int {
	if err == nil {
		return OK
	}
	var c Coded
	if errors.As(err, &c) {
		return c.ExitCode()
	}
	return Generic
}
