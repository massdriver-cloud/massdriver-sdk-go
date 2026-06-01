package instances_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func TestGetAlarm(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"instanceAlarm": map[string]any{
				"id":                 "alarm-1",
				"displayName":        "High CPU",
				"cloudResourceId":    "arn:aws:cloudwatch:alarm/foo",
				"comparisonOperator": "GREATER_THAN",
				"threshold":          80.0,
				"period":             300,
				"metric": map[string]any{
					"namespace":  "AWS/RDS",
					"name":       "CPUUtilization",
					"statistic":  "Average",
					"dimensions": []map[string]any{{"name": "DBInstanceIdentifier", "value": "db-abc"}},
				},
				"currentState": map[string]any{
					"id":      "state-1",
					"status":  "OK",
					"message": "back to normal",
				},
			},
		}),
	)

	got, err := newService(gqlClient).GetAlarm(t.Context(), "alarm-1")
	if err != nil {
		t.Fatalf("GetAlarm: %v", err)
	}
	if got.DisplayName != "High CPU" {
		t.Errorf("DisplayName = %q, want High CPU", got.DisplayName)
	}
	if got.Threshold != 80.0 {
		t.Errorf("Threshold = %v, want 80", got.Threshold)
	}
	if got.Metric == nil || got.Metric.Name != "CPUUtilization" {
		t.Errorf("Metric = %+v, want name CPUUtilization", got.Metric)
	}
	if len(got.Metric.Dimensions) != 1 || got.Metric.Dimensions[0].Value != "db-abc" {
		t.Errorf("Dimensions = %+v, want one with value db-abc", got.Metric.Dimensions)
	}
	if got.CurrentState == nil || got.CurrentState.Status != "OK" {
		t.Errorf("CurrentState = %+v, want status OK", got.CurrentState)
	}
}

// TestGetAlarm_NotFound confirms the wrapper surfaces gql.ErrNotFound
// when the API returns null for a missing alarm — the schema's
// `instanceAlarm` field is nullable, so a 404 manifests as a
// zero-valued struct on the wire.
func TestGetAlarm_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"instanceAlarm": nil,
		}),
	)
	_, err := newService(gqlClient).GetAlarm(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestListAlarms_FilterByInstance(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"instanceAlarms": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "alarm-1", "displayName": "High CPU"},
					{"id": "alarm-2", "displayName": "Low Memory"},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).IterAlarms(t.Context(), instances.ListAlarmsInput{
		InstanceID: "ecomm-prod-database",
	}))
	if err != nil {
		t.Fatalf("ListAlarms: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d alarms, want 2", len(got))
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	instID, _ := filter["instanceId"].(map[string]any)
	if instID["eq"] != "ecomm-prod-database" {
		t.Errorf("filter.instanceId.eq = %v, want ecomm-prod-database", instID["eq"])
	}
}

