// Package eventdestinations provides operations for Massdriver event
// destinations: places outside Massdriver, such as an HTTP endpoint, that
// receive the organization's audit log events.
//
// A destination selects events by event type and entity attributes, and can
// drop more of them with a jq condition. [Service.Preview] runs the same
// selection over recent audit log events, so a selection can be checked
// before a destination uses it. [Service.ListEventTypes] lists the event
// catalog, and [Service.ListAdapters] lists the ways events can be sent with
// the JSON schemas of each adapter's config and auth.
//
// Auth credentials are write-only: the API never returns them.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.EventDestinations] field on the top-level SDK
// client.
package eventdestinations

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/filters"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// EventDestination is a Massdriver event destination — alias of
// [types.EventDestination].
type EventDestination = types.EventDestination

// Delivery is one event's delivery to a destination — alias of
// [types.EventDelivery].
type Delivery = types.EventDelivery

// Adapter is a way to send events — alias of
// [types.EventDestinationAdapter].
type Adapter = types.EventDestinationAdapter

// EventType is an entry in the event catalog — alias of [types.EventType].
type EventType = types.EventType

// AuditLog is an audit log event — alias of [types.AuditLog].
type AuditLog = types.AuditLog

// Service is the receiver for event destination operations. Construct with
// [New]; for the typical case you'll use the
// [massdriver.Client.EventDestinations] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// AdapterHTTP posts each event to a URL as a CloudEvent, signed with
// Standard Webhooks headers. Its config takes `url` and `headers`, and its
// auth takes secret `headers`.
const AdapterHTTP = "HTTP"

// Status is whether a destination receives events.
type Status string

const (
	// StatusActive destinations receive events.
	StatusActive Status = "ACTIVE"
	// StatusPaused destinations send nothing until they are resumed. Events
	// recorded while paused wait and are sent after the resume. Massdriver
	// pauses a destination on its own after a period of failed sends.
	StatusPaused Status = "PAUSED"
)

// DeliveryStatus is the state of one event's delivery.
type DeliveryStatus string

const (
	// DeliveryDelivered means the destination accepted the event.
	DeliveryDelivered DeliveryStatus = "DELIVERED"
	// DeliverySkipped means the destination's condition did not pass the
	// event, so it was not sent.
	DeliverySkipped DeliveryStatus = "SKIPPED"
	// DeliveryFailed means no more attempts will run. [Service.Redeliver]
	// sends it again.
	DeliveryFailed DeliveryStatus = "FAILED"
	// DeliveryPending means the event waits to be sent or retried.
	DeliveryPending DeliveryStatus = "PENDING"
)

// SortField is the field a [Service.Iter] result can be ordered by.
type SortField string

const (
	SortByName      SortField = "NAME"
	SortByCreatedAt SortField = "CREATED_AT"
)

// DeliverySortField is the field a [Service.IterDeliveries] result can be
// ordered by.
type DeliverySortField string

const (
	DeliverySortByCreatedAt DeliverySortField = "CREATED_AT"
	DeliverySortByUpdatedAt DeliverySortField = "UPDATED_AT"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.Iter] call. The zero value lists every
// destination, sorted by name ascending.
type ListInput struct {
	// Statuses limits results to destinations with any of these statuses.
	Statuses []Status
	SortBy   SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder
	PageSize  int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// ListDeliveriesInput controls a [Service.IterDeliveries] call. The zero
// value lists every delivery the destination has kept, newest first.
type ListDeliveriesInput struct {
	// Statuses limits results to deliveries with any of these statuses.
	Statuses []DeliveryStatus
	// CreatedAfter and CreatedBefore narrow deliveries to a window of when
	// the event was given to the destination. Both inclusive; either may be
	// zero to leave the bound open.
	CreatedAfter  time.Time
	CreatedBefore time.Time
	SortBy        DeliverySortField
	// SortOrder controls sort direction. Empty = DESC (newest first).
	SortOrder SortOrder
	PageSize  int
	// After is the opaque cursor from a prior [types.Page].Next.
	After string
}

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// Name is unique in the organization.
	Name string
	// Adapter says how the destination receives events, e.g. [AdapterHTTP].
	Adapter string
	// Config is where and how events are sent, in the shape of the
	// adapter's ConfigSchema. The API returns it, so keep credentials in
	// Auth.
	Config map[string]any
	// Auth holds credentials in the shape of the adapter's AuthSchema. The
	// API never returns it. Nil sends none.
	Auth map[string]any
	// EventTypes are event type ids, or `<entity>.*` for every event of an
	// entity. Empty receives every event.
	EventTypes []string
	// Attributes are values the event's entity must have had, such as
	// {"md-environment": "prod"}. Empty receives events of any attributes.
	Attributes map[string]string
	// Condition is a jq expression run on each selected CloudEvent. The
	// event is sent only when it outputs true. Empty sends every selected
	// event.
	Condition string
}

