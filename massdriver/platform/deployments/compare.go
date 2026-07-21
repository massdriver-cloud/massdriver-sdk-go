package deployments

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Comparison is a side-by-side diff of two deployments — alias of
// [types.DeploymentComparison].
type Comparison = types.DeploymentComparison

// Compare diffs two deployments' snapshotted configuration: the bundle
// version on each side plus a flat, leaf-level diff of the params. Use it to
// audit what a deploy changed ("what did this deployment do to the params?")
// or to contrast deploys from different points in time. Runtime state, logs,
// and produced artifacts are out of scope.
//
// The two deployments need not target the same instance, though comparing
// unrelated instances naturally reports every param as present on one side
// only. Source and Target on the result carry slim deployment records
// (no params — those are diffed into Params instead).
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when either
// deployment does not exist in the configured organization.
func (s *Service) Compare(ctx context.Context, sourceID, targetID string) (*Comparison, error) {
	resp, err := gen.CompareDeployments(ctx, s.client.GQLv2, s.client.Config.OrganizationID, sourceID, targetID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("compare deployments %s..%s: %w", sourceID, targetID, err))
	}
	if resp.CompareDeployments.Source.Id == "" {
		return nil, fmt.Errorf("compare deployments %s..%s: %w", sourceID, targetID, gql.ErrNotFound)
	}
	cmp := &Comparison{}
	if err := decode.Decode(resp.CompareDeployments, cmp); err != nil {
		return nil, fmt.Errorf("decode deployment comparison: %w", err)
	}
	return cmp, nil
}
