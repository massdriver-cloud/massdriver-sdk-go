package instances

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Secret is metadata for an encrypted secret on an instance — alias of
// [types.InstanceSecret]. Secret values are never returned by the API; only
// the name, SHA-256 fingerprint, and timestamps are exposed.
type Secret = types.InstanceSecret

// SetSecret creates or updates a secret on an instance. The value is
// encrypted at rest and never returned in API responses; the returned
// [Secret] carries only metadata.
func (s *Service) SetSecret(ctx context.Context, instanceID, name, value string) (*Secret, error) {
	resp, err := gen.SetInstanceSecret(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, gen.SetInstanceSecretInput{
		Name:  name,
		Value: value,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("set instance %s secret %s: %w", instanceID, name, err))
	}
	if err := gql.CheckMutation("set instance secret", resp.SetInstanceSecret.Successful, resp.SetInstanceSecret.Messages); err != nil {
		return nil, err
	}
	return toSecret(resp.SetInstanceSecret.Result)
}

// RemoveSecret removes a secret from an instance. The change takes effect on
// the next deployment; running infrastructure retains the secret until
// redeployed.
func (s *Service) RemoveSecret(ctx context.Context, instanceID, name string) (*Secret, error) {
	resp, err := gen.RemoveInstanceSecret(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, name)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("remove instance %s secret %s: %w", instanceID, name, err))
	}
	if err := gql.CheckMutation("remove instance secret", resp.RemoveInstanceSecret.Successful, resp.RemoveInstanceSecret.Messages); err != nil {
		return nil, err
	}
	return toSecret(resp.RemoveInstanceSecret.Result)
}

func toSecret(v any) (*Secret, error) {
	sec := Secret{}
	if err := decode.Decode(v, &sec); err != nil {
		return nil, fmt.Errorf("decode instance secret: %w", err)
	}
	return &sec, nil
}
