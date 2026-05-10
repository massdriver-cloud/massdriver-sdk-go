package server_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/server"
)

func newService(gqlClient *gqltest.Client) *server.Service {
	return server.New(&client.Client{Config: config.Config{}, GQLv2: gqlClient})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"server": map[string]any{
				"appUrl":  "https://app.massdriver.cloud",
				"version": "1.2.3",
				"mode":    "cloud",
				"ssoProviders": []map[string]any{
					{"name": "google", "loginUrl": "https://app.example.com/sso/google", "uiLabel": "Sign in with Google"},
				},
				"emailAuthMethods": []map[string]any{
					{"name": "PASSKEY"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AppURL != "https://app.massdriver.cloud" {
		t.Errorf("AppURL = %q, want https://app.massdriver.cloud", got.AppURL)
	}
	if got.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", got.Version)
	}
	if len(got.SsoProviders) != 1 || got.SsoProviders[0].Name != "google" {
		t.Errorf("SsoProviders = %+v, want one google entry", got.SsoProviders)
	}
	if len(got.EmailAuthMethods) != 1 || got.EmailAuthMethods[0].Name != "PASSKEY" {
		t.Errorf("EmailAuthMethods = %+v, want one PASSKEY entry", got.EmailAuthMethods)
	}

	// Server query takes no organizationId — verify the request shape is
	// minimal.
	reqs := gqlClient.Requests()
	if reqs[0].OpName != "GetServer" {
		t.Errorf("OpName = %q, want GetServer", reqs[0].OpName)
	}
	if len(reqs[0].Variables) != 0 {
		t.Errorf("Variables = %v, want empty", reqs[0].Variables)
	}
}
