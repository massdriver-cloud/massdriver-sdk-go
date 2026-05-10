// Package ocirepos provides operations for Massdriver OCI repositories — the
// named containers in your organization's catalog that hold versioned OCI
// artifacts (today: bundles, with more types coming).
//
// The package surfaces two distinct ways to address a repository:
//
//   - GraphQL CRUD against the repository record (Get, List, Create, Update,
//     Delete). These operate on Massdriver's metadata for the repo —
//     attributes, timestamps, the OCI reference, etc.
//   - [Service.Target], which returns an oras.Target for pulling/pushing artifacts
//     directly via the OCI Distribution protocol. Use this for code that needs
//     to push a bundle or fetch a manifest by tag.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.OciRepos] field on the top-level SDK client.
package ocirepos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for OCI repository operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.OciRepos] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// OciRepo is an OCI repository — alias of [types.OciRepo].
type OciRepo = types.OciRepo

// SortField is the field a [Service.List] result can be ordered by.
type SortField string

const (
	// SortByName orders alphabetically by repository name.
	SortByName SortField = "NAME"
	// SortByCreatedAt orders chronologically by creation time.
	SortByCreatedAt SortField = "CREATED_AT"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ArtifactType narrows the catalog to a specific OCI artifact type. Empty in
// [ListInput] means "any type."
type ArtifactType string

const (
	// ArtifactTypeBundle is a Massdriver bundle.
	ArtifactTypeBundle ArtifactType = "BUNDLE"
)

// ListInput controls a [Service.List] call. Zero value lists every repository in the
// configured organization, sorted alphabetically by name.
//
// Name filters are server-side AND'd, so combining e.g. NameEquals + Search
// will narrow to repositories that match both. The common shape is to set at
// most one Name* field and optionally a Search.
type ListInput struct {
	// NameEquals limits results to a repository with this exact name.
	NameEquals string
	// NameIn limits results to any of the named repositories.
	NameIn []string
	// NameStartsWith limits results to repositories whose names begin with
	// this prefix (e.g. "aws-").
	NameStartsWith string

	// Search is a full-text search across name, readme, and changelog.
	// When set without an explicit SortBy, results are ranked by relevance
	// rather than alphabetically.
	Search string

	// ArtifactType narrows to a single artifact type. Empty = any.
	ArtifactType ArtifactType

	// SortBy controls sort field. Empty = NAME.
	SortBy SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder

	// PageSize sets the cursor page size (1..100). Zero uses the server
	// default. Wrappers auto-paginate, so this only affects per-page round-
	// trip size, not the total number of results.
	PageSize int
}

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// ID is the unique repository name within the organization.
	// Lowercase letters, numbers, dashes, underscores. Max 53 characters.
	// Immutable after creation.
	ID string
	// ArtifactType is the OCI artifact type stored here. Today only
	// [ArtifactTypeBundle] is accepted.
	ArtifactType ArtifactType
	// Attributes are optional ABAC tags. Reserved keys starting with `md-`
	// are rejected by the server.
	Attributes map[string]any
}

// UpdateInput is the input for [Service.Update]. Only attributes are mutable; the
// repository's name and artifact type are immutable.
type UpdateInput struct {
	Attributes map[string]any
}

// Get retrieves a repository by ID (its name).
func (s *Service) Get(ctx context.Context, id string) (*OciRepo, error) {
	resp, err := gen.GetOciRepo(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get oci repo %s: %w", id, err))
	}
	if resp.OciRepo.Id == "" {
		return nil, fmt.Errorf("get oci repo %s: %w", id, gql.ErrNotFound)
	}
	return toOciRepo(resp.OciRepo)
}

// List returns every repository the caller can see, applying the supplied
// filters and following pagination cursors automatically.
func (s *Service) List(ctx context.Context, input ListInput) ([]OciRepo, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []OciRepo
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListOciRepos(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list oci repos: %w", err))
		}
		for _, item := range resp.OciRepos.Items {
			r, rerr := toOciRepo(item)
			if rerr != nil {
				return nil, fmt.Errorf("decode oci repo: %w", rerr)
			}
			out = append(out, *r)
		}
		next := resp.OciRepos.Cursor.Next
		if next == "" {
			break
		}
		cursor = &scalars.Cursor{Next: next}
		if input.PageSize > 0 {
			cursor.Limit = input.PageSize
		}
	}
	return out, nil
}

