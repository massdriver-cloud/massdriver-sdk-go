// Package provisioning is the top-level entry point for the
// deployment-token-only REST surface used from inside a provisioner run.
//
// This is a separate API surface from the GraphQL platform SDK
// (massdriver.NewClient): different auth model (deployment tokens, not
// PATs/service accounts), different intended audience (the xo CLI, the
// Terraform provider, and similar deploy-time tooling — never
// general-purpose code). See massdriver/provisioning/README.md for the
// full audience + auth story.
//
// Typical usage from inside a provisioner container, where
// MASSDRIVER_DEPLOYMENT_ID and MASSDRIVER_TOKEN are set by the platform:
//
//	pc, err := provisioning.NewClient()
//	if err != nil {
//	    return err
//	}
//	res, err := pc.Resources.CreateResource(ctx, &resources.Resource{ ... })
package provisioning

import (
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning/deployments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning/resources"
)

// Client is the top-level provisioning SDK client. Construct with
// [NewClient]; read the resolved configuration via [Client.Config].
type Client struct {
	config Config

	Resources   *resources.Service
	Deployments *deployments.Service
}

// Config returns the resolved configuration this client uses. The
// returned value is a copy; mutating it has no effect on subsequent
// service calls.
func (c *Client) Config() Config { return c.config }

// Option configures a [*Client] built by [NewClient]. Options override
// values that would otherwise be sourced from environment variables.
type Option func(*options)

type options struct {
	baseURL    string
	timeout    time.Duration
	timeoutSet bool
}

// WithBaseURL sets the Massdriver API base URL. Useful for self-hosted
// installations or pointing the client at an httptest server in tests.
// Overrides MASSDRIVER_URL.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithTimeout sets the per-request HTTP timeout. Defaults to 30s when
// omitted. Pass 0 to disable the timeout entirely.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d; o.timeoutSet = true }
}

// NewClient constructs a provisioning client. Credentials are resolved
// from MASSDRIVER_DEPLOYMENT_ID + MASSDRIVER_TOKEN, which the platform
// injects into the provisioner container at deployment time. Returns
// an error if those credentials are missing, any of the standard
// MASSDRIVER_* deployment identifiers is unset, or the configured URL
// is malformed.
func NewClient(opts ...Option) (*Client, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	// config.Load resolves auth (deployment-token), URL with default,
	// and OrganizationID — also produces the http client we hand to
	// the services. We keep URL and OrganizationID and discard the rest;
	// the rest of provisioning.Config is loaded from env directly.
	cfg, err := config.Load(config.Overrides{URL: o.baseURL})
	if err != nil {
		return nil, err
	}

	var pcfg Config
	if err := loadDeploymentEnvs(&pcfg); err != nil {
		return nil, err
	}
	pcfg.URL = cfg.URL
	pcfg.OrganizationID = cfg.OrganizationID

	timeout := client.DefaultTimeout
	if o.timeoutSet {
		timeout = o.timeout
	}
	c := client.NewWithConfig(cfg, timeout)

	return &Client{
		config:      pcfg,
		Resources:   resources.NewService(c),
		Deployments: deployments.NewService(c),
	}, nil
}
