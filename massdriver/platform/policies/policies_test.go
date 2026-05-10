package policies_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/policies"
)

func newService(gqlClient *gqltest.Client) *policies.Service {
	return policies.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestListActions(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"policyActions": []map[string]any{
				{
					"id":          "project:view",
					"verb":        "view",
					"description": "View a project's contents.",
					"entity": map[string]any{
						"id":          "project",
						"description": "A Massdriver project.",
					},
				},
				{
					"id":          "instance:deploy",
					"verb":        "deploy",
					"description": "Deploy infrastructure for an instance.",
					"entity": map[string]any{
						"id":          "instance",
						"description": "A deployed bundle within an environment.",
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).ListActions(t.Context())
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d actions, want 2", len(got))
	}
	if got[0].ID != "project:view" {
		t.Errorf("got[0].ID = %q, want project:view", got[0].ID)
	}
	if got[0].Entity == nil || got[0].Entity.ID != "project" {
		t.Errorf("got[0].Entity = %+v, want id=project", got[0].Entity)
	}
}

func TestListEntities(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"policyEntities": []map[string]any{
				{"id": "project", "description": "A Massdriver project."},
				{"id": "environment", "description": "A deployment context within a project."},
			},
		}),
	)

	got, err := newService(gqlClient).ListEntities(t.Context())
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(got) != 2 || got[0].ID != "project" {
		t.Errorf("got = %+v, want first entity id=project", got)
	}
}

func TestEvaluate_Allowed(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"evaluatePolicy": map[string]any{
				"allowed":  true,
				"action":   "project:view",
				"entityId": "ecomm",
			},
		}),
	)

	got, err := newService(gqlClient).Evaluate(t.Context(), "project:view", "ecomm")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !got.Allowed {
		t.Errorf("Allowed = false, want true")
	}
	if got.Action != "project:view" || got.EntityID != "ecomm" {
		t.Errorf("Action/EntityID = %q/%q, want project:view/ecomm", got.Action, got.EntityID)
	}
}

func TestEvaluate_Denied(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"evaluatePolicy": map[string]any{
				"allowed":  false,
				"action":   "project:delete",
				"entityId": "ecomm",
			},
		}),
	)

	got, err := newService(gqlClient).Evaluate(t.Context(), "project:delete", "ecomm")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Allowed {
		t.Errorf("Allowed = true, want false (no policy grants project:delete)")
	}
}

func TestEvaluateBatch(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"evaluatePolicies": []map[string]any{
				{"allowed": true, "action": "project:view", "entityId": "ecomm"},
				{"allowed": false, "action": "project:delete", "entityId": "ecomm"},
				{"allowed": true, "action": "instance:deploy", "entityId": "ecomm-prod-database"},
			},
		}),
	)

	got, err := newService(gqlClient).EvaluateBatch(t.Context(), []policies.Check{
		{Action: "project:view", EntityID: "ecomm"},
		{Action: "project:delete", EntityID: "ecomm"},
		{Action: "instance:deploy", EntityID: "ecomm-prod-database"},
	})
	if err != nil {
		t.Fatalf("EvaluateBatch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d decisions, want 3", len(got))
	}
	if got[0].Allowed != true || got[1].Allowed != false || got[2].Allowed != true {
		t.Errorf("Decisions Allowed = [%v, %v, %v], want [true, false, true]",
			got[0].Allowed, got[1].Allowed, got[2].Allowed)
	}

	// Each decision echoes back the action + entityId from the request,
	// so callers can correlate without tracking positional indices.
	if got[1].Action != "project:delete" || got[1].EntityID != "ecomm" {
		t.Errorf("got[1] = %+v, want action=project:delete entityId=ecomm", got[1])
	}
}

func TestExplain(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"explainPolicy": []string{
				"Can create environments with identifiers [dev, staging, prod].",
				"Can create and update any project.",
			},
		}),
	)

	got, err := newService(gqlClient).Explain(t.Context(), policies.ExplainInput{
		Effect: policies.EffectAllow,
		Actions: []string{
			"project:create",
			"project:update",
			"environment:create",
		},
		Conditions: policies.PolicyConditions{
			"md-environment": []string{"dev", "staging", "prod"},
		},
	})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d sentences, want 2", len(got))
	}
	if got[0] != "Can create environments with identifiers [dev, staging, prod]." {
		t.Errorf("got[0] = %q, want first explanation sentence", got[0])
	}
}

