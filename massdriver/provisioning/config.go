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
	DeploymentID     string `envconfig:"DEPLOYMENT_ID" required:"true"`
	Token            string `envconfig:"TOKEN" required:"true"`
	BundleName       string `envconfig:"BUNDLE_NAME" required:"true"`
	BundleVersion    string `envconfig:"BUNDLE_VERSION" required:"true"`
	DeploymentAction string `envconfig:"DEPLOYMENT_ACTION" required:"true"`
	InstanceID       string `envconfig:"INSTANCE_ID" required:"true"`
	StepPath         string `envconfig:"STEP_PATH" required:"true"`
}

func loadDeploymentEnvs(c *Config) error {
	return envconfig.Process("MASSDRIVER", c)
}
