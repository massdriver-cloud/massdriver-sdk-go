package instances

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// RemoteReference is a per-instance override of a single connection slot —
// alias of [types.RemoteReference].
type RemoteReference = types.RemoteReference

// SetRemoteReference overrides one of an instance's connection slots with a
// resource from another project (or an imported resource). field names the
// key in the instance's bundle connectionsSchema to bind; resourceID is
// either a UUID for imported resources or "instance.field" for provisioned
// resources.
//
// The override takes priority over any blueprint-level Link wired into the
// same slot and reverts to the Link (or environment default) when removed
// with [Service.RemoveRemoteReference]. Like other configuration changes,
// the instance must not be in PROVISIONED or FAILED status.
func (s *Service) SetRemoteReference(ctx context.Context, instanceID, resourceID, field string) (*RemoteReference, error) {
	resp, err := gen.SetRemoteReference(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, resourceID, gen.SetRemoteReferenceInput{
		Field: field,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("set instance %s remote reference %s: %w", instanceID, field, err))
	}
	if err := gql.CheckMutation("set remote reference", resp.SetRemoteReference.Successful, resp.SetRemoteReference.Messages); err != nil {
		return nil, err
	}
	return toRemoteReference(resp.SetRemoteReference.Result)
}

// RemoveRemoteReference removes the remote-reference override from the named
// connection slot. The slot reverts to its blueprint Link (if any) or the
// environment default at the next deploy. Like other configuration changes,
// the instance must not be in PROVISIONED or FAILED status.
func (s *Service) RemoveRemoteReference(ctx context.Context, instanceID, field string) (*RemoteReference, error) {
	resp, err := gen.RemoveRemoteReference(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, gen.RemoveRemoteReferenceInput{
		Field: field,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("remove instance %s remote reference %s: %w", instanceID, field, err))
	}
	if err := gql.CheckMutation("remove remote reference", resp.RemoveRemoteReference.Successful, resp.RemoveRemoteReference.Messages); err != nil {
		return nil, err
	}
	return toRemoteReference(resp.RemoveRemoteReference.Result)
}

func toRemoteReference(v any) (*RemoteReference, error) {
	ref := RemoteReference{}
	if err := decode.Decode(v, &ref); err != nil {
		return nil, fmt.Errorf("decode remote reference: %w", err)
	}
	return &ref, nil
}
