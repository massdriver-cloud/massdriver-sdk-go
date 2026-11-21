package organizations

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
)

// DefaultBundleAccess is the access new bundle repositories receive at
// creation. Changing it only affects repositories created afterwards —
// access to existing repositories is managed through their grants.
type DefaultBundleAccess string

const (
	// DefaultBundleAccessNone keeps each new repository restricted until a
	// grant is authored for it.
	DefaultBundleAccessNone DefaultBundleAccess = "NONE"
	// DefaultBundleAccessAllProjects automatically creates an org-wide
	// repo:pull grant on each new bundle repository, making its bundles
	// usable by every project. The grant is a normal grant row: it is
	// listed on the repository and can be revoked like any other grant.
	DefaultBundleAccessAllProjects DefaultBundleAccess = "ALL_PROJECTS"
)

// Settings are organization-wide behavior settings. Each setting has a
// default, so organizations created before a setting existed read it as
// the default.
type Settings struct {
	// DefaultBundleAccess is the access granted to new bundle repositories
	// at creation. Defaults to [DefaultBundleAccessNone].
	DefaultBundleAccess DefaultBundleAccess
	// NamingConvention is the Liquid template that names every provisioned
	// cloud resource, e.g. `{{attrs.team}}-{{instance.id}}`. Empty means the
	// default convention `{{instance.id}}-{{component.suffix}}`. The resolved
	// name is burned into each instance when it is first created.
	NamingConvention string
	// DefaultTags are the attribute keys whose values are sent to
	// deployments as default tags. `managed-by` and `md-instance` are always
	// sent. Nil means the organization has never chosen its tags and still
	// sends the legacy set; once chosen, the list can change but can't
	// return to nil.
	DefaultTags []string
}

// UpdateSettingsInput is the input for [Service.UpdateSettings]. Only the
// settings you set are changed; zero-valued fields keep their current
// values.
type UpdateSettingsInput struct {
	// DefaultBundleAccess, when non-empty, sets the access granted to new
	// bundle repositories at creation. Changing it only affects
	// repositories created afterwards.
	DefaultBundleAccess DefaultBundleAccess
	// NamingConvention, when non-nil, sets the naming convention. A pointer
	// to "" restores the default convention.
	NamingConvention *string
	// DefaultTags, when non-nil, replaces the default tag keys. A pointer to
	// an empty slice sends only the always-on tags. Each key must be an
	// attribute that applies to instances.
	DefaultTags *[]string
}

// GetSettings retrieves the configured organization's behavior settings.
//
// Requires the `organization:manageSettings` action (organization admins);
// other callers receive a forbidden error.
func (s *Service) GetSettings(ctx context.Context) (*Settings, error) {
	resp, err := gen.GetOrganizationSettings(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get organization settings: %w", err))
	}
	if resp.Organization.Id == "" {
		return nil, fmt.Errorf("get organization settings: %w", gql.ErrNotFound)
	}
	if resp.Organization.Settings == nil {
		return nil, fmt.Errorf("get organization settings: server returned no settings")
	}
	st := resp.Organization.Settings
	return &Settings{
		DefaultBundleAccess: DefaultBundleAccess(st.DefaultBundleAccess),
		NamingConvention:    st.NamingConvention,
		DefaultTags:         st.DefaultTags,
	}, nil
}

// UpdateSettings changes the configured organization's behavior settings
// and returns the resulting settings. Only the fields set on input are
// changed; zero-valued fields keep their current values.
//
// Requires the `organization:manageSettings` action (organization admins);
// other callers receive a forbidden error.
func (s *Service) UpdateSettings(ctx context.Context, input UpdateSettingsInput) (*Settings, error) {
	gi := gen.UpdateOrganizationSettingsInput{
		DefaultBundleAccess: gen.OrganizationDefaultBundleAccess(input.DefaultBundleAccess),
		NamingConvention:    input.NamingConvention,
	}
	if input.DefaultTags != nil {
		// A nil slice would marshal as null, which the server reads as
		// "remove the default tags" rather than "no tags".
		tags := []string{}
		if *input.DefaultTags != nil {
			tags = *input.DefaultTags
		}
		gi.DefaultTags = &tags
	}
	resp, err := gen.UpdateOrganizationSettings(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gi)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update organization settings: %w", err))
	}
	if err := gql.CheckMutation("update organization settings", resp.UpdateOrganizationSettings.Successful, resp.UpdateOrganizationSettings.Messages); err != nil {
		return nil, err
	}
	r := resp.UpdateOrganizationSettings.Result
	if r.Settings == nil {
		return nil, fmt.Errorf("update organization settings: server returned no settings")
	}
	return &Settings{
		DefaultBundleAccess: DefaultBundleAccess(r.Settings.DefaultBundleAccess),
		NamingConvention:    r.Settings.NamingConvention,
		DefaultTags:         r.Settings.DefaultTags,
	}, nil
}
