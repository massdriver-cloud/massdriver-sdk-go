// Package server provides metadata about the Massdriver server you're
// connected to — its version, operating mode, and the authentication
// methods available on its login screen.
//
// The underlying GraphQL `server` query does not require authentication,
// so [Service.Get] can be called before the user signs in. Bootstrap
// flows commonly call this first to determine which login methods to
// render.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.Server] field on the top-level SDK
// client.
package server

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Server is server-level metadata — alias of [types.Server].
type Server = types.Server

// SsoProvider is one configured SSO provider — alias of [types.SsoProvider].
type SsoProvider = types.SsoProvider

// EmailAuthMethod is one email-based auth method — alias of
// [types.EmailAuthMethod].
type EmailAuthMethod = types.EmailAuthMethod

// Service is the receiver for server-metadata operations. Construct with
// [New]; for the typical case you'll use the [massdriver.Client.Server]
// field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Get retrieves the connected server's metadata. No authentication is
// required.
func (s *Service) Get(ctx context.Context) (*Server, error) {
	resp, err := gen.GetServer(ctx, s.client.GQLv2)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get server: %w", err))
	}
	srv := Server{}
	if derr := decode.Decode(resp.Server, &srv); derr != nil {
		return nil, fmt.Errorf("decode server: %w", derr)
	}
	return &srv, nil
}
