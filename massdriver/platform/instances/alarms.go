package instances

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Alarm is a cloud metric alarm attached to an instance — alias of
// [types.Alarm].
type Alarm = types.Alarm

// AlarmMetric describes the cloud metric an alarm evaluates — alias of
// [types.AlarmMetric].
type AlarmMetric = types.AlarmMetric

// AlarmMetricDimension is one dimension on an [AlarmMetric] — alias of
// [types.AlarmMetricDimension].
type AlarmMetricDimension = types.AlarmMetricDimension

// AlarmStatus is whether an alarm is firing or clear.
type AlarmStatus string

const (
	AlarmStatusOK    AlarmStatus = "OK"
	AlarmStatusAlarm AlarmStatus = "ALARM"
)

// AlarmSortField is the field a [Service.ListAlarms] result can be ordered by.
type AlarmSortField string

const (
	AlarmSortByDisplayName AlarmSortField = "DISPLAY_NAME"
	AlarmSortByCreatedAt   AlarmSortField = "CREATED_AT"
)

// ListAlarmsInput controls a [Service.ListAlarms] call. Zero value lists every alarm
// the caller can see across all instances.
type ListAlarmsInput struct {
	ProjectID     string
	EnvironmentID string
	ComponentID   string
	InstanceID    string
	OciRepoName   string

	SortBy    AlarmSortField
	SortOrder SortOrder

	PageSize int
}

// CreateAlarmInput is the input for [Service.CreateAlarm].
type CreateAlarmInput struct {
	// CloudResourceID is the cloud provider's unique identifier for the
	// alarm — used to correlate inbound webhooks back to this record.
	CloudResourceID string
	// DisplayName is the human-readable name shown in the UI/notifications.
	DisplayName string

	// ComparisonOperator (e.g. GREATER_THAN). Optional; not all providers expose this.
	ComparisonOperator string
	// Threshold is the value crossed to trigger the alarm. Optional.
	Threshold *float64
	// Period is the evaluation window in seconds. Optional.
	Period *int

	// Metric describes the cloud metric being evaluated. Optional.
	Metric *AlarmMetric
}

// UpdateAlarmInput is the input for [Service.UpdateAlarm]. All fields are optional;
// pointer fields distinguish "leave unchanged" (nil) from "set to zero
// value" (non-nil with zero).
type UpdateAlarmInput struct {
	CloudResourceID    string
	DisplayName        string
	ComparisonOperator string
	Threshold          *float64
	Period             *int
	Metric             *AlarmMetric
}

// GetAlarm retrieves an alarm by ID.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no alarm
// with the given ID exists in the configured organization.
func (s *Service) GetAlarm(ctx context.Context, id string) (*Alarm, error) {
	resp, err := gen.GetInstanceAlarm(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get instance alarm %s: %w", id, err))
	}
	if resp.InstanceAlarm.Id == "" {
		return nil, fmt.Errorf("get instance alarm %s: %w", id, gql.ErrNotFound)
	}
	return toAlarm(resp.InstanceAlarm)
}

