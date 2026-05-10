package urls_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/urls"
)

func TestNewWithBaseURL(t *testing.T) {
	h := urls.NewWithBaseURL("https://app.massdriver.cloud", "ecomm-corp")
	if h.BaseURL != "https://app.massdriver.cloud" {
		t.Errorf("BaseURL = %q, want https://app.massdriver.cloud", h.BaseURL)
	}
	if h.OrgID != "ecomm-corp" {
		t.Errorf("OrgID = %q, want ecomm-corp", h.OrgID)
	}
}

// TestNewWithBaseURL_TrimsTrailingSlash ensures URL composition doesn't
// produce double slashes.
func TestNewWithBaseURL_TrimsTrailingSlash(t *testing.T) {
	h := urls.NewWithBaseURL("https://app.massdriver.cloud/", "ecomm-corp")
	if got, want := h.OrganizationURL(), "https://app.massdriver.cloud/orgs/ecomm-corp/"; got != want {
		t.Errorf("OrganizationURL() = %q, want %q", got, want)
	}
}

func TestNew_UsesServerAppURL(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"server": map[string]any{
				"appUrl":  "https://app.custom.example.com",
				"version": "1.2.3",
				"mode":    "self_hosted",
			},
		}),
	)
	c := &client.Client{
		Config: config.Config{
			OrganizationID: "ecomm-corp",
			URL:            "https://api.custom.example.com",
		},
		GQLv2: gqlClient,
	}

	h := urls.New(c).Helper(t.Context())
	if h.BaseURL != "https://app.custom.example.com" {
		t.Errorf("BaseURL = %q, want server.AppURL https://app.custom.example.com", h.BaseURL)
	}
}

// TestNew_FallsBackToInferredHost covers the bootstrap case where the
// server query is unavailable (no auth, network, etc.) — the Helper must
// still construct, falling back to api. → app. substitution.
func TestNew_FallsBackToInferredHost(t *testing.T) {
	// gqltest with no responses queued: server.Get will return an error,
	// New should fall back.
	c := &client.Client{
		Config: config.Config{
			OrganizationID: "ecomm-corp",
			URL:            "https://api.massdriver.cloud",
		},
		GQLv2: gqltest.NewClient(),
	}
	h := urls.New(c).Helper(t.Context())
	if h.BaseURL != "https://app.massdriver.cloud" {
		t.Errorf("BaseURL = %q, want fallback https://app.massdriver.cloud", h.BaseURL)
	}
}

func TestURLBuilders(t *testing.T) {
	h := urls.NewWithBaseURL("https://app.massdriver.cloud", "ecomm-corp")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "OrganizationURL",
			got:  h.OrganizationURL(),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/",
		},
		{
			name: "ProjectsURL",
			got:  h.ProjectsURL(),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/projects",
		},
		{
			name: "ProjectURL",
			got:  h.ProjectURL("ecomm"),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/projects/ecomm/",
		},
		{
			name: "EnvironmentURL",
			got:  h.EnvironmentURL("ecomm-prod"),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/projects/ecomm/environments/prod",
		},
		{
			name: "InstanceURL",
			got:  h.InstanceURL("ecomm-prod-database"),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/projects/ecomm/environments/prod?package=database",
		},
		{
			name: "BundleURL",
			got:  h.BundleURL("aws-aurora-postgres", "1.2.3"),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/repos/aws-aurora-postgres/1.2.3",
		},
		{
			name: "RepoInstancesURL",
			got:  h.RepoInstancesURL("aws-aurora-postgres", "1.2.3"),
			want: "https://app.massdriver.cloud/orgs/ecomm-corp/repos/aws-aurora-postgres/1.2.3/instances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestEnvironmentURL_MalformedID and TestInstanceURL_MalformedID return
// empty strings when the ID doesn't match the expected segment count, so
// callers can detect "couldn't build a URL" without needing a separate
// error return.
func TestEnvironmentURL_MalformedID(t *testing.T) {
	h := urls.NewWithBaseURL("https://app.massdriver.cloud", "ecomm-corp")
	if got := h.EnvironmentURL("just-an-id"); got == "https://app.massdriver.cloud/orgs/ecomm-corp/projects/just/environments/an-id" {
		// SplitN behavior — verify we get something reasonable.
		t.Logf("got %q (acceptable splitN result)", got)
	}
	if got := h.EnvironmentURL("singleword"); got != "" {
		t.Errorf("EnvironmentURL(singleword) = %q, want empty for malformed id", got)
	}
}

func TestInstanceURL_MalformedID(t *testing.T) {
	h := urls.NewWithBaseURL("https://app.massdriver.cloud", "ecomm-corp")
	if got := h.InstanceURL("just-twoparts"); got != "" {
		t.Errorf("InstanceURL(just-twoparts) = %q, want empty for malformed id", got)
	}
}
