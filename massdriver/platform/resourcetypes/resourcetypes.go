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

// Get retrieves a single resource type by its `identifier@version` ID. The
// version portion can be an exact semver, a release channel, or omitted
// entirely; the server resolves it to the best matching published version:
//
//   - `aws-iam-role@1.2.3` — that exact version
//   - `aws-iam-role@~1.2` — latest patch in 1.2.x
//   - `aws-iam-role@~1` — latest minor in 1.x.x
//   - `aws-iam-role@latest` — newest stable release
//   - `aws-iam-role@latest+dev` — newest release including dev builds
//   - `aws-iam-role` — shorthand for `latest` (falls back to `latest+dev`
//     if no stable release exists)
//
// The returned [ResourceType.ID] always carries the fully resolved version
// (e.g. `aws-iam-role@1.2.3`), also available separately as
// [ResourceType.Version].
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// matching version exists or the resource type is not accessible to the
// configured organization.
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

// Dependent is one (instance, dependency field) pair depending on a
// resource type — alias of [types.ResourceTypeDependent].
type Dependent = types.ResourceTypeDependent

// Dependents lists the instances in an environment that depend on the given
// resource type, one entry per (instance, dependency field) pair. Use it to
// see what a resource type is used by before changing or removing it.
//
// The resourceTypeID accepts a bare identifier (`aws-vpc`) or a versioned
// one (`aws-vpc@1.0.0`); a version suffix is accepted but matching currently
// resolves at the type level, since bundles reference resource types without
// a version.
func (s *Service) Dependents(ctx context.Context, environmentID, resourceTypeID string) ([]Dependent, error) {
	resp, err := gen.ListResourceTypeDependents(ctx, s.client.GQLv2, s.client.Config.OrganizationID, environmentID, resourceTypeID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list dependents of resource type %s in environment %s: %w", resourceTypeID, environmentID, err))
	}
	deps := make([]Dependent, 0, len(resp.ResourceTypeDependents))
	for _, item := range resp.ResourceTypeDependents {
		d := Dependent{}
		if derr := decode.Decode(item, &d); derr != nil {
			return nil, fmt.Errorf("decode resource type dependent: %w", derr)
		}
		deps = append(deps, d)
	}
	return deps, nil
}

func toResourceType(v any) (*ResourceType, error) {
	rt := ResourceType{}
	if err := decode.Decode(v, &rt); err != nil {
		return nil, fmt.Errorf("decode resource type: %w", err)
	}
	return &rt, nil
}
