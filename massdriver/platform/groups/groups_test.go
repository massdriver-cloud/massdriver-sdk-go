package groups_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/groups"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *groups.Service {
	return groups.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"group": map[string]any{
				"id":          "g-1",
				"name":        "Platform",
				"description": "Platform engineering",
				"role":        "CUSTOM",
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "g-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Role != string(groups.RoleCustom) {
		t.Errorf("Role = %q, want CUSTOM", got.Role)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing group.
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"group": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"groups": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "g-1", "name": "Admins", "role": "ORGANIZATION_ADMIN"},
					{"id": "g-2", "name": "Viewers", "role": "ORGANIZATION_VIEWER"},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), groups.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createGroup": map[string]any{
				"result":     map[string]any{"id": "g-new", "name": "Backend", "role": "CUSTOM"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), groups.CreateInput{
		Name:        "Backend",
		Description: "Backend team",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "g-new" {
		t.Errorf("ID = %q, want g-new", got.ID)
	}
}

func TestAddUser_ExistingUser(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"addAccountToGroup": map[string]any{
				"result": map[string]any{
					"__typename": "Account",
					"id":         "u-alice",
					"email":      "alice@example.com",
					"firstName":  "Alice",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).AddUser(t.Context(), "g-1", "alice@example.com")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if got.User == nil {
		t.Fatal("User branch should be populated for existing user")
	}
	if got.Invitation != nil {
		t.Errorf("Invitation = %+v, want nil for existing user", got.Invitation)
	}
	if got.User.Email != "alice@example.com" {
		t.Errorf("User.Email = %q, want alice@example.com", got.User.Email)
	}
}

func TestAddUser_NewUser(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"addAccountToGroup": map[string]any{
				"result": map[string]any{
					"__typename": "GroupInvitation",
					"id":         "inv-1",
					"email":      "newuser@example.com",
					"createdAt":  "2026-05-08T10:00:00Z",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).AddUser(t.Context(), "g-1", "newuser@example.com")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if got.Invitation == nil {
		t.Fatal("Invitation branch should be populated for new user")
	}
	if got.User != nil {
		t.Errorf("User = %+v, want nil for new user", got.User)
	}
	if got.Invitation.Email != "newuser@example.com" {
		t.Errorf("Invitation.Email = %q, want newuser@example.com", got.Invitation.Email)
	}
}

func TestRemoveUser(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteGroupMember": map[string]any{
				"result":     map[string]any{"email": "alice@example.com"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).RemoveUser(t.Context(), "g-1", "alice@example.com"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
}

func TestAddServiceAccount(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"addServiceAccountToGroup": map[string]any{
				"result":     map[string]any{"id": "sa-1", "name": "ci-bot"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).AddServiceAccount(t.Context(), "g-1", "sa-1"); err != nil {
		t.Fatalf("AddServiceAccount: %v", err)
	}

	reqs := gqlClient.Requests()
	if reqs[0].Variables["serviceAccountId"] != "sa-1" {
		t.Errorf("serviceAccountId = %v, want sa-1", reqs[0].Variables["serviceAccountId"])
	}
	if reqs[0].Variables["groupId"] != "g-1" {
		t.Errorf("groupId = %v, want g-1", reqs[0].Variables["groupId"])
	}
}

func TestRemoveServiceAccount(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"removeServiceAccountFromGroup": map[string]any{
				"result":     map[string]any{"id": "sa-1", "name": "ci-bot"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).RemoveServiceAccount(t.Context(), "g-1", "sa-1"); err != nil {
		t.Fatalf("RemoveServiceAccount: %v", err)
	}
}
