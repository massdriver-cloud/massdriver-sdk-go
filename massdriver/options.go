package massdriver

import (
	"time"

	"github.com/Khan/genqlient/graphql"
)

// Option configures a [*Client] built by [NewClient]. Options
// override values that would otherwise be sourced from environment
// variables and the active profile in ~/.config/massdriver/config.yaml.
//
// Resolution order (highest precedence first):
//
//  1. Options passed to [NewClient].
//  2. Environment variables (MASSDRIVER_API_KEY, MASSDRIVER_ORGANIZATION_ID,
//     MASSDRIVER_URL, MASSDRIVER_PROFILE, etc.).
//  3. The active profile in ~/.config/massdriver/config.yaml.
type Option func(*options)

// options is the resolved bag of caller-supplied configuration. The
// timeoutSet flag distinguishes "caller passed [WithTimeout](0)" (an
// explicit no-timeout request) from "caller passed nothing" (use the
// default).
type options struct {
	apiKey         string
	organizationID string
	baseURL        string
	profile        string
	gqlClient      graphql.Client

	timeout    time.Duration
	timeoutSet bool
}

// WithAPIKey sets the API credential. Overrides MASSDRIVER_API_KEY and
// any value in the active config-file profile. Strings prefixed with
// "mds_" or "md_" are treated as personal access tokens (Bearer
// auth); all other values are treated as legacy API keys (Basic auth).
func WithAPIKey(key string) Option {
	return func(o *options) { o.apiKey = key }
}

// WithOrganizationID sets the organization id this client operates
// against. Overrides MASSDRIVER_ORGANIZATION_ID and any value in the
// active config-file profile.
func WithOrganizationID(id string) Option {
	return func(o *options) { o.organizationID = id }
}

// WithBaseURL sets the Massdriver API base URL. Useful for self-hosted
// installations or pointing the client at an httptest server in tests.
// Overrides MASSDRIVER_URL and any value in the active config-file
// profile.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithProfile selects which profile to read from
// ~/.config/massdriver/config.yaml. Overrides MASSDRIVER_PROFILE and
// the default ("default").
func WithProfile(name string) Option {
	return func(o *options) { o.profile = name }
}

// WithGQLClient supplies a pre-built GraphQL client and bypasses
// credential resolution entirely — for tests that exercise SDK
// methods against a mocked transport. Pair with the
// [github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest]
// package to construct a Client backed by scripted responses.
//
// When set, only [WithOrganizationID] and [WithBaseURL] retain
// meaning; credential and HTTP-client construction are skipped.
func WithGQLClient(c graphql.Client) Option {
	return func(o *options) { o.gqlClient = c }
}

// WithTimeout sets the per-request HTTP timeout. Defaults to 30s when
// omitted. Pass 0 to disable the timeout entirely — appropriate only
// when callers enforce their own deadlines via [context.Context].
//
// The timeout applies to standard read/write requests; streaming
// operations ([deployments.Service.StreamLogs] and friends) ignore it
// because they hold long-lived websocket connections gated solely by
// the caller's context.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d; o.timeoutSet = true }
}
