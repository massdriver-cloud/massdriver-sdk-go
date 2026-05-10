// Package bundles provides operations for the Massdriver bundle catalog —
// the published, versioned IaC packages your organization can deploy.
//
// A [Bundle] is one specific version (e.g. `aws-aurora-postgres@1.2.3`)
// living inside an OCI repository. For repository-level operations
// (catalog metadata, attributes, OCI pull/push), see platform/ocirepos.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.Bundles] field on the top-level SDK
// client.
package bundles

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
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

// SortField is the field a [Service.List] result can be ordered by.
type SortField string

const (
	SortByName      SortField = "NAME"
	SortByCreatedAt SortField = "CREATED_AT"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.List] call. Zero value lists every bundle the
// caller can see, sorted by name ascending.
type ListInput struct {
	// OciRepoName limits results to a specific repository (e.g. "aws-rds").
	OciRepoName string
	// ResourceType limits to bundles that PRODUCE a resource of this type
	// (e.g. "aws-iam-role").
	ResourceType string
	// DependencyType limits to bundles that REQUIRE a dependency of this
	// type (e.g. "kubernetes-cluster").
	DependencyType string
	// Search is a full-text search over name/description/operator-guide/readme.
	// When set without an explicit SortBy, results are ranked by relevance.
	Search string

	SortBy    SortField
	SortOrder SortOrder

	PageSize int
}

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

// List returns bundles matching the supplied filters, following pagination
// cursors automatically. Returned [Bundle]s do not include
// dependencies/resources — call [Service.Get] for the full per-version shape.
func (s *Service) List(ctx context.Context, input ListInput) ([]Bundle, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []Bundle
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListBundles(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list bundles: %w", err))
		}
		for _, item := range resp.Bundles.Items {
			b, berr := toBundle(item)
			if berr != nil {
				return nil, fmt.Errorf("decode bundle: %w", berr)
			}
			out = append(out, *b)
		}
		next := resp.Bundles.Cursor.Next
		if next == "" {
			break
		}
		cursor = &scalars.Cursor{Next: next}
		if input.PageSize > 0 {
			cursor.Limit = input.PageSize
		}
	}
	return out, nil
}

func toBundle(v any) (*Bundle, error) {
	b := Bundle{}
	if err := decode.Decode(v, &b); err != nil {
		return nil, fmt.Errorf("decode bundle: %w", err)
	}
	return &b, nil
}

func buildListFilter(input ListInput) *gen.BundlesFilter {
	filter := &gen.BundlesFilter{}
	set := false
	if input.OciRepoName != "" {
		filter.OciRepo = &gen.OciRepoNameFilter{Eq: input.OciRepoName}
		set = true
	}
	if input.ResourceType != "" {
		filter.ResourceType = &gen.StringFilter{Eq: input.ResourceType}
		set = true
	}
	if input.DependencyType != "" {
		filter.DependencyType = &gen.StringFilter{Eq: input.DependencyType}
		set = true
	}
	if input.Search != "" {
		filter.Search = input.Search
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.BundlesSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.BundlesSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.BundlesSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.BundlesSort{Field: field, Order: order}
}