func TestExplain_WildcardConditions(t *testing.T) {
	// Whole-policy wildcard (Conditions: nil) is the most common
	// authoring shape; ensure it round-trips through the explainer.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"explainPolicy": []string{"Can view any project."},
		}),
	)

	_, err := newService(gqlClient).Explain(t.Context(), policies.ExplainInput{
		Effect:     policies.EffectAllow,
		Actions:    []string{"project:view"},
		Conditions: nil, // whole-policy wildcard
	})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	// Verify the wire shape: conditions should be the JSON-string "*".
	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["conditions"] != "*" {
		t.Errorf("input.conditions = %v, want \"*\"", input["conditions"])
	}
}

func TestCustomAttributeSchema(t *testing.T) {
	// The server returns an arbitrary JSON Schema document; the wrapper
	// just hands the raw bytes back to the caller.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"md-environment": map[string]any{
				"type": "string",
				"enum": []string{"dev", "staging", "prod"},
			},
		},
	}
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"customAttributeSchema": schema,
		}),
	)

	got, err := newService(gqlClient).CustomAttributeSchema(t.Context(), "project:create")
	if err != nil {
		t.Fatalf("CustomAttributeSchema: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("got empty schema bytes")
	}
	// Sanity-check that the bytes are valid JSON containing the
	// expected attribute key.
	if !contains(got, "md-environment") {
		t.Errorf("schema bytes don't contain expected key: %s", string(got))
	}
}

func TestCustomAttributeValues(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"customAttributeValues": []string{"dev", "staging", "prod"},
		}),
	)

	got, err := newService(gqlClient).CustomAttributeValues(t.Context(), organizations.AttributeScopeEnvironment, "md-environment")
	if err != nil {
		t.Fatalf("CustomAttributeValues: %v", err)
	}
	if len(got) != 3 || got[0] != "dev" {
		t.Errorf("got = %v, want [dev staging prod]", got)
	}

	// Verify the scope and key reach the wire.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["scope"] != "ENVIRONMENT" {
		t.Errorf("scope variable = %v, want ENVIRONMENT", reqs[0].Variables["scope"])
	}
	if reqs[0].Variables["key"] != "md-environment" {
		t.Errorf("key variable = %v, want md-environment", reqs[0].Variables["key"])
	}
}

// contains is a tiny helper to avoid pulling in strings.Contains for a
// one-off byte-slice check.
func contains(b []byte, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == sub {
			return true
		}
	}
	return false
}

// --- Policy CRUD ---

