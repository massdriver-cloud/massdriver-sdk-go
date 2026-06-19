// Package auditlogs provides read access to the organization's audit
// trail — the record of every state-changing operation performed in
// Massdriver, captured following the CloudEvents specification.
//
// Audit logs are read-only; there are no mutations. Typical use cases:
//
//   - "Who changed this and when?" — [Service.Get] by id, or [Service.Iter]
//     filtered by subject/type/actor.
//   - "What did Alice do last week?" — [Service.Iter] with [ListInput.ActorSearch]
//     and a [ListInput.TimeRange].
//   - "Show me a dropdown of every event type" — [Service.ListEventTypes]
//     returns the static catalog (e.g. "project.created",
//     "deployment.completed").
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.AuditLogs] field on the top-level SDK
// client.
package auditlogs

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

// AuditLog is one event in the audit trail — alias of [types.AuditLog].
type AuditLog = types.AuditLog

// Actor identifies the entity that performed an audited action — alias
// of [types.AuditLogActor]. Inspect [Actor.Type] to know which kind it
// is.
type Actor = types.AuditLogActor

// Service is the receiver for audit-log operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.AuditLogs] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// ActorType is the kind of entity that performed an audited action.
type ActorType string

const (
	// ActorAccount is a human user authenticated with their personal account.
	ActorAccount ActorType = "ACCOUNT"
	// ActorServiceAccount is a service account (API key or access token).
	ActorServiceAccount ActorType = "SERVICE_ACCOUNT"
	// ActorDeployment is an automated deployment process (e.g., Terraform apply).
	ActorDeployment ActorType = "DEPLOYMENT"
	// ActorSystem is an internal system action with no specific user or
	// service account. Also used for legacy events recorded before
	// actor tracking was introduced.
	ActorSystem ActorType = "SYSTEM"
)

// SortField is the field a [Service.Iter] result can be ordered by.
type SortField string

const (
	// SortByOccurredAt orders chronologically by event time. Default.
	SortByOccurredAt SortField = "OCCURRED_AT"
	// SortByType orders alphabetically by event type id.
	SortByType SortField = "TYPE"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.Iter] call. Zero value lists every audit log
// event the caller can see, sorted by newest first.
//
// All fields are server-side AND'd. The common shapes are
// "everything in this time window" (set TimeRange*),
// "every action by this user" (set ActorID or ActorSearch), or
// "every event of this type" (set Type).
type ListInput struct {
	// TimeRangeStart and TimeRangeEnd narrow events to a window. Both
	// inclusive; either may be zero to leave the bound open.
	TimeRangeStart time.Time
	TimeRangeEnd   time.Time

	// Type limits results to a single event type id (e.g.
	// "project.created"). Use [Service.ListEventTypes] to enumerate the
	// catalog.
	Type string

	// ActorTypes limits results to the named actor kinds. Useful for
	// e.g. "show only events triggered by service accounts."
	ActorTypes []ActorType

	// ActorID limits results to one specific actor. Pass the actor's
	// id (e.g. an account id, service-account id, or deployment id).
	ActorID string

	// ActorSearch is a case-insensitive substring search across the
	// actor's identifying fields (email/name for users, name for
	// service accounts, id for deployments and system actions).
	ActorSearch string

	// SortBy controls sort field. Empty = OCCURRED_AT.
	SortBy SortField
	// SortOrder controls sort direction. Empty = DESC (newest first).
	SortOrder SortOrder

	// PageSize sets the cursor page size (1..100). Zero uses the
	// server default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// Get retrieves a single audit log event by id.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// audit log event with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*AuditLog, error) {
	resp, err := gen.GetAuditLog(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get audit log %s: %w", id, err))
	}
	if resp.AuditLog.Id == "" {
		return nil, fmt.Errorf("get audit log %s: %w", id, gql.ErrNotFound)
	}
	return toAuditLog(resp.AuditLog)
}

