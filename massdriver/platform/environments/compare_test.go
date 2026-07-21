package environments_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
)

func TestCompare(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"compareEnvironments": map[string]any{
				"source": map[string]any{"id": "ecomm-staging", "name": "Staging"},
				"target": map[string]any{"id": "ecomm-prod", "name": "Production"},
				"instances": []map[string]any{
					{
						"component": map[string]any{"id": "ecomm-database", "name": "Primary Database"},
						"source":    map[string]any{"id": "ecomm-staging-database", "status": "PROVISIONED", "resolvedVersion": "1.0.0"},
						"target":    map[string]any{"id": "ecomm-prod-database", "status": "PROVISIONED", "resolvedVersion": "1.1.0"},
						"version":   map[string]any{"source": "1.0.0", "target": "1.1.0", "equal": false},
						"params": []map[string]any{
							{
								"path":   ".database.instance_type",
								"source": map[string]any{"present": true, "value": "db.t3.medium"},
								"target": map[string]any{"present": true, "value": "db.r5.large"},
								"equal":  false,
							},
						},
						"equal": false,
					},
					{
						"component": map[string]any{"id": "ecomm-cache", "name": "Cache"},
						"source":    nil,
						"target":    map[string]any{"id": "ecomm-prod-cache", "status": "PROVISIONED", "resolvedVersion": "2.0.0"},
						"version":   map[string]any{"source": nil, "target": "2.0.0", "equal": false},
						"params":    []map[string]any{},
						"equal":     false,
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Compare(t.Context(), "ecomm-staging", "ecomm-prod")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if got.Source.ID != "ecomm-staging" || got.Target.ID != "ecomm-prod" {
		t.Errorf("Source/Target IDs = %q/%q, want ecomm-staging/ecomm-prod", got.Source.ID, got.Target.ID)
	}
	if len(got.Instances) != 2 {
		t.Fatalf("Instances len = %d, want 2", len(got.Instances))
	}

	db := got.Instances[0]
	if db.Component.ID != "ecomm-database" {
		t.Errorf("Instances[0].Component.ID = %q, want ecomm-database", db.Component.ID)
	}
	if db.Source == nil || db.Source.ID != "ecomm-staging-database" {
		t.Errorf("Instances[0].Source = %+v, want ecomm-staging-database", db.Source)
	}
	if db.Version.Source != "1.0.0" || db.Version.Target != "1.1.0" || db.Version.Equal {
		t.Errorf("Instances[0].Version = %+v, want 1.0.0 -> 1.1.0, not equal", db.Version)
	}
	if len(db.Params) != 1 || db.Params[0].Path != ".database.instance_type" {
		t.Errorf("Instances[0].Params = %+v, want one entry for .database.instance_type", db.Params)
	}

	// A component deployed on only one side must have a nil pointer on the
	// missing side — not a zero-valued Instance.
	cache := got.Instances[1]
	if cache.Source != nil {
		t.Errorf("Instances[1].Source = %+v, want nil (not deployed in staging)", cache.Source)
	}
	if cache.Target == nil || cache.Target.ID != "ecomm-prod-cache" {
		t.Errorf("Instances[1].Target = %+v, want ecomm-prod-cache", cache.Target)
	}

	reqs := gqlClient.Requests()
	if reqs[0].OpName != "CompareEnvironments" {
		t.Errorf("OpName = %q, want CompareEnvironments", reqs[0].OpName)
	}
	if reqs[0].Variables["sourceId"] != "ecomm-staging" || reqs[0].Variables["targetId"] != "ecomm-prod" {
		t.Errorf("variables = %v, want sourceId=ecomm-staging targetId=ecomm-prod", reqs[0].Variables)
	}
}

// TestCompare_NotFound confirms a null comparison (unknown environment ID)
// surfaces gql.ErrNotFound.
func TestCompare_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"compareEnvironments": nil,
		}),
	)

	_, err := newService(gqlClient).Compare(t.Context(), "missing", "ecomm-prod")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}
