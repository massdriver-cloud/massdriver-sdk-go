package organizations

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// CustomAttribute is a user-declared attribute key — alias of
// [types.CustomAttribute].
type CustomAttribute = types.CustomAttribute

// AttributeScope is the resource level where a [CustomAttribute] applies.
// Hierarchy scopes cascade values downward.
type AttributeScope string

const (
	// AttributeScopeProject sets the attribute on individual projects.
	// Values cascade to environments, instances, deployments, and
	// resources within the project.
	AttributeScopeProject AttributeScope = "PROJECT"
	// AttributeScopeEnvironment sets the attribute on individual environments.
	AttributeScopeEnvironment AttributeScope = "ENVIRONMENT"
	// AttributeScopeComponent sets the attribute on blueprint components.
	AttributeScopeComponent AttributeScope = "COMPONENT"
	// AttributeScopeRepo sets the attribute on individual OCI repositories.
	AttributeScopeRepo AttributeScope = "REPO"
)

// CreateCustomAttributeInput is the input for [Service.CreateCustomAttribute].
type CreateCustomAttributeInput struct {
	// Key is 1-64 characters, identifier-like (starts with letter/underscore;
	// letters/digits/underscores only). Case-insensitive. Keys starting
	// with `md-` are reserved for system use.
	Key string
	// Scope is the resource level where this attribute applies.
	Scope AttributeScope
	// Required, when true, makes the attribute mandatory at create time
	// for resources at the specified scope. Optional; pass nil to use
	// the server default (false).
	Required *bool
	// Values is the closed set of values the attribute may take. Must
	// have at least one entry; the literal "*" is reserved.
	Values []string
}

// UpdateCustomAttributeInput is the input for [Service.UpdateCustomAttribute].
// The Key and Scope are immutable; only Required and Values can change.
type UpdateCustomAttributeInput struct {
	Required *bool
	Values   []string
}

// CreateCustomAttribute declares a new custom attribute for the
// organization. Once declared, the attribute applies immediately to new
// resources at its scope; existing resources are not retroactively
// validated.
func (s *Service) CreateCustomAttribute(ctx context.Context, input CreateCustomAttributeInput) (*CustomAttribute, error) {
	resp, err := gen.CreateCustomAttribute(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateCustomAttributeInput{
		Key:      input.Key,
		Scope:    gen.AttributeScope(input.Scope),
		Required: input.Required,
		Values:   input.Values,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create custom attribute %s: %w", input.Key, err))
	}
	if err := gql.CheckMutation("create custom attribute", resp.CreateCustomAttribute.Successful, resp.CreateCustomAttribute.Messages); err != nil {
		return nil, err
	}
	return toCustomAttribute(resp.CreateCustomAttribute.Result)
}

// UpdateCustomAttribute replaces the closed set of values (and
// optionally toggles Required) on an existing attribute. Changing
// Values does not retroactively validate or rewrite resources tagged
// before the update.
func (s *Service) UpdateCustomAttribute(ctx context.Context, id string, input UpdateCustomAttributeInput) (*CustomAttribute, error) {
	resp, err := gen.UpdateCustomAttribute(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateCustomAttributeInput{
		Required: input.Required,
		Values:   input.Values,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update custom attribute %s: %w", id, err))
	}
	if err := gql.CheckMutation("update custom attribute", resp.UpdateCustomAttribute.Successful, resp.UpdateCustomAttribute.Messages); err != nil {
		return nil, err
	}
	return toCustomAttribute(resp.UpdateCustomAttribute.Result)
}

// DeleteCustomAttribute removes a custom attribute. Existing tags on
// resources are not removed.
func (s *Service) DeleteCustomAttribute(ctx context.Context, id string) (*CustomAttribute, error) {
	resp, err := gen.DeleteCustomAttribute(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete custom attribute %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete custom attribute", resp.DeleteCustomAttribute.Successful, resp.DeleteCustomAttribute.Messages); err != nil {
		return nil, err
	}
	return toCustomAttribute(resp.DeleteCustomAttribute.Result)
}

func toCustomAttribute(v any) (*CustomAttribute, error) {
	a := CustomAttribute{}
	if err := decode.Decode(v, &a); err != nil {
		return nil, fmt.Errorf("decode custom attribute: %w", err)
	}
	return &a, nil
}
