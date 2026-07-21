package instances_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
)

func TestSetRemoteReference(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"setRemoteReference": map[string]any{
				"result": map[string]any{
					"id":    "ref-1",
					"field": "vpc",
					"resource": map[string]any{
						"id":           "networking-prod-vpc.vpc",
						"name":         "Shared VPC",
						"resourceType": map[string]any{"id": "aws-vpc", "name": "AWS VPC"},
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).SetRemoteReference(t.Context(), "ecomm-prod-database", "networking-prod-vpc.vpc", "vpc")
	if err != nil {
		t.Fatalf("SetRemoteReference: %v", err)
	}
	if got.ID != "ref-1" {
		t.Errorf("ID = %q, want ref-1", got.ID)
	}
	if got.Field != "vpc" {
		t.Errorf("Field = %q, want vpc", got.Field)
	}
	if got.Resource.ID != "networking-prod-vpc.vpc" {
		t.Errorf("Resource.ID = %q, want networking-prod-vpc.vpc", got.Resource.ID)
	}
	if got.Resource.ResourceType == nil || got.Resource.ResourceType.ID != "aws-vpc" {
		t.Errorf("Resource.ResourceType = %+v, want id aws-vpc", got.Resource.ResourceType)
	}

	reqs := gqlClient.Requests()
	if reqs[0].OpName != "SetRemoteReference" {
		t.Errorf("OpName = %q, want SetRemoteReference", reqs[0].OpName)
	}
	if reqs[0].Variables["instanceId"] != "ecomm-prod-database" {
		t.Errorf("instanceId = %v, want ecomm-prod-database", reqs[0].Variables["instanceId"])
	}
	if reqs[0].Variables["resourceId"] != "networking-prod-vpc.vpc" {
		t.Errorf("resourceId = %v, want networking-prod-vpc.vpc", reqs[0].Variables["resourceId"])
	}
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["field"] != "vpc" {
		t.Errorf("input.field = %v, want vpc", input["field"])
	}
}

func TestSetRemoteReference_Provisioned(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"setRemoteReference": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "invalid", "field": "instanceId", "message": "instance must not be provisioned"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).SetRemoteReference(t.Context(), "ecomm-prod-database", "res-1", "vpc")
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "set remote reference" {
		t.Errorf("Op = %q, want set remote reference", mf.Op)
	}
}

func TestRemoveRemoteReference(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"removeRemoteReference": map[string]any{
				"result": map[string]any{
					"id":    "ref-1",
					"field": "vpc",
					"resource": map[string]any{
						"id":   "networking-prod-vpc.vpc",
						"name": "Shared VPC",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).RemoveRemoteReference(t.Context(), "ecomm-prod-database", "vpc")
	if err != nil {
		t.Fatalf("RemoveRemoteReference: %v", err)
	}
	if got.Field != "vpc" {
		t.Errorf("Field = %q, want vpc", got.Field)
	}

	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["field"] != "vpc" {
		t.Errorf("input.field = %v, want vpc", input["field"])
	}
}
