// Package stash provides operations for the Massdriver stash: small keyed
// values that scripts and deployments store in a project, environment,
// component, instance, or deployment and read back later.
//
// Each scope is its own namespace. An entry in an environment is not visible
// from its instances or deployments.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Stash] field on the top-level SDK client.
package stash

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Entry is a stash entry — alias of [types.StashEntry].
type Entry = types.StashEntry

// Service is the receiver for stash operations. Construct with [New]; for
// the typical case you'll use the [massdriver.Client.Stash] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// ScopeKind is the kind of resource that holds stash entries.
type ScopeKind string

const (
	ScopeProject     ScopeKind = "project"
	ScopeEnvironment ScopeKind = "environment"
	ScopeComponent   ScopeKind = "component"
	ScopeInstance    ScopeKind = "instance"
	ScopeDeployment  ScopeKind = "deployment"
)

// Scope is the one resource that holds an entry. ID is the resource's
// identifier, such as "api-prod" for an environment, or the deployment's
// UUID.
type Scope struct {
	Kind ScopeKind
	ID   string
}

// String renders the scope as `<kind>:<id>`.
func (s Scope) String() string { return string(s.Kind) + ":" + s.ID }

// Format is how an entry's value is read.
type Format string

const (
	// FormatText values are plain strings.
	FormatText Format = "TEXT"
	// FormatJSON values are JSON objects. The entry's Data holds the
	// parsed object.
	FormatJSON Format = "JSON"
)

// ListInput controls a [Service.Iter] call. The zero value lists every
// entry in the scope.
type ListInput struct {
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next. Empty
	// starts at the first page.
	After string
}

// SetInput is the input for [Service.Set]. Label is not encrypted and is
// returned even for sensitive entries, so keep secrets out of it.
type SetInput struct {
	Value string
	// Label is a human-readable name for the entry, up to 255 characters.
	// Empty clears any label the entry had.
	Label string
	// Format is how Value is read. Empty is [FormatText]. With
	// [FormatJSON], Value must be a JSON object.
	Format Format
	// TTL is how long the entry lives after this write, truncated to whole
	// seconds. Zero keeps the entry until it is deleted.
	TTL time.Duration
	// Immutable entries keep their first value. A later Set returns an
	// error.
	Immutable bool
	// Sensitive values are returned only to callers that can write the
	// scope. Other callers get the entry with a nil Value.
	Sensitive bool
}

// Get retrieves the entry stored under key in scope.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when the scope
// does not exist, the scope has no entry under key, or the entry has expired.
func (s *Service) Get(ctx context.Context, scope Scope, key string) (*Entry, error) {
	gs, err := toGenScope(scope)
	if err != nil {
		return nil, err
	}
	resp, err := gen.GetStashEntry(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gs, key)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get stash entry %s in %s: %w", key, scope, err))
	}
	if resp.StashEntry.Key == "" {
		return nil, fmt.Errorf("get stash entry %s in %s: %w", key, scope, gql.ErrNotFound)
	}
	return toEntry(resp.StashEntry)
}

// Iter returns a lazy [iter.Seq2] over the entries in scope, fetching pages
// on demand. The yielded error is non-nil exactly once, on a failed page
// fetch, after which iteration stops.
//
// To buffer every entry into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, scope Scope, input ListInput) iter.Seq2[Entry, error] {
	return paging.Iter(ctx, input.After, s.page(scope, input))
}

// ListPage returns a single page of the entries in scope. input.PageSize
// bounds the page and input.After selects which page.
func (s *Service) ListPage(ctx context.Context, scope Scope, input ListInput) (types.Page[Entry], error) {
	return s.page(scope, input)(ctx, input.After)
}

func (s *Service) page(scope Scope, input ListInput) paging.FetchFunc[Entry] {
	return paging.DecodeFetch[Entry](input.PageSize, "stash entry", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		gs, err := toGenScope(scope)
		if err != nil {
			return paging.RawPage{}, err
		}
		resp, err := gen.ListStashEntries(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gs, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list stash entries in %s: %w", scope, err))
		}
		return paging.RawPage{
			Items:    resp.StashEntries.Items,
			Next:     resp.StashEntries.Cursor.Next,
			Previous: resp.StashEntries.Cursor.Previous,
		}, nil
	})
}

// Set stores input.Value under key in scope, and replaces the entry's
// format, TTL, and flags.
//
// Returns a [*gql.MutationFailedError] (wrapped) if the server reports
// `successful: false`, for example when the entry is immutable or a JSON
// value is not an object.
func (s *Service) Set(ctx context.Context, scope Scope, key string, input SetInput) (*Entry, error) {
	gs, err := toGenScope(scope)
	if err != nil {
		return nil, err
	}
	if input.TTL < 0 || (input.TTL > 0 && input.TTL < time.Second) {
		return nil, fmt.Errorf("set stash entry %s in %s: ttl must be zero or at least one second, got %s", key, scope, input.TTL)
	}
	format := input.Format
	if format == "" {
		format = FormatText
	}
	resp, err := gen.SetStashEntry(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gs, key, gen.SetStashEntryInput{
		Value:     input.Value,
		Label:     input.Label,
		Format:    gen.StashFormat(format),
		Ttl:       int(input.TTL / time.Second),
		Immutable: input.Immutable,
		Sensitive: input.Sensitive,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("set stash entry %s in %s: %w", key, scope, err))
	}
	if err := gql.CheckMutation("set stash entry", resp.SetStashEntry.Successful, resp.SetStashEntry.Messages); err != nil {
		return nil, err
	}
	return toEntry(resp.SetStashEntry.Result)
}

// Delete removes the entry stored under key in scope. Immutable entries can
// be deleted.
//
// Returns a [*gql.MutationFailedError] (wrapped) when the scope does not
// exist or has no entry under key. The server reports both as a failed
// payload, not as a not-found error.
func (s *Service) Delete(ctx context.Context, scope Scope, key string) error {
	gs, err := toGenScope(scope)
	if err != nil {
		return err
	}
	resp, err := gen.DeleteStashEntry(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gs, key)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("delete stash entry %s in %s: %w", key, scope, err))
	}
	return gql.CheckMutation("delete stash entry", resp.DeleteStashEntry.Successful, resp.DeleteStashEntry.Messages)
}

func toGenScope(scope Scope) (gen.StashScope, error) {
	if scope.ID == "" {
		return gen.StashScope{}, fmt.Errorf("stash scope %q has no ID", scope.Kind)
	}
	switch scope.Kind {
	case ScopeProject:
		return gen.StashScope{Project: scope.ID}, nil
	case ScopeEnvironment:
		return gen.StashScope{Environment: scope.ID}, nil
	case ScopeComponent:
		return gen.StashScope{Component: scope.ID}, nil
	case ScopeInstance:
		return gen.StashScope{Instance: scope.ID}, nil
	case ScopeDeployment:
		return gen.StashScope{Deployment: scope.ID}, nil
	}
	return gen.StashScope{}, fmt.Errorf("unknown stash scope kind %q", scope.Kind)
}

func toEntry(v any) (*Entry, error) {
	e := Entry{}
	if err := decode.Decode(v, &e); err != nil {
		return nil, fmt.Errorf("decode stash entry: %w", err)
	}
	return &e, nil
}
