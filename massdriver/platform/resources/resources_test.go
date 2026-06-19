package resources_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/resources"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *resources.Service {
	return resources.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resource": map[string]any{
				"id":      "res-1",
				"name":    "ci-deploy-role",
				"origin":  "IMPORTED",
				"field":   nil,
				"formats": []string{"json"},
				"payload": map[string]any{
					"arn":    "arn:aws:iam::123:role/ci",
					"secret": "[SENSITIVE]",
				},
				"attributes": map[string]any{"team": "platform"},
				"resourceType": map[string]any{
					"id":   "aws-iam-role",
					"name": "AWS IAM Role",
				},
				"instance": nil,
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "res-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Origin != string(resources.OriginImported) {
		t.Errorf("Origin = %q, want IMPORTED", got.Origin)
	}
	if got.Payload["secret"] != "[SENSITIVE]" {
		t.Errorf("Payload[secret] = %v, want [SENSITIVE] (masked)", got.Payload["secret"])
	}
	if got.ResourceType == nil || got.ResourceType.ID != "aws-iam-role" {
		t.Errorf("ResourceType = %+v, want ID aws-iam-role", got.ResourceType)
	}
	// Imported resource: Instance ref is nil.
	if got.Instance != nil {
		t.Errorf("Instance = %+v, want nil for imported resource", got.Instance)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing resource (the schema's `resource` field
// is nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resource": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList_FilterByOriginAndType(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resources": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "res-1", "name": "ci-role", "origin": "IMPORTED"},
				},
			},
		}),
	)

	_, err := types.Collect(newService(gqlClient).Iter(t.Context(), resources.ListInput{
		Origin:       resources.OriginImported,
		ResourceType: "aws-iam-role",
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	origin, _ := filter["origin"].(map[string]any)
	if origin["eq"] != "IMPORTED" {
		t.Errorf("filter.origin.eq = %v, want IMPORTED", origin["eq"])
	}
	rtype, _ := filter["resourceType"].(map[string]any)
	if rtype["eq"] != "aws-iam-role" {
		t.Errorf("filter.resourceType.eq = %v, want aws-iam-role", rtype["eq"])
	}
}

func TestList_AutoPaginates(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"resources": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items":  []map[string]any{{"id": "r-1", "name": "a"}},
		},
	})
	page2 := gqltest.RespondWithData(map[string]any{
		"resources": map[string]any{
			"cursor": map[string]any{},
			"items":  []map[string]any{{"id": "r-2", "name": "b"}},
		},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), resources.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 across two pages", len(got))
	}
}

