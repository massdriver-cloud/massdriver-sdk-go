package gql_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestMutationFailedError_Error_NoMessages(t *testing.T) {
	err := gql.NewMutationFailedError("create project", nil)
	want := "unable to create project"
	if err.Error() != want {
		t.Errorf("got %q, wanted %q", err.Error(), want)
	}
}

func TestMutationFailedError_Error_WithMessages(t *testing.T) {
	err := gql.NewMutationFailedError("create project", []gql.MutationMessage{
		{Code: "required", Field: "name", Message: "name is required"},
		{Code: "format", Field: "id", Message: "id must be lowercase"},
	})
	want := "unable to create project:\n  - name is required\n  - id must be lowercase"
	if err.Error() != want {
		t.Errorf("got %q, wanted %q", err.Error(), want)
	}
}

func TestAsMutationFailedError_DirectAndWrapped(t *testing.T) {
	original := gql.NewMutationFailedError("delete project", []gql.MutationMessage{
		{Field: "id", Message: "not found"},
	})

	mf, ok := gql.AsMutationFailedError(original)
	if !ok {
		t.Fatal("expected to unwrap MutationFailedError directly")
	}
	if mf.Op != "delete project" {
		t.Errorf("got Op %q, wanted delete project", mf.Op)
	}

	wrapped := fmt.Errorf("api call failed: %w", original)
	mf2, ok := gql.AsMutationFailedError(wrapped)
	if !ok {
		t.Fatal("expected to unwrap MutationFailedError through fmt.Errorf wrapping")
	}
	if mf2.Op != "delete project" {
		t.Errorf("got Op %q, wanted delete project", mf2.Op)
	}
}

func TestAsMutationFailedError_OtherError(t *testing.T) {
	if _, ok := gql.AsMutationFailedError(errors.New("network failure")); ok {
		t.Error("expected ok=false for non-MutationFailedError error")
	}
	if _, ok := gql.AsMutationFailedError(nil); ok {
		t.Error("expected ok=false for nil error")
	}
}

func TestClassifyError_HTTPStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, gql.ErrUnauthenticated},
		{http.StatusForbidden, gql.ErrForbidden},
		{http.StatusNotFound, gql.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			httpErr := &graphql.HTTPError{StatusCode: tc.status}
			wrapped := fmt.Errorf("get project p1: %w", httpErr)

			got := gql.ClassifyError(wrapped)
			if !errors.Is(got, tc.want) {
				t.Errorf("errors.Is(%v, %v) = false, want true", got, tc.want)
			}

			// The original transport error must remain reachable.
			var unwrappedHTTP *graphql.HTTPError
			if !errors.As(got, &unwrappedHTTP) {
				t.Error("classified error should still expose *graphql.HTTPError via errors.As")
			}
		})
	}
}

func TestClassifyError_GraphQLExtensionCodes(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{"UNAUTHENTICATED", gql.ErrUnauthenticated},
		{"FORBIDDEN", gql.ErrForbidden},
		{"PERMISSION_DENIED", gql.ErrForbidden},
		{"NOT_FOUND", gql.ErrNotFound},
		{"not_found", gql.ErrNotFound}, // case-insensitive
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			gqlErrs := gqlerror.List{
				&gqlerror.Error{
					Message:    "denied",
					Extensions: map[string]any{"code": tc.code},
				},
			}
			got := gql.ClassifyError(fmt.Errorf("list projects: %w", gqlErrs))
			if !errors.Is(got, tc.want) {
				t.Errorf("code %q: errors.Is = false, want true", tc.code)
			}
		})
	}
}

func TestClassifyError_NoMatch(t *testing.T) {
	// Unrecognized errors pass through unchanged — no false positives.
	plain := errors.New("connection refused")
	if got := gql.ClassifyError(plain); got != plain {
		t.Errorf("ClassifyError mutated unrelated error: got %v, want %v", got, plain)
	}
	if got := gql.ClassifyError(nil); got != nil {
		t.Errorf("ClassifyError(nil) = %v, want nil", got)
	}
}