func TestCreateAlarm(t *testing.T) {
	threshold := 80.0
	period := 300

	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createInstanceAlarm": map[string]any{
				"result": map[string]any{
					"id":              "alarm-new",
					"displayName":     "High CPU",
					"cloudResourceId": "arn:aws:cloudwatch:alarm/new",
					"threshold":       80.0,
					"period":          300,
					"metric": map[string]any{
						"namespace": "AWS/RDS",
						"name":      "CPUUtilization",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).CreateAlarm(t.Context(), "ecomm-prod-database", instances.CreateAlarmInput{
		CloudResourceID:    "arn:aws:cloudwatch:alarm/new",
		DisplayName:        "High CPU",
		ComparisonOperator: "GREATER_THAN",
		Threshold:          &threshold,
		Period:             &period,
		Metric: &instances.AlarmMetric{
			Namespace:  "AWS/RDS",
			Name:       "CPUUtilization",
			Statistic:  "Average",
			Dimensions: []instances.AlarmMetricDimension{{Name: "DBInstanceIdentifier", Value: "db-abc"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateAlarm: %v", err)
	}
	if got.ID != "alarm-new" {
		t.Errorf("ID = %q, want alarm-new", got.ID)
	}

	// Verify nested AlarmMetric reaches the wire.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["instanceId"] != "ecomm-prod-database" {
		t.Errorf("instanceId variable = %v, want ecomm-prod-database", reqs[0].Variables["instanceId"])
	}
	input, _ := reqs[0].Variables["input"].(map[string]any)
	metric, _ := input["metric"].(map[string]any)
	if metric["name"] != "CPUUtilization" {
		t.Errorf("input.metric.name = %v, want CPUUtilization", metric["name"])
	}
	dims, _ := metric["dimensions"].([]any)
	if len(dims) != 1 {
		t.Fatalf("input.metric.dimensions len = %d, want 1", len(dims))
	}
}

func TestUpdateAlarm(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateInstanceAlarm": map[string]any{
				"result":     map[string]any{"id": "alarm-1", "displayName": "High CPU (revised)"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).UpdateAlarm(t.Context(), "alarm-1", instances.UpdateAlarmInput{
		DisplayName: "High CPU (revised)",
	})
	if err != nil {
		t.Fatalf("UpdateAlarm: %v", err)
	}
	if got.DisplayName != "High CPU (revised)" {
		t.Errorf("DisplayName = %q, want High CPU (revised)", got.DisplayName)
	}
}

func TestDeleteAlarm(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteInstanceAlarm": map[string]any{
				"result":     map[string]any{"id": "alarm-1", "displayName": "High CPU"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).DeleteAlarm(t.Context(), "alarm-1")
	if err != nil {
		t.Fatalf("DeleteAlarm: %v", err)
	}
	if got.ID != "alarm-1" {
		t.Errorf("ID = %q, want alarm-1", got.ID)
	}
}

func TestCreateAlarm_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createInstanceAlarm": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "required", "field": "displayName", "message": "displayName is required"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).CreateAlarm(t.Context(), "ecomm-prod-database", instances.CreateAlarmInput{
		CloudResourceID: "arn:foo",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T", err)
	}
	if mf.Op != "create instance alarm" {
		t.Errorf("Op = %q, want create instance alarm", mf.Op)
	}
}

// TestIterAlarms_StopsEarly confirms that breaking out of the range loop
// stops further page requests — the iterator is lazy.
func TestIterAlarms_StopsEarly(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"instanceAlarms": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "alarm-1", "displayName": "High CPU"},
				{"id": "alarm-2", "displayName": "Low Memory"},
			},
		},
	})
	// page2 would only be requested if the iterator follows the cursor.
	page2 := gqltest.RespondWithData(map[string]any{
		"instanceAlarms": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	count := 0
	for a, err := range newService(gqlClient).IterAlarms(t.Context(), instances.ListAlarmsInput{}) {
		if err != nil {
			t.Fatalf("iter err: %v", err)
		}
		count++
		if a.ID == "alarm-1" {
			break // caller bails after the first alarm
		}
	}

	if count != 1 {
		t.Errorf("iterated %d items, want 1 (caller bailed after alarm-1)", count)
	}
	// Critical invariant: page2 was never requested because we stopped early.
	if got := len(gqlClient.Requests()); got != 1 {
		t.Errorf("issued %d requests, want 1 — iterator should not pre-fetch", got)
	}
	if pending := gqlClient.Pending(); pending != 1 {
		t.Errorf("expected 1 unconsumed mock response (page2), got %d", pending)
	}
}

// TestIterAlarms_TransportErrorYieldsOnce confirms a transport error is
// surfaced through the yielded error and the iterator stops.
func TestIterAlarms_TransportErrorYieldsOnce(t *testing.T) {
	wantErr := errors.New("dial tcp: refused")
	gqlClient := gqltest.NewClient(gqltest.RespondWithTransportError(wantErr))

	var observed []error
	for _, err := range newService(gqlClient).IterAlarms(t.Context(), instances.ListAlarmsInput{}) {
		observed = append(observed, err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d yields, want 1", len(observed))
	}
	if !errors.Is(observed[0], wantErr) {
		t.Errorf("err = %v, want it to wrap %v", observed[0], wantErr)
	}
}