// UpdateInput is the input for [Service.Update]. Nil fields keep their
// current values. The adapter cannot change.
type UpdateInput struct {
	Name *string
	// Config replaces the current config.
	Config map[string]any
	// Auth replaces the current credentials. An empty, non-nil map removes
	// them.
	Auth map[string]any
	// EventTypes replaces the current event types. A pointer to an empty
	// slice receives every event.
	EventTypes *[]string
	// Attributes replaces the current attributes. An empty, non-nil map
	// receives events of any attributes.
	Attributes map[string]string
	// Condition replaces the current condition. A pointer to "" removes it.
	Condition *string
}

// PreviewInput is the input for [Service.IterPreview] and
// [Service.PreviewPage]. It takes the same selection as [CreateInput].
type PreviewInput struct {
	EventTypes []string
	Attributes map[string]string
	Condition  string
	PageSize   int
	// After is the opaque cursor from a prior [types.Page].Next.
	After string
}

// ListEventTypesInput controls a [Service.ListEventTypes] call. The zero
// value lists the whole catalog.
type ListEventTypesInput struct {
	// Entity limits results to the event types of one entity, e.g.
	// "deployment".
	Entity string
}

// Get retrieves a destination by ID.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// destination with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*EventDestination, error) {
	resp, err := gen.GetEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get event destination %s: %w", id, err))
	}
	if resp.EventDestination.Id == "" {
		return nil, fmt.Errorf("get event destination %s: %w", id, gql.ErrNotFound)
	}
	return toEventDestination(resp.EventDestination)
}

// Iter returns a lazy [iter.Seq2] over destinations matching input, fetching
// pages on demand. The yielded error is non-nil exactly once, on a failed
// page fetch, after which iteration stops.
//
// To buffer every match into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[EventDestination, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of destinations matching input.
// input.PageSize bounds the page and input.After (an opaque cursor from a
// prior page's Next) selects which page.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[EventDestination], error) {
	return s.page(input)(ctx, input.After)
}

func (s *Service) page(input ListInput) paging.FetchFunc[EventDestination] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	return paging.DecodeFetch[EventDestination](input.PageSize, "event destination", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListEventDestinations(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list event destinations: %w", err))
		}
		return paging.RawPage{
			Items:    resp.EventDestinations.Items,
			Next:     resp.EventDestinations.Cursor.Next,
			Previous: resp.EventDestinations.Cursor.Previous,
		}, nil
	})
}

// IterDeliveries returns a lazy [iter.Seq2] over a destination's deliveries
// matching input, fetching pages on demand.
func (s *Service) IterDeliveries(ctx context.Context, id string, input ListDeliveriesInput) iter.Seq2[Delivery, error] {
	return paging.Iter(ctx, input.After, s.deliveriesPage(id, input))
}

// ListDeliveriesPage returns a single page of a destination's deliveries.
//
// Returns [gql.ErrNotFound] (wrapped) when the destination does not exist.
func (s *Service) ListDeliveriesPage(ctx context.Context, id string, input ListDeliveriesInput) (types.Page[Delivery], error) {
	return s.deliveriesPage(id, input)(ctx, input.After)
}

func (s *Service) deliveriesPage(id string, input ListDeliveriesInput) paging.FetchFunc[Delivery] {
	filter := buildDeliveriesFilter(input)
	sort := buildDeliveriesSort(input)
	return paging.DecodeFetch[Delivery](input.PageSize, "event delivery", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListEventDeliveries(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, filter, sort, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list deliveries for event destination %s: %w", id, err))
		}
		if resp.EventDestination.Id == "" {
			return paging.RawPage{}, fmt.Errorf("list deliveries for event destination %s: %w", id, gql.ErrNotFound)
		}
		return paging.RawPage{
			Items:    resp.EventDestination.Deliveries.Items,
			Next:     resp.EventDestination.Deliveries.Cursor.Next,
			Previous: resp.EventDestination.Deliveries.Cursor.Previous,
		}, nil
	})
}

