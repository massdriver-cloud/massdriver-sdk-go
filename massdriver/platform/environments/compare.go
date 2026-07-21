package environments

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Comparison is a side-by-side diff of two environments — alias of
// [types.EnvironmentComparison].
type Comparison = types.EnvironmentComparison

// Compare diffs two environments in the same project, instance-by-instance.
// Instances are paired by component; for each component the result reports
// the resolved version on each side and a flat, leaf-level diff of the
// configured params. When a component is deployed on only one side, the
// other side's entry is nil. Environment-level attributes and default
// resource wiring are not part of the comparison.
//
// Both environments must belong to the same project — components don't cross
// project boundaries, so cross-project comparisons return a FORBIDDEN error.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when either
// environment does not exist in the configured organization.
func (s *Service) Compare(ctx context.Context, sourceID, targetID string) (*Comparison, error) {
	resp, err := gen.CompareEnvironments(ctx, s.client.GQLv2, s.client.Config.OrganizationID, sourceID, targetID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("compare environments %s..%s: %w", sourceID, targetID, err))
	}
	if resp.CompareEnvironments.Source.Id == "" {
		return nil, fmt.Errorf("compare environments %s..%s: %w", sourceID, targetID, gql.ErrNotFound)
	}
	cmp := &Comparison{}
	if err := decode.Decode(resp.CompareEnvironments, cmp); err != nil {
		return nil, fmt.Errorf("decode environment comparison: %w", err)
	}
	return cmp, nil
}
