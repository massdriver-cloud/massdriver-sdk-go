// Package urls builds Massdriver web-app URLs for surfaces in tools and
// CLIs — links to projects, environments, instances, bundles, and the
// like, suitable for printing in terminal output, embedding in error
// messages, or opening from a `massdriver` CLI subcommand.
//
// All URLs are constructed from the configured server's app URL (resolved
// via platform/server on first use). The helper does no I/O after
// construction.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.URLs] field on the top-level SDK
// client. Then call [Service.Helper] to obtain a [*Helper] for building
// URLs, or use [NewWithBaseURL] when the app URL is already known.
package urls

import (
	"context"
	"fmt"
	"strings"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/server"
)

// Service is the receiver for URL-builder operations. Construct with
// [New]; for the typical case you'll use the [massdriver.Client.URLs]
// field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Helper builds Massdriver app URLs scoped to a single organization.
type Helper struct {
	// BaseURL is the Massdriver web-app base (e.g. "https://app.massdriver.cloud").
	BaseURL string

	// OrgID is the organization id used in URL paths.
	OrgID string
}

// Helper returns a [*Helper] for the configured organization. It tries to
// resolve the app URL via platform/server.Service.Get; if that fails
// (e.g. the caller hasn't authenticated yet, or the server is
// unreachable), it falls back to inferring the app host by replacing
// `api.` with `app.` in the configured API URL.
//
// The fallback covers most Massdriver Cloud installs and self-hosted
// deployments that follow the same naming convention. For self-hosted
// installs that don't, callers should pass an explicit BaseURL via
// [NewWithBaseURL].
func (s *Service) Helper(ctx context.Context) *Helper {
	appURL := strings.Replace(s.client.Config.URL, "api.", "app.", 1)
	if srv, err := server.New(s.client).Get(ctx); err == nil && srv.AppURL != "" {
		appURL = srv.AppURL
	}
	return &Helper{
		BaseURL: strings.TrimRight(appURL, "/"),
		OrgID:   s.client.Config.OrganizationID,
	}
}

// NewWithBaseURL builds a [Helper] from an explicit app URL. Use this in
// tests or when the caller has already determined the app URL by some
// other means.
func NewWithBaseURL(baseURL, orgID string) *Helper {
	return &Helper{
		BaseURL: strings.TrimRight(baseURL, "/"),
		OrgID:   orgID,
	}
}

// OrganizationURL returns the URL for the configured organization's home
// page.
func (h *Helper) OrganizationURL() string {
	return fmt.Sprintf("%s/orgs/%s/", h.BaseURL, h.OrgID)
}

// ProjectsURL returns the URL for the projects list.
func (h *Helper) ProjectsURL() string {
	return fmt.Sprintf("%s/orgs/%s/projects", h.BaseURL, h.OrgID)
}

// ProjectURL returns the URL for a specific project.
func (h *Helper) ProjectURL(projectID string) string {
	return fmt.Sprintf("%s/orgs/%s/projects/%s/", h.BaseURL, h.OrgID, projectID)
}

// EnvironmentURL returns the URL for a specific environment. The
// environment's id is in `<projectID>-<envName>` form (e.g. "ecomm-prod");
// the URL nests the environment under its project.
func (h *Helper) EnvironmentURL(environmentID string) string {
	parts := strings.SplitN(environmentID, "-", 2)
	if len(parts) != 2 {
		return ""
	}
	return fmt.Sprintf("%s/orgs/%s/projects/%s/environments/%s", h.BaseURL, h.OrgID, parts[0], parts[1])
}

// InstanceURL returns the URL for a specific instance. The id is in
// `<projectID>-<envName>-<componentName>` form; the URL nests under the
// owning environment with the instance selected as the active package.
func (h *Helper) InstanceURL(instanceID string) string {
	parts := strings.SplitN(instanceID, "-", 3)
	if len(parts) != 3 {
		return ""
	}
	return fmt.Sprintf("%s/orgs/%s/projects/%s/environments/%s?package=%s",
		h.BaseURL, h.OrgID, parts[0], parts[1], parts[2])
}

// BundleURL returns the URL for a specific bundle version in the catalog.
func (h *Helper) BundleURL(bundleName, version string) string {
	return fmt.Sprintf("%s/orgs/%s/repos/%s/%s", h.BaseURL, h.OrgID, bundleName, version)
}

// RepoInstancesURL returns the URL listing instances of a specific bundle
// version.
func (h *Helper) RepoInstancesURL(bundleName, version string) string {
	return fmt.Sprintf("%s/orgs/%s/repos/%s/%s/instances", h.BaseURL, h.OrgID, bundleName, version)
}
