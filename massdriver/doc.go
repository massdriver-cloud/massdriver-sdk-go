/*
Package massdriver is the entry point for the Massdriver Go SDK.

# Quickstart

	import (
	    "context"
	    "fmt"

	    "github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	)

	func main() {
	    c, err := massdriver.NewClient()
	    if err != nil {
	        // Configuration is missing or unreadable. See "Configuration" below.
	        panic(err)
	    }

	    proj, err := c.Projects.Get(context.Background(), "ecommerce")
	    if err != nil {
	        panic(err)
	    }
	    fmt.Println(proj.Name)
	}

# Configuration

[NewClient] resolves credentials from (in order of precedence):

  - Functional options passed to [NewClient]: [WithAPIKey],
    [WithOrganizationID], [WithBaseURL], [WithProfile].
  - Environment variables: MASSDRIVER_API_KEY,
    MASSDRIVER_ORGANIZATION_ID, MASSDRIVER_URL, MASSDRIVER_PROFILE.
  - The active profile in ~/.config/massdriver/config.yaml. The
    profile is selected by MASSDRIVER_PROFILE / [WithProfile]
    (default: "default").

Common shapes:

	// Default — env + config file
	c, _ := massdriver.NewClient()

	// Explicit credentials (e.g. CI):
	c, _ := massdriver.NewClient(
	    massdriver.WithAPIKey(os.Getenv("DEPLOY_KEY")),
	    massdriver.WithOrganizationID("ecommerce"),
	)

	// Self-hosted Massdriver instance:
	c, _ := massdriver.NewClient(
	    massdriver.WithBaseURL("https://massdriver.internal.acme.com"),
	)

# Testing

Most tests should mock at the caller's own interface boundary, not at
the SDK boundary — define a small interface for the methods your code
calls and inject a fake. The SDK's domain types ([projects.Project],
[deployments.Deployment], etc.) are public and constructable.

For tests that genuinely need to exercise SDK methods end-to-end,
combine [WithGQLClient] with the
[github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest]
package's mock GraphQL client:

	mock := gqltest.NewClient(
	    gqltest.RespondWithData(map[string]any{
	        "project": map[string]any{"id": "x", "name": "X"},
	    }),
	)
	c, _ := massdriver.NewClient(
	    massdriver.WithGQLClient(mock),
	    massdriver.WithOrganizationID("test-org"),
	)
	proj, _ := c.Projects.Get(ctx, "x")

For HTTP-level integration tests — when you need to assert against the
exact request shape, exercise auth headers, or test transport
behavior — point the SDK at an [net/http/httptest.Server] via
[WithBaseURL]:

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	    // inspect r.Header, r.URL, r.Body
	    w.Write([]byte(`{"data":{"project":{"id":"x","name":"X"}}}`))
	}))
	defer srv.Close()

	c, _ := massdriver.NewClient(
	    massdriver.WithAPIKey("test-key"),
	    massdriver.WithOrganizationID("test-org"),
	    massdriver.WithBaseURL(srv.URL),
	)
	_, _ = c.Projects.Get(ctx, "x")

# Errors

Every wrapper returns errors that callers can classify with
[errors.Is] against sentinels in package gql:

  - [gql.ErrNotFound] — the requested record doesn't exist (or the
    caller cannot see it).
  - [gql.ErrUnauthenticated] — credentials missing, invalid, or expired.
  - [gql.ErrForbidden] — caller is authenticated but lacks permission.

	proj, err := c.Projects.Get(ctx, id)
	switch {
	case errors.Is(err, gql.ErrNotFound):
	    // 404 path
	case errors.Is(err, gql.ErrUnauthenticated):
	    // re-auth
	case errors.Is(err, gql.ErrForbidden):
	    // surface permission gap
	case err != nil:
	    return err
	}

For mutations that fail server-side validation (the response says
`successful: false` with field-scoped messages), use
[gql.AsMutationFailed] to inspect each [gql.MutationMessage]:

	_, err := c.Projects.Create(ctx, projects.CreateInput{ID: ""})
	if mf, ok := gql.AsMutationFailed(err); ok {
	    for _, m := range mf.Messages {
	        log.Printf("%s: %s (%s)", m.Field, m.Message, m.Code)
	    }
	}

The original transport error (a [*graphql.HTTPError] or [gqlerror.List])
remains in the error chain for callers needing more detail; reach it
with [errors.As].

# Pagination

List methods automatically follow pagination cursors and buffer every
matching record into a slice. This is convenient and correct for
small-to-medium result sets.

For potentially-unbounded queries — most often [auditlogs.Service.List]
on a large organization — prefer the iterator form:

	for ev, err := range c.AuditLogs.Iter(ctx, auditlogs.ListInput{TimeRangeStart: yesterday}) {
	    if err != nil { return err }
	    // process(ev)
	}

The iterator is lazy: it requests the next page only when the loop
asks for the next item. Break out to stop, or cancel ctx.

# Mutation results

Most Create/Update/Delete methods return the mutated record so callers
can inspect server-assigned fields (timestamps, generated IDs) without
a follow-up Get. When the server reports `successful: false`, the
returned error is a [*gql.MutationFailed] — the result pointer is nil.

# Streaming

[deployments.Service.StreamLogs] and [deployments.Service.TailLogs]
provide live access to deployment logs over an Absinthe websocket
subscription. The Service owns the socket lifetime; callers only
own the returned channel and must drain or cancel ctx to stop.

# Stability

This package is in beta. Breaking changes may land between minor
versions while the API surface stabilizes; pin a specific module
version (or a commit) if your build needs to be reproducible.

# Architecture

The SDK is organized into per-domain packages under
massdriver/platform/. Each exports a *Service that exposes that
domain's operations as methods. The top-level [Client] in this
package wires every Service onto a single struct so callers don't
have to import each platform package by hand.

The transport (HTTP, credentials, GraphQL client) lives in unexported
internal packages and is hidden from callers. Construction goes
through [NewClient] and its functional options.

The generated GraphQL code lives in massdriver/internal/gen and is
not part of the public API; queries and types may change without
notice. Build against the typed wrappers in massdriver/platform/.
*/
package massdriver