// TestIter_StopsEarly confirms that breaking out of the range loop
// stops further page requests — the iterator is lazy.
func TestIter_StopsEarly(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"resources": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "res-1", "name": "a", "origin": "IMPORTED"},
				{"id": "res-2", "name": "b", "origin": "IMPORTED"},
			},
		},
	})
	// page2 would only be requested if the iterator follows the cursor.
	page2 := gqltest.RespondWithData(map[string]any{
		"resources": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	count := 0
	for r, err := range newService(gqlClient).Iter(t.Context(), resources.ListInput{}) {
		if err != nil {
			t.Fatalf("iter err: %v", err)
		}
		count++
		if r.ID == "res-1" {
			break // caller bails after the first resource
		}
	}

	if count != 1 {
		t.Errorf("iterated %d items, want 1 (caller bailed after res-1)", count)
	}
	// Critical invariant: page2 was never requested because we stopped early.
	if got := len(gqlClient.Requests()); got != 1 {
		t.Errorf("issued %d requests, want 1 — iterator should not pre-fetch", got)
	}
	if pending := gqlClient.Pending(); pending != 1 {
		t.Errorf("expected 1 unconsumed mock response (page2), got %d", pending)
	}
}

// TestIter_TransportErrorYieldsOnce confirms a transport error is
// surfaced through the yielded error and the iterator stops.
func TestIter_TransportErrorYieldsOnce(t *testing.T) {
	wantErr := errors.New("dial tcp: refused")
	gqlClient := gqltest.NewClient(gqltest.RespondWithTransportError(wantErr))

	var observed []error
	for _, err := range newService(gqlClient).Iter(t.Context(), resources.ListInput{}) {
		observed = append(observed, err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d yields, want 1", len(observed))
	}
	if !errors.Is(observed[0], wantErr) {
		t.Errorf("err = %v, want it to wrap %v", observed[0], wantErr)
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createResource": map[string]any{
				"result": map[string]any{
					"id":     "res-new",
					"name":   "CI/CD Role",
					"origin": "IMPORTED",
					"resourceType": map[string]any{
						"id":   "aws-iam-role",
						"name": "AWS IAM Role",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "aws-iam-role", resources.CreateInput{
		Name: "CI/CD Role",
		Payload: map[string]any{
			"arn": "arn:aws:iam::123:role/ci",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Origin != string(resources.OriginImported) {
		t.Errorf("Origin = %q, want IMPORTED", got.Origin)
	}

	reqs := gqlClient.Requests()
	if reqs[0].Variables["resourceTypeId"] != "aws-iam-role" {
		t.Errorf("resourceTypeId variable = %v, want aws-iam-role", reqs[0].Variables["resourceTypeId"])
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateResource": map[string]any{
				"result":     map[string]any{"id": "res-1", "name": "Renamed Role", "origin": "IMPORTED"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "res-1", resources.UpdateInput{
		Name: "Renamed Role",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Renamed Role" {
		t.Errorf("Name = %q, want Renamed Role", got.Name)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteResource": map[string]any{
				"result":     map[string]any{"id": "res-1", "name": "ci-role", "origin": "IMPORTED"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Delete(t.Context(), "res-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.ID != "res-1" {
		t.Errorf("ID = %q, want res-1", got.ID)
	}
}

func TestExport(t *testing.T) {
	// Export returns the unmasked payload + a server-rendered string.
	// In real usage this is audit-logged; the wrapper has no special
	// handling for that — the server records the access.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"exportResource": map[string]any{
				"result": map[string]any{
					"id":     "res-1",
					"name":   "ci-deploy-role",
					"origin": "IMPORTED",
					"payload": map[string]any{
						"arn":    "arn:aws:iam::123:role/ci",
						"secret": "actual-secret-value",
					},
					"rendered": `{"arn":"arn:aws:iam::123:role/ci","secret":"actual-secret-value"}`,
					"resourceType": map[string]any{
						"id":   "aws-iam-role",
						"name": "AWS IAM Role",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Export(t.Context(), "res-1", resources.FormatJSON)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// The unmasked secret value comes through (vs Get/List which would
	// have masked it as [SENSITIVE]).
	if got.Payload["secret"] != "actual-secret-value" {
		t.Errorf("Payload[secret] = %v, want actual-secret-value (unmasked)", got.Payload["secret"])
	}
	if got.Rendered == "" {
		t.Errorf("Rendered = empty, want non-empty serialized payload")
	}

	// Compile-time check: Exported embeds the canonical Resource type.
	var _ types.Resource = got.Resource
}

func TestExport_DefaultFormat(t *testing.T) {
	// Empty format string triggers the server default (json). Verify
	// the wrapper omits the variable on the wire.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"exportResource": map[string]any{
				"result": map[string]any{
					"id":       "res-1",
					"name":     "ci-deploy-role",
					"origin":   "IMPORTED",
					"rendered": "{}",
				},
				"successful": true,
			},
		}),
	)

	if _, err := newService(gqlClient).Export(t.Context(), "res-1", ""); err != nil {
		t.Fatalf("Export: %v", err)
	}

	reqs := gqlClient.Requests()
	if _, present := reqs[0].Variables["format"]; present {
		t.Errorf("format variable should be omitted when caller passes empty string, got %v", reqs[0].Variables["format"])
	}
}

func TestCreateGrant_WildcardRecipients(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createResourceGrant": map[string]any{
				"result": map[string]any{
					"id":                  "g-1",
					"action":              "resource:export",
					"recipientConditions": "*",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).CreateGrant(t.Context(), "res-1", resources.CreateGrantInput{
		Action:              "resource:export",
		RecipientConditions: nil, // wildcard
	})
	if err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}
	if got.Action != "resource:export" {
		t.Errorf("Action = %q, want resource:export", got.Action)
	}
	if got.RecipientConditions != nil {
		t.Errorf("RecipientConditions = %v, want nil (wildcard)", got.RecipientConditions)
	}
}

func TestCreateGrant_AttributeRecipients(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createResourceGrant": map[string]any{
				"result": map[string]any{
					"id":                  "g-2",
					"action":              "resource:export",
					"recipientConditions": `{"md-environment":["prod"]}`,
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).CreateGrant(t.Context(), "res-1", resources.CreateGrantInput{
		Action: "resource:export",
		RecipientConditions: types.PolicyConditions{
			"md-environment": []string{"prod"},
		},
	})
	if err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}
	if envs := got.RecipientConditions["md-environment"]; len(envs) != 1 || envs[0] != "prod" {
		t.Errorf("RecipientConditions[md-environment] = %v, want [prod]", envs)
	}
}

func TestDeleteGrant(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteGrant": map[string]any{
				"result":     map[string]any{"id": "g-1", "action": "resource:export"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).DeleteGrant(t.Context(), "g-1"); err != nil {
		t.Fatalf("DeleteGrant: %v", err)
	}
}
