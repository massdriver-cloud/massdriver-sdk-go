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
[gql.AsMutationFailedError] to inspect each [gql.MutationMessage]:

	_, err := c.Projects.Create(ctx, projects.CreateInput{ID: ""})
	if mf, ok := gql.AsMutationFailedError(err); ok {
	    for _, m := range mf.Messages {
	        log.Printf("%s: %s (%s)", m.Field, m.Message, m.Code)
	    }
	}

The original transport error (a [*graphql.HTTPError] or [gqlerror.List])
remains in the error chain for callers needing more detail; reach it
with [errors.As].

# Pagination

Each list endpoint exposes two methods. Iter returns a lazy iterator
([iter.Seq2]) that fetches pages on demand — the recommended default,
since ranging it streams results without buffering the whole match set,
and breaking out of the loop stops requesting further pages:

	for ev, err := range c.AuditLogs.Iter(ctx, auditlogs.ListInput{TimeRangeStart: yesterday}) {
	    if err != nil { return err }
	    // process(ev)
	}

ListPage fetches a single page and returns opaque cursors
([types.Page.Next]/Previous) for stateless pagination — hand Next back as
the next request's ListInput.After to advance, e.g. from a web or CLI
handler that can't hold an iterator across requests.

When you genuinely want every match buffered into one slice — appropriate
for bounded sets — wrap an iterator with [types.Collect]:

	all, err := types.Collect(c.Projects.Iter(ctx, projects.ListInput{}))

# Mutation results

Most Create/Update/Delete methods return the mutated record so callers
can inspect server-assigned fields (timestamps, generated IDs) without
a follow-up Get. When the server reports `successful: false`, the
returned error is a [*gql.MutationFailedError] — the result pointer is nil.

# Streaming

The SDK exposes two flavors of live data over Absinthe WebSocket
subscriptions.

Deployment logs:

  - [deployments.Service.StreamLogs] yields one
    [deployments.LogBatch] per provisioner flush.
  - [deployments.Service.TailLogs] is the high-level form that folds
    in backfill, live tailing, and terminal-state detection.

Lifecycle events — each Service's StreamEvents method opens a typed
[types.Event] channel; callers type-assert to a concrete variant
([*types.InstanceEvent], [*types.AlarmEvent], etc.):

  - [organizations.Service.StreamEvents] — projects created, OCI
    repositories created, bundle versions published.
  - [projects.Service.StreamEvents] — the project itself,
    environments, and blueprint (components, links).
  - [environments.Service.StreamEvents] — environment defaults,
    instances, connections, alarms, and deployments within an
    environment.
  - [instances.Service.StreamEvents] — instance config changes,
    incoming connections, alarms, and deployments for a single
    instance.
  - [deployments.Service.StreamEvents] — lifecycle transitions for
    a single deployment.

Every Stream* method requires a personal-access-token credential and
returns [streaming.ErrRequiresPAT] otherwise. The Service owns the
socket lifetime; callers own the returned channel and must drain or
cancel ctx to stop.

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