// IterPreview returns a lazy [iter.Seq2] over the organization's recent
// audit log events that input's event types, attributes, and condition
// select, newest first.
//
// The condition runs on each page after the selection, so a page can hold
// fewer items than PageSize. If the condition cannot run on an event, the
// preview returns an error that names it.
func (s *Service) IterPreview(ctx context.Context, input PreviewInput) iter.Seq2[AuditLog, error] {
	return paging.Iter(ctx, input.After, s.previewPage(input))
}

// PreviewPage returns a single page of [Service.IterPreview].
func (s *Service) PreviewPage(ctx context.Context, input PreviewInput) (types.Page[AuditLog], error) {
	return s.previewPage(input)(ctx, input.After)
}

func (s *Service) previewPage(input PreviewInput) paging.FetchFunc[AuditLog] {
	attributes := attributesMap(input.Attributes)
	return paging.DecodeFetch[AuditLog](input.PageSize, "event destination preview", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.PreviewEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, input.EventTypes, attributes, input.Condition, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("preview event destination: %w", err))
		}
		return paging.RawPage{
			Items:    resp.PreviewEventDestination.Items,
			Next:     resp.PreviewEventDestination.Cursor.Next,
			Previous: resp.PreviewEventDestination.Cursor.Previous,
		}, nil
	})
}

// ListAdapters returns the ways events can be sent, with the JSON schemas
// of each adapter's config and auth.
func (s *Service) ListAdapters(ctx context.Context) ([]Adapter, error) {
	resp, err := gen.ListEventDestinationAdapters(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list event destination adapters: %w", err))
	}
	adapters := []Adapter{}
	if err := decode.Decode(resp.EventDestinationAdapters.Items, &adapters); err != nil {
		return nil, fmt.Errorf("decode event destination adapters: %w", err)
	}
	return adapters, nil
}

// ListEventTypes returns the event catalog, sorted by id. The catalog is
// small and returned in one response.
func (s *Service) ListEventTypes(ctx context.Context, input ListEventTypesInput) ([]EventType, error) {
	var filter *gen.EventTypesFilter
	if input.Entity != "" {
		filter = &gen.EventTypesFilter{Entity: &gen.StringFilter{Eq: input.Entity}}
	}
	resp, err := gen.ListEventTypes(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list event types: %w", err))
	}
	eventTypes := []EventType{}
	if err := decode.Decode(resp.EventTypes, &eventTypes); err != nil {
		return nil, fmt.Errorf("decode event types: %w", err)
	}
	return eventTypes, nil
}

// Create adds a destination. The result carries the destination's
// SigningSecret when the adapter signs.
//
// Returns a [*gql.MutationFailedError] (wrapped) if the server reports
// `successful: false`.
func (s *Service) Create(ctx context.Context, input CreateInput) (*EventDestination, error) {
	resp, err := gen.CreateEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateEventDestinationInput{
		Name:       input.Name,
		Adapter:    gen.EventDestinationAdapter(input.Adapter),
		Config:     input.Config,
		Auth:       input.Auth,
		EventTypes: input.EventTypes,
		Attributes: attributesMap(input.Attributes),
		Condition:  input.Condition,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create event destination: %w", err))
	}
	if err := gql.CheckMutation("create event destination", resp.CreateEventDestination.Successful, resp.CreateEventDestination.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.CreateEventDestination.Result)
}

// Update changes a destination's name, config, auth, event types,
// attributes, or condition. New event types and attributes apply to events
// recorded after the change.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*EventDestination, error) {
	resp, err := gen.UpdateEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateEventDestinationInput{
		Name:       input.Name,
		Config:     input.Config,
		Auth:       input.Auth,
		EventTypes: input.EventTypes,
		Attributes: attributesMap(input.Attributes),
		Condition:  input.Condition,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update event destination %s: %w", id, err))
	}
	if err := gql.CheckMutation("update event destination", resp.UpdateEventDestination.Successful, resp.UpdateEventDestination.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.UpdateEventDestination.Result)
}