// Iter returns a lazy [iter.Seq2] over audit log events matching input, fetching
// pages on demand. It is the recommended way to list: ranging the sequence
// streams results without buffering the whole match set, and breaking out of the
// loop stops requesting further pages. The yielded error is non-nil exactly once,
// on a failed page fetch, after which iteration stops.
//
// To buffer every match into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[AuditLog, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of audit log events matching input.
// input.PageSize bounds the page and input.After (an opaque cursor from a prior
// page's Next) selects which page. Use it for stateless pagination — e.g. a UI or
// CLI that hands the returned [types.Page].Next back to its own client to fetch
// the next page on demand.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[AuditLog], error) {
	return s.page(input)(ctx, input.After)
}

// page builds the single-page fetcher shared by Iter and ListPage.
func (s *Service) page(input ListInput) paging.FetchFunc[AuditLog] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[AuditLog], error) {
		resp, err := gen.ListAuditLogs(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[AuditLog]{}, gql.ClassifyError(fmt.Errorf("list audit logs: %w", err))
		}
		items := make([]AuditLog, 0, len(resp.AuditLogs.Items))
		for _, item := range resp.AuditLogs.Items {
			a, derr := toAuditLog(item)
			if derr != nil {
				return types.Page[AuditLog]{}, derr
			}
			items = append(items, *a)
		}
		return types.Page[AuditLog]{
			Items:    items,
			Next:     resp.AuditLogs.Cursor.Next,
			Previous: resp.AuditLogs.Cursor.Previous,
		}, nil
	}
}

// ListEventTypes returns the complete catalog of audit log event types
// emitted by Massdriver — strings in dot notation (e.g.
// "project.created", "deployment.completed").
//
// The catalog is small and static, returned in a single response with
// no pagination. Useful for populating filter dropdowns or grouping
// events by category in a UI.
func (s *Service) ListEventTypes(ctx context.Context) ([]string, error) {
	resp, err := gen.ListAuditLogEventTypes(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list audit log event types: %w", err))
	}
	return resp.AuditLogEventTypes, nil
}

func toAuditLog(v any) (*AuditLog, error) {
	a := AuditLog{}
	if err := decode.Decode(v, &a); err != nil {
		return nil, fmt.Errorf("decode audit log: %w", err)
	}
	return &a, nil
}

func buildListFilter(input ListInput) *gen.AuditLogsFilter {
	filter := &gen.AuditLogsFilter{}
	set := false

	if !input.TimeRangeStart.IsZero() || !input.TimeRangeEnd.IsZero() {
		dt := &gen.DatetimeFilter{}
		if !input.TimeRangeStart.IsZero() {
			dt.Gte = input.TimeRangeStart
		}
		if !input.TimeRangeEnd.IsZero() {
			dt.Lte = input.TimeRangeEnd
		}
		filter.OccurredAt = dt
		set = true
	}
	if input.Type != "" {
		filter.Type = &gen.StringFilter{Eq: input.Type}
		set = true
	}
	if len(input.ActorTypes) > 0 {
		in := make([]gen.AuditLogActorType, 0, len(input.ActorTypes))
		for _, t := range input.ActorTypes {
			in = append(in, gen.AuditLogActorType(t))
		}
		filter.ActorType = &gen.AuditLogActorTypeFilter{In: in}
		set = true
	}
	if input.ActorID != "" {
		filter.ActorId = &gen.IdFilter{Eq: input.ActorID}
		set = true
	}
	if input.ActorSearch != "" {
		filter.Actor = &gen.ActorSearchFilter{Search: input.ActorSearch}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.AuditLogsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.AuditLogsSortFieldOccurredAt
	if input.SortBy == SortByType {
		field = gen.AuditLogsSortFieldType
	}
	order := gen.SortOrderDesc
	if input.SortOrder == SortAsc {
		order = gen.SortOrderAsc
	}
	return &gen.AuditLogsSort{Field: field, Order: order}
}
