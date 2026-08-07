// Package resourcetypes provides read operations for Massdriver resource
// types — the contracts behind the connection system.
//
// A [ResourceType] defines what kind of infrastructure a resource
// represents. Every dependency a bundle declares and every resource a
// bundle produces references a resource type; that shared contract is what
// makes bundles composable. The catalog includes both public types provided
// by Massdriver (e.g. `aws-iam-role`) and private types defined by your
// organization.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.ResourceTypes] field on the top-level
// SDK client.
package resourcetypes

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// ResourceType is a Massdriver resource type — alias of [types.ResourceType].
type ResourceType = types.ResourceType

// Service is the receiver for resource-type operations. Construct with
// [New]; for the typical case you'll use the
// [massdriver.Client.ResourceTypes] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Get retrieves a single resource type by its identifier.
//
// The ID accepts:
//   - A bare identifier: `aws-iam-role` (resolves to the latest published
//     version)
//   - A specific published version: `aws-iam-role@1.2.3`
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// resource type with the given ID exists or is accessible to the configured
// organization.
func (s *Service) Get(ctx context.Context, id string) (*ResourceType, error) {
	resp, err := gen.GetResourceType(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get resource type %s: %w", id, err))
	}
	if resp.ResourceType.Id == "" {
		return nil, fmt.Errorf("get resource type %s: %w", id, gql.ErrNotFound)
	}
	return toResourceType(resp.ResourceType)
}

func toResourceType(v any) (*ResourceType, error) {
	rt := ResourceType{}
	if err := decode.Decode(v, &rt); err != nil {
		return nil, fmt.Errorf("decode resource type: %w", err)
	}
	return &rt, nil
}
