package massdriver

import (
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/accesstokens"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/auditlogs"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/bundles"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/components"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/groups"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/ocirepos"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/policies"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/resources"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/server"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/serviceaccounts"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/urls"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/viewer"
)

// Client is the top-level SDK client. Each domain service is a field;
// access is just `c.<Service>.<Method>(ctx, ...)`. Construct with
// [NewClient].
type Client struct {
	// Config is the resolved configuration this client uses —
	// organization id, base URL, the credential, the source it
	// came from. Read-only after construction; mutating it has no
	// effect on subsequent service calls because each service has
	// already captured its own transport client.
	Config config.Config

	// AccessTokens manages personal access tokens (PATs) for the
	// authenticated identity.
	AccessTokens *accesstokens.Service
	// AuditLogs reads the organization's audit trail.
	AuditLogs *auditlogs.Service
	// Bundles reads the published bundle catalog.
	Bundles *bundles.Service
	// Components manages a project blueprint's components and links.
	Components *components.Service
	// Deployments triggers and inspects infrastructure provisioning runs.
	Deployments *deployments.Service
	// Environments manages deployment contexts within a project.
	Environments *environments.Service
	// Groups manages access-control groups, members, and invitations.
	Groups *groups.Service
	// Instances manages deployed bundle instances and their alarms,
	// secrets, and produced resources.
	Instances *instances.Service
	// OciRepos manages OCI repositories and provides oras.Target
	// handles for direct artifact pulls/pushes.
	OciRepos *ocirepos.Service
	// Organizations inspects and updates the organization record and
	// its custom-attribute schema.
	Organizations *organizations.Service
	// Policies manages ABAC policies, the action catalog, and the
	// policy evaluator/explainer.
	Policies *policies.Service
	// Projects manages top-level projects (the blueprints that own
	// environments and components).
	Projects *projects.Service
	// Resources manages provisioned and imported resources, exports,
	// and grants.
	Resources *resources.Service
	// Server reports the connected server's version, mode, and
	// available login methods.
	Server *server.Service
	// ServiceAccounts manages programmatic API client identities.
	ServiceAccounts *serviceaccounts.Service
	// URLs builds deep links into the Massdriver web app.
	URLs *urls.Service
	// Viewer reports the currently-authenticated identity.
	Viewer *viewer.Service
}

// NewClient constructs the SDK client.
//
// Without options, configuration is resolved from environment variables
// (MASSDRIVER_API_KEY, MASSDRIVER_ORGANIZATION_ID, MASSDRIVER_URL,
// MASSDRIVER_PROFILE) and the active profile in
// ~/.config/massdriver/config.yaml. Options override environment and
// file values:
//
//	c, err := massdriver.NewClient(
//	    massdriver.WithAPIKey(os.Getenv("DEPLOY_KEY")),
//	    massdriver.WithOrganizationID("ecommerce"),
//	)
//
// For tests, supply a mock GraphQL client to skip credential
// resolution entirely:
//
//	c, _ := massdriver.NewClient(
//	    massdriver.WithGQLClient(gqlMock),
//	    massdriver.WithOrganizationID("test-org"),
//	)
//
// See options.go for every available [Option].
//
// Returns an error if required credentials cannot be resolved or the
// configured URL is malformed.
func NewClient(opts ...Option) (*Client, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if o.gqlClient != nil {
		return wrap(&client.Client{
			Config: config.Config{
				OrganizationID: o.organizationID,
				URL:            o.baseURL,
			},
			GQLv2: o.gqlClient,
		}), nil
	}

	cfg, err := config.Load(config.Overrides{
		APIKey:         o.apiKey,
		OrganizationID: o.organizationID,
		URL:            o.baseURL,
		Profile:        o.profile,
	})
	if err != nil {
		return nil, err
	}
	timeout := client.DefaultTimeout
	if o.timeoutSet {
		timeout = o.timeout
	}
	return wrap(client.NewWithConfig(cfg, timeout)), nil
}

// wrap returns a [*Client] with every domain service pre-wired around
// the supplied transport client, and Config populated from it.
// Internal — used by [NewClient] and by internal tests.
func wrap(c *client.Client) *Client {
	return &Client{
		Config:          c.Config,
		AccessTokens:    accesstokens.New(c),
		AuditLogs:       auditlogs.New(c),
		Bundles:         bundles.New(c),
		Components:      components.New(c),
		Deployments:     deployments.New(c),
		Environments:    environments.New(c),
		Groups:          groups.New(c),
		Instances:       instances.New(c),
		OciRepos:        ocirepos.New(c),
		Organizations:   organizations.New(c),
		Policies:        policies.New(c),
		Projects:        projects.New(c),
		Resources:       resources.New(c),
		Server:          server.New(c),
		ServiceAccounts: serviceaccounts.New(c),
		URLs:            urls.New(c),
		Viewer:          viewer.New(c),
	}
}
