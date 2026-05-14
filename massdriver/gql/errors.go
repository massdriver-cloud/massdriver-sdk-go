package gql

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Sentinel errors callers can match with [errors.Is] to classify common
// failure modes without parsing error strings or HTTP status codes.
//
// Returned errors are wrapped so the original transport error
// (typically [*graphql.HTTPError] or [gqlerror.List]) remains accessible
// via [errors.As] for callers needing more detail.
var (
	// ErrNotFound indicates the requested record does not exist (or the
	// caller cannot see it). Returned by Get methods when the API yields
	// HTTP 404 or a GraphQL `NOT_FOUND` extension code, and when a
	// nullable Get target query resolves to null.
	ErrNotFound = errors.New("not found")

	// ErrUnauthenticated indicates the caller's credentials are missing,
	// invalid, or expired. Re-authenticate before retrying.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrForbidden indicates the caller is authenticated but lacks
	// permission for this operation. Re-authenticating will not help —
	// the call requires different credentials or a policy change.
	ErrForbidden = errors.New("forbidden")
)

// ClassifyError inspects err's chain for a transport-level error
// ([*graphql.HTTPError] or [gqlerror.List]) and returns an error that
// matches the appropriate sentinel ([ErrNotFound], [ErrUnauthenticated],
// [ErrForbidden]) under [errors.Is]. The returned error's Error()
// string is the original message — no duplicate "not found" lines.
//
// Recognized signals:
//   - HTTP 401 → [ErrUnauthenticated]
//   - HTTP 403 → [ErrForbidden]
//   - HTTP 404 → [ErrNotFound]
//   - GraphQL `extensions.code`: `UNAUTHENTICATED` → [ErrUnauthenticated],
//     `FORBIDDEN` / `PERMISSION_DENIED` → [ErrForbidden],
//     `NOT_FOUND` → [ErrNotFound]
//
// When nothing matches, err is returned unchanged.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}
	if sentinel := classify(err); sentinel != nil {
		return &classifiedError{err: err, sentinel: sentinel}
	}
	return err
}

// classifiedError carries the original transport error alongside a
// sentinel for [errors.Is] matching. Error() renders only the original
// message so output doesn't duplicate the sentinel's text (e.g. the
// trailing "not found" that an [errors.Join] chain would print on its
// own line).
type classifiedError struct {
	err      error
	sentinel error
}

func (e *classifiedError) Error() string { return e.err.Error() }
func (e *classifiedError) Unwrap() error { return e.err }
func (e *classifiedError) Is(target error) bool {
	return target == e.sentinel
}

func classify(err error) error {
	var httpErr *graphql.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusUnauthorized:
			return ErrUnauthenticated
		case http.StatusForbidden:
			return ErrForbidden
		case http.StatusNotFound:
			return ErrNotFound
		}
	}
	var gqlErrs gqlerror.List
	if errors.As(err, &gqlErrs) {
		for _, e := range gqlErrs {
			if e == nil {
				continue
			}
			if code, _ := e.Extensions["code"].(string); code != "" {
				switch strings.ToUpper(code) {
				case "UNAUTHENTICATED":
					return ErrUnauthenticated
				case "FORBIDDEN", "PERMISSION_DENIED":
					return ErrForbidden
				case "NOT_FOUND":
					return ErrNotFound
				}
			}
		}
	}
	return nil
}

// MutationFailed is returned by a mutation wrapper when the GraphQL response
// reports `successful: false`. It carries the per-field validation messages
// the API produced.
//
// Callers can type-assert (or `errors.As`) to inspect the messages
// programmatically — for example, to surface field-level errors back into a
// form UI or a Terraform diagnostic.
type MutationFailed struct {
	// Op is a short human-readable label for the operation that failed
	// (e.g. "create project"). It is rendered as the prefix of Error().
	Op string

	// Messages are the per-field validation messages returned by the API.
	// Empty when the server reported `successful: false` without populating
	// `messages` (rare but legal).
	Messages []MutationMessage
}

// MutationMessage is a single field-scoped validation error from a mutation.
// Mirrors the GraphQL `ValidationMessage` type.
type MutationMessage struct {
	Code    string
	Field   string
	Message string
}

// Error implements the error interface, formatting each message on its own
// indented line under the operation prefix.
func (e *MutationFailed) Error() string {
	if len(e.Messages) == 0 {
		return "unable to " + e.Op
	}
	var sb strings.Builder
	sb.WriteString("unable to ")
	sb.WriteString(e.Op)
	sb.WriteString(":")
	for _, msg := range e.Messages {
		sb.WriteString("\n  - ")
		sb.WriteString(msg.Message)
	}
	return sb.String()
}

// NewMutationFailed builds a *MutationFailed for the given operation. The
// `messages` slice is copied so callers can safely reuse the source.
func NewMutationFailed(op string, messages []MutationMessage) error {
	return &MutationFailed{Op: op, Messages: append([]MutationMessage(nil), messages...)}
}

// AsMutationFailed unwraps err to a *MutationFailed if one is in the chain.
// Returns nil and false otherwise.
func AsMutationFailed(err error) (*MutationFailed, bool) {
	var mf *MutationFailed
	if errors.As(err, &mf) {
		return mf, true
	}
	return nil, false
}

// validationMessage is the structural shape every genqlient-generated
// payload-messages type satisfies (the methods come from genqlient's
// auto-emitted Get* accessors).
type validationMessage interface {
	GetCode() string
	GetField() string
	GetMessage() string
}

// CheckMutation builds a [*MutationFailed] from a mutation payload. Returns
// nil when `successful` is true; otherwise wraps the validation messages.
//
// The accessor methods are pointer-receiver in genqlient's output, so the
// generic takes an element type M plus a constraint that *M satisfies the
// validation-message accessor interface. Go infers both from the call site:
//
//	if err := gql.CheckMutation("create project", resp.CreateProject.Successful, resp.CreateProject.Messages); err != nil {
//	    return nil, err
//	}
func CheckMutation[M any, PM interface {
	*M
	validationMessage
}](op string, successful bool, msgs []M) error {
	if successful {
		return nil
	}
	out := make([]MutationMessage, 0, len(msgs))
	for i := range msgs {
		var p PM = &msgs[i]
		out = append(out, MutationMessage{
			Code:    p.GetCode(),
			Field:   p.GetField(),
			Message: p.GetMessage(),
		})
	}
	return NewMutationFailed(op, out)
}
