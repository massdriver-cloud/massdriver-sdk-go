package policies

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Action is one entry in the action catalog — alias of [types.PolicyAction].
type Action = types.PolicyAction

// Entity is one entity kind from the entity catalog — alias of
// [types.PolicyEntity].
type Entity = types.PolicyEntity

// ListActions returns the complete catalog of ABAC actions available
// in the organization. The list is sorted alphabetically by id.
//
// Use this to populate a policy-authoring UI, validate an incoming
// action id, or generate shell completions. Don't memoize the result
// indefinitely — the catalog can grow as the server adds actions.
func (s *Service) ListActions(ctx context.Context) ([]Action, error) {
	resp, err := gen.ListPolicyActions(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list policy actions: %w", err))
	}
	out := make([]Action, 0, len(resp.PolicyActions))
	for _, item := range resp.PolicyActions {
		a := Action{}
		if derr := decode.Decode(item, &a); derr != nil {
			return nil, fmt.Errorf("decode policy action: %w", derr)
		}
		out = append(out, a)
	}
	return out, nil
}

// ListEntities returns the complete catalog of entity kinds an action
// can apply to (e.g. "project", "environment"). Useful for grouping
// actions in a UI.
func (s *Service) ListEntities(ctx context.Context) ([]Entity, error) {
	resp, err := gen.ListPolicyEntities(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list policy entities: %w", err))
	}
	out := make([]Entity, 0, len(resp.PolicyEntities))
	for _, item := range resp.PolicyEntities {
		e := Entity{}
		if derr := decode.Decode(item, &e); derr != nil {
			return nil, fmt.Errorf("decode policy entity: %w", derr)
		}
		out = append(out, e)
	}
	return out, nil
}
