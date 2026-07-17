// Package bundles provides operations for the Massdriver bundle catalog —
// the published, versioned IaC packages your organization can deploy.
//
// A [Bundle] is one specific version (e.g. `aws-aurora-postgres@1.2.3`)
// living inside an OCI repository. For repository-level operations
// (catalog metadata, attributes, listing, OCI pull/push), see
// platform/ocirepos.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.Bundles] field on the top-level SDK
// client.
package bundles

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Bundle is a published bundle version — alias of [types.Bundle].
type Bundle = types.Bundle

// Service is the receiver for bundle operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Bundles] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Get retrieves a single bundle by its composite identifier.
//
// The ID accepts:
//   - An exact version: `aws-aurora-postgres@1.2.3`
//   - A release channel: `aws-aurora-postgres@~1`, `aws-aurora-postgres@latest`
//   - Or just the repo name: `aws-aurora-postgres` (resolves to `latest`,
//     falling back to `latest+dev` if no stable release exists)
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// bundle with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Bundle, error) {
	resp, err := gen.GetBundle(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get bundle %s: %w", id, err))
	}
	if resp.Bundle.Id == "" {
		return nil, fmt.Errorf("get bundle %s: %w", id, gql.ErrNotFound)
	}
	return toBundle(resp.Bundle)
}

func toBundle(v any) (*Bundle, error) {
	b := Bundle{}
	if err := decode.Decode(v, &b); err != nil {
		return nil, fmt.Errorf("decode bundle: %w", err)
	}
	return &b, nil
}
