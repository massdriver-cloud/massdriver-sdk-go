// Package viewer answers "who am I?" — the authenticated entity making
// the current request. The GraphQL `viewer` query returns a union of
// AccountViewer (a human user) and ServiceAccountViewer (an API client);
// this package flattens both into a single [Viewer] type with a Kind
// discriminator.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.Viewer] field on the top-level SDK
// client.
package viewer

import (
	"context"
	"errors"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Viewer is the authenticated entity — alias of [types.Viewer].
type Viewer = types.Viewer

// Kind is the discriminator for [Viewer.Kind].
type Kind = types.ViewerKind

const (
	// KindAccount is a human user (PAT or session auth).
	KindAccount = types.ViewerKindAccount
	// KindServiceAccount is a programmatic service account.
	KindServiceAccount = types.ViewerKindServiceAccount
)

// Service is the receiver for viewer operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Viewer] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Get returns the currently authenticated entity. Use it to bootstrap UI
// state, verify which credentials are active, or distinguish a user from
// a service account.
//
// The returned Organization is the org this client operates against: for
// service accounts, the org the credential belongs to; for accounts, the
// configured organization id resolved via a second lookup (nil when none
// is configured). An account authenticated against an organization it
// cannot access gets an error wrapping [gql.ErrNotFound] — surfacing the
// credential/org mismatch is the point of a "who am I?" check.
//
// Returns an error if no viewer is returned (typically because the
// request is unauthenticated or the credentials are invalid).
func (s *Service) Get(ctx context.Context) (*Viewer, error) {
	resp, err := gen.GetViewer(ctx, s.client.GQLv2)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get viewer: %w", err))
	}
	if resp.Viewer == nil {
		return nil, errors.New("no authenticated viewer (check MASSDRIVER_API_KEY)")
	}

	switch v := resp.Viewer.(type) {
	case *gen.GetViewerViewerAccountViewer:
		view := &Viewer{
			Kind:      types.ViewerKindAccount,
			ID:        v.Id,
			Email:     v.Email,
			FirstName: v.FirstName,
			LastName:  v.LastName,
		}
		// A human credential can span many orgs, so the API can't tell us
		// which one is "current" — resolve the org this client is
		// configured to operate against instead. Left nil when no
		// organization id is configured.
		if orgID := s.client.Config.OrganizationID; orgID != "" {
			org, err := gen.GetOrganization(ctx, s.client.GQLv2, orgID)
			if err != nil {
				return nil, gql.ClassifyError(fmt.Errorf("get viewer: resolve configured organization %q: %w", orgID, err))
			}
			if org.Organization.Id == "" {
				return nil, fmt.Errorf("get viewer: configured organization %q: %w", orgID, gql.ErrNotFound)
			}
			view.Organization = &types.Organization{
				ID:        org.Organization.Id,
				Name:      org.Organization.Name,
				CreatedAt: org.Organization.CreatedAt,
				UpdatedAt: org.Organization.UpdatedAt,
			}
		}
		return view, nil

	case *gen.GetViewerViewerServiceAccountViewer:
		return &Viewer{
			Kind:        types.ViewerKindServiceAccount,
			ID:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			Organization: &types.Organization{
				ID:        v.Organization.Id,
				Name:      v.Organization.Name,
				CreatedAt: v.Organization.CreatedAt,
				UpdatedAt: v.Organization.UpdatedAt,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unexpected viewer type %T", v)
	}
}
