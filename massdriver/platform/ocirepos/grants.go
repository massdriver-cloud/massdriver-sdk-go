package ocirepos

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Grant is a sharing rule on a repository — alias of [types.Grant]. The
// caller must have `repo:grant` on the source repository to create or
// delete grants.
type Grant = types.Grant

// CreateGrantInput is the input for [Service.CreateGrant].
type CreateGrantInput struct {
	// Action is the action being granted on the repository. Currently
	// the only grantable repo action is "repo:pull" — repo visibility
	// is inferred from any granted action.
	Action string

	// RecipientConditions restricts the grant to recipient projects
	// matching attribute conditions. Nil (the zero value) is the
	// wildcard — every project in the org qualifies. A non-nil map
	// describes the required attribute values; per-key, an empty/nil
	// slice is the per-key wildcard (any value of that attribute), a
	// non-empty slice is a closed set.
	//
	// Same convention as [policies.PolicyConditions]; see [types.PolicyConditions]
	// for the wire encoding.
	RecipientConditions types.PolicyConditions
}

// ListGrantsInput controls a [Service.IterGrants]/[Service.ListGrantsPage]
// call. Zero value lists every grant on the repository.
type ListGrantsInput struct {
	// PageSize sets the cursor page size (1..100). Zero uses the server
	// default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page.
	After string
}

// CreateGrant creates a sharing grant on the named repository. Grants
// are immutable — to change action or conditions, delete and
// re-create.
func (s *Service) CreateGrant(ctx context.Context, repoID string, input CreateGrantInput) (*Grant, error) {
	resp, err := gen.CreateRepoGrant(ctx, s.client.GQLv2, s.client.Config.OrganizationID, repoID, gen.CreateRepoGrantInput{
		Action:              input.Action,
		RecipientConditions: input.RecipientConditions,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create grant on oci repo %s: %w", repoID, err))
	}
	if err := gql.CheckMutation("create repo grant", resp.CreateRepoGrant.Successful, resp.CreateRepoGrant.Messages); err != nil {
		return nil, err
	}
	return toGrant(resp.CreateRepoGrant.Result)
}

// DeleteGrant deletes a grant by ID. The caller must have `repo:grant`
// on the grant's source repository. The server treats grants uniformly
// by id, so this also deletes resource grants — though for those,
// prefer the resources service's DeleteGrant.
func (s *Service) DeleteGrant(ctx context.Context, grantID string) error {
	resp, err := gen.DeleteGrant(ctx, s.client.GQLv2, s.client.Config.OrganizationID, grantID)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("delete grant %s: %w", grantID, err))
	}
	return gql.CheckMutation("delete grant", resp.DeleteGrant.Successful, resp.DeleteGrant.Messages)
}

// IterGrants returns a lazy [iter.Seq2] over the grants authored on the
// named repository, fetching pages on demand. Ranging the sequence streams
// results without buffering the whole set, and breaking out of the loop
// stops requesting further pages. The yielded error is non-nil exactly
// once, on a failed page fetch, after which iteration stops.
//
// Yields [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// repository with the given ID exists in the configured organization.
// To buffer every grant into a slice, wrap with [types.Collect].
func (s *Service) IterGrants(ctx context.Context, repoID string, input ListGrantsInput) iter.Seq2[Grant, error] {
	return paging.Iter(ctx, input.After, s.grantsPage(repoID, input))
}

// ListGrantsPage returns a single page of the named repository's grants.
// input.PageSize bounds the page and input.After (an opaque cursor from a
// prior page's Next) selects which page. Use it for stateless pagination —
// e.g. a UI or CLI that hands the returned [types.Page].Next back to its
// own client to fetch the next page on demand.
func (s *Service) ListGrantsPage(ctx context.Context, repoID string, input ListGrantsInput) (types.Page[Grant], error) {
	return s.grantsPage(repoID, input)(ctx, input.After)
}

// grantsPage builds the single-page fetcher shared by IterGrants and ListGrantsPage.
func (s *Service) grantsPage(repoID string, input ListGrantsInput) paging.FetchFunc[Grant] {
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[Grant], error) {
		resp, err := gen.ListOciRepoGrants(ctx, s.client.GQLv2, s.client.Config.OrganizationID, repoID, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[Grant]{}, gql.ClassifyError(fmt.Errorf("list oci repo %s grants: %w", repoID, err))
		}
		if resp.OciRepo.Id == "" {
			return types.Page[Grant]{}, fmt.Errorf("list oci repo %s grants: %w", repoID, gql.ErrNotFound)
		}
		items := make([]Grant, 0, len(resp.OciRepo.Grants.Items))
		for _, item := range resp.OciRepo.Grants.Items {
			g, gerr := toGrant(item)
			if gerr != nil {
				return types.Page[Grant]{}, gerr
			}
			items = append(items, *g)
		}
		return types.Page[Grant]{
			Items:    items,
			Next:     resp.OciRepo.Grants.Cursor.Next,
			Previous: resp.OciRepo.Grants.Cursor.Previous,
		}, nil
	}
}

// toGrant decodes a genqlient grant result. RecipientConditions
// translation happens through [types.PolicyConditions]'s json
// (un)marshaler, so mapstructure copies the field through natively.
func toGrant(v any) (*Grant, error) {
	g := Grant{}
	if err := decode.Decode(v, &g); err != nil {
		return nil, fmt.Errorf("decode grant: %w", err)
	}
	return &g, nil
}
