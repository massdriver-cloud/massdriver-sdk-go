package policies

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
)

// CustomAttributeSchema returns a JSON Schema document describing the
// custom-attribute keys and permitted values that policy conditions
// can use for the given action, narrowed to what the caller's own
// policies permit.
//
// The schema is most useful for rendering a form schema in a
// policy-authoring UI — only the choices the API will actually accept
// on write are surfaced. Org admins see the full closed set;
// non-admins with no matching policy get an "additionalProperties:
// false" schema that rejects every write.
func (s *Service) CustomAttributeSchema(ctx context.Context, action string) (json.RawMessage, error) {
	resp, err := gen.CustomAttributeSchema(ctx, s.client.GQLv2, s.client.Config.OrganizationID, action)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("custom attribute schema for %s: %w", action, err))
	}
	b, err := json.Marshal(resp.CustomAttributeSchema)
	if err != nil {
		return nil, fmt.Errorf("marshal custom attribute schema: %w", err)
	}
	return b, nil
}

// CustomAttributeValues returns the closed set of values declared for
// one (scope, key) custom attribute. Useful for populating a single
// dropdown (e.g. a TEAM picker) without paginating through the
// organization's full custom-attribute list.
//
// Returns an error when (scope, key) doesn't correspond to a declared
// custom attribute.
func (s *Service) CustomAttributeValues(ctx context.Context, scope organizations.AttributeScope, key string) ([]string, error) {
	resp, err := gen.CustomAttributeValues(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.AttributeScope(scope), key)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("custom attribute values for %s/%s: %w", scope, key, err))
	}
	return resp.CustomAttributeValues, nil
}