// Pause stops sending events to a destination. Events recorded while it is
// paused wait and are sent after [Service.Resume].
func (s *Service) Pause(ctx context.Context, id string) (*EventDestination, error) {
	resp, err := gen.PauseEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("pause event destination %s: %w", id, err))
	}
	if err := gql.CheckMutation("pause event destination", resp.PauseEventDestination.Successful, resp.PauseEventDestination.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.PauseEventDestination.Result)
}

// Resume starts sending events to a paused destination again, beginning
// with the events that waited.
func (s *Service) Resume(ctx context.Context, id string) (*EventDestination, error) {
	resp, err := gen.ResumeEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("resume event destination %s: %w", id, err))
	}
	if err := gql.CheckMutation("resume event destination", resp.ResumeEventDestination.Successful, resp.ResumeEventDestination.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.ResumeEventDestination.Result)
}

// Delete deletes a destination and its deliveries. Events that wait to be
// sent are not sent. The result carries only the ID and Name.
func (s *Service) Delete(ctx context.Context, id string) (*EventDestination, error) {
	resp, err := gen.DeleteEventDestination(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete event destination %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete event destination", resp.DeleteEventDestination.Successful, resp.DeleteEventDestination.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.DeleteEventDestination.Result)
}

// Redeliver sends a destination's FAILED deliveries again, with a new set
// of retries. Empty deliveryIDs sends every failed delivery again.
// Deliveries that are not FAILED are left as they are. A paused destination
// sends them after it is resumed. The result carries only the ID and Name.
func (s *Service) Redeliver(ctx context.Context, id string, deliveryIDs []string) (*EventDestination, error) {
	resp, err := gen.RedeliverEvents(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, deliveryIDs)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("redeliver events for event destination %s: %w", id, err))
	}
	if err := gql.CheckMutation("redeliver events", resp.RedeliverEvents.Successful, resp.RedeliverEvents.Messages); err != nil {
		return nil, err
	}
	return toEventDestination(resp.RedeliverEvents.Result)
}

func toEventDestination(v any) (*EventDestination, error) {
	d := EventDestination{}
	if err := decode.Decode(v, &d); err != nil {
		return nil, fmt.Errorf("decode event destination: %w", err)
	}
	return &d, nil
}

// attributesMap widens attributes to the Map scalar's Go type, keeping nil
// as nil so an unset field stays off the wire.
func attributesMap(attributes map[string]string) map[string]any {
	if attributes == nil {
		return nil
	}
	out := make(map[string]any, len(attributes))
	for k, v := range attributes {
		out[k] = v
	}
	return out
}

func buildListFilter(input ListInput) *gen.EventDestinationsFilter {
	if len(input.Statuses) == 0 {
		return nil
	}
	in := make([]gen.EventDestinationStatus, 0, len(input.Statuses))
	for _, st := range input.Statuses {
		in = append(in, gen.EventDestinationStatus(st))
	}
	return &gen.EventDestinationsFilter{Status: &gen.EventDestinationStatusFilter{In: in}}
}

func buildListSort(input ListInput) *gen.EventDestinationsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.EventDestinationsSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.EventDestinationsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.EventDestinationsSort{Field: field, Order: order}
}

func buildDeliveriesFilter(input ListDeliveriesInput) *gen.EventDeliveriesFilter {
	filter := &gen.EventDeliveriesFilter{}
	set := false
	if len(input.Statuses) > 0 {
		in := make([]gen.EventDeliveryStatus, 0, len(input.Statuses))
		for _, st := range input.Statuses {
			in = append(in, gen.EventDeliveryStatus(st))
		}
		filter.Status = &gen.EventDeliveryStatusFilter{In: in}
		set = true
	}
	if !input.CreatedAfter.IsZero() || !input.CreatedBefore.IsZero() {
		filter.CreatedAt = filters.Datetime(input.CreatedAfter, input.CreatedBefore)
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildDeliveriesSort(input ListDeliveriesInput) *gen.EventDeliveriesSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.EventDeliveriesSortFieldCreatedAt
	if input.SortBy == DeliverySortByUpdatedAt {
		field = gen.EventDeliveriesSortFieldUpdatedAt
	}
	order := gen.SortOrderDesc
	if input.SortOrder == SortAsc {
		order = gen.SortOrderAsc
	}
	return &gen.EventDeliveriesSort{Field: field, Order: order}
}
