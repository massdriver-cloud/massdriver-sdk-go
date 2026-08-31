package provisioning

import "github.com/kelseyhightower/envconfig"

// Config is the resolved configuration a [*Client] uses — auth, target
// URL, and the deployment identifiers the platform injects into every
// provisioner container. Accessible via [Client.Config]; loaded once in
// [NewClient].
//
// URL and OrganizationID are populated from [config.Load]'s resolution
// (env vars, profile file, options). The remaining fields are read
// directly from the standard MASSDRIVER_* environment variables and are
// required — [NewClient] fails if any are unset.
type Config struct {
	URL              string
	OrganizationID   string
	DeploymentID     string `envconfig:"MASSDRIVER_DEPLOYMENT_ID" required:"true"`
	Token            string `envconfig:"MASSDRIVER_TOKEN" required:"true"`
	BundleName       string `envconfig:"MASSDRIVER_BUNDLE_NAME" required:"true"`
	BundleVersion    string `envconfig:"MASSDRIVER_BUNDLE_VERSION" required:"true"`
	DeploymentAction string `envconfig:"MASSDRIVER_DEPLOYMENT_ACTION" required:"true"`
	InstanceID       string `envconfig:"MASSDRIVER_INSTANCE_ID" required:"true"`
	StepPath         string `envconfig:"MASSDRIVER_STEP_PATH"`
}

func loadDeploymentEnvs(c *Config) error {
	return envconfig.Process("", c)
}
