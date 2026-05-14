package massdriver

import "github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"

// Re-exports of the most-used error helpers from
// [github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql]. Most
// callers only need these — they're surfaced at the top-level package
// so error handling doesn't require a separate import.
//
// The gql package remains the canonical home for the deeper API
// (ClassifyError, NewMutationFailedError, etc.); these aliases cover the
// common path.

// ErrNotFound matches when an operation's target doesn't exist (or the
// caller cannot see it). Match with [errors.Is].
var ErrNotFound = gql.ErrNotFound

// ErrUnauthenticated matches when the caller's credentials are missing,
// invalid, or expired. Match with [errors.Is].
var ErrUnauthenticated = gql.ErrUnauthenticated

// ErrForbidden matches when the caller is authenticated but lacks
// permission for this operation. Match with [errors.Is].
var ErrForbidden = gql.ErrForbidden

// MutationFailedError is the error type returned by mutation wrappers when
// the server reports `successful: false`. Use [AsMutationFailedError] to
// pull it out of an error chain, or [errors.As].
type MutationFailedError = gql.MutationFailedError

// MutationMessage is one field-scoped validation error on a
// [MutationFailedError].
type MutationMessage = gql.MutationMessage

// AsMutationFailedError unwraps err to a [*MutationFailedError] if one is in the
// chain. Returns nil and false otherwise.
func AsMutationFailedError(err error) (*MutationFailedError, bool) {
	return gql.AsMutationFailedError(err)
}
