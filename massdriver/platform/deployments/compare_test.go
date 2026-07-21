package deployments_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
)

func TestCompare(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"compareDeployments": map[string]any{
				"source": map[string]any{"id": "dep-1", "status": "COMPLETED", "action": "PROVISION", "version": "1.0.0"},
				"target": map[string]any{"id": "dep-2", "status": "COMPLETED", "action": "PROVISION", "version": "1.1.0"},
				"version": map[string]any{
					"source": "1.0.0",
					"target": "1.1.0",
					"equal":  false,
				},
				"params": []map[string]any{
					{
						"path":   ".database.instance_type",
						"source": map[string]any{"present": true, "value": "db.t3.medium"},
						"target": map[string]any{"present": true, "value": "db.r5.large"},
						"equal":  false,
					},
					{
						"path":   ".database.port",
						"source": map[string]any{"present": true, "value": "5432"},
						"target": map[string]any{"present": true, "value": "5432"},
						"equal":  true,
					},
					{
						"path":   ".backup.enabled",
						"source": map[string]any{"present": false, "value": nil},
						"target": map[string]any{"present": true, "value": "true"},
						"equal":  false,
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Compare(t.Context(), "dep-1", "dep-2")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if got.Source.ID != "dep-1" || got.Target.ID != "dep-2" {
		t.Errorf("Source/Target IDs = %q/%q, want dep-1/dep-2", got.Source.ID, got.Target.ID)
	}
	if got.Version.Equal || got.Version.Source != "1.0.0" || got.Version.Target != "1.1.0" {
		t.Errorf("Version = %+v, want 1.0.0 -> 1.1.0, not equal", got.Version)
	}
	if len(got.Params) != 3 {
		t.Fatalf("Params len = %d, want 3", len(got.Params))
	}
	if got.Params[0].Path != ".database.instance_type" || got.Params[0].Equal {
		t.Errorf("Params[0] = %+v, want unequal .database.instance_type", got.Params[0])
	}
	// A key missing on one side must arrive Present=false, not empty-equal.
	if got.Params[2].Source.Present || !got.Params[2].Target.Present {
		t.Errorf("Params[2] presence = %+v, want source absent / target present", got.Params[2])
	}

	reqs := gqlClient.Requests()
	if reqs[0].OpName != "CompareDeployments" {
		t.Errorf("OpName = %q, want CompareDeployments", reqs[0].OpName)
	}
	if reqs[0].Variables["sourceId"] != "dep-1" || reqs[0].Variables["targetId"] != "dep-2" {
		t.Errorf("variables = %v, want sourceId=dep-1 targetId=dep-2", reqs[0].Variables)
	}
}

// TestCompare_NotFound confirms a null comparison (unknown deployment ID)
// surfaces gql.ErrNotFound.
func TestCompare_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"compareDeployments": nil,
		}),
	)

	_, err := newService(gqlClient).Compare(t.Context(), "missing", "dep-2")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}
