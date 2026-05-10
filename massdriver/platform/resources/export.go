package resources

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Exported is the result of [Service.Export] — a [Resource] with sensitive
// payload values unmasked, plus a server-rendered string in the
// requested format.
//
// The embedded [Resource]'s Payload carries the unmasked values. Fields
// not selected by the underlying export query (Field, Formats,
// Attributes, Instance) are zero — call [Service.Get] if you need those.
type Exported struct {
	types.Resource `mapstructure:",squash"`
	// Rendered is the resource serialized in the requested format (e.g.
	// the JSON-encoded payload, or the YAML/env template output declared
	// by the resource type's schema).
	Rendered string `json:"rendered" mapstructure:"rendered"`
}

// FormatJSON is the canonical export format — serializes the payload
// as JSON. Always supported for every resource type.
const FormatJSON = "json"

// Export retrieves a resource's full payload with sensitive fields
// unmasked, plus a [Exported.Rendered] string serializing the payload
// in the requested format. Pass FormatJSON for the canonical JSON
// payload, or any format declared by the resource type's schema (e.g.
// "yaml", "env"). Pass an empty string to use the server default
// (json).
//
// Export is recorded in the audit log so access to sensitive payload
// data — credentials, connection strings, IaC outputs — is
// attributable to the actor who performed it. Works for both
// imported and provisioned resources; the caller must have permission
// to view the resource.
func (s *Service) Export(ctx context.Context, id, format string) (*Exported, error) {
	resp, err := gen.ExportResource(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, format)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("export resource %s: %w", id, err))
	}
	if err := gql.CheckMutation("export resource", resp.ExportResource.Successful, resp.ExportResource.Messages); err != nil {
		return nil, err
	}
	e := Exported{}
	if derr := decode.Decode(resp.ExportResource.Result, &e); derr != nil {
		return nil, fmt.Errorf("decode exported resource: %w", derr)
	}
	return &e, nil
}
