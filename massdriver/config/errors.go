package config

import "errors"

// Sentinel errors returned by [Load] (and therefore by
// massdriver.NewClient / provisioning.NewClient). Classify with
// [errors.Is] rather than matching message text.
var (
	// ErrNoCredentials indicates no API key or personal access token
	// was found in any configuration layer.
	ErrNoCredentials = errors.New("no credentials found")

	// ErrDeploymentCredentialsMissing indicates deployment-token auth
	// was requested but MASSDRIVER_DEPLOYMENT_ID and MASSDRIVER_TOKEN
	// are not both set.
	ErrDeploymentCredentialsMissing = errors.New("deployment token authentication requires the MASSDRIVER_DEPLOYMENT_ID and MASSDRIVER_TOKEN environment variables")

	// ErrOrganizationIDRequired indicates no organization ID was found
	// in any configuration layer.
	ErrOrganizationIDRequired = errors.New("organization ID is required")
)
