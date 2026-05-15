// Package gqltest provides a lightweight mock graphql.Client for testing
// code that calls into massdriver/platform/* domain wrappers.
//
// The mock supports queued responses (returned in FIFO order across calls),
// canned error responses, and request recording so tests can assert which
// operation was sent and with what variables.
//
// Example:
//
//	c := gqltest.NewClient(
//	    gqltest.RespondWithData(map[string]any{
//	        "project": map[string]any{"id": "proj-1", "name": "P1"},
//	    }),
//	)
//	mdClient := &client.Client{GQLv2: c, Config: config.Config{OrganizationID: "my-org"}}
//	got, err := projects.Get(t.Context(), mdClient, "proj-1")
//
//	if got := c.Requests()[0].OpName; got != "GetProject" {
//	    t.Errorf("got OpName %q, wanted GetProject", got)
//	}
package gqltest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Client is a fake graphql.Client. Instantiate with [NewClient].
type Client struct {
	mu        sync.Mutex
	responses []Response
	requests  []Request
}

// Response is a queued mock response. Construct via [RespondWithData],
// [RespondWithJSON], or [RespondWithError].
type Response struct {
	// payload is the full GraphQL envelope ({"data": ..., "errors": ...}) that
	// will be JSON-decoded into the *graphql.Response the wrapper hands us.
	payload map[string]any
	// transportErr, if non-nil, is returned directly from MakeRequest before
	// any JSON decoding happens — simulates a network/transport failure.
	transportErr error
}

// Request is a recorded request. Variables is the request's input struct
// flattened to a map[string]any for ergonomic assertion in tests.
type Request struct {
	OpName    string
	Query     string
	Variables map[string]any
}

// NewClient returns a *Client preloaded with the given responses, returned in
// FIFO order. A test that issues more requests than queued responses will get
// an error from MakeRequest.
func NewClient(responses ...Response) *Client {
	return &Client{responses: append([]Response(nil), responses...)}
}

// MakeRequest implements graphql.Client.
func (c *Client) MakeRequest(_ context.Context, req *graphql.Request, resp *graphql.Response) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requests = append(c.requests, recordRequest(req))

	if len(c.responses) == 0 {
		return fmt.Errorf("gqltest: no response queued for op %q (request %d)", req.OpName, len(c.requests))
	}
	next := c.responses[0]
	c.responses = c.responses[1:]

	if next.transportErr != nil {
		return next.transportErr
	}

	body, err := json.Marshal(next.payload)
	if err != nil {
		return fmt.Errorf("gqltest: marshal mock payload for op %q: %w", req.OpName, err)
	}
	if err := json.NewDecoder(strings.NewReader(string(body))).Decode(resp); err != nil {
		return fmt.Errorf("gqltest: decode mock payload for op %q: %w", req.OpName, err)
	}
	if len(resp.Errors) > 0 {
		return resp.Errors
	}
	return nil
}

// Requests returns the recorded requests in the order they were issued.
func (c *Client) Requests() []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Request, len(c.requests))
	copy(out, c.requests)
	return out
}

// Pending returns the count of responses queued but not yet consumed. Useful
// in t.Cleanup to assert that every queued response was used.
func (c *Client) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.responses)
}

// RespondWithData returns a Response whose payload is `{"data": <data>}`. The
// genqlient response decoder writes the contents of "data" into the typed
// response struct that the operation function passed to MakeRequest.
func RespondWithData(data map[string]any) Response {
	return Response{payload: map[string]any{"data": data}}
}

// RespondWithJSON returns a Response whose payload is the supplied envelope
// verbatim. Use this when you need to send custom shapes — the full envelope
// (including the outer "data" key) must be provided.
func RespondWithJSON(envelope map[string]any) Response {
	return Response{payload: envelope}
}

// RespondWithError returns a Response that surfaces a GraphQL `errors` array
// to the wrapper. Each message becomes a separate gqlerror entry.
func RespondWithError(messages ...string) Response {
	errs := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		errs = append(errs, map[string]any{"message": m})
	}
	return Response{payload: map[string]any{"errors": errs}}
}

// RespondWithTransportError returns a Response that simulates a non-GraphQL
// error from the transport layer (e.g. network failure). The error is
// returned directly from MakeRequest with no JSON decoding.
func RespondWithTransportError(err error) Response {
	return Response{transportErr: err}
}

// recordRequest captures a *graphql.Request as a Request, flattening the
// typed Variables struct to a map[string]any so tests can inspect inputs
// without importing the (unexported) generated input types.
func recordRequest(req *graphql.Request) Request {
	rec := Request{OpName: req.OpName, Query: req.Query}
	if req.Variables == nil {
		return rec
	}
	b, err := json.Marshal(req.Variables)
	if err != nil {
		// Fall through with empty map; an unmarshalable Variables value is
		// effectively a test bug, and surfacing it via assertion failure on
		// the recorded shape beats panicking from here.
		return rec
	}
	_ = json.Unmarshal(b, &rec.Variables)
	return rec
}

// Compile-time assertion that *Client satisfies graphql.Client.
var _ graphql.Client = (*Client)(nil)

// Compile-time assertion that gqlerror.List satisfies error, which we rely on
// in MakeRequest to return GraphQL errors. (gqlerror.List has Error() built-in.)
var _ error = gqlerror.List{}