// Create creates a new (empty) repository. Returns a [*gql.MutationFailed]
// (wrapped) if the server reports `successful: false`.
func (s *Service) Create(ctx context.Context, input CreateInput) (*OciRepo, error) {
	resp, err := gen.CreateOciRepo(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateOciRepoInput{
		Id:           input.ID,
		ArtifactType: gen.OciArtifactType(input.ArtifactType),
		Attributes:   input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create oci repo: %w", err))
	}
	if err := gql.CheckMutation("create oci repo", resp.CreateOciRepo.Successful, resp.CreateOciRepo.Messages); err != nil {
		return nil, err
	}
	return toOciRepo(resp.CreateOciRepo.Result)
}

// Update updates a repository's mutable metadata (today: attributes only).
// Name and artifact type are immutable.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*OciRepo, error) {
	resp, err := gen.UpdateOciRepo(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateOciRepoInput{
		Attributes: input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update oci repo %s: %w", id, err))
	}
	if err := gql.CheckMutation("update oci repo", resp.UpdateOciRepo.Successful, resp.UpdateOciRepo.Messages); err != nil {
		return nil, err
	}
	return toOciRepo(resp.UpdateOciRepo.Result)
}

// Delete deletes a repository. Refused by the server if the repository has
// any published versions.
func (s *Service) Delete(ctx context.Context, id string) (*OciRepo, error) {
	resp, err := gen.DeleteOciRepo(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete oci repo %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete oci repo", resp.DeleteOciRepo.Successful, resp.DeleteOciRepo.Messages); err != nil {
		return nil, err
	}
	return toOciRepo(resp.DeleteOciRepo.Result)
}

// Target returns an oras.Target for pulling and pushing OCI artifacts in the
// named repository. The target is authenticated using the configured
// credentials on the underlying client.
//
// This is the OCI distribution path — separate from the GraphQL CRUD. Use it
// for code that needs to push a manifest or fetch a tag's contents directly.
func (s *Service) Target(repoName string) (oras.Target, error) {
	mdURL, err := url.Parse(s.client.Config.URL)
	if err != nil {
		return nil, fmt.Errorf("parse massdriver url: %w", err)
	}

	repo, err := remote.NewRepository(path.Join(mdURL.Host, s.client.Config.OrganizationID, repoName))
	if err != nil {
		return nil, fmt.Errorf("create oci repository handle for %s: %w", repoName, err)
	}

	if mdURL.Scheme == "http" {
		repo.PlainHTTP = true
	}

	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Header: http.Header{
			"authorization": []string{s.client.Config.Credentials.AuthHeaderValue},
		},
	}
	return repo, nil
}

func toOciRepo(v any) (*OciRepo, error) {
	r := OciRepo{}
	if err := decode.Decode(v, &r); err != nil {
		return nil, fmt.Errorf("decode oci repo: %w", err)
	}
	return &r, nil
}

// buildListFilter compiles a ListInput's name/search/artifact filters into the
// genqlient input. Returns nil when no filter fields are set so the request
// omits the variable entirely.
func buildListFilter(input ListInput) *gen.OciReposFilter {
	var nameFilter *gen.OciRepoNameFilter
	if input.NameEquals != "" || len(input.NameIn) > 0 || input.NameStartsWith != "" {
		nameFilter = &gen.OciRepoNameFilter{
			Eq:         input.NameEquals,
			In:         input.NameIn,
			StartsWith: input.NameStartsWith,
		}
	}
	if nameFilter == nil && input.Search == "" && input.ArtifactType == "" {
		return nil
	}
	return &gen.OciReposFilter{
		Name:         nameFilter,
		Search:       input.Search,
		ArtifactType: string(input.ArtifactType),
	}
}

// buildListSort compiles a ListInput's sort options. Returns nil when neither
// SortBy nor SortOrder is set so the server applies its default.
func buildListSort(input ListInput) *gen.OciReposSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.OciReposSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.OciReposSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.OciReposSort{Field: field, Order: order}
}