func TestCreate_Wildcard(t *testing.T) {
	// Nil Conditions on input = whole-policy wildcard. Server returns the
	// JSON-string `"*"`, which the wrapper decodes back to a nil map.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createGroupPolicy": map[string]any{
				"result": map[string]any{
					"id":         "p-1",
					"effect":     "ALLOW",
					"actions":    []string{"project:view", "project:update"},
					"conditions": "*",
					"group":      map[string]any{"id": "g-1", "name": "Platform"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "g-1", policies.CreatePolicyInput{
		Effect:     policies.EffectAllow,
		Actions:    []string{"project:view", "project:update"},
		Conditions: nil,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Effect != string(policies.EffectAllow) {
		t.Errorf("Effect = %q, want ALLOW", got.Effect)
	}
	if len(got.Actions) != 2 {
		t.Errorf("Actions len = %d, want 2", len(got.Actions))
	}
	if got.Conditions != nil {
		t.Errorf("Conditions = %v, want nil (wildcard)", got.Conditions)
	}
}

func TestCreate_AttributeConditions(t *testing.T) {
	// Non-nil Conditions on input = attribute conditions. The wrapper
	// JSON-encodes the map for the wire and decodes the response back
	// into a typed map.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createGroupPolicy": map[string]any{
				"result": map[string]any{
					"id":         "p-2",
					"effect":     "ALLOW",
					"actions":    []string{"project:create"},
					"conditions": map[string]any{"md-environment": []string{"dev", "staging"}},
					"group":      map[string]any{"id": "g-1", "name": "Platform"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "g-1", policies.CreatePolicyInput{
		Effect:  policies.EffectAllow,
		Actions: []string{"project:create"},
		Conditions: policies.PolicyConditions{
			"md-environment": []string{"dev", "staging"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Conditions == nil {
		t.Fatal("Conditions = nil, want populated map")
	}
	if envs := got.Conditions["md-environment"]; len(envs) != 2 || envs[0] != "dev" || envs[1] != "staging" {
		t.Errorf("Conditions[md-environment] = %v, want [dev staging]", envs)
	}
}

// TestCreate_PerKeyWildcard exercises the mixed shape: one key uses the
// per-key wildcard ([policies.Wildcard], a nil slice), another key uses
// a closed set. Wire form has `"*"` for the wildcard key and an array
// for the closed-set key; round-trip preserves both.
func TestCreate_PerKeyWildcard(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createGroupPolicy": map[string]any{
				"result": map[string]any{
					"id":      "p-3",
					"effect":  "ALLOW",
					"actions": []string{"project:view"},
					"conditions": map[string]any{
						"md-project": "*",
						"md-team":    []string{"platform"},
					},
					"group": map[string]any{"id": "g-1", "name": "Platform"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "g-1", policies.CreatePolicyInput{
		Effect:  policies.EffectAllow,
		Actions: []string{"project:view"},
		Conditions: policies.PolicyConditions{
			"md-project": policies.Wildcard,
			"md-team":    []string{"platform"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if v, ok := got.Conditions["md-project"]; !ok {
		t.Errorf("Conditions missing md-project key: %+v", got.Conditions)
	} else if len(v) != 0 {
		t.Errorf("Conditions[md-project] = %v, want nil/empty (wildcard)", v)
	}
	if team := got.Conditions["md-team"]; len(team) != 1 || team[0] != "platform" {
		t.Errorf("Conditions[md-team] = %v, want [platform]", team)
	}

	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	conds, _ := input["conditions"].(map[string]any)
	if conds["md-project"] != "*" {
		t.Errorf("wire conditions[md-project] = %v, want \"*\"", conds["md-project"])
	}
	if team, _ := conds["md-team"].([]any); len(team) != 1 || team[0] != "platform" {
		t.Errorf("wire conditions[md-team] = %v, want [platform]", conds["md-team"])
	}
}

// TestUpdate_LeaveConditionsAlone confirms the three-way Update
// semantics: nil pointer = leave conditions unchanged, [WildcardConditions]
// = set to wildcard, &PolicyConditions{...} = set to attribute conditions.
func TestUpdate_LeaveConditionsAlone(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updatePolicy": map[string]any{
				"result": map[string]any{
					"id":         "p-1",
					"effect":     "DENY",
					"actions":    []string{"project:delete"},
					"conditions": "*",
				},
				"successful": true,
			},
		}),
	)

	_, err := newService(gqlClient).Update(t.Context(), "p-1", policies.UpdatePolicyInput{
		Effect: policies.EffectDeny,
		// Conditions intentionally unset — leave alone.
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if _, present := input["conditions"]; present {
		t.Errorf("conditions should be omitted when input.Conditions is nil, got %v", input["conditions"])
	}
}

func TestUpdate_SetWildcard(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updatePolicy": map[string]any{
				"result": map[string]any{
					"id":         "p-1",
					"effect":     "ALLOW",
					"actions":    []string{"project:view"},
					"conditions": "*",
				},
				"successful": true,
			},
		}),
	)

	_, err := newService(gqlClient).Update(t.Context(), "p-1", policies.UpdatePolicyInput{
		Conditions: policies.WildcardConditions(),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["conditions"] != "*" {
		t.Errorf("conditions = %v, want *", input["conditions"])
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deletePolicy": map[string]any{
				"result":     map[string]any{"id": "p-1"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).Delete(t.Context(), "p-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