// IterAlarms returns a [iter.Seq2] over alarms matching the supplied
// filters, fetching pages lazily on demand. Use this for large or
// potentially-unbounded result sets where buffering every match in
// memory ([Service.ListAlarms]) is impractical.
//
// The yielded error is non-nil exactly when the underlying transport
// or decode failed. Stop iterating after observing one — subsequent
// items are not produced.
//
// Cancellation is via ctx; break out of the range loop to stop
// requesting further pages.
//
// Example:
//
//	for a, err := range svc.IterAlarms(ctx, instances.ListAlarmsInput{InstanceID: "ecomm-prod-database"}) {
//	    if err != nil { return err }
//	    process(a)
//	}
func (s *Service) IterAlarms(ctx context.Context, input ListAlarmsInput) iter.Seq2[Alarm, error] {
	filter := buildAlarmsListFilter(input)
	sort := buildAlarmsListSort(input)

	return func(yield func(Alarm, error) bool) {
		var cursor *scalars.Cursor
		if input.PageSize > 0 {
			cursor = &scalars.Cursor{Limit: input.PageSize}
		}
		for {
			resp, err := gen.ListInstanceAlarms(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
			if err != nil {
				yield(Alarm{}, gql.ClassifyError(fmt.Errorf("list instance alarms: %w", err)))
				return
			}
			for _, item := range resp.InstanceAlarms.Items {
				a, aerr := toAlarm(item)
				if aerr != nil {
					yield(Alarm{}, fmt.Errorf("decode instance alarm: %w", aerr))
					return
				}
				if !yield(*a, nil) {
					return
				}
			}
			next := resp.InstanceAlarms.Cursor.Next
			if next == "" {
				return
			}
			cursor = &scalars.Cursor{Next: next}
			if input.PageSize > 0 {
				cursor.Limit = input.PageSize
			}
		}
	}
}

// ListAlarms returns every alarm matching the supplied filters across all
// instances the caller can see, following pagination cursors automatically
// and buffering every match into a single slice.
//
// To list alarms for a specific instance, set ListAlarmsInput.InstanceID;
// for a project or environment, set ProjectID / EnvironmentID. The metric
// shape is not selected on list to keep page payloads small — call [Service.GetAlarm]
// for full per-alarm metric details.
//
// For large result sets — anything where the full match could be tens
// of thousands of rows — prefer [Service.IterAlarms], which yields one
// alarm at a time. Cancel ctx to stop early.
func (s *Service) ListAlarms(ctx context.Context, input ListAlarmsInput) ([]Alarm, error) {
	filter := buildAlarmsListFilter(input)
	sort := buildAlarmsListSort(input)

	var (
		out    []Alarm
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListInstanceAlarms(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list instance alarms: %w", err))
		}
		for _, item := range resp.InstanceAlarms.Items {
			a, aerr := toAlarm(item)
			if aerr != nil {
				return nil, fmt.Errorf("decode instance alarm: %w", aerr)
			}
			out = append(out, *a)
		}
		next := resp.InstanceAlarms.Cursor.Next
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

// CreateAlarm registers a cloud metric alarm with an instance. The alarm
// appears in the UI immediately and starts receiving state transitions as
// soon as the cloud provider reports them.
func (s *Service) CreateAlarm(ctx context.Context, instanceID string, input CreateAlarmInput) (*Alarm, error) {
	resp, err := gen.CreateInstanceAlarm(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, gen.CreateInstanceAlarmInput{
		CloudResourceId:    input.CloudResourceID,
		DisplayName:        input.DisplayName,
		ComparisonOperator: input.ComparisonOperator,
		Threshold:          input.Threshold,
		Period:             input.Period,
		Metric:             toAlarmMetricInput(input.Metric),
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create instance %s alarm: %w", instanceID, err))
	}
	if err := gql.CheckMutation("create instance alarm", resp.CreateInstanceAlarm.Successful, resp.CreateInstanceAlarm.Messages); err != nil {
		return nil, err
	}
	return toAlarm(resp.CreateInstanceAlarm.Result)
}

// UpdateAlarm updates a registered alarm's mutable fields. Empty/nil fields
// in input are left unchanged.
func (s *Service) UpdateAlarm(ctx context.Context, id string, input UpdateAlarmInput) (*Alarm, error) {
	resp, err := gen.UpdateInstanceAlarm(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateInstanceAlarmInput{
		CloudResourceId:    input.CloudResourceID,
		DisplayName:        input.DisplayName,
		ComparisonOperator: input.ComparisonOperator,
		Threshold:          input.Threshold,
		Period:             input.Period,
		Metric:             toAlarmMetricInput(input.Metric),
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update instance alarm %s: %w", id, err))
	}
	if err := gql.CheckMutation("update instance alarm", resp.UpdateInstanceAlarm.Successful, resp.UpdateInstanceAlarm.Messages); err != nil {
		return nil, err
	}
	return toAlarm(resp.UpdateInstanceAlarm.Result)
}

// DeleteAlarm removes an alarm registration. The underlying cloud provider
// alarm is unaffected — this only removes Massdriver's record of it.
func (s *Service) DeleteAlarm(ctx context.Context, id string) (*Alarm, error) {
	resp, err := gen.DeleteInstanceAlarm(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete instance alarm %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete instance alarm", resp.DeleteInstanceAlarm.Successful, resp.DeleteInstanceAlarm.Messages); err != nil {
		return nil, err
	}
	return toAlarm(resp.DeleteInstanceAlarm.Result)
}

func toAlarm(v any) (*Alarm, error) {
	a := Alarm{}
	if err := decode.Decode(v, &a); err != nil {
		return nil, fmt.Errorf("decode alarm: %w", err)
	}
	return &a, nil
}

func toAlarmMetricInput(m *AlarmMetric) *gen.AlarmMetricInput {
	if m == nil {
		return nil
	}
	dims := make([]gen.AlarmMetricDimensionInput, 0, len(m.Dimensions))
	for _, d := range m.Dimensions {
		dims = append(dims, gen.AlarmMetricDimensionInput{Name: d.Name, Value: d.Value})
	}
	return &gen.AlarmMetricInput{
		Namespace:  m.Namespace,
		Name:       m.Name,
		Statistic:  m.Statistic,
		Region:     m.Region,
		Dimensions: dims,
	}
}

func buildAlarmsListFilter(input ListAlarmsInput) *gen.InstanceAlarmsFilter {
	filter := &gen.InstanceAlarmsFilter{}
	set := false
	if input.ProjectID != "" {
		filter.ProjectId = &gen.IdFilter{Eq: input.ProjectID}
		set = true
	}
	if input.EnvironmentID != "" {
		filter.EnvironmentId = &gen.IdFilter{Eq: input.EnvironmentID}
		set = true
	}
	if input.ComponentID != "" {
		filter.ComponentId = &gen.IdFilter{Eq: input.ComponentID}
		set = true
	}
	if input.InstanceID != "" {
		filter.InstanceId = &gen.IdFilter{Eq: input.InstanceID}
		set = true
	}
	if input.OciRepoName != "" {
		filter.OciRepoName = &gen.OciRepoNameFilter{Eq: input.OciRepoName}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildAlarmsListSort(input ListAlarmsInput) *gen.InstanceAlarmsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.InstanceAlarmsSortFieldDisplayName
	if input.SortBy == AlarmSortByCreatedAt {
		field = gen.InstanceAlarmsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.InstanceAlarmsSort{Field: field, Order: order}
}
